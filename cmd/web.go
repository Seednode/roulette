/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yosssi/gohtml"
)

const (
	LogDate            string        = `2006-01-02T15:04:05.000-07:00`
	Prefix             string        = `/src`
	RedirectStatusCode int           = http.StatusSeeOther
	Timeout            time.Duration = 10 * time.Second
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

	if cache && cacheFile != "" {
		i.Export(cacheFile)
	}
}

func (i *Index) IsEmpty() bool {
	i.mutex.RLock()
	length := len(i.list)
	i.mutex.RUnlock()

	return length == 0
}

func (i *Index) Export(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	z, err := zstd.NewWriter(file)
	if err != nil {
		return err
	}
	defer z.Close()

	enc := gob.NewEncoder(z)

	i.mutex.RLock()

	enc.Encode(&i.list)

	i.mutex.RUnlock()

	return nil
}

func (i *Index) Import(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	z, err := zstd.NewReader(file)
	if err != nil {
		return err
	}
	defer z.Close()

	dec := gob.NewDecoder(z)

	i.mutex.Lock()

	err = dec.Decode(&i.list)

	i.mutex.Unlock()

	if err != nil {
		return err
	}

	return nil
}

type ServeStats struct {
	mutex sync.RWMutex
	list  []string
	count map[string]uint64
	size  map[string]string
	times map[string][]string
}

type exportedServeStats struct {
	List  []string
	Count map[string]uint64
	Size  map[string]string
	Times map[string][]string
}

func (s *ServeStats) incrementCounter(image string, timestamp time.Time, filesize string) {
	s.mutex.Lock()

	s.count[image]++

	s.times[image] = append(s.times[image], timestamp.Format(LogDate))

	_, exists := s.size[image]
	if !exists {
		s.size[image] = filesize
	}

	if !contains(s.list, image) {
		s.list = append(s.list, image)
	}

	s.mutex.Unlock()
}

func (s *ServeStats) toExported() *exportedServeStats {
	stats := &exportedServeStats{
		List:  make([]string, len(s.list)),
		Count: make(map[string]uint64),
		Size:  make(map[string]string),
		Times: make(map[string][]string),
	}

	s.mutex.RLock()

	copy(stats.List, s.list)

	for k, v := range s.count {
		stats.Count[k] = v
	}

	for k, v := range s.size {
		stats.Size[k] = v
	}

	for k, v := range s.times {
		stats.Times[k] = v
	}

	s.mutex.RUnlock()

	return stats
}

func (s *ServeStats) toImported(stats *exportedServeStats) {
	s.mutex.Lock()

	s.list = make([]string, len(stats.List))

	copy(s.list, stats.List)

	for k, v := range stats.Count {
		s.count[k] = v
	}

	for k, v := range stats.Size {
		s.size[k] = v
	}

	for k, v := range stats.Times {
		s.times[k] = v
	}

	s.mutex.Unlock()
}

