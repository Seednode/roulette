/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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

func preparePath(prefix, path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s/%s", prefix, filepath.ToSlash(path))
	}

	return prefix + path
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

func serveRoot(paths []string, regexes *regexes, index *fileIndex, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		strippedRefererUri := strings.TrimPrefix(refererUri, Prefix+mediaPrefix)

		filters := &filters{
			included: splitQueryParams(r.URL.Query().Get("include"), regexes),
			excluded: splitQueryParams(r.URL.Query().Get("exclude"), regexes),
		}

		sortOrder := sortOrder(r)

		_, refreshInterval := refreshInterval(r)

		var path string

		if refererUri != "" {
			path, err = nextFile(strippedRefererUri, sortOrder, regexes, formats)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

		list, err := fileList(paths, filters, sortOrder, index, formats)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

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

			path, err = newFile(list, sortOrder, regexes, formats)
			switch {
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

func serveMedia(paths []string, regexes *regexes, index *fileIndex, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filters := &filters{
			included: splitQueryParams(r.URL.Query().Get("include"), regexes),
			excluded: splitQueryParams(r.URL.Query().Get("exclude"), regexes),
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

		mimeType := format.MimeType(path)

		fileUri := Prefix + generateFileUri(path)

		fileName := filepath.Base(path)

		w.Header().Add("Content-Type", "text/html")

		refreshTimer, refreshInterval := refreshInterval(r)

		rootUrl := Prefix + "/" + generateQueryParams(filters, sortOrder, refreshInterval)

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html class="bg" lang="en"><head>`)
		htmlBody.WriteString(faviconHtml)
		htmlBody.WriteString(fmt.Sprintf(`<style>%s</style>`, format.Css()))

		title, err := format.Title(rootUrl, fileUri, path, fileName, Prefix, mimeType)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}
		htmlBody.WriteString(title)
		htmlBody.WriteString(`</head><body>`)
		if refreshInterval != "0ms" {
			htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){clear = setInterval(function(){window.location.href = '%s';}, %d); document.body.onkeyup = function(e) { if (e.key == \"\" || e.code == \"Space\" || e.keyCode == 32){clearInterval(clear)}}};</script>",
				rootUrl,
				refreshTimer))
		}

		body, err := format.Body(rootUrl, fileUri, path, fileName, Prefix, mimeType)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}
		htmlBody.WriteString(body)
		htmlBody.WriteString(`</body></html>`)

		startTime := time.Now()

		formattedPage := gohtml.Format(htmlBody.String())

		written, err := io.WriteString(w, formattedPage)
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
				kill(path, index)
			}
		}
	}
}

func serveVersion() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		data := []byte(fmt.Sprintf("roulette v%s\n", ReleaseVersion))

		w.Header().Write(bytes.NewBufferString("Content-Length: " + strconv.Itoa(len(data))))

		w.Write(data)
	}
}

func registerHandler(mux *httprouter.Router, path string, handle httprouter.Handle) {
	mux.GET(path, handle)

	if Handlers {
		fmt.Printf("%s | SERVE: Registered handler for %s\n",
			time.Now().Format(logDate),
			path,
		)
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
	log.SetFlags(0)

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
		formats.Add(audio.Format{Fun: Fun})
	}

	if Code || All {
		formats.Add(code.Format{Fun: Fun, Theme: CodeTheme})
	}

	if Flash || All {
		formats.Add(flash.Format{Fun: Fun})
	}

	if Text || All {
		formats.Add(text.Format{Fun: Fun})
	}

	if Videos || All {
		formats.Add(video.Format{Fun: Fun})
	}

	// enable image support if no other flags are passed, to retain backwards compatibility
	// to be replaced with rootCmd.MarkFlagsOneRequired on next spf13/cobra update
	if Images || All || len(formats) == 0 {
		formats.Add(images.Format{Fun: Fun})
	}

	paths, err := validatePaths(args, formats)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return ErrNoMediaFound
	}

	regexes := &regexes{
		filename:     regexp.MustCompile(`(.+?)([0-9]*)(\..+)`),
		alphanumeric: regexp.MustCompile(`^[A-z0-9]*$`),
	}

	if !strings.HasSuffix(Prefix, "/") {
		Prefix = Prefix + "/"
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

	registerHandler(mux, Prefix, serveRoot(paths, regexes, index, formats, errorChannel))

	Prefix = strings.TrimSuffix(Prefix, "/")

	if Prefix != "" {
		registerHandler(mux, "/", redirectRoot())
	}

	registerHandler(mux, Prefix+"/favicons/*favicon", serveFavicons())

	registerHandler(mux, Prefix+"/favicon.ico", serveFavicons())

	registerHandler(mux, Prefix+mediaPrefix+"/*media", serveMedia(paths, regexes, index, formats, errorChannel))

	registerHandler(mux, Prefix+sourcePrefix+"/*static", serveStaticFile(paths, index, errorChannel))

	registerHandler(mux, Prefix+"/version", serveVersion())

	if Index {
		err = registerIndexHandlers(mux, args, index, formats, errorChannel)
		if err != nil {
			return err
		}

		err = importIndex(paths, index, formats)
		if err != nil {
			return err
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

	go func() {
		for err := range errorChannel {
			fmt.Printf("%s | ERROR: %v\n", time.Now().Format(logDate), err)

			if ExitOnError {
				fmt.Printf("%s | ERROR: Shutting down...\n", time.Now().Format(logDate))

				srv.Shutdown(context.Background())
			}
		}
	}()

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
