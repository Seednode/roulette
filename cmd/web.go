/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yosssi/gohtml"
)

const (
	logDate            string = `2006-01-02T15:04:05.000-07:00`
	prefix             string = `/src`
	redirectStatusCode int    = http.StatusSeeOther
)

type Regexes struct {
	alphanumeric *regexp.Regexp
	filename     *regexp.Regexp
	units        *regexp.Regexp
}

type Filters struct {
	includes []string
	excludes []string
}

func (f *Filters) IsEmpty() bool {
	return !(f.HasIncludes() || f.HasExcludes())
}

func (f *Filters) HasIncludes() bool {
	return len(f.includes) != 0
}

func (f *Filters) Includes() string {
	return strings.Join(f.includes, ",")
}

func (f *Filters) HasExcludes() bool {
	return len(f.excludes) != 0
}

func (f *Filters) Excludes() string {
	return strings.Join(f.excludes, ",")
}

type Index struct {
	mutex sync.RWMutex
	list  []string
}

func (i *Index) Index() []string {
	i.mutex.RLock()
	val := i.list
	i.mutex.RUnlock()

	return val
}

func (i *Index) setIndex(val []string) {
	i.mutex.Lock()
	i.list = val
	i.mutex.Unlock()
}

func (i *Index) generateCache(args []string) {
	i.mutex.Lock()
	i.list = []string{}
	i.mutex.Unlock()

	fileList(args, &Filters{}, "", i)
}

func (i *Index) IsEmpty() bool {
	i.mutex.RLock()
	length := len(i.list)
	i.mutex.RUnlock()

	return length == 0
}

type ServeStats struct {
	mutex sync.RWMutex
	list  []string
	count map[string]uint64
	size  map[string]string
	times map[string][]string
}

func (s *ServeStats) incrementCounter(image string, timestamp time.Time, filesize string) {
	s.mutex.Lock()

	s.count[image]++

	s.times[image] = append(s.times[image], timestamp.Format(logDate))

	_, exists := s.size[image]
	if !exists {
		s.size[image] = filesize
	}

	if !contains(s.list, image) {
		s.list = append(s.list, image)
	}

	s.mutex.Unlock()
}

