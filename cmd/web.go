/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
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
	"strconv"
	"strings"
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

func notFound(w http.ResponseWriter, r *http.Request, filePath string) error {
	startTime := time.Now()

	if Verbose {
		fmt.Printf("%v | Unavailable file %v requested by %v\n",
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

func getRefreshInterval(r *http.Request) string {
	refreshInterval := r.URL.Query().Get("refresh")

	num, err := strconv.Atoi(refreshInterval)

	if err != nil || num < 0 {
		refreshInterval = "0"
	}

	return refreshInterval
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

		queryParams.WriteString(fmt.Sprintf("sort=%v", sortOrder))

		hasParams = true
	}

	if hasParams {
		queryParams.WriteString("&")
	}
	queryParams.WriteString(fmt.Sprintf("refresh=%v", refreshInterval))

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

func serveHtml(w http.ResponseWriter, r *http.Request, filePath string, dimensions *Dimensions, filters *Filters) error {
	fileName := filepath.Base(filePath)

	w.Header().Add("Content-Type", "text/html")

	sortOrder := getSortOrder(r)

	refreshInterval := getRefreshInterval(r)

	queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

	var htmlBody strings.Builder
	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(`<style>html,body{margin:0;padding:0;height:100%;}`)
	htmlBody.WriteString(`a{display:block;height:100%;width:100%;text-decoration:none;}`)
	htmlBody.WriteString(`img{margin:auto;display:block;max-width:97%;max-height:97%;object-fit:scale-down;`)
	htmlBody.WriteString(`position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}</style>`)
	htmlBody.WriteString(fmt.Sprintf(`<title>%v (%vx%v)</title>`,
		fileName,
		dimensions.Width,
		dimensions.Height))
	htmlBody.WriteString(`</head><body>`)
	if refreshInterval != "0" {
		r, err := strconv.Atoi(refreshInterval)
		if err != nil {
			return err
		}
		refreshTimer := strconv.Itoa(r * 1000)

		htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%v';}, %v);};</script>",
			queryParams,
			refreshTimer))
	}
	htmlBody.WriteString(fmt.Sprintf(`<a href="/%v"><img src="%v" width="%v" height="%v" alt="Roulette selected: %v"></a>`,
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

func serveStaticFile(w http.ResponseWriter, r *http.Request, paths []string) error {
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

	if Verbose {
		remoteAddr := ""
		cfIP := w.Header().Get("Cf-Connecting-Ip")
		xRealIp := w.Header().Get("X-Real-Ip")
		if cfIP != "" {
			remoteAddr = cfIP
		} else if xRealIp != "" {
			remoteAddr = xRealIp
		} else {
			remoteAddr = r.RemoteAddr
		}

		fmt.Printf("%v | Served %v (%v) to %v in %v\n",
			startTime.Format(LogDate),
			filePath,
			humanReadableSize(len(buf)),
			remoteAddr,
			time.Since(startTime).Round(time.Microsecond),
		)
	}

	return nil
}

func serveStaticFileHandler(paths []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := serveStaticFile(w, r, paths)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func serveHtmlHandler(paths []string, regexes *Regexes, fileCache *[]string) http.HandlerFunc {
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

		refreshInterval := getRefreshInterval(r)

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
				filePath, err = getNewFile(paths, filters, sortOrder, regexes, fileCache)
				switch {
				case err != nil && err == ErrNoImagesFound:
					notFound(w, r, filePath)

					return
				case err != nil:
					log.Fatal(err)
				}
			}

			queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

			newUrl := fmt.Sprintf("http://%v%v%v",
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

			err = serveHtml(w, r, filePath, dimensions, filters)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage(args []string) error {
	fmt.Printf("roulette v%v\n\n", Version)

	paths, err := normalizePaths(args)
	if err != nil {
		return err
	}

	regexes := &Regexes{
		Filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		Alphanumeric: regexp.MustCompile(`^[a-zA-Z0-9]*$`),
	}

	rand.Seed(time.Now().UnixNano())

	fileCache := &[]string{}

	http.Handle("/", serveHtmlHandler(paths, regexes, fileCache))
	http.Handle(Prefix+"/", http.StripPrefix(Prefix, serveStaticFileHandler(paths)))
	http.HandleFunc("/favicon.ico", doNothing)

	err = http.ListenAndServe(":"+strconv.FormatInt(int64(Port), 10), nil)
	if err != nil {
		return err
	}

	return nil
}
