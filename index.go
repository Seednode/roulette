/*
Copyright © 2025 Seednode <seednode@seedno.de>
*/

package main

import (
	"encoding/gob"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/Seednode/roulette/types"
	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
)

type fileIndex struct {
	mutex     *sync.RWMutex
	pathMap   map[string][]string
	pathIndex []string
	list      []string
}

func (index *fileIndex) remove(path string) {
	index.mutex.RLock()
	t := make([]string, len(index.list))
	copy(t, index.list)
	index.mutex.RUnlock()

	var position int

	for k, v := range t {
		if path == v {
			position = k

			break
		}
	}

	t[position] = t[len(t)-1]

	index.mutex.Lock()
	index.list = make([]string, len(t)-1)
	copy(index.list, t[:len(t)-1])
	index.mutex.Unlock()
}

func (index *fileIndex) getDirectory() string {
	index.mutex.RLock()
	retVal := index.pathIndex[rand.IntN(len(index.pathIndex))]
	index.mutex.RUnlock()

	return retVal
}

func (index *fileIndex) generate() {
	i := make([]string, 0)
	d := make(map[string][]string)

	index.mutex.RLock()
	for _, v := range index.list {
		dir, _ := path.Split(v)

		d[dir] = append(d[dir], v)

		if !slices.Contains(i, dir) {
			i = append(i, dir)
		}
	}
	index.mutex.RUnlock()

	for k := range d {
		slices.Sort(d[k])
	}

	slices.Sort(i)

	index.mutex.Lock()
	index.pathMap = d
	index.pathIndex = i
	index.mutex.Unlock()
}

func (index *fileIndex) set(val []string, errorChannel chan<- error) {
	length := len(val)

	if length < 1 {
		return
	}

	index.mutex.Lock()
	index.list = make([]string, length)
	copy(index.list, val)
	index.mutex.Unlock()

	index.generate()

	if Index && IndexFile != "" {
		index.Export(IndexFile, errorChannel)
	}
}

func (index *fileIndex) clear() {
	index.mutex.Lock()
	index.list = nil
	index.mutex.Unlock()
}

func (index *fileIndex) isEmpty() bool {
	index.mutex.RLock()
	length := len(index.list)
	index.mutex.RUnlock()

	return length == 0
}

func (index *fileIndex) Export(path string, errorChannel chan<- error) {
	startTime := time.Now()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		errorChannel <- err

		return
	}
	defer file.Close()

	encoder, err := zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		errorChannel <- err

		return
	}
	defer encoder.Close()

	enc := gob.NewEncoder(encoder)

	index.mutex.RLock()
	err = enc.Encode(&index.list)
	if err != nil {
		index.mutex.RUnlock()

		errorChannel <- err

		return
	}
	length := len(index.list)
	index.mutex.RUnlock()

	// Close encoder prior to checking file size,
	// to ensure the correct value is returned.
	encoder.Close()

	stats, err := file.Stat()
	if err != nil {
		errorChannel <- err

		return
	}

	if Verbose {
		fmt.Printf("%s | INDEX: Exported %d entries to %s (%s) in %s\n",
			time.Now().Format(logDate),
			length,
			path,
			humanReadableSize(int(stats.Size())),
			time.Since(startTime).Round(time.Microsecond),
		)
	}
}

func (index *fileIndex) Import(path string, errorChannel chan<- error) {
	startTime := time.Now()

	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		errorChannel <- err

		return
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil {
		errorChannel <- err

		return
	}

	reader, err := zstd.NewReader(file)
	if err != nil {
		errorChannel <- err

		return
	}
	defer reader.Close()

	dec := gob.NewDecoder(reader)

	list := make([]string, 0)

	err = dec.Decode(&list)
	if err != nil {
		errorChannel <- err

		return
	}

	index.mutex.Lock()
	index.list = list
	length := len(index.list)
	index.mutex.Unlock()

	index.generate()

	if Verbose {
		fmt.Printf("%s | INDEX: Imported %d entries from %s (%s) in %s\n",
			time.Now().Format(logDate),
			length,
			path,
			humanReadableSize(int(stats.Size())),
			time.Since(startTime).Round(time.Microsecond),
		)
	}
}

func rebuildIndex(paths []string, index *fileIndex, formats types.Types, errorChannel chan<- error) {
	index.clear()

	fileList(paths, index, formats, errorChannel)
}

func importIndex(paths []string, index *fileIndex, formats types.Types, errorChannel chan<- error) {
	if IndexFile != "" {
		index.Import(IndexFile, errorChannel)
	}

	fileList(paths, index, formats, errorChannel)
}

func serveIndexRebuild(paths []string, index *fileIndex, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if Verbose {
			fmt.Printf("%s | SERVE: Index rebuild requested by %s\n",
				time.Now().Format(logDate),
				realIP(r))
		}

		w.Header().Add("Content-Security-Policy", "default-src 'self';")

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

		securityHeaders(w)

		rebuildIndex(paths, index, formats, errorChannel)

		_, err := w.Write([]byte("Ok\n"))
		if err != nil {
			errorChannel <- err

			return
		}
	}
}

func registerIndexInterval(paths []string, index *fileIndex, formats types.Types, quit <-chan struct{}, errorChannel chan<- error) {
	interval, err := time.ParseDuration(IndexInterval)
	if err != nil {
		errorChannel <- err

		return
	}

	ticker := time.NewTicker(interval)

	if Verbose {
		next := time.Now().Add(interval).Truncate(time.Second)
		fmt.Printf("%s | INDEX: Next scheduled rebuild will run at %s\n", time.Now().Format(logDate), next.Format(logDate))
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				next := time.Now().Add(interval).Truncate(time.Second)

				if Verbose {
					fmt.Printf("%s | INDEX: Started scheduled index rebuild\n", time.Now().Format(logDate))
				}

				rebuildIndex(paths, index, formats, errorChannel)

				if Verbose {
					fmt.Printf("%s | INDEX: Next scheduled rebuild will run at %s\n", time.Now().Format(logDate), next.Format(logDate))
				}
			case <-quit:
				ticker.Stop()

				return
			}
		}
	}()
}
