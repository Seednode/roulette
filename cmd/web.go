/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"io"
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

type Filters struct {
	Includes []string
	Excludes []string
}

func (f *Filters) IsEmpty() bool {
	return !(f.HasIncludes() && f.HasExcludes())
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

type appHandler func(http.ResponseWriter, *http.Request) error

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func notFound(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(404)
	w.Header().Add("Content-Type", "text/html")

	htmlBody := `<html lang="en">
  <head>
    <style>
      a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}
    </style>
    <title>
      Not Found
    </title>
  </head>
  <body>
    <a href="/">404 page not found</a>
  </body>
</html>`

	_, err := io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func splitQueryParams(query string) []string {
	if query == "" {
		return []string{}
	}

	params := strings.Split(query, ",")

	for i := 0; i < len(params); i++ {
		params[i] = strings.ToLower(params[i])
	}

	return params
}

func generateQueryParams(filters *Filters, sortOrder, refreshInterval string) (string, error) {
	refresh, err := strconv.Atoi(refreshInterval)
	if err != nil {
		return "", err
	}

	var hasParams bool

	var queryParams string
	if Filter || Sort || (refresh != 0) {
		queryParams += "?"
	}

	if Filter {
		queryParams += "include="
		if filters.HasIncludes() {
			queryParams += filters.GetIncludes()

		}

		queryParams += "&exclude="
		if filters.HasExcludes() {
			queryParams += filters.GetExcludes()
		}

		hasParams = true
	}

	if Sort {
		if hasParams {
			queryParams += "&"
		}

		queryParams += fmt.Sprintf("sort=%v", sortOrder)

		hasParams = true
	}

	if refresh != 0 {
		if hasParams {
			queryParams += "&"
		}
		queryParams += fmt.Sprintf("refresh=%v", refresh)
	}

	return queryParams, nil
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
	htmlBody := Prefix
	if runtime.GOOS == "windows" {
		htmlBody += "/"
	}
	htmlBody += filePath

	return htmlBody
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

	refreshInterval := r.URL.Query().Get("refresh")
	if refreshInterval == "" {
		refreshInterval = "0"
	}

	queryParams, err := generateQueryParams(filters, r.URL.Query().Get("sort"), refreshInterval)

	var htmlBody strings.Builder
	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(`<style>a{display:block;height:100%;width:100%;text-decoration:none;}`)
	htmlBody.WriteString(`img{max-width:100%;max-height:97vh;object-fit:contain;}</style>`)
	htmlBody.WriteString(fmt.Sprintf(`<title>%v (%vx%v)</title>`,
		fileName,
		dimensions.Width,
		dimensions.Height))
	htmlBody.WriteString(`</head><body>`)
	htmlBody.WriteString(fmt.Sprintf(`<a href="/%v"><img src="%v" width="%v" height="%v" alt="Roulette selected: %v"></a>`,
		queryParams,
		generateFilePath(filePath),
		dimensions.Width,
		dimensions.Height,
		fileName))
	if refreshInterval != "0" {
		r, err := strconv.Atoi(refreshInterval)
		if err != nil {
			return err
		}
		refreshTimer := strconv.Itoa(r * 1000)
		htmlBody.WriteString(fmt.Sprintf(`<script>setTimeout(function(){window.location.href = '/%v';},%v);</script>`,
			queryParams,
			refreshTimer))
	}
	htmlBody.WriteString(`</body></html>`)

	_, err = io.WriteString(w, gohtml.Format(htmlBody.String()))
	if err != nil {
		return err
	}

	return nil
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, paths []string) error {
	PrefixedFilePath, err := stripQueryParams(r.URL.Path)
	if err != nil {
		return err
	}

	filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(PrefixedFilePath, Prefix))
	if err != nil {
		return err
	}

	if !pathIsValid(filePath, paths) {
		notFound(w, r)
	}

	exists, err := fileExists(filePath)
	if err != nil {
		return err
	}

	if !exists {
		notFound(w, r)

		return nil
	}

	startTime := time.Now()

	buf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	w.Write(buf)

	if Verbose {
		fmt.Printf("%v | Served %v (%v) to %v in %v\n",
			startTime.Format(LogDate),
			filePath,
			humanReadableSize(len(buf)),
			r.RemoteAddr,
			time.Since(startTime).Round(time.Microsecond),
		)
	}

	return nil
}

func serveStaticFileHandler(paths []string) appHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		err := serveStaticFile(w, r, paths)
		if err != nil {
			return err
		}

		return nil
	}
}

func serveHtmlHandler(paths []string, re regexp.Regexp, fileCache *[]string) appHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			return err
		}

		filters := Filters{}
		filters.Includes = splitQueryParams(r.URL.Query().Get("include"))
		filters.Excludes = splitQueryParams(r.URL.Query().Get("exclude"))

		sortOrder := r.URL.Query().Get("sort")

		refreshInterval := r.URL.Query().Get("refresh")
		if refreshInterval == "" {
			refreshInterval = "0"
		}

		if r.URL.Path == "/" {
			var filePath string
			var err error

			if refererUri != "" {
				filePath, err = getNextFile(refererUri, sortOrder, re)
				if err != nil {
					return err
				}
			}

			if filePath == "" {
				filePath, err = getNewFile(paths, &filters, sortOrder, re, fileCache)
				switch {
				case err != nil && err == ErrNoImagesFound:
					http.NotFound(w, r)
				case err != nil:
					return err
				}
			}

			queryParams, err := generateQueryParams(&filters, sortOrder, refreshInterval)
			if err != nil {
				return err
			}

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
				return err
			}
			if !exists {
				notFound(w, r)
			}

			image, err := isImage(filePath)
			if err != nil {
				return err
			}
			if !image {
				notFound(w, r)
			}

			dimensions, err := getImageDimensions(filePath)
			if err != nil {
				return err
			}

			err = serveHtml(w, r, filePath, dimensions, &filters)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage(args []string) error {
	fmt.Printf("roulette v%v\n\n", Version)

	paths, err := normalizePaths(args)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`(.+)([0-9]{3})(\..+)`)

	rand.Seed(time.Now().UnixNano())

	fileCache := []string{}

	http.Handle("/", serveHtmlHandler(paths, *re, &fileCache))
	http.Handle(Prefix+"/", http.StripPrefix(Prefix, serveStaticFileHandler(paths)))
	http.HandleFunc("/favicon.ico", doNothing)

	err = http.ListenAndServe(":"+strconv.FormatInt(int64(Port), 10), nil)
	if err != nil {
		return err
	}

	return nil
}
