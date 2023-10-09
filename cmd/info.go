/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/yosssi/gohtml"
	"seedno.de/seednode/roulette/types"
)

func paginate(page int, fileCount int, ending bool) string {
	var html strings.Builder

	var firstPage int = 1
	var lastPage int

	if fileCount%PageLength == 0 {
		lastPage = fileCount / PageLength
	} else {
		lastPage = (fileCount / PageLength) + 1
	}

	var prevStatus, nextStatus string = "", ""

	if page <= 1 {
		prevStatus = " disabled"
	}

	if page >= lastPage {
		nextStatus = " disabled"
	}

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1
	}

	nextPage := page + 1
	if nextPage > lastPage {
		nextPage = fileCount / PageLength
	}

	if ending {
		html.WriteString("<tr><td style=\"border-bottom:none;\">")
	} else {
		html.WriteString("<tr><td>")
	}

	html.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\">First</button>",
		firstPage))

	html.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\"%s>Prev</button>",
		prevPage,
		prevStatus))

	html.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\"%s>Next</button>",
		nextPage,
		nextStatus))

	html.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\">Last</button>",
		lastPage))

	html.WriteString("</td></tr>\n")

	return html.String()
}

func serveIndexHtml(args []string, index *fileIndex, shouldPaginate bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/html")

		startTime := time.Now()

		indexDump := index.List()

		fileCount := len(indexDump)

		var startIndex, stopIndex int

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page <= 0 {
			startIndex = 0
			stopIndex = fileCount
		} else {
			startIndex = ((page - 1) * PageLength)
			stopIndex = (startIndex + PageLength)
		}

		if startIndex > (fileCount - 1) {
			indexDump = []string{}
		}

		if stopIndex > fileCount {
			stopIndex = fileCount
		}

		sort.SliceStable(indexDump, func(p, q int) bool {
			return strings.ToLower(indexDump[p]) < strings.ToLower(indexDump[q])
		})

		var htmlBody strings.Builder
		htmlBody.WriteString(`<!DOCTYPE html><html lang="en"><head>`)
		htmlBody.WriteString(faviconHtml)
		htmlBody.WriteString(`<style>a{text-decoration:none;height:100%;width:100%;color:inherit;cursor:pointer}`)
		htmlBody.WriteString(`table,td,tr{border:none;}td{border-bottom:1px solid black;}td{white-space:nowrap;padding:.5em}</style>`)
		htmlBody.WriteString(fmt.Sprintf("<title>Index contains %d files</title></head><body><table>", fileCount))

		if shouldPaginate {
			htmlBody.WriteString(paginate(page, fileCount, false))
		}

		if len(indexDump) > 0 {
			for _, v := range indexDump[startIndex:stopIndex] {
				var shouldSort = ""

				if Sorting {
					shouldSort = "?sort=asc"
				}
				htmlBody.WriteString(fmt.Sprintf("<tr><td><a href=\"%s%s%s%s\">%s</a></td></tr>\n", Prefix, mediaPrefix, v, shouldSort, v))
			}
		}

		if shouldPaginate {
			htmlBody.WriteString(paginate(page, fileCount, true))
		}

		htmlBody.WriteString(`</table></body></html>`)

		length, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
		if err != nil {
			return
		}

		if Verbose {
			fmt.Printf("%s | SERVE: HTML index page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(length),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveIndexJson(args []string, index *fileIndex, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		indexedFiles := index.List()

		fileCount := len(indexedFiles)

		sort.SliceStable(indexedFiles, func(p, q int) bool {
			return strings.ToLower(indexedFiles[p]) < strings.ToLower(indexedFiles[q])
		})

		var startIndex, stopIndex int

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page <= 0 {
			startIndex = 0
			stopIndex = fileCount
		} else {
			startIndex = ((page - 1) * PageLength)
			stopIndex = (startIndex + PageLength)
		}

		if startIndex > (fileCount - 1) {
			indexedFiles = []string{}
		}

		if stopIndex > fileCount {
			stopIndex = fileCount
		}

		response, err := json.MarshalIndent(indexedFiles[startIndex:stopIndex], "", "    ")
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		written, err := w.Write(response)
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: JSON index page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveAvailableExtensions(errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		written, err := w.Write([]byte(types.SupportedFormats.GetExtensions()))
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Available extension list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveEnabledExtensions(formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		written, err := w.Write([]byte(formats.GetExtensions()))
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Registered extension list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveAvailableMediaTypes(errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		written, err := w.Write([]byte(types.SupportedFormats.GetMediaTypes()))
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Available media type list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveEnabledMediaTypes(formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		written, err := w.Write([]byte(formats.GetMediaTypes()))
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Registered media type list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func registerInfoHandlers(mux *httprouter.Router, args []string, index *fileIndex, formats types.Types, errorChannel chan<- error) {
	if Index {
		registerHandler(mux, Prefix+"/index/html", serveIndexHtml(args, index, false))
		if PageLength != 0 {
			registerHandler(mux, Prefix+"/index/html/:page", serveIndexHtml(args, index, true))
		}

		registerHandler(mux, Prefix+"/index/json", serveIndexJson(args, index, errorChannel))
		if PageLength != 0 {
			registerHandler(mux, Prefix+"/index/json/:page", serveIndexJson(args, index, errorChannel))
		}
	}

	registerHandler(mux, Prefix+"/extensions/available", serveAvailableExtensions(errorChannel))
	registerHandler(mux, Prefix+"/extensions/enabled", serveEnabledExtensions(formats, errorChannel))
	registerHandler(mux, Prefix+"/types/available", serveAvailableMediaTypes(errorChannel))
	registerHandler(mux, Prefix+"/types/enabled", serveEnabledMediaTypes(formats, errorChannel))
}
