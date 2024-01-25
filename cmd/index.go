/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
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

func (index *fileIndex) List() []string {
	index.mutex.RLock()
	val := make([]string, len(index.list))
	copy(val, index.list)
	index.mutex.RUnlock()

	return val
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

func (index *fileIndex) set(val []string, encoder *zstd.Encoder, errorChannel chan<- error) {
	length := len(val)

	if length < 1 {
		return
	}

	index.mutex.Lock()
	index.list = make([]string, length)
	copy(index.list, val)
	index.mutex.Unlock()

	if Index && IndexFile != "" {
		index.Export(IndexFile, encoder, errorChannel)
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

func (index *fileIndex) Export(path string, encoder *zstd.Encoder, errorChannel chan<- error) {
	startTime := time.Now()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		errorChannel <- err

		return
	}
	defer file.Close()

	encoder.Reset(file)

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

func serveIndexRebuild(args []string, index *fileIndex, formats types.Types, encoder *zstd.Encoder, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		index.clear()

		fileList(args, &filters{}, "", index, formats, encoder, errorChannel)

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

		_, err := w.Write([]byte("Ok\n"))
		if err != nil {
			errorChannel <- err

			return
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Index rebuild requested by %s took %s\n",
				startTime.Format(logDate),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func importIndex(args []string, index *fileIndex, formats types.Types, encoder *zstd.Encoder, errorChannel chan<- error) {
	if IndexFile != "" {
		index.Import(IndexFile, errorChannel)
	}

	fileList(args, &filters{}, "", index, formats, encoder, errorChannel)
}