func (s *ServeStats) ListImages() ([]byte, error) {
	s.mutex.RLock()

	sortedList := s.list

	sort.SliceStable(sortedList, func(p, q int) bool {
		return sortedList[p] < sortedList[q]
	})

	a := []timesServed{}

	for _, image := range s.list {
		a = append(a, timesServed{image, s.count[image], s.size[image], s.times[image]})
	}

	s.mutex.RUnlock()

	r, err := json.MarshalIndent(a, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

type timesServed struct {
	file   string
	served uint64
	size   string
	times  []string
}

func notFound(w http.ResponseWriter, r *http.Request, filePath string) error {
	startTime := time.Now()

	if Verbose {
		fmt.Printf("%s | Unavailable file %s requested by %s\n",
			startTime.Format(logDate),
			filePath,
			r.RemoteAddr,
		)
	}

	w.WriteHeader(404)
	w.Header().Add("Content-Type", "text/html")

	var htmlBody strings.Builder
	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(`<style>a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}</style>`)
	htmlBody.WriteString(`<title>Not Found</title></head>`)
	htmlBody.WriteString(`<body><a href="/">404 page not found</a></body></html>`)

	_, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
	if err != nil {
		return err
	}

	return nil
}

func refreshInterval(r *http.Request, regexes *Regexes) (int64, string) {
	refreshInterval := r.URL.Query().Get("refresh")

	if !regexes.units.MatchString(refreshInterval) {
		return 0, "0ms"
	}

	duration, err := time.ParseDuration(refreshInterval)
	if err != nil {
		return 0, "0ms"
	}

	return duration.Milliseconds(), refreshInterval
}

func sortOrder(r *http.Request) string {
	sortOrder := r.URL.Query().Get("sort")
	if sortOrder == "asc" || sortOrder == "desc" {
		return sortOrder
	}

	return ""
}

func splitQueryParams(query string, regexes *Regexes) []string {
	results := []string{}

	if query == "" {
		return results
	}

	params := strings.Split(query, ",")

	for i := 0; i < len(params); i++ {
		if regexes.alphanumeric.MatchString(params[i]) {
			results = append(results, strings.ToLower(params[i]))
		}
	}

	return results
}

func generateQueryParams(filters *Filters, sortOrder, refreshInterval string) string {
	var hasParams bool

	var queryParams strings.Builder

	queryParams.WriteString("?")

	if Filter {
		queryParams.WriteString("include=")
		if filters.HasIncludes() {
			queryParams.WriteString(filters.Includes())
		}

		queryParams.WriteString("&exclude=")
		if filters.HasExcludes() {
			queryParams.WriteString(filters.Excludes())
		}

		hasParams = true
	}

	if Sort {
		if hasParams {
			queryParams.WriteString("&")
		}

		queryParams.WriteString(fmt.Sprintf("sort=%s", sortOrder))

		hasParams = true
	}

	if hasParams {
		queryParams.WriteString("&")
	}
	queryParams.WriteString(fmt.Sprintf("refresh=%s", refreshInterval))

	return queryParams.String()
}

func stripQueryParams(u string) (string, error) {
	uri, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	uri.RawQuery = ""

	escapedUri, err := url.QueryUnescape(uri.String())
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		return strings.TrimPrefix(escapedUri, "/"), nil
	}

	return escapedUri, nil
}

func generateFilePath(filePath string) string {
	var htmlBody strings.Builder

	htmlBody.WriteString(prefix)
	if runtime.GOOS == "windows" {
		htmlBody.WriteString(`/`)
	}
	htmlBody.WriteString(filePath)

	return htmlBody.String()
}

func refererToUri(referer string) string {
	parts := strings.SplitAfterN(referer, "/", 4)

	if len(parts) < 4 {
		return ""
	}

	return "/" + parts[3]
}

func realIP(r *http.Request) string {
	remoteAddr := strings.SplitAfter(r.RemoteAddr, ":")

	if len(remoteAddr) < 1 {
		return r.RemoteAddr
	}

	remotePort := remoteAddr[len(remoteAddr)-1]

	cfIP := r.Header.Get("Cf-Connecting-Ip")
	xRealIP := r.Header.Get("X-Real-Ip")

	switch {
	case cfIP != "":
		return cfIP + ":" + remotePort
	case xRealIP != "":
		return xRealIP + ":" + remotePort
	default:
		return r.RemoteAddr
	}
}

func html(w http.ResponseWriter, r *http.Request, filePath string, dimensions *Dimensions, filters *Filters, regexes *Regexes) error {
	fileName := filepath.Base(filePath)

	w.Header().Add("Content-Type", "text/html")

	sortOrder := sortOrder(r)

	refreshTimer, refreshInterval := refreshInterval(r, regexes)

	queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

	var htmlBody strings.Builder
	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(`<style>html,body{margin:0;padding:0;height:100%;}`)
	htmlBody.WriteString(`a{display:block;height:100%;width:100%;text-decoration:none;}`)
	htmlBody.WriteString(`img{margin:auto;display:block;max-width:97%;max-height:97%;object-fit:scale-down;`)
	htmlBody.WriteString(`position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}</style>`)
	htmlBody.WriteString(fmt.Sprintf(`<title>%s (%dx%d)</title>`,
		fileName,
		dimensions.width,
		dimensions.height))
	htmlBody.WriteString(`</head><body>`)
	if refreshInterval != "0ms" {
		htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%s';}, %d);};</script>",
			queryParams,
			refreshTimer))
	}
	htmlBody.WriteString(fmt.Sprintf(`<a href="/%s"><img src="%s" width="%d" height="%d" alt="Roulette selected: %s"></a>`,
		queryParams,
		generateFilePath(filePath),
		dimensions.width,
		dimensions.height,
		fileName))
	htmlBody.WriteString(`</body></html>`)

	_, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
	if err != nil {
		return err
	}

	return nil
}

