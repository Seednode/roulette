/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"bytes"
	"embed"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"net/http/pprof"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"github.com/yosssi/gohtml"
)

//go:embed favicons/*
var favicons embed.FS

const (
	LogDate            string        = `2006-01-02T15:04:05.000-07:00`
	SourcePrefix       string        = `/source`
	ImagePrefix        string        = `/view`
	RedirectStatusCode int           = http.StatusSeeOther
	Timeout            time.Duration = 10 * time.Second

	FaviconHtml string = `<link rel="apple-touch-icon" sizes="180x180" href="/favicons/apple-touch-icon.png">
	<link rel="icon" type="image/png" sizes="32x32" href="/favicons/favicon-32x32.png">
	<link rel="icon" type="image/png" sizes="16x16" href="/favicons/favicon-16x16.png">
	<link rel="manifest" href="/favicons/site.webmanifest">
	<link rel="mask-icon" href="/favicons/safari-pinned-tab.svg" color="#5bbad5">
	<meta name="msapplication-TileColor" content="#da532c">
	<meta name="theme-color" content="#ffffff">`
)

type Regexes struct {
	alphanumeric *regexp.Regexp
	filename     *regexp.Regexp
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

func (i *Index) Remove(path string) {
	i.mutex.RLock()
	tempIndex := make([]string, len(i.list))
	copy(tempIndex, i.list)
	i.mutex.RUnlock()

	var position int

	for k, v := range tempIndex {
		if path == v {
			position = k

			break
		}
	}

	tempIndex[position] = tempIndex[len(tempIndex)-1]

	i.mutex.Lock()
	i.list = make([]string, len(tempIndex)-1)
	copy(i.list, tempIndex[:len(tempIndex)-1])
	i.mutex.Unlock()
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
	count map[string]uint32
	size  map[string]string
	times map[string][]string
}

type exportedServeStats struct {
	List  []string
	Count map[string]uint32
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
		Count: make(map[string]uint32),
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

func (s *ServeStats) ListImages(page int) ([]byte, error) {
	stats := s.toExported()

	sort.SliceStable(stats.List, func(p, q int) bool {
		return strings.ToLower(stats.List[p]) < strings.ToLower(stats.List[q])
	})

	var startIndex, stopIndex int

	if page == -1 {
		startIndex = 0
		stopIndex = len(stats.List) - 1
	} else {
		startIndex = ((page - 1) * int(pageLength))
		stopIndex = (startIndex + int(pageLength))
	}

	if startIndex > len(stats.List)-1 {
		return []byte("{}"), nil
	}

	if stopIndex > len(stats.List)-1 {
		stopIndex = len(stats.List) - 1
	}

	a := make([]timesServed, stopIndex-startIndex)

	for k, v := range stats.List[startIndex:stopIndex] {
		a[k] = timesServed{v, stats.Count[v], stats.Size[v], stats.Times[v]}
	}

	r, err := json.MarshalIndent(a, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

func (s *ServeStats) Export(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
		return err
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
		Count: make(map[string]uint32),
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
	Served uint32
	Size   string
	Times  []string
}

func newErrorPage(title, body string) string {
	var htmlBody strings.Builder

	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(FaviconHtml)
	htmlBody.WriteString(`<style>a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}</style>`)
	htmlBody.WriteString(fmt.Sprintf("<title>%s</title></head>", title))
	htmlBody.WriteString(fmt.Sprintf("<body><a href=\"/\">%s</a></body></html>", body))

	return htmlBody.String()
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

	w.WriteHeader(http.StatusNotFound)
	w.Header().Add("Content-Type", "text/html")

	_, err := io.WriteString(w, gohtml.Format(newErrorPage("Not Found", "404 Page not found")))
	if err != nil {
		return err
	}

	return nil
}

func serverError(w http.ResponseWriter, r *http.Request, i interface{}) {
	startTime := time.Now()

	if verbose {
		fmt.Printf("%s | Invalid request for %s from %s\n",
			startTime.Format(LogDate),
			r.URL.Path,
			r.RemoteAddr,
		)
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Add("Content-Type", "text/html")

	io.WriteString(w, gohtml.Format(newErrorPage("Server Error", "500 Internal Server Error")))
}

func serverErrorHandler() func(http.ResponseWriter, *http.Request, interface{}) {
	return serverError
}

func RefreshInterval(r *http.Request) (int64, string) {
	var interval string

	if refreshInterval == "" {
		interval = r.URL.Query().Get("refresh")
	} else {
		interval = refreshInterval
	}

	duration, err := time.ParseDuration(interval)

	switch {
	case err != nil || duration == 0:
		return 0, "0ms"
	case duration < 500*time.Millisecond:
		return 500, "500ms"
	default:
		return duration.Milliseconds(), interval
	}
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

	htmlBody.WriteString(SourcePrefix)
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

func serveCacheClear(args []string, index *Index) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		index.generateCache(args)

		w.Header().Set("Content-Type", "text/plain")

		w.Write([]byte("Ok"))
	}
}

func serveStats(args []string, stats *ServeStats) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

		startTime := time.Now()

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page == 0 {
			page = -1
		}

		response, err := stats.ListImages(page)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		w.Header().Set("Content-Type", "application/json")

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

func serveDebugHtml(args []string, index *Index, paginate bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/html")

		startTime := time.Now()

		indexDump := index.Index()

		fileCount := len(indexDump)

		var startIndex, stopIndex int

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page <= 0 {
			startIndex = 0
			stopIndex = fileCount
		} else {
			startIndex = ((page - 1) * int(pageLength))
			stopIndex = (startIndex + int(pageLength))
		}

		if startIndex > (fileCount - 1) {
			indexDump = []string{}
		}

		if stopIndex > fileCount {
			stopIndex = fileCount
		}

		sort.SliceStable(indexDump, func(p, q int) bool {
			return strings.ToLower(indexDump[p]) < strings.ToLower(indexDump[q])
		})

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
		htmlBody.WriteString(FaviconHtml)
		htmlBody.WriteString(`<style>a{text-decoration:none;height:100%;width:100%;color:inherit;cursor:pointer}`)
		htmlBody.WriteString(`table,td,tr{border:1px solid black;border-collapse:collapse}td{white-space:nowrap;padding:.5em}</style>`)
		htmlBody.WriteString(fmt.Sprintf("<title>Index contains %d files</title></head><body><table>", fileCount))
		if len(indexDump) > 0 {
			for _, v := range indexDump[startIndex:stopIndex] {
				var shouldSort = ""

				if sorting {
					shouldSort = "?sort=asc"
				}
				htmlBody.WriteString(fmt.Sprintf("<tr><td><a href=\"%s%s%s\">%s</a></td></tr>\n", ImagePrefix, v, shouldSort, v))
			}
		}
		if pageLength != 0 {
			nextPage := page + 1

			if nextPage > (fileCount/int(pageLength)) && fileCount%int(pageLength) == 0 {
				nextPage = fileCount / int(pageLength)
			} else if nextPage > (fileCount / int(pageLength)) {
				nextPage = (fileCount / int(pageLength)) + 1
			}

			prevPage := page - 1
			if prevPage < 1 {
				prevPage = 1
			}

			if paginate {
				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\">Prev</button>", prevPage))
				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\">Next</button>", nextPage))
			}
		}

		htmlBody.WriteString(`</table></body></html>`)

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

func serveDebugJson(args []string, index *Index) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		indexDump := index.Index()

		fileCount := len(indexDump)

		sort.SliceStable(indexDump, func(p, q int) bool {
			return strings.ToLower(indexDump[p]) < strings.ToLower(indexDump[q])
		})

		var startIndex, stopIndex int

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page <= 0 {
			startIndex = 0
			stopIndex = fileCount
		} else {
			startIndex = ((page - 1) * int(pageLength))
			stopIndex = (startIndex + int(pageLength))
		}

		if startIndex > (fileCount - 1) {
			indexDump = []string{}
		}

		if stopIndex > fileCount {
			stopIndex = fileCount
		}

		response, err := json.MarshalIndent(indexDump[startIndex:stopIndex], "", "    ")
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

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

func serveStaticFile(paths []string, stats *ServeStats, index *Index) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		path := strings.TrimPrefix(r.URL.Path, SourcePrefix)

		prefixedFilePath, err := stripQueryParams(path)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, SourcePrefix))
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		if !pathIsValid(filePath, paths) {
			notFound(w, r, filePath)

			return
		}

		exists, err := fileExists(filePath)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

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

			serverError(w, r, nil)

			return
		}

		w.Write(buf)

		fileSize := humanReadableSize(len(buf))

		if russian {
			if cache {
				index.Remove(filePath)
			}

			err = os.Remove(filePath)
			if err != nil {
				fmt.Println(err)

				serverError(w, r, nil)

				return
			}
		}

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

