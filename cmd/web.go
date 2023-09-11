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
	"seedno.de/seednode/roulette/formats"
)

const (
	LogDate            string        = `2006-01-02T15:04:05.000-07:00`
	SourcePrefix       string        = `/source`
	MediaPrefix        string        = `/view`
	RedirectStatusCode int           = http.StatusSeeOther
	Timeout            time.Duration = 10 * time.Second
)

type Regexes struct {
	alphanumeric *regexp.Regexp
	filename     *regexp.Regexp
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
			err = os.Remove(filePath)
			if err != nil {
				fmt.Println(err)

				serverError(w, r, nil)

				return
			}

			if cache {
				index.Remove(filePath)
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

func serveRoot(paths []string, Regexes *Regexes, index *Index, registeredFormats *formats.SupportedFormats) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		refererUri, err := stripQueryParams(refererToUri(r.Referer()))
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		strippedRefererUri := strings.TrimPrefix(refererUri, MediaPrefix)

		filters := &Filters{
			includes: splitQueryParams(r.URL.Query().Get("include"), Regexes),
			excludes: splitQueryParams(r.URL.Query().Get("exclude"), Regexes),
		}

		sortOrder := SortOrder(r)

		_, refreshInterval := RefreshInterval(r)

		var filePath string

		if refererUri != "" {
			filePath, err = nextFile(strippedRefererUri, sortOrder, Regexes, registeredFormats)
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

			filePath, err = newFile(paths, filters, sortOrder, Regexes, index, registeredFormats)
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

func serveMedia(paths []string, Regexes *Regexes, index *Index, registeredFormats *formats.SupportedFormats) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filters := &Filters{
			includes: splitQueryParams(r.URL.Query().Get("include"), Regexes),
			excludes: splitQueryParams(r.URL.Query().Get("exclude"), Regexes),
		}

		sortOrder := SortOrder(r)

		filePath := strings.TrimPrefix(r.URL.Path, MediaPrefix)

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

		registered, fileType, mimeType, err := formats.FileType(filePath, registeredFormats)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		if !registered {
			notFound(w, r, filePath)

			return
		}

		fileUri := generateFileUri(filePath)

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
		htmlBody.WriteString((fileType.Title(queryParams, fileUri, filePath, fileName, mimeType)))
		htmlBody.WriteString(`</head><body>`)
		if refreshInterval != "0ms" {
			htmlBody.WriteString(fmt.Sprintf("<script>window.onload = function(){setInterval(function(){window.location.href = '/%s';}, %d);};</script>",
				queryParams,
				refreshTimer))
		}
		htmlBody.WriteString((fileType.Body(queryParams, fileUri, filePath, fileName, mimeType)))
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

	registeredFormats := &formats.SupportedFormats{}

	if audio {
		registeredFormats.Add(formats.RegisterAudioFormats())
	}

	if images {
		registeredFormats.Add(formats.RegisterImageFormats())
	}

	if videos {
		registeredFormats.Add(formats.RegisterVideoFormats())
	}

	paths, err := normalizePaths(args, registeredFormats)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		return ErrNoMediaFound
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

	mux.GET("/", serveRoot(paths, regexes, index, registeredFormats))

	mux.GET("/favicons/*favicon", serveFavicons())

	mux.GET("/favicon.ico", serveFavicons())

	mux.GET(MediaPrefix+"/*media", serveMedia(paths, regexes, index, registeredFormats))

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
			index.generateCache(args, registeredFormats)
		}

		mux.GET("/clear_cache", serveCacheClear(args, index, registeredFormats))
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
