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
)

const (
	LogDate            string        = `2006-01-02T15:04:05.000-07:00`
	SourcePrefix       string        = `/source`
	MediaPrefix        string        = `/view`
	RedirectStatusCode int           = http.StatusSeeOther
	Timeout            time.Duration = 10 * time.Second
)

func serveStaticFile(paths []string, stats *ServeStats, index *FileIndex) httprouter.Handle {
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

		if Russian {
			err = os.Remove(filePath)
			if err != nil {
				fmt.Println(err)

				serverError(w, r, nil)

				return
			}

			if Cache {
				index.Remove(filePath)
			}
		}

		if Verbose {
			fmt.Printf("%s | Served %s (%s) to %s in %s\n",
				startTime.Format(LogDate),
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

func serveRoot(paths []string, Regexes *Regexes, index *FileIndex, formats *types.Types) httprouter.Handle {
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

		_, refreshInterval := refreshInterval(r)

		var filePath string

		if refererUri != "" {
			filePath, err = nextFile(strippedRefererUri, sortOrder, Regexes, formats)
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

			filePath, err = newFile(paths, filters, sortOrder, Regexes, index, formats)
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

func serveMedia(paths []string, Regexes *Regexes, index *FileIndex, formats *types.Types) httprouter.Handle {
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

		registered, fileType, mimeType, err := types.FileType(filePath, formats)
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

		refreshTimer, refreshInterval := refreshInterval(r)

		queryParams := generateQueryParams(filters, sortOrder, refreshInterval)

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
		htmlBody.WriteString(FaviconHtml)
		htmlBody.WriteString(fmt.Sprintf(`<style>%s</style>`, fileType.Css()))
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
		Extensions: make(map[string]*types.Type),
		MimeTypes:  make(map[string]*types.Type),
	}

	if Audio || All {
		formats.Add(types.RegisterAudio())
	}

	if Flash || All {
		formats.Add(types.RegisterFlash())
	}

	if Images || All {
		formats.Add(types.RegisterImages())
	}

	if Text || All {
		formats.Add(types.RegisterText())
	}

	if Videos || All {
		formats.Add(types.RegisterVideos())
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

	index := &FileIndex{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	regexes := &Regexes{
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

	mux.GET("/", serveRoot(paths, regexes, index, formats))

	mux.GET("/favicons/*favicon", serveFavicons())

	mux.GET("/favicon.ico", serveFavicons())

	mux.GET(MediaPrefix+"/*media", serveMedia(paths, regexes, index, formats))

	mux.GET(SourcePrefix+"/*static", serveStaticFile(paths, stats, index))

	mux.GET("/version", serveVersion())

	if Cache {
		skipIndex := false

		if CacheFile != "" {
			err := index.Import(CacheFile)
			if err == nil {
				skipIndex = true
			}
		}

		if !skipIndex {
			index.generateCache(args, formats)
		}

		mux.GET("/clear_cache", serveCacheClear(args, index, formats))
	}

	if Index {
		mux.GET("/html/", serveIndexHtml(args, index, false))
		if PageLength != 0 {
			mux.GET("/html/:page", serveIndexHtml(args, index, true))
		}

		mux.GET("/json", serveIndexJson(args, index))
		if PageLength != 0 {
			mux.GET("/json/:page", serveIndexJson(args, index))
		}
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
