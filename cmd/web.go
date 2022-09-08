/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

func generateHeader(fileName string) string {
	htmlHeader := `<html lang="en">
  <head>
    <style>img{max-width:100%;max-height:97vh;height:auto;}</style>
	<title>`
	htmlHeader += fileName
	htmlHeader += `</title>
  </head>
  <body>
`

	return htmlHeader
}

func generateFooter() string {
	htmlFooter := `  </body>
</html>`

	return htmlFooter
}

func generatePageHtml(w http.ResponseWriter, paths []string, output []string) {
	fileList, err := getFileList(paths)
	if err != nil {
		panic(err)
	}

	fileName, filePath := pickFile(fileList)

	w.Header().Add("Content-Type", "text/html")

	_, err = io.WriteString(w, generateHeader(fileName))
	if err != nil {
		fmt.Println(err)
	}

	htmlBody := `    <a href="/"><img src="`
	htmlBody += filePath
	htmlBody += `"></img></a>
`
	_, err = io.WriteString(w, htmlBody)

	_, err = io.WriteString(w, generateFooter())
	if err != nil {
		fmt.Println(err)
	}
}

func servePageHandler(paths []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var output []string

		request := r.RequestURI

		if r.RequestURI == "/" {
			generatePageHtml(w, paths, output)
		} else {
			f, err := url.QueryUnescape(request)
			if err != nil {
				log.Fatal(err)
				return
			}
			buf, err := os.ReadFile(f)
			if err != nil {
				panic(err)
			}
			w.Write(buf)
		}
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage(args []string) {
	defer HandleExit()

	paths := normalizePaths(args)

	http.HandleFunc("/", servePageHandler(paths))
	http.HandleFunc("/favicon.ico", doNothing)

	port := strconv.Itoa(Port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
