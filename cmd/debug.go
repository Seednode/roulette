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
)

func serveDebugHtml(args []string, index *Index, paginate bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "text/html")

		startTime := time.Now()

		indexDump := index.Index()

		fileCount := len(indexDump)

		var startIndex, stopIndex int

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page <= 0 {
			startIndex = 0
			stopIndex = fileCount
		} else {
			startIndex = ((page - 1) * int(pageLength))
			stopIndex = (startIndex + int(pageLength))
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
		htmlBody.WriteString(FaviconHtml)
		htmlBody.WriteString(`<style>a{text-decoration:none;height:100%;width:100%;color:inherit;cursor:pointer}`)
		htmlBody.WriteString(`table,td,tr{border:1px solid black;border-collapse:collapse}td{white-space:nowrap;padding:.5em}</style>`)
		htmlBody.WriteString(fmt.Sprintf("<title>Index contains %d files</title></head><body><table>", fileCount))
		if len(indexDump) > 0 {
			for _, v := range indexDump[startIndex:stopIndex] {
				var shouldSort = ""

				if sorting {
					shouldSort = "?sort=asc"
				}
				htmlBody.WriteString(fmt.Sprintf("<tr><td><a href=\"%s%s%s\">%s</a></td></tr>\n", MediaPrefix, v, shouldSort, v))
			}
		}
		if pageLength != 0 {
			var lastPage int

			if fileCount%int(pageLength) == 0 {
				lastPage = fileCount / int(pageLength)
			} else {
				lastPage = (fileCount / int(pageLength)) + 1
			}

			if paginate {
				var prevStatus, nextStatus string = "", ""

				if page <= 1 {
					prevStatus = "disabled"
				}

				if page >= lastPage {
					nextStatus = "disabled"
				}

				prevPage := page - 1
				if prevPage < 1 {
					prevPage = 1
				}

				nextPage := page + 1
				if nextPage > lastPage {
					nextPage = fileCount / int(pageLength)
				}

				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\" %s>Prev</button>",
					prevPage,
					prevStatus))

				htmlBody.WriteString(fmt.Sprintf("<button onclick=\"window.location.href = '/html/%d';\" %s>Next</button>",
					nextPage,
					nextStatus))
			}
		}

		htmlBody.WriteString(`</table></body></html>`)

		b, err := io.WriteString(w, gohtml.Format(htmlBody.String()))
		if err != nil {
			return
		}

		if verbose {
			fmt.Printf("%s | Served HTML debug page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(b),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveDebugJson(args []string, index *Index) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")

		startTime := time.Now()

		indexDump := index.Index()

		fileCount := len(indexDump)

		sort.SliceStable(indexDump, func(p, q int) bool {
			return strings.ToLower(indexDump[p]) < strings.ToLower(indexDump[q])
		})

		var startIndex, stopIndex int

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page <= 0 {
			startIndex = 0
			stopIndex = fileCount
		} else {
			startIndex = ((page - 1) * int(pageLength))
			stopIndex = (startIndex + int(pageLength))
		}

		if startIndex > (fileCount - 1) {
			indexDump = []string{}
		}

		if stopIndex > fileCount {
			stopIndex = fileCount
		}

		response, err := json.MarshalIndent(indexDump[startIndex:stopIndex], "", "    ")
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		w.Write(response)

		if verbose {
			fmt.Printf("%s | Served JSON debug page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}
