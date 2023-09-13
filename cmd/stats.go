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

type ServeStats struct {
	mutex sync.RWMutex
	list  []string
	count map[string]uint32
	size  map[string]string
	times map[string][]string
}

type exportedServeStats struct {
	List  []string
	Count map[string]uint32
	Size  map[string]string
	Times map[string][]string
}

func (s *ServeStats) incrementCounter(file string, timestamp time.Time, filesize string) {
	s.mutex.Lock()

	s.count[file]++

	s.times[file] = append(s.times[file], timestamp.Format(logDate))

	_, exists := s.size[file]
	if !exists {
		s.size[file] = filesize
	}

	if !contains(s.list, file) {
		s.list = append(s.list, file)
	}

	s.mutex.Unlock()
}

func (s *ServeStats) toExported() *exportedServeStats {
	stats := &exportedServeStats{
		List:  make([]string, len(s.list)),
		Count: make(map[string]uint32),
		Size:  make(map[string]string),
		Times: make(map[string][]string),
	}

	s.mutex.RLock()

	copy(stats.List, s.list)

	for k, v := range s.count {
		stats.Count[k] = v
	}

	for k, v := range s.size {
		stats.Size[k] = v
	}

	for k, v := range s.times {
		stats.Times[k] = v
	}

	s.mutex.RUnlock()

	return stats
}

func (s *ServeStats) toImported(stats *exportedServeStats) {
	s.mutex.Lock()

	s.list = make([]string, len(stats.List))

	copy(s.list, stats.List)

	for k, v := range stats.Count {
		s.count[k] = v
	}

	for k, v := range stats.Size {
		s.size[k] = v
	}

	for k, v := range stats.Times {
		s.times[k] = v
	}

	s.mutex.Unlock()
}

func (s *ServeStats) ListFiles(page int) ([]byte, error) {
	stats := s.toExported()

	sort.SliceStable(stats.List, func(p, q int) bool {
		return strings.ToLower(stats.List[p]) < strings.ToLower(stats.List[q])
	})

	var startIndex, stopIndex int

	if page == -1 {
		startIndex = 0
		stopIndex = len(stats.List) - 1
	} else {
		startIndex = ((page - 1) * int(PageLength))
		stopIndex = (startIndex + int(PageLength))
	}

	if startIndex > len(stats.List)-1 {
		return []byte("{}"), nil
	}

	if stopIndex > len(stats.List)-1 {
		stopIndex = len(stats.List) - 1
	}

	a := make([]timesServed, stopIndex-startIndex)

	for k, v := range stats.List[startIndex:stopIndex] {
		a[k] = timesServed{v, stats.Count[v], stats.Size[v], stats.Times[v]}
	}

	r, err := json.MarshalIndent(a, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

func (s *ServeStats) Export(path string) error {
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

	stats := s.toExported()

	err = enc.Encode(&stats)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServeStats) Import(path string) error {
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

	stats := &exportedServeStats{
		List:  []string{},
		Count: make(map[string]uint32),
		Size:  make(map[string]string),
		Times: make(map[string][]string),
	}

	err = dec.Decode(stats)
	if err != nil {
		return err
	}

	s.toImported(stats)

	return nil
}

type timesServed struct {
	File   string
	Served uint32
	Size   string
	Times  []string
}

func serveStats(args []string, stats *ServeStats) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

		startTime := time.Now()

		page, err := strconv.Atoi(p.ByName("page"))
		if err != nil || page == 0 {
			page = -1
		}

		response, err := stats.ListFiles(page)
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
			stats.Export(StatisticsFile)
		}
	}
}
