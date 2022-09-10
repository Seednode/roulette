/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func generatePageHtml(w http.ResponseWriter, r http.Request, paths []string) error {
	fileList, err := getFileList(paths)
	if err != nil {
		return err
	}

	fileName, filePath, err := pickFile(fileList)
	if err != nil {
		http.NotFound(w, &r)
		return nil
	}

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
	htmlBody += filePath
	htmlBody += `"></img></a>
  </body>
</html>`

	_, err = io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func serveStaticFile(w http.ResponseWriter, r http.Request, paths []string) error {
	request := r.RequestURI

	filePath, err := url.QueryUnescape(request)
	if err != nil {
		return err
	}

	var matchesPrefix = false

	for i := 0; i < len(paths); i++ {
		if strings.HasPrefix(filePath, paths[i]) {
			matchesPrefix = true
		}
	}

	if matchesPrefix == false {
		if Verbose {
			log.Println("Failed to serve file outside specified path(s): " + filePath)
		}

		http.NotFound(w, &r)

		return nil
	}

	_, err = os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		if Verbose {
			log.Println("Failed to serve non-existent file: " + filePath)
		}

		http.NotFound(w, &r)

		return nil
	} else if !errors.Is(err, os.ErrNotExist) && err != nil {
		return err
	}

	buf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	w.Write(buf)

	if Verbose {
		log.Println("Served file: " + filePath)
	}

	return nil
}

func servePageHandler(paths []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/" {
			err := generatePageHtml(w, *r, paths)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			err := serveStaticFile(w, *r, paths)
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

	http.HandleFunc("/", servePageHandler(paths))
	http.HandleFunc("/favicon.ico", doNothing)

	port := strconv.Itoa(Port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
