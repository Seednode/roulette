/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	LOGDATE            string = "2006-01-02T15:04:05.000-07:00"
	PREFIX             string = "/src"
	RedirectStatusCode int    = http.StatusSeeOther
)

type Filters struct {
	Includes []string
	Excludes []string
}

func (f *Filters) IsEmpty() bool {
	if !(f.HasIncludes() && f.HasExcludes()) {
		return true
	}

	return false
}

func (f *Filters) HasIncludes() bool {
	if len(f.Includes) == 0 {
		return false
	}

	return true
}

func (f *Filters) GetIncludes() string {
	return strings.Join(f.Includes, ",")
}

func (f *Filters) HasExcludes() bool {
	if len(f.Excludes) == 0 {
		return false
	}

	return true
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

func generateQueryParams(filters *Filters, sort string) string {
	switch {
	case Filter && !Sort:
		return fmt.Sprintf("?include=%v&exclude=%v",
			filters.GetIncludes(),
			filters.GetExcludes(),
		)
	case !Filter && Sort:
		return fmt.Sprintf("?sort=%v", sort)
	case Filter && Sort:
		return fmt.Sprintf("?include=%v&exclude=%v&sort=%v",
			filters.GetIncludes(),
			filters.GetExcludes(),
			sort,
		)
	}

	return ""
}

func stripQueryParams(inUrl string) (string, error) {
	url, err := url.Parse(inUrl)
	if err != nil {
		return "", err
	}

	url.RawQuery = ""

	if runtime.GOOS == "windows" {
		return strings.TrimPrefix(url.String(), "/"), nil
	}

	return url.String(), nil
}

func refererToUri(referer string) string {
	parts := strings.SplitAfterN(referer, "/", 4)

	if len(parts) < 4 {
		return ""
	}

	return "/" + parts[3]
}

func serveHtml(w http.ResponseWriter, r *http.Request, filePath, dimensions string) error {
	fileName := filepath.Base(filePath)

	w.Header().Add("Content-Type", "text/html")

	htmlBody := `<html lang="en">
  <head>
    <style>img{max-width:100%;max-height:97vh;height:auto;}</style>
	<title>`
	htmlBody += fmt.Sprintf("%v (%v)", fileName, dimensions)
	htmlBody += `</title>
  </head>
  <body>`
	switch {
	case Filter && Sort:
		htmlBody += fmt.Sprintf(`<a href="/?include=%v&exclude=%v&sort=%v"><img src="`,
			r.URL.Query().Get("include"),
			r.URL.Query().Get("exclude"),
			r.URL.Query().Get("sort"),
		)
	case Filter && !Sort:
		htmlBody += fmt.Sprintf(`<a href="/?include=%v&exclude=%v"><img src="`,
			r.URL.Query().Get("include"),
			r.URL.Query().Get("exclude"),
		)
	case !Filter && Sort:
		htmlBody += fmt.Sprintf(`<a href="/?sort=%v"><img src="`,
			r.URL.Query().Get("sort"),
		)
	default:
		htmlBody += `<a href="/"><img src="`
	}
	htmlBody += PREFIX
	if runtime.GOOS == "windows" {
		htmlBody += "/"
	}
	htmlBody += filePath
	htmlBody += `"></img></a>
  </body>
</html>`

	_, err := io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, paths []string) error {
	strippedUrl, err := stripQueryParams(r.URL.Path)
	if err != nil {
		return err
	}

	prefixedFilePath, err := url.QueryUnescape(strippedUrl)
	if err != nil {
		return err
	}

	filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, PREFIX))
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
			startTime.Format(LOGDATE),
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
		if Filter {
			filters.Includes = splitQueryParams(r.URL.Query().Get("include"))
			filters.Excludes = splitQueryParams(r.URL.Query().Get("exclude"))
		}

		sortOrder := ""
		if Sort {
			sortOrder = r.URL.Query().Get("sort")
		}

		switch {
		case r.URL.Path == "/" && sortOrder == "asc" && refererUri != "":
			query, err := url.QueryUnescape(refererUri)
			if err != nil {
				return err
			}

			path, err := splitPath(query, re)
			if err != nil {
				return err
			}

			filePath, err := getNextFile(path)
			if err != nil {
				return err
			}

			if filePath == "" {
				filePath, err = pickFile(paths, &filters, sortOrder, fileCache)
				switch {
				case err != nil && err == ErrNoImagesFound:
					http.NotFound(w, r)

					return nil
				case err != nil:
					return err
				}

				path, err := splitPath(filePath, re)
				if err != nil {
					return err
				}

				filePath, err = getFirstFile(path)
				if err != nil {
					return err
				}
			}

			newUrl := fmt.Sprintf("http://%v%v%v",
				r.Host,
				preparePath(filePath),
				generateQueryParams(&filters, sortOrder),
			)
			http.Redirect(w, r, newUrl, RedirectStatusCode)
		case r.URL.Path == "/" && sortOrder == "asc" && refererUri == "":
			filePath, err := pickFile(paths, &filters, sortOrder, fileCache)
			if err != nil && err == ErrNoImagesFound {
				notFound(w, r)

				return nil
			} else if err != nil {
				return err
			}

			path, err := splitPath(filePath, re)
			if err != nil {
				return err
			}

			filePath, err = getFirstFile(path)
			if err != nil {
				return err
			}

			newUrl := fmt.Sprintf("http://%v%v%v",
				r.Host,
				preparePath(filePath),
				generateQueryParams(&filters, sortOrder),
			)
			http.Redirect(w, r, newUrl, RedirectStatusCode)
		case r.URL.Path == "/" && sortOrder == "desc" && refererUri != "":
			query, err := url.QueryUnescape(refererUri)
			if err != nil {
				return err
			}

			path, err := splitPath(query, re)
			if err != nil {
				return err
			}

			filePath, err := getPreviousFile(path)
			if err != nil {
				return err
			}

			if filePath == "" {
				filePath, err = pickFile(paths, &filters, sortOrder, fileCache)
				switch {
				case err != nil && err == ErrNoImagesFound:
					notFound(w, r)

					return nil
				case err != nil:
					return err
				}

				path, err := splitPath(filePath, re)
				if err != nil {
					return err
				}

				filePath, err = getLastFile(path)
				if err != nil {
					return err
				}
			}

			newUrl := fmt.Sprintf("http://%v%v%v",
				r.Host,
				preparePath(filePath),
				generateQueryParams(&filters, sortOrder),
			)
			http.Redirect(w, r, newUrl, RedirectStatusCode)
		case r.URL.Path == "/" && sortOrder == "desc" && refererUri == "":
			filePath, err := pickFile(paths, &filters, sortOrder, fileCache)
			if err != nil && err == ErrNoImagesFound {
				notFound(w, r)

				return nil
			} else if err != nil {
				return err
			}

			path, err := splitPath(filePath, re)
			if err != nil {
				return err
			}

			filePath, err = getLastFile(path)
			if err != nil {
				return err
			}

			newUrl := fmt.Sprintf("http://%v%v%v",
				r.Host,
				preparePath(filePath),
				generateQueryParams(&filters, sortOrder),
			)
			http.Redirect(w, r, newUrl, RedirectStatusCode)
		case r.URL.Path == "/":
			filePath, err := pickFile(paths, &filters, sortOrder, fileCache)
			if err != nil && err == ErrNoImagesFound {
				notFound(w, r)

				return nil
			} else if err != nil {
				return err
			}

			newUrl := fmt.Sprintf("http://%v%v%v",
				r.Host,
				preparePath(filePath),
				generateQueryParams(&filters, sortOrder),
			)
			http.Redirect(w, r, newUrl, RedirectStatusCode)
		default:
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

				return nil
			}

			image, err := isImage(filePath)
			if err != nil {
				return err
			}
			if !image {
				notFound(w, r)

				return nil
			}

			dimensions, err := getImageDimensions(filePath)
			if err != nil {
				return err
			}

			err = serveHtml(w, r, filePath, dimensions)
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

	fileCache := []string{}

	http.Handle("/", serveHtmlHandler(paths, *re, &fileCache))
	http.Handle(PREFIX+"/", http.StripPrefix(PREFIX, serveStaticFileHandler(paths)))
	http.HandleFunc("/favicon.ico", doNothing)

	err = http.ListenAndServe(":"+strconv.FormatInt(int64(Port), 10), nil)
	if err != nil {
		return err
	}

	return nil
}
