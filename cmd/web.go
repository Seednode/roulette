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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	RedirectStatusCode int           = http.StatusSeeOther
	timeout            time.Duration = 10 * time.Second
)

func serveStaticFile(paths []string, cache *fileCache, errorChannel chan<- error) httprouter.Handle {
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

		w.Write(buf)

		fileSize := humanReadableSize(len(buf))

		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		if Russian && refererUri != "" {
			err = kill(filePath, cache)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

		if Verbose {
			fmt.Printf("%s | Serve: %s (%s) to %s in %s\n",
				startTime.Format(logDate),
				filePath,
				fileSize,
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveRoot(paths []string, regexes *regexes, cache *fileCache, formats *types.Types, errorChannel chan<- error) httprouter.Handle {
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

		var filePath string

		if refererUri != "" {
			filePath, err = nextFile(strippedRefererUri, sortOrder, regexes, formats)
			if err != nil {
				errorChannel <- err

				serverError(w, r, nil)

				return
			}
		}

	loop:
		for timeout := time.After(timeout); ; {
			select {
			case <-timeout:
				break loop
			default:
			}

			if filePath != "" {
				break loop
			}

			filePath, err = newFile(paths, filters, sortOrder, regexes, cache, formats)
			switch {
			case err != nil && err == ErrNoMediaFound:
				notFound(w, r, filePath)

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
			preparePath(filePath),
			queryParams,
		)
		http.Redirect(w, r, newUrl, RedirectStatusCode)
	}
}

func serveMedia(paths []string, regexes *regexes, cache *fileCache, formats *types.Types, errorChannel chan<- error) httprouter.Handle {
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
			serverError(w, r, nil)

			return
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
			htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '%s';}, %d);};</script>",
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

		_, err = io.WriteString(w, formattedPage)
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		if format.Type() != "embed" {
			if Verbose {
				fmt.Printf("%s | Serve: %s (%d) to %s in %s\n",
					startTime.Format(logDate),
					path,
					len(formattedPage),
					realIP(r),
					time.Since(startTime).Round(time.Microsecond),
				)
			}

			if Russian {
				kill(path, cache)
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

func register(mux *httprouter.Router, path string, handle httprouter.Handle) {
	mux.GET(path, handle)

	if Handlers {
		fmt.Printf("Registered handler for path %s\n", path)
	}
}

func redirectRoot() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		newUrl := fmt.Sprintf("http://%s%s",
			r.Host,
			Prefix,
		)

		http.Redirect(w, r, newUrl, RedirectStatusCode)
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

	bindHost, err := net.LookupHost(Bind)
	if err != nil {
		return err
	}

	bindAddr := net.ParseIP(bindHost[0])
	if bindAddr == nil {
		return errors.New("invalid bind address provided")
	}

	formats := &types.Types{
		Extensions: make(map[string]types.Type),
	}

	if Audio || All {
		formats.Add(audio.New())
	}

	if Code || All {
		formats.Add(code.New(CodeTheme))
	}

	if Flash || All {
		formats.Add(flash.New())
	}

	if Text || All {
		formats.Add(text.New())
	}

	if Videos || All {
		formats.Add(video.New())
	}

	// enable image support if no other flags are passed, to retain backwards compatibility
	// to be replaced with rootCmd.MarkFlagsOneRequired on next spf13/cobra update
	if Images || All || len(formats.Extensions) == 0 {
		formats.Add(images.New())
	}

	paths, err := validatePaths(args, formats)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return ErrNoMediaFound
	}

	cache := &fileCache{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	regexes := &regexes{
		filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		alphanumeric: regexp.MustCompile(`^[A-z0-9]*$`),
	}

	mux := httprouter.New()

	srv := &http.Server{
		Addr:         net.JoinHostPort(Bind, strconv.Itoa(int(Port))),
		Handler:      mux,
		IdleTimeout:  10 * time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Minute,
	}

	mux.PanicHandler = serverErrorHandler()

	if !strings.HasSuffix(Prefix, "/") {
		Prefix = Prefix + "/"
	}

	errorChannel := make(chan error)

	register(mux, Prefix, serveRoot(paths, regexes, cache, formats, errorChannel))

	Prefix = strings.TrimSuffix(Prefix, "/")

	if Prefix != "" {
		register(mux, "/", redirectRoot())
	}

	register(mux, Prefix+"/favicons/*favicon", serveFavicons(errorChannel))

	register(mux, Prefix+"/favicon.ico", serveFavicons(errorChannel))

	register(mux, Prefix+mediaPrefix+"/*media", serveMedia(paths, regexes, cache, formats, errorChannel))

	register(mux, Prefix+sourcePrefix+"/*static", serveStaticFile(paths, cache, errorChannel))

	register(mux, Prefix+"/version", serveVersion())

	if Cache {
		err = registerCacheHandlers(mux, args, cache, formats, errorChannel)
		if err != nil {
			return err
		}
	}

	if Info {
		registerInfoHandlers(mux, args, cache, formats, errorChannel)
	}

	if Profile {
		registerProfileHandlers(mux)
	}

	if Russian {
		fmt.Printf("WARNING! Files *will* be deleted after serving!\n\n")
	}

	go func() {
		for err := range errorChannel {
			fmt.Printf("%s | Error: %v\n", time.Now().Format(logDate), err)

			if ExitOnError {
				fmt.Printf("%s | Error: Shutting down...\n", time.Now().Format(logDate))

				srv.Shutdown(context.Background())
			}
		}
	}()

	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