func (s *ServeStats) ListImages() ([]byte, error) {
	stats := s.toExported()

	sort.SliceStable(stats.List, func(p, q int) bool {
		return stats.List[p] < stats.List[q]
	})

	a := make([]timesServed, len(stats.List))

	for k, v := range stats.List {
		a[k] = timesServed{v, stats.Count[v], stats.Size[v], stats.Times[v]}
	}

	r, err := json.MarshalIndent(a, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

func (s *ServeStats) Export(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	z, err := zstd.NewWriter(file)
	if err != nil {
		return err
	}
	defer z.Close()

	enc := gob.NewEncoder(z)

	stats := s.toExported()

	err = enc.Encode(&stats)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func (s *ServeStats) Import(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	z, err := zstd.NewReader(file)
	if err != nil {
		return err
	}
	defer z.Close()

	dec := gob.NewDecoder(z)

	stats := &exportedServeStats{
		List:  []string{},
		Count: make(map[string]uint64),
		Size:  make(map[string]string),
		Times: make(map[string][]string),
	}

	err = dec.Decode(stats)
	if err != nil {
		return err
	}

	s.toImported(stats)

	return nil
}

type timesServed struct {
	File   string
	Served uint64
	Size   string
	Times  []string
}

func notFound(w http.ResponseWriter, r *http.Request, filePath string) error {
	startTime := time.Now()

	if verbose {
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

func RefreshInterval(r *http.Request, Regexes *Regexes) (int64, string) {
	refreshInterval := r.URL.Query().Get("refresh")

	if !Regexes.units.MatchString(refreshInterval) {
		return 0, "0ms"
	}

	duration, err := time.ParseDuration(refreshInterval)
	if err != nil {
		return 0, "0ms"
	}

	return duration.Milliseconds(), refreshInterval
}

func SortOrder(r *http.Request) string {
	sortOrder := r.URL.Query().Get("sort")
	if sortOrder == "asc" || sortOrder == "desc" {
		return sortOrder
	}

	return ""
}

func splitQueryParams(query string, Regexes *Regexes) []string {
	results := []string{}

	if query == "" {
		return results
	}

	params := strings.Split(query, ",")

	for i := 0; i < len(params); i++ {
		if Regexes.alphanumeric.MatchString(params[i]) {
			results = append(results, strings.ToLower(params[i]))
		}
	}

	return results
}

func generateQueryParams(filters *Filters, sortOrder, refreshInterval string) string {
	var hasParams bool

	var queryParams strings.Builder

	queryParams.WriteString("?")

	if filtering {
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

	if sorting {
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

func realIP(r *http.Request) string {
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

func serveCacheClear(args []string, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index.generateCache(args)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Ok"))
	}
}

func serveStats(args []string, stats *ServeStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		response, err := stats.ListImages()
		if err != nil {
			fmt.Println(err)
			return
		}

		w.Write(response)

		if verbose {
			fmt.Printf("%s | Served statistics page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}

		if statisticsFile != "" {
			stats.Export(statisticsFile)
		}
	}
}

func serveDebugHtml(args []string, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html")

		startTime := time.Now()

		indexDump := index.Index()

		sort.SliceStable(indexDump, func(p, q int) bool {
			return indexDump[p] < indexDump[q]
		})

		var htmlBody strings.Builder

		htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
		htmlBody.WriteString(`<style>a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}</style>`)
		htmlBody.WriteString(`<title>Debug</title></head><body>`)
		for _, v := range indexDump {
			var shouldSort = ""

			if sorting {
				shouldSort = "?sort=asc"
			}
			htmlBody.WriteString(fmt.Sprintf("<a href=\"%s%s\">%s</a>\n", v, shouldSort, v))
		}
		htmlBody.WriteString(`</body></html>`)

		b, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
		if err != nil {
			return
		}

		if verbose {
			fmt.Printf("%s | Served HTML debug page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(b),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveDebugJson(args []string, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		indexDump := index.Index()

		sort.SliceStable(indexDump, func(p, q int) bool {
			return indexDump[p] < indexDump[q]
		})

		response, err := json.MarshalIndent(indexDump, "", "    ")
		if err != nil {
			return
		}

		w.Write(response)

		if verbose {
			fmt.Printf("%s | Served JSON debug page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveStaticFile(paths []string, stats *ServeStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefixedFilePath, err := stripQueryParams(r.URL.Path)
		if err != nil {
			fmt.Println(err)
			return
		}

		filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, Prefix))
		if err != nil {
			fmt.Println(err)
			return
		}

		if !pathIsValid(filePath, paths) {
			notFound(w, r, filePath)

			return
		}

		exists, err := fileExists(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		if !exists {
			notFound(w, r, filePath)

			return
		}

		startTime := time.Now()

		buf, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		w.Write(buf)

		fileSize := humanReadableSize(len(buf))

		if verbose {
			fmt.Printf("%s | Served %s (%s) to %s in %s\n",
				startTime.Format(LogDate),
				filePath,
				fileSize,
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}

		if statistics {
			stats.incrementCounter(filePath, startTime, fileSize)
		}
	}
}

func serveMedia(paths []string, Regexes *Regexes, index *Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			fmt.Println(err)
			return
		}

		filters := &Filters{
			includes: splitQueryParams(r.URL.Query().Get("include"), Regexes),
			excludes: splitQueryParams(r.URL.Query().Get("exclude"), Regexes),
		}

		sortOrder := SortOrder(r)

		_, refreshInterval := RefreshInterval(r, Regexes)

		if r.URL.Path == "/" {
			var filePath string
			var err error

			if refererUri != "" {
				filePath, err = nextFile(refererUri, sortOrder, Regexes)
				if err != nil {
					fmt.Println(err)
					return
				}
			}

		loop:
			for timeout := time.After(Timeout); ; {
				select {
				case <-timeout:
					break loop
				default:
				}

				if filePath != "" {
					break loop
				}

				filePath, err = newFile(paths, filters, sortOrder, Regexes, index)
				switch {
				case err != nil && err == ErrNoImagesFound:
					notFound(w, r, filePath)

					return
				case err != nil:
					fmt.Println(err)
					return
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
				fmt.Println(err)
				return
			}
			if !exists {
				notFound(w, r, filePath)

				return
			}

			image, video, err := isSupportedFileType(filePath)
			if err != nil {
				fmt.Println(err)
				return
			}

			if !(image || video) {
				notFound(w, r, filePath)

				return
			}

			var dimensions *Dimensions

			if image {
				dimensions, err = imageDimensions(filePath)
				if err != nil {
					fmt.Println(err)
					return
				}
			}

			fileName := filepath.Base(filePath)

			w.Header().Add("Content-Type", "text/html")

			sortOrder := SortOrder(r)

			refreshTimer, refreshInterval := RefreshInterval(r, Regexes)

			queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

			var htmlBody strings.Builder
			htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
			htmlBody.WriteString(`<style>html,body{margin:0;padding:0;height:100%;}`)
			htmlBody.WriteString(`a{display:block;height:100%;width:100%;text-decoration:none;}`)
			htmlBody.WriteString(`img,video{margin:auto;display:block;max-width:97%;max-height:97%;object-fit:scale-down;`)
			htmlBody.WriteString(`position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}</style>`)
			switch {
			case image:
				htmlBody.WriteString(fmt.Sprintf(`<title>%s (%dx%d)</title>`,
					fileName,
					dimensions.width,
					dimensions.height))
			case video:
				htmlBody.WriteString(fmt.Sprintf(`<title>%s</title>`,
					fileName))
			}
			htmlBody.WriteString(`</head><body>`)
			if refreshInterval != "0ms" {
				htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%s';}, %d);};</script>",
					queryParams,
					refreshTimer))
			}
			switch {
			case image:
				htmlBody.WriteString(fmt.Sprintf(`<a href="/%s"><img src="%s" width="%d" height="%d" alt="Roulette selected: %s"></a>`,
					queryParams,
					generateFilePath(filePath),
					dimensions.width,
					dimensions.height,
					fileName))
			case video:
				htmlBody.WriteString(fmt.Sprintf(`<a href="/%s"><video src="%s" alt="Roulette selected: %s" autoplay controls></a>`,
					queryParams,
					generateFilePath(filePath),
					fileName))
			}
			htmlBody.WriteString(`</body></html>`)

			_, err = io.WriteString(w, gohtml.Format(htmlBody.String()))
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage(args []string) error {
	bindHost, err := net.LookupHost(bind)
	if err != nil {
		return err
	}

	bindAddr := net.ParseIP(bindHost[0])
	if bindAddr == nil {
		fmt.Println("Invalid bind address provided. Please specify a hostname, or an IPv4 or IPv6 address in dotted decimal or IPv6 format.")
		os.Exit(1)
	}

	paths, err := normalizePaths(args)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		fmt.Println("No supported files found in provided paths. Exiting.")
		os.Exit(0)
	}

	Regexes := &Regexes{
		filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		alphanumeric: regexp.MustCompile(`^[a-zA-Z0-9]*$`),
		units:        regexp.MustCompile(`^[0-9]+(ns|us|µs|ms|s|m|h)$`),
	}

	rand.New(rand.NewSource(time.Now().UnixNano()))

	index := &Index{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	if cache {
		skipIndex := false

		if cacheFile != "" {
			err := index.Import(cacheFile)
			if err == nil {
				skipIndex = true
			}
		}

		if !skipIndex {
			index.generateCache(args)
		}

		http.Handle("/_/clear_cache", serveCacheClear(args, index))
	}

	stats := &ServeStats{
		mutex: sync.RWMutex{},
		list:  []string{},
		count: make(map[string]uint64),
		size:  make(map[string]string),
		times: make(map[string][]string),
	}

	if statistics && statisticsFile != "" {
		stats.Import(statisticsFile)

		gracefulShutdown := make(chan os.Signal, 1)
		signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-gracefulShutdown
			stats.Export(statisticsFile)
			os.Exit(0)
		}()
	}

	http.Handle("/", serveMedia(paths, Regexes, index))
	http.Handle(Prefix+"/", http.StripPrefix(Prefix, serveStaticFile(paths, stats)))
	http.HandleFunc("/favicon.ico", doNothing)

	if statistics {
		http.Handle("/_/stats", serveStats(args, stats))
	}

	if debug {
		http.Handle("/_/html", serveDebugHtml(args, index))
		http.Handle("/_/json", serveDebugJson(args, index))
	}

	err = http.ListenAndServe(net.JoinHostPort(bind, strconv.FormatInt(int64(port), 10)), nil)
	if err != nil {
		return err
	}

	return nil
}
