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

func serveIndexHtml(args []string, cache *fileCache, paginate bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/html")

		startTime := time.Now()

		indexDump := cache.List()

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
		htmlBody.WriteString(`table,td,tr{border:1px solid black;border-collapse:collapse}td{white-space:nowrap;padding:.5em}</style>`)
		htmlBody.WriteString(fmt.Sprintf("<title>Index contains %d files</title></head><body><table>", fileCount))
		if len(indexDump) > 0 {
			for _, v := range indexDump[startIndex:stopIndex] {
				var shouldSort = ""

				if Sorting {
					shouldSort = "?sort=asc"
				}
				htmlBody.WriteString(fmt.Sprintf("<tr><td><a href=\"%s%s%s%s\">%s</a></td></tr>\n", Prefix, mediaPrefix, v, shouldSort, v))
			}
		}
		if PageLength != 0 {
			var firstPage int = 1
			var lastPage int

			if fileCount%PageLength == 0 {
				lastPage = fileCount / PageLength
			} else {
				lastPage = (fileCount / PageLength) + 1
			}

			if paginate {
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

				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\">First</button>",
					firstPage))

				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\"%s>Prev</button>",
					prevPage,
					prevStatus))

				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\"%s>Next</button>",
					nextPage,
					nextStatus))

				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\">Last</button>",
					lastPage))
			}
		}

		htmlBody.WriteString(`</table></body></html>`)

		b, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
		if err != nil {
			return
		}

		if Verbose {
			fmt.Printf("%s | Serve: HTML index page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(b),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveIndexJson(args []string, index *fileCache, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		cachedFiles := index.List()

		fileCount := len(cachedFiles)

		sort.SliceStable(cachedFiles, func(p, q int) bool {
			return strings.ToLower(cachedFiles[p]) < strings.ToLower(cachedFiles[q])
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
			cachedFiles = []string{}
		}

		if stopIndex > fileCount {
			stopIndex = fileCount
		}

		response, err := json.MarshalIndent(cachedFiles[startIndex:stopIndex], "", "    ")
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | SERVE: JSON index page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveAvailableExtensions() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		response := []byte(types.SupportedFormats.GetExtensions())

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | SERVE: Available extension list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveEnabledExtensions(formats *types.Types) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		response := []byte(formats.GetExtensions())

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | SERVE: Registered extension list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveAvailableMimeTypes() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		response := []byte(types.SupportedFormats.GetMimeTypes())

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | SERVE: Available MIME type list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveEnabledMimeTypes(formats *types.Types) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/plain")

		startTime := time.Now()

		response := []byte(formats.GetMimeTypes())

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | SERVE: Registered MIME type list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func registerInfoHandlers(mux *httprouter.Router, args []string, cache *fileCache, formats *types.Types, errorChannel chan<- error) {
	if Cache {
		register(mux, Prefix+"/html/", serveIndexHtml(args, cache, false))
		if PageLength != 0 {
			register(mux, Prefix+"/html/:page", serveIndexHtml(args, cache, true))
		}

		register(mux, Prefix+"/json", serveIndexJson(args, cache, errorChannel))
		if PageLength != 0 {
			register(mux, Prefix+"/json/:page", serveIndexJson(args, cache, errorChannel))
		}
	}

	register(mux, Prefix+"/available_extensions", serveAvailableExtensions())
	register(mux, Prefix+"/enabled_extensions", serveEnabledExtensions(formats))
	register(mux, Prefix+"/available_mime_types", serveAvailableMimeTypes())
	register(mux, Prefix+"/enabled_mime_types", serveEnabledMimeTypes(formats))
}
