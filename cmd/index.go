/*
Copyright © 2023 Seednode <seednode@seedno.de>
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

func (index *fileIndex) set(val []string) {
	length := len(val)

	if length < 1 {
		return
	}

	index.mutex.Lock()
	index.list = make([]string, length)
	copy(index.list, val)
	index.mutex.Unlock()

	if Index && IndexFile != "" {
		index.Export(IndexFile)
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

func (index *fileIndex) Export(path string) error {
	startTime := time.Now()

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

	index.mutex.RLock()
	err = enc.Encode(&index.list)
	if err != nil {
		return err
	}
	length := len(index.list)
	index.mutex.RUnlock()

	if Verbose {
		fmt.Printf("%s | INDEX: Exported %d entries to %s in %s\n",
			time.Now().Format(logDate),
			length,
			path,
			time.Since(startTime),
		)
	}

	return nil
}

func (index *fileIndex) Import(path string) error {
	startTime := time.Now()

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

	index.mutex.Lock()
	err = dec.Decode(&index.list)
	if err != nil {
		return err
	}
	length := len(index.list)
	index.mutex.Unlock()

	if Verbose {
		fmt.Printf("%s | INDEX: Imported %d entries from %s in %s\n",
			time.Now().Format(logDate),
			length,
			path,
			time.Since(startTime),
		)
	}

	return nil
}

func serveIndexRebuild(args []string, index *fileIndex, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		index.clear()

		_, err := fileList(args, &filters{}, "", index, formats)
		if err != nil {
			errorChannel <- err

			return
		}

		w.Header().Set("Content-Type", "text/plain")

		w.Write([]byte("Ok"))
	}
}

func registerIndexHandlers(mux *httprouter.Router, args []string, index *fileIndex, formats types.Types, errorChannel chan<- error) error {
	registerHandler(mux, Prefix+"/index/rebuild", serveIndexRebuild(args, index, formats, errorChannel))

	return nil
}

func importIndex(args []string, index *fileIndex, formats types.Types) error {
	if IndexFile != "" {
		err := index.Import(IndexFile)
		if err == nil {
			return nil
		}
	}

	_, err := fileList(args, &filters{}, "", index, formats)
	if err != nil {
		return err
	}

	return nil
}
