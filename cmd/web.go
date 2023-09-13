/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"net/http/pprof"

	"github.com/julienschmidt/httprouter"
	"github.com/yosssi/gohtml"
	"seedno.de/seednode/roulette/types"
	"seedno.de/seednode/roulette/types/audio"
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

func serveStaticFile(paths []string, stats *ServeStats, cache *fileCache) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		path := strings.TrimPrefix(r.URL.Path, sourcePrefix)

		prefixedFilePath, err := stripQueryParams(path)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		filePath, err := filepath.EvalSymlinks(strings.TrimPrefix(prefixedFilePath, sourcePrefix))
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

		if Russian {
			err = os.Remove(filePath)
			if err != nil {
				fmt.Println(err)

				serverError(w, r, nil)

				return
			}

			if Cache {
				cache.remove(filePath)
			}
		}

		if Verbose {
			fmt.Printf("%s | Served %s (%s) to %s in %s\n",
				startTime.Format(logDate),
				filePath,
				fileSize,
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}

		if Statistics {
			stats.incrementCounter(filePath, startTime, fileSize)
		}

	}
}

func serveRoot(paths []string, regexes *regexes, cache *fileCache, formats *types.Types) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		strippedRefererUri := strings.TrimPrefix(refererUri, mediaPrefix)

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
				fmt.Println(err)

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

func serveMedia(paths []string, regexes *regexes, formats *types.Types) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filters := &filters{
			included: splitQueryParams(r.URL.Query().Get("include"), regexes),
			excluded: splitQueryParams(r.URL.Query().Get("exclude"), regexes),
		}

		sortOrder := sortOrder(r)

		path := strings.TrimPrefix(r.URL.Path, mediaPrefix)

		if runtime.GOOS == "windows" {
			path = strings.TrimPrefix(path, "/")
		}

		exists, err := fileExists(path)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}
		if !exists {
			notFound(w, r, path)

			return
		}

		registered, fileType, mimeType, err := formats.FileType(path)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		if !registered {
			notFound(w, r, path)

			return
		}

		fileUri := generateFileUri(path)

		fileName := filepath.Base(path)

		w.Header().Add("Content-Type", "text/html")

		refreshTimer, refreshInterval := refreshInterval(r)

		queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
		htmlBody.WriteString(faviconHtml)
		htmlBody.WriteString(fmt.Sprintf(`<style>%s</style>`, fileType.Css()))
		htmlBody.WriteString((fileType.Title(queryParams, fileUri, path, fileName, mimeType)))
		htmlBody.WriteString(`</head><body>`)
		if refreshInterval != "0ms" {
			htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%s';}, %d);};</script>",
				queryParams,
				refreshTimer))
		}
		htmlBody.WriteString((fileType.Body(queryParams, fileUri, path, fileName, mimeType)))
		htmlBody.WriteString(`</body></html>`)

		_, err = io.WriteString(w, gohtml.Format(htmlBody.String()))
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
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

	mux := httprouter.New()

	formats := &types.Types{
		Extensions: make(map[string]string),
		MimeTypes:  make(map[string]types.Type),
	}

	if Audio || All {
		formats.Add(audio.New())
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

	paths, err := normalizePaths(args, formats)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return ErrNoMediaFound
	}

	if Russian {
		fmt.Printf("WARNING! Files *will* be deleted after serving!\n\n")
	}

	cache := &fileCache{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	regexes := &regexes{
		filename:     regexp.MustCompile(`(.+)([0-9]{3})(\..+)`),
		alphanumeric: regexp.MustCompile(`^[A-z0-9]*$`),
	}

	srv := &http.Server{
		Addr:         net.JoinHostPort(Bind, strconv.Itoa(int(Port))),
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

	mux.GET("/", serveRoot(paths, regexes, cache, formats))

	mux.GET("/favicons/*favicon", serveFavicons())

	mux.GET("/favicon.ico", serveFavicons())

	mux.GET(mediaPrefix+"/*media", serveMedia(paths, regexes, formats))

	mux.GET(sourcePrefix+"/*static", serveStaticFile(paths, stats, cache))

	mux.GET("/version", serveVersion())

	if Cache {
		skipIndex := false

		if CacheFile != "" {
			err := cache.Import(CacheFile)
			if err == nil {
				skipIndex = true
			}
		}

		if !skipIndex {
			cache.generate(args, formats)
		}

		mux.GET("/clear_cache", serveCacheClear(args, cache, formats))
	}

	if Info {
		if Cache {
			mux.GET("/html/", serveIndexHtml(args, cache, false))
			if PageLength != 0 {
				mux.GET("/html/:page", serveIndexHtml(args, cache, true))
			}

			mux.GET("/json", serveIndexJson(args, cache))
			if PageLength != 0 {
				mux.GET("/json/:page", serveIndexJson(args, cache))
			}
		}

		mux.GET("/available_extensions", serveAvailableExtensions())
		mux.GET("/enabled_extensions", serveEnabledExtensions(formats))
		mux.GET("/available_mime_types", serveAvailableMimeTypes())
		mux.GET("/enabled_mime_types", serveEnabledMimeTypes(formats))
	}

	if Profile {
		mux.HandlerFunc("GET", "/debug/pprof/", pprof.Index)
		mux.HandlerFunc("GET", "/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandlerFunc("GET", "/debug/pprof/profile", pprof.Profile)
		mux.HandlerFunc("GET", "/debug/pprof/symbol", pprof.Symbol)
		mux.HandlerFunc("GET", "/debug/pprof/trace", pprof.Trace)
	}

	if Statistics {
		if StatisticsFile != "" {
			stats.Import(StatisticsFile)

			gracefulShutdown := make(chan os.Signal, 1)
			signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-gracefulShutdown

				stats.Export(StatisticsFile)

				os.Exit(0)
			}()
		}

		mux.GET("/stats", serveStats(args, stats))
		if PageLength != 0 {
			mux.GET("/stats/:page", serveStats(args, stats))
		}
	}

	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
