/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"github.com/yosssi/gohtml"
	"seedno.de/seednode/roulette/types"
	"seedno.de/seednode/roulette/types/audio"
	"seedno.de/seednode/roulette/types/code"
	"seedno.de/seednode/roulette/types/flash"
	"seedno.de/seednode/roulette/types/images"
	"seedno.de/seednode/roulette/types/text"
	"seedno.de/seednode/roulette/types/video"
)

const (
	logDate            string        = `2006-01-02T15:04:05.000-07:00`
	sourcePrefix       string        = `/source`
	mediaPrefix        string        = `/view`
	redirectStatusCode int           = http.StatusSeeOther
	timeout            time.Duration = 10 * time.Second
)

func newPage(title, body string) string {
	var htmlBody strings.Builder

	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(faviconHtml)
	htmlBody.WriteString(`<style>html,body,a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}</style>`)
	htmlBody.WriteString(fmt.Sprintf("<title>%s</title></head>", title))
	htmlBody.WriteString(fmt.Sprintf("<body><a href=\"/\">%s</a></body></html>", body))

	return htmlBody.String()
}

func noFiles(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

	w.Write([]byte("No files found in the specified path(s).\n"))

	if Verbose {
		fmt.Printf("%s | SERVE: Empty path notification to %s\n",
			startTime.Format(logDate),
			r.RemoteAddr,
		)
	}
}