func staticFile(w http.ResponseWriter, r *http.Request, paths []string, stats *ServeStats) error {
	prefixedFilePath, err := stripQueryParams(r.URL.Path)
	if err != nil {
		return err
	}

	filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, prefix))
	if err != nil {
		return err
	}

	if !pathIsValid(filePath, paths) {
		notFound(w, r, filePath)

		return nil
	}

	exists, err := fileExists(filePath)
	if err != nil {
		return err
	}

	if !exists {
		notFound(w, r, filePath)

		return nil
	}

	startTime := time.Now()

	buf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	w.Write(buf)

	fileSize := humanReadableSize(len(buf))

	if Verbose {
		fmt.Printf("%s | Served %s (%s) to %s in %s\n",
			startTime.Format(logDate),
			filePath,
			fileSize,
			realIP(r),
			time.Since(startTime).Round(time.Microsecond),
		)
	}

	if Debug {
		stats.incrementCounter(filePath, startTime, fileSize)
	}

	return nil
}

func cacheHandler(args []string, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index.generateCache(args)

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Ok"))
	}
}

func statsHandler(args []string, stats *ServeStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		response, err := stats.ListImages()
		if err != nil {
			fmt.Println(err)
		}

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | Served statistics page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func staticFileHandler(paths []string, stats *ServeStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := staticFile(w, r, paths, stats)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func htmlHandler(paths []string, regexes *Regexes, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			fmt.Println(err)
		}

		filters := &Filters{
			includes: splitQueryParams(r.URL.Query().Get("include"), regexes),
			excludes: splitQueryParams(r.URL.Query().Get("exclude"), regexes),
		}

		sortOrder := sortOrder(r)

		_, refreshInterval := refreshInterval(r, regexes)

		if r.URL.Path == "/" {
			var filePath string
			var err error

			if refererUri != "" {
				filePath, err = nextFile(refererUri, sortOrder, regexes)
				if err != nil {
					fmt.Println(err)
				}
			}

			if filePath == "" {
				filePath, err = newFile(paths, filters, sortOrder, regexes, index)
				switch {
				case err != nil && err == errNoImagesFound:
					notFound(w, r, filePath)

					return
				case err != nil:
					fmt.Println(err)
				}
			}

			queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

			newUrl := fmt.Sprintf("http://%s%s%s",
				r.Host,
				preparePath(filePath),
				queryParams,
			)
			http.Redirect(w, r, newUrl, redirectStatusCode)
		} else {
			filePath := r.URL.Path

			if runtime.GOOS == "windows" {
				filePath = strings.TrimPrefix(filePath, "/")
			}

			exists, err := fileExists(filePath)
			if err != nil {
				fmt.Println(err)
			}
			if !exists {
				notFound(w, r, filePath)

				return
			}

			image, err := isImage(filePath)
			if err != nil {
				fmt.Println(err)
			}
			if !image {
				notFound(w, r, filePath)

				return
			}

			dimensions, err := imageDimensions(filePath)
			if err != nil {
				fmt.Println(err)
			}

			err = html(w, r, filePath, dimensions, filters, regexes)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage(args []string) error {
	fmt.Printf("roulette v%s\n\n", Version)

	paths, err := normalizePaths(args)
	if err != nil {
		return err
	}

	regexes := &Regexes{
		filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		alphanumeric: regexp.MustCompile(`^[a-zA-Z0-9]*$`),
		units:        regexp.MustCompile(`^[0-9]+(ns|us|µs|ms|s|m|h)$`),
	}

	rand.Seed(time.Now().UnixNano())

	index := &Index{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	if Cache {
		index.generateCache(args)

		http.Handle("/_/clear_cache", cacheHandler(args, index))
	}

	stats := &ServeStats{
		mutex: sync.RWMutex{},
		list:  []string{},
		count: make(map[string]uint64),
		size:  make(map[string]string),
		times: make(map[string][]string),
	}

	http.Handle("/", htmlHandler(paths, regexes, index))
	http.Handle(prefix+"/", http.StripPrefix(prefix, staticFileHandler(paths, stats)))
	http.HandleFunc("/favicon.ico", doNothing)

	if Debug {
		http.Handle("/_/stats", statsHandler(args, stats))
	}

	err = http.ListenAndServe(":"+strconv.FormatInt(int64(Port), 10), nil)
	if err != nil {
		return err
	}

	return nil
}
