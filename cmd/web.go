/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const LOGDATE string = "2006-01-02T15:04:05.000000000-07:00"

const PREFIX string = "/src"

func refererToUri(referer string) string {
	parts := strings.SplitAfterN(referer, "/", 4)

	if len(parts) < 4 {
		return ""
	}

	return "/" + parts[3]
}

func serveHtml(w http.ResponseWriter, r http.Request, filePath string) error {
	fileName := filepath.Base(filePath)

	w.Header().Add("Content-Type", "text/html")

	htmlBody := `<html lang="en">
  <head>
    <style>img{max-width:100%;max-height:97vh;height:auto;}</style>
	<title>`
	htmlBody += fileName
	htmlBody += `</title>
  </head>
  <body>
    <a href="/"><img src="`
	htmlBody += PREFIX + filePath
	htmlBody += `"></img></a>
  </body>
</html>`

	_, err := io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func serveStaticFile(w http.ResponseWriter, r http.Request, paths []string) error {
	prefixedFilePath, err := url.QueryUnescape(r.RequestURI)
	if err != nil {
		return err
	}

	filePath := strings.TrimPrefix(prefixedFilePath, PREFIX)

	var matchesPrefix = false
	for i := 0; i < len(paths); i++ {
		if strings.HasPrefix(filePath, paths[i]) {
			matchesPrefix = true
		}
	}
	if matchesPrefix == false {
		if Verbose {
			fmt.Printf("%v Failed to serve file outside specified path(s): %v", time.Now().Format(LOGDATE), filePath)
		}

		http.NotFound(w, &r)

		return nil
	}

	_, err = os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		if Verbose {
			fmt.Printf("%v Failed to serve non-existent file: %v", time.Now().Format(LOGDATE), filePath)
		}

		http.NotFound(w, &r)

		return nil
	} else if !errors.Is(err, os.ErrNotExist) && err != nil {
		return err
	}

	var startTime time.Time
	if Verbose {
		startTime = time.Now()
		fmt.Printf("%v Serving file: %v", startTime.Format(LOGDATE), filePath)
	}

	buf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	w.Write(buf)

	if Verbose {
		fmt.Printf(" (Finished in %v)\n", time.Now().Sub(startTime).Round(time.Microsecond))
	}

	return nil
}

func serveStaticFileHandler(paths []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := serveStaticFile(w, *r, paths)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func serveHtmlHandler(paths []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refererUri := refererToUri(r.Referer())

		switch {
		case r.RequestURI == "/" && Successive && refererUri != "":
			query, err := url.QueryUnescape(refererUri)
			if err != nil {
				log.Fatal(err)
			}

			filePath, err := getNextFile(query)
			if err != nil {
				log.Fatal(err)
			}

			if filePath == "" {
				filePath, err := pickFile(paths)
				if err != nil {
					log.Fatal(err)
				}

				filePath, err = getFirstFile(filePath)
				if err != nil {
					log.Fatal(err)
				}
			}

			newUrl := r.URL.Host + filePath
			http.Redirect(w, r, newUrl, http.StatusSeeOther)
		case r.RequestURI == "/" && Successive && refererUri == "":
			filePath, err := pickFile(paths)
			if err != nil {
				log.Fatal(err)
			}

			filePath, err = getFirstFile(filePath)
			if err != nil {
				log.Fatal(err)
			}

			newUrl := r.URL.Host + filePath
			http.Redirect(w, r, newUrl, http.StatusSeeOther)
		case r.RequestURI == "/":
			filePath, err := pickFile(paths)
			if err != nil {
				log.Fatal(err)
			}

			newUrl := r.URL.Host + filePath
			http.Redirect(w, r, newUrl, http.StatusSeeOther)
		default:
			filePath, err := url.QueryUnescape(r.RequestURI)
			if err != nil {
				log.Fatal(err)
			}

			isImage, err := checkIfImage(filePath)
			if err != nil {
				log.Fatal(err)
			}
			if !isImage {
				http.NotFound(w, r)
			}

			err = serveHtml(w, *r, filePath)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage(args []string) {
	paths, err := normalizePaths(args)
	if err != nil {
		log.Fatal(err)
	}

	for _, i := range paths {
		fmt.Println("Paths: " + i)
	}

	http.HandleFunc("/", serveHtmlHandler(paths))
	http.Handle(PREFIX+"/", http.StripPrefix(PREFIX, serveStaticFileHandler(paths)))
	http.HandleFunc("/favicon.ico", doNothing)

	port := strconv.Itoa(Port)

	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