func serveRoot(paths []string, Regexes *Regexes, index *Index) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		strippedRefererUri := strings.TrimPrefix(refererUri, ImagePrefix)

		filters := &Filters{
			includes: splitQueryParams(r.URL.Query().Get("include"), Regexes),
			excludes: splitQueryParams(r.URL.Query().Get("exclude"), Regexes),
		}

		sortOrder := SortOrder(r)

		_, refreshInterval := RefreshInterval(r)

		var filePath string

		if refererUri != "" {
			filePath, err = nextFile(strippedRefererUri, sortOrder, Regexes)
			if err != nil {
				fmt.Println(err)

				serverError(w, r, nil)

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

				serverError(w, r, nil)

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
	}
}

func serveImage(paths []string, Regexes *Regexes, index *Index) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filters := &Filters{
			includes: splitQueryParams(r.URL.Query().Get("include"), Regexes),
			excludes: splitQueryParams(r.URL.Query().Get("exclude"), Regexes),
		}

		sortOrder := SortOrder(r)

		filePath := strings.TrimPrefix(r.URL.Path, ImagePrefix)

		if runtime.GOOS == "windows" {
			filePath = strings.TrimPrefix(filePath, "/")
		}

		exists, err := fileExists(filePath)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}
		if !exists {
			notFound(w, r, filePath)

			return
		}

		supported, fileType, err := isSupportedFileType(filePath)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		if !supported {
			notFound(w, r, filePath)

			return
		}

		dimensions, err := imageDimensions(filePath)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		fileName := filepath.Base(filePath)

		w.Header().Add("Content-Type", "text/html")

		refreshTimer, refreshInterval := RefreshInterval(r)

		queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
		htmlBody.WriteString(FaviconHtml)
		htmlBody.WriteString(`<style>html,body{margin:0;padding:0;height:100%;}`)
		htmlBody.WriteString(`a{display:block;height:100%;width:100%;text-decoration:none;}`)
		htmlBody.WriteString(`img{margin:auto;display:block;max-width:97%;max-height:97%;object-fit:scale-down;`)
		htmlBody.WriteString(`position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}</style>`)

		switch fileType {
		case "image":
			htmlBody.WriteString(fmt.Sprintf(`<title>%s (%dx%d)</title>`,
				fileName,
				dimensions.width,
				dimensions.height))
		case "video":
			htmlBody.WriteString(fmt.Sprintf(`<title>%s</title>`,
				fileName))
		}

		htmlBody.WriteString(`</head><body>`)
		if refreshInterval != "0ms" {
			htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%s';}, %d);};</script>",
				queryParams,
				refreshTimer))
		}

		switch fileType {
		case "image":
			htmlBody.WriteString(fmt.Sprintf(`<a href="/%s"><img src="%s" width="%d" height="%d" alt="Roulette selected: %s"></a>`,
				queryParams,
				generateFilePath(filePath),
				dimensions.width,
				dimensions.height,
				fileName))
			htmlBody.WriteString(`</body></html>`)
		case "video":
			htmlBody.WriteString(fmt.Sprintf(`<a href="/%s"><video controls autoplay><source src="%s" alt="Roulette selected: %s">Your browser does not support the video tag.</video></a>`,
				queryParams,
				generateFilePath(filePath),
				fileName))
			htmlBody.WriteString(`</body></html>`)
		}

		_, err = io.WriteString(w, gohtml.Format(htmlBody.String()))
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}
	}
}

