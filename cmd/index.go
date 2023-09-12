/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"github.com/yosssi/gohtml"
	"seedno.de/seednode/roulette/types"
)

type FileIndex struct {
	mutex sync.RWMutex
	list  []string
}

func (i *FileIndex) Index() []string {
	i.mutex.RLock()
	val := i.list
	i.mutex.RUnlock()

	return val
}

func (i *FileIndex) Remove(path string) {
	i.mutex.RLock()
	tempIndex := make([]string, len(i.list))
	copy(tempIndex, i.list)
	i.mutex.RUnlock()

	var position int

	for k, v := range tempIndex {
		if path == v {
			position = k

			break
		}
	}

	tempIndex[position] = tempIndex[len(tempIndex)-1]

	i.mutex.Lock()
	i.list = make([]string, len(tempIndex)-1)
	copy(i.list, tempIndex[:len(tempIndex)-1])
	i.mutex.Unlock()
}

func (i *FileIndex) setIndex(val []string) {
	i.mutex.Lock()
	i.list = val
	i.mutex.Unlock()
}

func (i *FileIndex) generateCache(args []string, formats *types.Types) {
	i.mutex.Lock()
	i.list = []string{}
	i.mutex.Unlock()

	fileList(args, &Filters{}, "", i, formats)

	if Cache && CacheFile != "" {
		i.Export(CacheFile)
	}
}

func (i *FileIndex) IsEmpty() bool {
	i.mutex.RLock()
	length := len(i.list)
	i.mutex.RUnlock()

	return length == 0
}

func (i *FileIndex) Export(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	z, err := zstd.NewWriter(file)
	if err != nil {
		return err
	}
	defer z.Close()

	enc := gob.NewEncoder(z)

	i.mutex.RLock()

	enc.Encode(&i.list)

	i.mutex.RUnlock()

	return nil
}

func (i *FileIndex) Import(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	z, err := zstd.NewReader(file)
	if err != nil {
		return err
	}
	defer z.Close()

	dec := gob.NewDecoder(z)

	i.mutex.Lock()

	err = dec.Decode(&i.list)

	i.mutex.Unlock()

	if err != nil {
		return err
	}

	return nil
}

func serveCacheClear(args []string, index *FileIndex, formats *types.Types) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		index.generateCache(args, formats)

		w.Header().Set("Content-Type", "text/plain")

		w.Write([]byte("Ok"))
	}
}

func serveIndexHtml(args []string, index *FileIndex, paginate bool) httprouter.Handle {
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
			startIndex = ((page - 1) * int(PageLength))
			stopIndex = (startIndex + int(PageLength))
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

				if Sorting {
					shouldSort = "?sort=asc"
				}
				htmlBody.WriteString(fmt.Sprintf("<tr><td><a href=\"%s%s%s\">%s</a></td></tr>\n", MediaPrefix, v, shouldSort, v))
			}
		}
		if PageLength != 0 {
			var firstPage int = 1
			var lastPage int

			if fileCount%int(PageLength) == 0 {
				lastPage = fileCount / int(PageLength)
			} else {
				lastPage = (fileCount / int(PageLength)) + 1
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
					nextPage = fileCount / int(PageLength)
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
			fmt.Printf("%s | Served HTML index page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(b),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveIndexJson(args []string, index *FileIndex) httprouter.Handle {
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
			startIndex = ((page - 1) * int(PageLength))
			stopIndex = (startIndex + int(PageLength))
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

		if Verbose {
			fmt.Printf("%s | Served JSON index page (%s) to %s in %s\n",
				startTime.Format(LogDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}
