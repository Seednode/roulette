/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/gob"
	"net/http"
	"os"
	"sync"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"seedno.de/seednode/roulette/formats"
)

type Index struct {
	mutex sync.RWMutex
	list  []string
}

func (i *Index) Index() []string {
	i.mutex.RLock()
	val := i.list
	i.mutex.RUnlock()

	return val
}

func (i *Index) Remove(path string) {
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

func (i *Index) setIndex(val []string) {
	i.mutex.Lock()
	i.list = val
	i.mutex.Unlock()
}

func (i *Index) generateCache(args []string, registeredFormats *formats.SupportedFormats) {
	i.mutex.Lock()
	i.list = []string{}
	i.mutex.Unlock()

	fileList(args, &Filters{}, "", i, registeredFormats)

	if cache && cacheFile != "" {
		i.Export(cacheFile)
	}
}

func (i *Index) IsEmpty() bool {
	i.mutex.RLock()
	length := len(i.list)
	i.mutex.RUnlock()

	return length == 0
}

func (i *Index) Export(path string) error {
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

func (i *Index) Import(path string) error {
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

func serveCacheClear(args []string, index *Index, registeredFormats *formats.SupportedFormats) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		index.generateCache(args, registeredFormats)

		w.Header().Set("Content-Type", "text/plain")

		w.Write([]byte("Ok"))
	}
}
