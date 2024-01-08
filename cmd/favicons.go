/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"bytes"
	"embed"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

//go:embed favicons/*
var favicons embed.FS

const (
	faviconHtml string = `<link rel="apple-touch-icon" sizes="180x180" href="/favicons/apple-touch-icon.png">
	<link rel="icon" type="image/png" sizes="32x32" href="/favicons/favicon-32x32.png">
	<link rel="icon" type="image/png" sizes="16x16" href="/favicons/favicon-16x16.png">
	<link rel="manifest" href="/favicons/site.webmanifest">
	<link rel="mask-icon" href="/favicons/safari-pinned-tab.svg" color="#5bbad5">
	<meta name="msapplication-TileColor" content="#da532c">
	<meta name="theme-color" content="#ffffff">`
)

func serveFavicons(errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fname := strings.TrimPrefix(r.URL.Path, "/")

		data, err := favicons.ReadFile(fname)
		if err != nil {
			return
		}

		err = w.Header().Write(bytes.NewBufferString("Content-Length: " + strconv.Itoa(len(data))))
		if err != nil {
			errorChannel <- err

			return
		}

		_, err = w.Write(data)
		if err != nil {
			errorChannel <- err

			return
		}
	}
}
