/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	LogDate            string = `2006-01-02T15:04:05.000-07:00`
	Prefix             string = `/src`
	RedirectStatusCode int    = http.StatusSeeOther
)

type Regexes struct {
	Alphanumeric *regexp.Regexp
	Filename     *regexp.Regexp
	Units        *regexp.Regexp
}

type Filters struct {
	Includes []string
	Excludes []string
}

func (f *Filters) IsEmpty() bool {
	return !(f.HasIncludes() || f.HasExcludes())
}

func (f *Filters) HasIncludes() bool {
	return len(f.Includes) != 0
}

func (f *Filters) GetIncludes() string {
	return strings.Join(f.Includes, ",")
}

func (f *Filters) HasExcludes() bool {
	return len(f.Excludes) != 0
}

func (f *Filters) GetExcludes() string {
	return strings.Join(f.Excludes, ",")
}

type Index struct {
	Mutex sync.RWMutex
	List  []string
}

func (i *Index) Get() []string {
	i.Mutex.RLock()
	val := i.List
	i.Mutex.RUnlock()

	return val
}

func (i *Index) Set(val []string) {
	i.Mutex.Lock()
	i.List = val
	i.Mutex.Unlock()
}

func (i *Index) GenerateCache(args []string) {
	i.Mutex.Lock()
	i.List = []string{}
	i.Mutex.Unlock()

	getFileList(args, &Filters{}, "", i)
}

func (i *Index) IsEmpty() bool {
	i.Mutex.RLock()
	length := len(i.List)
	i.Mutex.RUnlock()

	return length == 0
}

type ServeStats struct {
	Mutex sync.RWMutex
	List  []string
	Count map[string]uint64
	Size  map[string]string
	Times map[string][]string
}

func (s *ServeStats) IncrementCounter(image string, timestamp time.Time, filesize string) {
	s.Mutex.Lock()

	s.Count[image]++

	s.Times[image] = append(s.Times[image], timestamp.Format(LogDate))

	_, exists := s.Size[image]
	if !exists {
		s.Size[image] = filesize
	}

	if !contains(s.List, image) {
		s.List = append(s.List, image)
	}

	s.Mutex.Unlock()
}

func (s *ServeStats) ListImages() ([]byte, error) {
	s.Mutex.RLock()

	sortedList := s.List

	sort.SliceStable(sortedList, func(p, q int) bool {
		return sortedList[p] < sortedList[q]
	})

	a := []TimesServed{}

	for _, image := range s.List {
		a = append(a, TimesServed{image, s.Count[image], s.Size[image], s.Times[image]})
	}

	s.Mutex.RUnlock()

	r, err := json.MarshalIndent(a, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

type TimesServed struct {
	File   string
	Served uint64
	Size   string
	Times  []string
}

func notFound(w http.ResponseWriter, r *http.Request, filePath string) error {
	startTime := time.Now()

	if Verbose {
		fmt.Printf("%s | Unavailable file %s requested by %s\n",
			startTime.Format(LogDate),
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

func getRefreshInterval(r *http.Request, regexes *Regexes) (int64, string) {
	refreshInterval := r.URL.Query().Get("refresh")

	if !regexes.Units.MatchString(refreshInterval) {
		return 0, "0ms"
	}

	duration, err := time.ParseDuration(refreshInterval)
	if err != nil {
		return 0, "0ms"
	}

	durationInMs := duration.Milliseconds()

	return durationInMs, refreshInterval
}

func getSortOrder(r *http.Request) string {
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
		if regexes.Alphanumeric.MatchString(params[i]) {
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
			queryParams.WriteString(filters.GetIncludes())
		}

		queryParams.WriteString("&exclude=")
		if filters.HasExcludes() {
			queryParams.WriteString(filters.GetExcludes())
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

	htmlBody.WriteString(Prefix)
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

func getRealIp(r *http.Request) string {
	remoteAddr := strings.SplitAfter(r.RemoteAddr, ":")

	if len(remoteAddr) < 1 {
		return r.RemoteAddr
	}

	remotePort := remoteAddr[len(remoteAddr)-1]

	cfIP := r.Header.Get("Cf-Connecting-Ip")
	xRealIp := r.Header.Get("X-Real-Ip")

	switch {
	case cfIP != "":
		return cfIP + ":" + remotePort
	case xRealIp != "":
		return xRealIp + ":" + remotePort
	default:
		return r.RemoteAddr
	}
}

func serveHtml(w http.ResponseWriter, r *http.Request, filePath string, dimensions *Dimensions, filters *Filters, regexes *Regexes) error {
	fileName := filepath.Base(filePath)

	w.Header().Add("Content-Type", "text/html")

	sortOrder := getSortOrder(r)

	refreshTimer, refreshInterval := getRefreshInterval(r, regexes)

	queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

	var htmlBody strings.Builder
	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(`<style>html,body{margin:0;padding:0;height:100%;}`)
	htmlBody.WriteString(`a{display:block;height:100%;width:100%;text-decoration:none;}`)
	htmlBody.WriteString(`img{margin:auto;display:block;max-width:97%;max-height:97%;object-fit:scale-down;`)
	htmlBody.WriteString(`position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}</style>`)
	htmlBody.WriteString(fmt.Sprintf(`<title>%s (%dx%d)</title>`,
		fileName,
		dimensions.Width,
		dimensions.Height))
	htmlBody.WriteString(`</head><body>`)
	if refreshInterval != "0ms" {
		htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%s';}, %d);};</script>",
			queryParams,
			refreshTimer))
	}
	htmlBody.WriteString(fmt.Sprintf(`<a href="/%s"><img src="%s" width="%d" height="%d" alt="Roulette selected: %s"></a>`,
		queryParams,
		generateFilePath(filePath),
		dimensions.Width,
		dimensions.Height,
		fileName))
	htmlBody.WriteString(`</body></html>`)

	_, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
	if err != nil {
		return err
	}

	return nil
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, paths []string, stats *ServeStats) error {
	prefixedFilePath, err := stripQueryParams(r.URL.Path)
	if err != nil {
		return err
	}

	filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, Prefix))
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
			startTime.Format(LogDate),
			filePath,
			fileSize,
			getRealIp(r),
			time.Since(startTime).Round(time.Microsecond),
		)
	}

	if Debug {
		stats.IncrementCounter(filePath, startTime, fileSize)
	}

	return nil
}

func serveCacheClearHandler(args []string, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index.GenerateCache(args)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Ok"))
	}
}

