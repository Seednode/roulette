/*
Copyright © 2025 Seednode <seednode@seedno.de>
*/

package main

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

	"github.com/Seednode/roulette/types"
	"github.com/Seednode/roulette/types/audio"
	"github.com/Seednode/roulette/types/code"
	"github.com/Seednode/roulette/types/flash"
	"github.com/Seednode/roulette/types/images"
	"github.com/Seednode/roulette/types/text"
	"github.com/Seednode/roulette/types/video"
	"github.com/julienschmidt/httprouter"
)

const (
	logDate            string        = `2006-01-02T15:04:05.000-07:00`
	sourcePrefix       string        = `/source`
	mediaPrefix        string        = `/view`
	redirectStatusCode int           = http.StatusSeeOther
	timeout            time.Duration = 10 * time.Second
)

func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
	w.Header().Set("Permissions-Policy", "geolocation=(), midi=(), sync-xhr=(), microphone=(), camera=(), magnetometer=(), gyroscope=(), fullscreen=(), payment=()")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("X-Xss-Protection", "1; mode=block")
}

func newPage(title, body string) string {
	var htmlBody strings.Builder

	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(getFavicon())
	htmlBody.WriteString(`<style>`)
	htmlBody.WriteString(`html,body,a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}</style>`)
	htmlBody.WriteString(fmt.Sprintf("<title>%s</title></head>", title))
	htmlBody.WriteString(fmt.Sprintf("<body><a href=\"/\">%s</a></body></html>", body))

	return htmlBody.String()
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

func serveRoot(paths []string, index *fileIndex, filename *regexp.Regexp, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		strippedRefererUri := strings.TrimPrefix(refererUri, Prefix+mediaPrefix)

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

		list := fileList(paths, index, formats, errorChannel)

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
				startTime := time.Now()

				w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

				w.Write([]byte("No files found in the specified path(s).\n"))

				if Verbose {
					fmt.Printf("%s | SERVE: Empty path notification to %s\n",
						startTime.Format(logDate),
						r.RemoteAddr,
					)
				}

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

		queryParams := generateQueryParams(sortOrder, refreshInterval)

		newUrl := fmt.Sprintf("%s://%s%s%s%s",
			Scheme,
			r.Host,
			Prefix,
			preparePath(mediaPrefix, path),
			queryParams,
		)
		http.Redirect(w, r, newUrl, redirectStatusCode)
	}
}

func serveMedia(index *fileIndex, filename *regexp.Regexp, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		securityHeaders(w)

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
				newUrl := fmt.Sprintf("%s://%s%s%s%s",
					Scheme,
					r.Host,
					Prefix,
					preparePath(sourcePrefix, path),
					generateQueryParams(sortOrder, refreshInterval),
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

		queryParams := generateQueryParams(sortOrder, refreshInterval)

		rootUrl := Prefix + "/" + queryParams

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html class="bg" lang="en"><head>`)
		htmlBody.WriteString(getFavicon())
		htmlBody.WriteString(fmt.Sprintf(`<style>%s</style>`, format.CSS()))

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

		if Index && !NoButtons && sortOrder != "" {
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

		formattedPage := htmlBody.String()

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

		data := fmt.Appendf(nil, "roulette v%s\n", ReleaseVersion)

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

		securityHeaders(w)

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
		newUrl := fmt.Sprintf("%s://%s%s",
			Scheme,
			r.Host,
			Prefix,
		)

		http.Redirect(w, r, newUrl, redirectStatusCode)
	}
}

func ServePage(args []string) error {
	var err error

	timeZone := os.Getenv("TZ")
	if timeZone != "" {
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
		formats.Add(images.Format{NoButtons: NoButtons, Fun: Fun})
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
			case ErrorExit:
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

	filename := regexp.MustCompile(`(.+?)([0-9]*)(\..+)`)

	if !strings.HasSuffix(Prefix, "/") {
			Prefix = Prefix + "/"
				}

					mux.GET(Prefix, serveRoot(paths, index, filename, formats, errorChannel))

						Prefix = strings.TrimSuffix(Prefix, "/")

							if Prefix != "" {
									mux.GET("/", redirectRoot())
										}

	mux.GET(Prefix+"/favicons/*favicon", serveFavicons(errorChannel))

	mux.GET(Prefix+"/favicon.webp", serveFavicons(errorChannel))

	mux.GET(Prefix+mediaPrefix+"/*media", serveMedia(index, filename, formats, errorChannel))

	mux.GET(Prefix+sourcePrefix+"/*static", serveStaticFile(paths, index, errorChannel))

	mux.GET(Prefix+"/version", serveVersion(errorChannel))

	quit := make(chan struct{})
	defer close(quit)

	if API {
		registerAPIHandlers(mux, paths, index, formats, errorChannel)
	}

	if Index {
		importIndex(paths, index, formats, errorChannel)

		if IndexInterval != "" {
			registerIndexInterval(paths, index, formats, quit, errorChannel)
		}
	}

	if Profile {
		registerProfileHandlers(mux)
	}

	if Russian {
		fmt.Printf("WARNING! Files *will* be deleted after serving!\n\n")
	}

	if Verbose {
		if TLSKey != "" && TLSCert != "" {
			fmt.Printf("%s | SERVE: Listening on %s://%s/\n",
				time.Now().Format(logDate),
				Scheme,
				srv.Addr)

			err = srv.ListenAndServeTLS(TLSCert, TLSKey)
		} else {
			fmt.Printf("%s | SERVE: Listening on %s://%s/\n",
				time.Now().Format(logDate),
				Scheme,
				srv.Addr)

			err = srv.ListenAndServe()
		}
	}

	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
