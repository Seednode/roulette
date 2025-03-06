/*
Copyright © 2025 Seednode <seednode@seedno.de>
*/

package main

import (
	"embed"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

//go:embed favicons/*
var favicons embed.FS

func getFavicon() string {
	return `<link rel="apple-touch-icon" sizes="180x180" href="/favicons/apple-touch-icon.webp">
	<link rel="icon" type="image/webp" sizes="32x32" href="/favicons/favicon-32x32.webp">
	<link rel="icon" type="image/webp" sizes="16x16" href="/favicons/favicon-16x16.webp">
	<link rel="manifest" href="/favicons/site.webmanifest" crossorigin="use-credentials">
	<link rel="mask-icon" href="/favicons/safari-pinned-tab.svg" color="#5bbad5">
	<meta name="msapplication-TileColor" content="#da532c">
	<meta name="theme-color" content="#ffffff">`
}

func serveFavicons(errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fname := strings.TrimPrefix(r.URL.Path, "/")

		data, err := favicons.ReadFile(fname)
		if err != nil {
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(data)))

		_, err = w.Write(data)
		if err != nil {
			errorChannel <- err

			return
		}
	}
}
