/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yosssi/gohtml"
)

var (
	ErrInvalidPort      = errors.New("listen port must be an integer between 1 and 65535 inclusive")
	ErrInvalidScanCount = errors.New("scan count must be a positive integer")
	ErrNoMediaFound     = errors.New("no supported media formats found which match all criteria")
)

func newErrorPage(title, body string) string {
	var htmlBody strings.Builder

	htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
	htmlBody.WriteString(faviconHtml)
	htmlBody.WriteString(`<style>a{display:block;height:100%;width:100%;text-decoration:none;color:inherit;cursor:auto;}</style>`)
	htmlBody.WriteString(fmt.Sprintf("<title>%s</title></head>", title))
	htmlBody.WriteString(fmt.Sprintf("<body><a href=\"/\">%s</a></body></html>", body))

	return htmlBody.String()
}

func notFound(w http.ResponseWriter, r *http.Request, path string) error {
	startTime := time.Now()

	if Verbose {
		fmt.Printf("%s | Error: Unavailable file %s requested by %s\n",
			startTime.Format(logDate),
			path,
			r.RemoteAddr,
		)
	}

	w.WriteHeader(http.StatusNotFound)
	w.Header().Add("Content-Type", "text/html")

	_, err := io.WriteString(w, gohtml.Format(newErrorPage("Not Found", "404 Page not found")))
	if err != nil {
		return err
	}

	return nil
}

func serverError(w http.ResponseWriter, r *http.Request, i interface{}) {
	startTime := time.Now()

	if Verbose {
		fmt.Printf("%s | Error: Invalid request for %s from %s\n",
			startTime.Format(logDate),
			r.URL.Path,
			r.RemoteAddr,
		)
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Add("Content-Type", "text/html")

	io.WriteString(w, gohtml.Format(newErrorPage("Server Error", "500 Internal Server Error")))
}

func serverErrorHandler() func(http.ResponseWriter, *http.Request, interface{}) {
	return serverError
}
