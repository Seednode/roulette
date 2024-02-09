/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"seedno.de/seednode/roulette/types"
)

type fileIndex struct {
	mutex *sync.RWMutex
	list  []string
}

func makeTree(list []string) ([]byte, error) {
	tree := make(map[string]any)

	current := tree

	for _, entry := range list {
		path := strings.Split(entry, string(os.PathSeparator))

		for i, last := 0, len(path)-1; i < len(path); i++ {
			if i == last {
				current[path[i]] = nil

				break
			}

			v, ok := current[path[i]].(map[string]any)
			if !ok || v == nil {
				v = make(map[string]any)
				current[path[i]] = v
			}

			current = v
		}

		current = tree
	}

	resp, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return []byte{}, err
	}

	return resp, nil
}

func (index *fileIndex) List() []string {
	index.mutex.RLock()
	list := make([]string, len(index.list))
	copy(list, index.list)
	index.mutex.RUnlock()

	sort.SliceStable(list, func(p, q int) bool {
		return strings.ToLower(list[p]) < strings.ToLower(list[q])
	})

	return list
}

func (index *fileIndex) remove(path string) {
	index.mutex.RLock()
	tempIndex := make([]string, len(index.list))
	copy(tempIndex, index.list)
	index.mutex.RUnlock()

	var position int

	for k, v := range tempIndex {
		if path == v {
			position = k

			break
		}
	}

	tempIndex[position] = tempIndex[len(tempIndex)-1]

	index.mutex.Lock()
	index.list = make([]string, len(tempIndex)-1)
	copy(index.list, tempIndex[:len(tempIndex)-1])
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

	index.mutex.Lock()
	err = dec.Decode(&index.list)
	if err != nil {
		index.mutex.Unlock()

		errorChannel <- err

		return
	}
	length := len(index.list)
	index.mutex.Unlock()

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

	fileList(paths, &filters{}, "", index, formats, errorChannel)
}

func importIndex(paths []string, index *fileIndex, formats types.Types, errorChannel chan<- error) {
	if IndexFile != "" {
		index.Import(IndexFile, errorChannel)
	}

	fileList(paths, &filters{}, "", index, formats, errorChannel)
}

func serveIndex(index *fileIndex, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		w.Header().Add("Content-Security-Policy", "default-src 'self';")

		w.Header().Set("Content-Type", "application/json;charset=UTF-8")

		response, err := makeTree(index.List())
		if err != nil {
			errorChannel <- err

			serverError(w, r, nil)

			return
		}

		response = append(response, []byte("\n")...)

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

func serveIndexRebuild(paths []string, index *fileIndex, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if Verbose {
			fmt.Printf("%s | SERVE: Index rebuild requested by %s\n",
				time.Now().Format(logDate),
				realIP(r))
		}

		w.Header().Add("Content-Security-Policy", "default-src 'self';")

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

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

	go func() {
		for {
			select {
			case <-ticker.C:
				if Verbose {
					fmt.Printf("%s | INDEX: Started scheduled index rebuild\n", time.Now().Format(logDate))
				}

				rebuildIndex(paths, index, formats, errorChannel)
			case <-quit:
				ticker.Stop()

				return
			}
		}
	}()
}