func serveStatsHandler(args []string, stats *ServeStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		response, err := stats.ListImages()
		if err != nil {
			log.Fatal(err)
		}

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | Served statistics page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(len(response)),
				getRealIp(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveStaticFileHandler(paths []string, stats *ServeStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := serveStaticFile(w, r, paths, stats)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func serveHtmlHandler(paths []string, regexes *Regexes, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			log.Fatal(err)
		}

		filters := &Filters{
			Includes: splitQueryParams(r.URL.Query().Get("include"), regexes),
			Excludes: splitQueryParams(r.URL.Query().Get("exclude"), regexes),
		}

		sortOrder := getSortOrder(r)

		_, refreshInterval := getRefreshInterval(r, regexes)

		if r.URL.Path == "/" {
			var filePath string
			var err error

			if refererUri != "" {
				filePath, err = getNextFile(refererUri, sortOrder, regexes)
				if err != nil {
					log.Fatal(err)
				}
			}

			if filePath == "" {
				filePath, err = getNewFile(paths, filters, sortOrder, regexes, index)
				switch {
				case err != nil && err == ErrNoImagesFound:
					notFound(w, r, filePath)

					return
				case err != nil:
					log.Fatal(err)
				}
			}

			queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

			newUrl := fmt.Sprintf("http://%s%s%s",
				r.Host,
				preparePath(filePath),
				queryParams,
			)
			http.Redirect(w, r, newUrl, RedirectStatusCode)
		} else {
			filePath := r.URL.Path

			if runtime.GOOS == "windows" {
				filePath = strings.TrimPrefix(filePath, "/")
			}

			exists, err := fileExists(filePath)
			if err != nil {
				log.Fatal(err)
			}
			if !exists {
				notFound(w, r, filePath)

				return
			}

			image, err := isImage(filePath)
			if err != nil {
				log.Fatal(err)
			}
			if !image {
				notFound(w, r, filePath)

				return
			}

			dimensions, err := getImageDimensions(filePath)
			if err != nil {
				log.Fatal(err)
			}

			err = serveHtml(w, r, filePath, dimensions, filters, regexes)
			if err != nil {
				log.Fatal(err)
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
		Filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		Alphanumeric: regexp.MustCompile(`^[a-zA-Z0-9]*$`),
		Units:        regexp.MustCompile(`^[0-9]+(ns|us|µs|ms|s|m|h)$`),
	}

	rand.Seed(time.Now().UnixNano())

	index := &Index{
		Mutex: sync.RWMutex{},
		List:  []string{},
	}

	if Cache {
		index.GenerateCache(args)

		http.Handle("/_/clear_cache", serveCacheClearHandler(args, index))
	}

	stats := &ServeStats{
		Mutex: sync.RWMutex{},
		List:  []string{},
		Count: make(map[string]uint64),
		Size:  make(map[string]string),
		Times: make(map[string][]string),
	}

	http.Handle("/", serveHtmlHandler(paths, regexes, index))
	http.Handle(Prefix+"/", http.StripPrefix(Prefix, serveStaticFileHandler(paths, stats)))
	http.HandleFunc("/favicon.ico", doNothing)

	if Debug {
		http.Handle("/_/stats", serveStatsHandler(args, stats))
	}

	err = http.ListenAndServe(":"+strconv.FormatInt(int64(Port), 10), nil)
	if err != nil {
		return err
	}

	return nil
}
