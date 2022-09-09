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
	"strconv"
	"strings"
)

func generatePageHtml(w http.ResponseWriter, paths []string) error {
	fileList, err := getFileList(paths)
	if err != nil {
		return err
	}

	fileName, filePath := pickFile(fileList)

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
`
	_, err = io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func statusNotFound(w http.ResponseWriter, filePath string) error {
	fmt.Println("Client requested non-existent file " + filePath + ".")

	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "txt/plain")
	htmlBody := "File not found."
	_, err := io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func statusForbidden(w http.ResponseWriter, filePath string) error {
	fmt.Println("Client requested forbidden file " + filePath + ".")

	w.WriteHeader(http.StatusForbidden)
	w.Header().Set("Content-Type", "txt/plain")
	htmlBody := "Access denied."
	_, err := io.WriteString(w, htmlBody)
	if err != nil {
		return err
	}

	return nil
}

func serveStaticFile(w http.ResponseWriter, request string, paths []string) error {
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
		err := statusNotFound(w, filePath)
		if err != nil {
			return err
		}

		return nil
	}

	_, err = os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		err := statusNotFound(w, filePath)
		if err != nil {
			return err
		}

		return nil
	} else if !errors.Is(err, os.ErrNotExist) && err != nil {
		return err
	}

	buf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	w.Write(buf)

	return nil
}

func servePageHandler(paths []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/" {
			err := generatePageHtml(w, paths)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			err := serveStaticFile(w, r.RequestURI, paths)
			if err != nil {
				log.Fatal(err)
			}
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