func serveFavicons() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fname := strings.TrimPrefix(r.URL.Path, "/")

		data, err := favicons.ReadFile(fname)
		if err != nil {
			return
		}

		w.Header().Write(bytes.NewBufferString("Content-Length: " + strconv.Itoa(len(data))))

		w.Write(data)
	}
}

func serveVersion() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		data := []byte(fmt.Sprintf("roulette v%s\n", Version))

		w.Header().Write(bytes.NewBufferString("Content-Length: " + strconv.Itoa(len(data))))

		w.Write(data)
	}
}

func ServePage(args []string) error {
	timeZone := os.Getenv("TZ")
	if timeZone != "" {
		var err error
		time.Local, err = time.LoadLocation(timeZone)
		if err != nil {
			return err
		}
	}

	bindHost, err := net.LookupHost(bind)
	if err != nil {
		return err
	}

	bindAddr := net.ParseIP(bindHost[0])
	if bindAddr == nil {
		return errors.New("invalid bind address provided")
	}

	paths, err := normalizePaths(args)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return errors.New("no supported files found in provided paths")
	}

	if russian {
		fmt.Printf("WARNING! Files *will* be deleted after serving!\n\n")
	}

	mux := httprouter.New()

	index := &Index{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	regexes := &Regexes{
		filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		alphanumeric: regexp.MustCompile(`^[A-z0-9]*$`),
	}

	srv := &http.Server{
		Addr:         net.JoinHostPort(bind, strconv.Itoa(int(port))),
		Handler:      mux,
		IdleTimeout:  10 * time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Minute,
	}

	stats := &ServeStats{
		mutex: sync.RWMutex{},
		list:  []string{},
		count: make(map[string]uint32),
		size:  make(map[string]string),
		times: make(map[string][]string),
	}

	mux.PanicHandler = serverErrorHandler()

	mux.GET("/", serveRoot(paths, regexes, index))

	mux.GET("/favicons/*favicon", serveFavicons())

	mux.GET("/favicon.ico", serveFavicons())

	mux.GET(ImagePrefix+"/*image", serveImage(paths, regexes, index))

	mux.GET(SourcePrefix+"/*static", serveStaticFile(paths, stats, index))

	mux.GET("/version", serveVersion())

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

		mux.GET("/clear_cache", serveCacheClear(args, index))
	}

	if debug {
		mux.GET("/html/", serveDebugHtml(args, index, false))
		if pageLength != 0 {
			mux.GET("/html/:page", serveDebugHtml(args, index, true))
		}

		mux.GET("/json", serveDebugJson(args, index))
		if pageLength != 0 {
			mux.GET("/json/:page", serveDebugJson(args, index))
		}
	}

	if profile {
		mux.HandlerFunc("GET", "/debug/pprof/", pprof.Index)
		mux.HandlerFunc("GET", "/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandlerFunc("GET", "/debug/pprof/profile", pprof.Profile)
		mux.HandlerFunc("GET", "/debug/pprof/symbol", pprof.Symbol)
		mux.HandlerFunc("GET", "/debug/pprof/trace", pprof.Trace)
	}

	if statistics {
		if statisticsFile != "" {
			stats.Import(statisticsFile)

			gracefulShutdown := make(chan os.Signal, 1)
			signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-gracefulShutdown

				stats.Export(statisticsFile)

				os.Exit(0)
			}()
		}

		mux.GET("/stats", serveStats(args, stats))
		if pageLength != 0 {
			mux.GET("/stats/:page", serveStats(args, stats))
		}
	}

	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