func serveStaticFile(paths []string, index *fileIndex, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		prefix := Prefix + sourcePrefix

		path := strings.TrimPrefix(r.URL.Path, prefix)

		prefixedFilePath, err := stripQueryParams(path)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, prefix))
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		if !pathIsValid(filePath, paths) {
			notFound(w, r, filePath)

			return
		}

		exists, err := fileExists(filePath)
		if err != nil {
			errorChannel <- err

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
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		var status string

		written, err := w.Write(buf)
		switch {
		case errors.Is(err, syscall.EPIPE):
			status = " (incomplete)"
		case err != nil:
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		if Russian && refererUri != "" {
			err = kill(filePath, index)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

		if Verbose {
			fmt.Printf("%s | SERVE: %s (%s) to %s in %s%s\n",
				startTime.Format(logDate),
				filePath,
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
				status,
			)
		}
	}
}

func serveRoot(paths []string, index *fileIndex, filename *regexp.Regexp, formats types.Types, encoder *zstd.Encoder, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		strippedRefererUri := strings.TrimPrefix(refererUri, Prefix+mediaPrefix)

		filters := &filters{
			included: splitQueryParams(r.URL.Query().Get("include")),
			excluded: splitQueryParams(r.URL.Query().Get("exclude")),
		}

		sortOrder := sortOrder(r)

		_, refreshInterval := refreshInterval(r)

		var path string

		if refererUri != "" {
			path, err = nextFile(strippedRefererUri, sortOrder, filename, formats)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

		list := fileList(paths, filters, sortOrder, index, formats, encoder, errorChannel)

	loop:
		for timeout := time.After(timeout); ; {
			select {
			case <-timeout:
				break loop
			default:
			}

			if path != "" {
				break loop
			}

			path, err = newFile(list, sortOrder, filename, formats)
			switch {
			case path == "":
				noFiles(w, r)

				return
			case err != nil && err == ErrNoMediaFound:
				notFound(w, r, path)

				return
			case err != nil:
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

		queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

		newUrl := fmt.Sprintf("http://%s%s%s%s",
			r.Host,
			Prefix,
			preparePath(mediaPrefix, path),
			queryParams,
		)
		http.Redirect(w, r, newUrl, redirectStatusCode)
	}
}

func serveMedia(paths []string, index *fileIndex, filename *regexp.Regexp, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		filters := &filters{
			included: splitQueryParams(r.URL.Query().Get("include")),
			excluded: splitQueryParams(r.URL.Query().Get("exclude")),
		}

		sortOrder := sortOrder(r)

		path := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, Prefix), mediaPrefix)

		if runtime.GOOS == "windows" {
			path = strings.TrimPrefix(path, "/")
		}

		exists, err := fileExists(path)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}
		if !exists {
			notFound(w, r, path)

			return
		}

		format := formats.FileType(path)
		if format == nil {
			if Fallback {
				w.Header().Add("Content-Type", "application/octet-stream")

				_, refreshInterval := refreshInterval(r)

				// redirect to static url for file
				newUrl := fmt.Sprintf("http://%s%s%s%s",
					r.Host,
					Prefix,
					preparePath(sourcePrefix, path),
					generateQueryParams(filters, sortOrder, refreshInterval),
				)

				http.Redirect(w, r, newUrl, redirectStatusCode)

				return
			} else {
				notFound(w, r, path)

				return

			}
		}

		if !format.Validate(path) {
			notFound(w, r, path)

			return
		}

		mediaType := format.MediaType(filepath.Ext(path))

		fileUri := Prefix + generateFileUri(path)

		fileName := filepath.Base(path)

		w.Header().Add("Content-Type", "text/html")

		refreshTimer, refreshInterval := refreshInterval(r)

		queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

		rootUrl := Prefix + "/" + queryParams

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html class="bg" lang="en"><head>`)
		htmlBody.WriteString(faviconHtml)
		htmlBody.WriteString(fmt.Sprintf(`<style>%s</style>`, format.Css()))

		title, err := format.Title(rootUrl, fileUri, path, fileName, Prefix, mediaType)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}
		htmlBody.WriteString(title)
		htmlBody.WriteString(`</head><body>`)

		var first, last string

		if Index && sortOrder != "" {
			first, last, err = getRange(path, index, filename)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

		if Index && !DisableButtons && sortOrder != "" {
			paginated, err := paginate(path, first, last, queryParams, filename, formats)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}

			htmlBody.WriteString(paginated)
		}

		if refreshInterval != "0ms" {
			htmlBody.WriteString(refreshFunction(rootUrl, refreshTimer))
		}

		body, err := format.Body(rootUrl, fileUri, path, fileName, Prefix, mediaType)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}
		htmlBody.WriteString(body)

		htmlBody.WriteString(`</body></html>`)

		formattedPage := gohtml.Format(htmlBody.String())

		written, err := io.WriteString(w, formattedPage+"\n")
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		if format.Type() != "embed" {
			if Verbose {
				fmt.Printf("%s | SERVE: %s (%s) to %s in %s\n",
					startTime.Format(logDate),
					path,
					humanReadableSize(written),
					realIP(r),
					time.Since(startTime).Round(time.Microsecond),
				)
			}

			if Russian {
				err := kill(path, index)
				if err != nil {
					errorChannel <- err

					return
				}
			}
		}
	}
}

func serveVersion(errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		data := []byte(fmt.Sprintf("roulette v%s\n", ReleaseVersion))

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

		w.Header().Set("Content-Length", strconv.Itoa(len(data)))

		written, err := w.Write(data)
		if err != nil {
			errorChannel <- err

			return
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Version page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func redirectRoot() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		newUrl := fmt.Sprintf("http://%s%s",
			r.Host,
			Prefix,
		)

		http.Redirect(w, r, newUrl, redirectStatusCode)
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

	if Verbose {
		fmt.Printf("%s | START: roulette v%s\n",
			time.Now().Format(logDate),
			ReleaseVersion,
		)
	}

	bindHost, err := net.LookupHost(Bind)
	if err != nil {
		return err
	}

	bindAddr := net.ParseIP(bindHost[0])
	if bindAddr == nil {
		return errors.New("invalid bind address provided")
	}

	formats := make(types.Types)

	if Audio || All {
		formats.Add(audio.Format{})
	}

	if Code || All {
		formats.Add(code.Format{Fun: Fun, Theme: CodeTheme})
	}

	if Flash || All {
		formats.Add(flash.Format{})
	}

	if Text || All {
		formats.Add(text.Format{})
	}

	if Videos || All {
		formats.Add(video.Format{})
	}

	if Images || All {
		formats.Add(images.Format{DisableButtons: DisableButtons, Fun: Fun})
	}

	paths, err := validatePaths(args, formats)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return ErrNoMediaFound
	}

	listenHost := net.JoinHostPort(Bind, strconv.Itoa(Port))

	index := &fileIndex{
		mutex: &sync.RWMutex{},
		list:  []string{},
	}

	mux := httprouter.New()

	srv := &http.Server{
		Addr:         listenHost,
		Handler:      mux,
		IdleTimeout:  10 * time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Minute,
	}

	mux.PanicHandler = serverErrorHandler()

	errorChannel := make(chan error)

	go func() {
		for err := range errorChannel {
			switch {
			case ExitOnError:
				fmt.Printf("%s | FATAL: %v\n", time.Now().Format(logDate), err)
			case Debug && errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission):
				fmt.Printf("%s | DEBUG: %v\n", time.Now().Format(logDate), err)
			case errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission):
				continue
			default:
				fmt.Printf("%s | ERROR: %v\n", time.Now().Format(logDate), err)
			}
		}
	}()

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return err
	}

	filename := regexp.MustCompile(`(.+?)([0-9]*)(\..+)`)

	if !strings.HasSuffix(Prefix, "/") {
		Prefix = Prefix + "/"
	}

	mux.GET(Prefix, serveRoot(paths, index, filename, formats, encoder, errorChannel))

	Prefix = strings.TrimSuffix(Prefix, "/")

	if Prefix != "" {
		mux.GET("/", redirectRoot())
	}

	mux.GET(Prefix+"/favicons/*favicon", serveFavicons(errorChannel))

	mux.GET(Prefix+"/favicon.ico", serveFavicons(errorChannel))

	mux.GET(Prefix+mediaPrefix+"/*media", serveMedia(paths, index, filename, formats, errorChannel))

	mux.GET(Prefix+sourcePrefix+"/*static", serveStaticFile(paths, index, errorChannel))

	mux.GET(Prefix+"/version", serveVersion(errorChannel))

	quit := make(chan struct{})
	defer close(quit)

	if Index {
		mux.GET(Prefix+AdminPrefix+"/index/rebuild", serveIndexRebuild(args, index, formats, encoder, errorChannel))

		importIndex(paths, index, formats, encoder, errorChannel)

		if IndexInterval != "" {
			interval, err := time.ParseDuration(IndexInterval)
			if err != nil {
				return err
			}

			ticker := time.NewTicker(interval)

			go func() {
				for {
					select {
					case <-ticker.C:
						startTime := time.Now()

						rebuildIndex(args, index, formats, encoder, errorChannel)

						if Verbose {
							fmt.Printf("%s | INDEX: Automatic rebuild took %s\n",
								startTime.Format(logDate),
								time.Since(startTime).Round(time.Microsecond),
							)
						}
					case <-quit:
						ticker.Stop()

						return
					}
				}
			}()
		}
	}

	if Info {
		registerInfoHandlers(mux, args, index, formats, errorChannel)
	}

	if Profile {
		registerProfileHandlers(mux)
	}

	if Russian {
		fmt.Printf("WARNING! Files *will* be deleted after serving!\n\n")
	}

	if Verbose {
		fmt.Printf("%s | SERVE: Listening on http://%s%s/\n",
			time.Now().Format(logDate),
			listenHost,
			Prefix,
		)
	}

	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
