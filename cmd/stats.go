/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
)

type serveStats struct {
	mutex sync.RWMutex
	list  []string
	count map[string]uint32
	size  map[string]string
	times map[string][]string
}

type publicServeStats struct {
	List  []string
	Count map[string]uint32
	Size  map[string]string
	Times map[string][]string
}

type timesServed struct {
	File   string
	Served uint32
	Size   string
	Times  []string
}

func (stats *serveStats) incrementCounter(file string, timestamp time.Time, filesize string) {
	stats.mutex.Lock()

	stats.count[file]++

	stats.times[file] = append(stats.times[file], timestamp.Format(logDate))

	_, exists := stats.size[file]
	if !exists {
		stats.size[file] = filesize
	}

	if !contains(stats.list, file) {
		stats.list = append(stats.list, file)
	}

	stats.mutex.Unlock()
}

func (stats *serveStats) Import(source *publicServeStats) {
	stats.mutex.Lock()

	copy(stats.list, source.List)

	for k, v := range source.Count {
		fmt.Printf("Setting count[%s] to %d\n", k, v)
		stats.count[k] = v
	}

	for k, v := range source.Size {
		fmt.Printf("Setting size[%s] to %v\n", k, v)

		stats.size[k] = v
	}

	for k, v := range source.Times {
		fmt.Printf("Setting times[%s] to %v\n", k, v)
		stats.times[k] = v
	}

	stats.mutex.Unlock()
}

func (source *serveStats) Export() *publicServeStats {
	source.mutex.RLock()

	stats := &publicServeStats{
		List:  make([]string, len(source.list)),
		Count: make(map[string]uint32, len(source.count)),
		Size:  make(map[string]string, len(source.size)),
		Times: make(map[string][]string, len(source.times)),
	}

	copy(stats.List, source.list)

	for k, v := range source.count {
		stats.Count[k] = v
	}

	for k, v := range source.size {
		stats.Size[k] = v
	}

	for k, v := range source.times {
		stats.Times[k] = v
	}

	source.mutex.RUnlock()

	return stats
}

func (stats *serveStats) exportFile(path string) error {
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

	err = enc.Encode(stats.Export())
	if err != nil {
		return err
	}

	return nil
}

func (stats *serveStats) importFile(path string) error {
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

	source := &publicServeStats{
		List:  []string{},
		Count: make(map[string]uint32),
		Size:  make(map[string]string),
		Times: make(map[string][]string),
	}

	err = dec.Decode(source)
	if err != nil {
		return err
	}

	stats.Import(source)

	return nil
}

func (source *serveStats) listFiles(page int) ([]byte, error) {
	stats := source.Export()

	sort.SliceStable(stats.List, func(p, q int) bool {
		return strings.ToLower(stats.List[p]) < strings.ToLower(stats.List[q])
	})

	var startIndex, stopIndex int

	if page == -1 {
		startIndex = 0
		stopIndex = len(stats.List)
	} else {
		startIndex = ((page - 1) * int(PageLength))
		stopIndex = (startIndex + int(PageLength))
	}

	if startIndex > len(stats.List)-1 {
		return []byte("{}"), nil
	}

	if stopIndex > len(stats.List) {
		stopIndex = len(stats.List)
	}

	a := make([]timesServed, (stopIndex - startIndex))

	for k, v := range stats.List[startIndex:stopIndex] {
		a[k] = timesServed{v, stats.Count[v], stats.Size[v], stats.Times[v]}
	}

	r, err := json.MarshalIndent(a, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

func serveStatsPage(args []string, stats *serveStats) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page == 0 {
			page = -1
		}

		response, err := stats.listFiles(page)
		if err != nil {
			fmt.Println(err)

			serverError(w, r, nil)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		w.Write(response)

		if Verbose {
			fmt.Printf("%s | Served statistics page (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(len(response)),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}

		if StatisticsFile != "" {
			stats.exportFile(StatisticsFile)
		}
	}
}

func registerStatsHandlers(mux *httprouter.Router, args []string, stats *serveStats) {
	mux.GET("/stats", serveStatsPage(args, stats))
	if PageLength != 0 {
		mux.GET("/stats/:page", serveStatsPage(args, stats))
	}
}
