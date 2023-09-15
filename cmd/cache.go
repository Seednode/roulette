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
	"seedno.de/seednode/roulette/types"
)

type fileCache struct {
	mutex sync.RWMutex
	list  []string
}

func (cache *fileCache) List() []string {
	cache.mutex.RLock()
	val := make([]string, len(cache.list))
	copy(val, cache.list)
	cache.mutex.RUnlock()

	return val
}

func (cache *fileCache) remove(path string) {
	cache.mutex.RLock()
	tempIndex := make([]string, len(cache.list))
	copy(tempIndex, cache.list)
	cache.mutex.RUnlock()

	var position int

	for k, v := range tempIndex {
		if path == v {
			position = k

			break
		}
	}

	tempIndex[position] = tempIndex[len(tempIndex)-1]

	cache.mutex.Lock()
	cache.list = make([]string, len(tempIndex)-1)
	copy(cache.list, tempIndex[:len(tempIndex)-1])
	cache.mutex.Unlock()
}

func (cache *fileCache) set(val []string) {
	length := len(val)

	if length < 1 {
		return
	}

	cache.mutex.Lock()
	cache.list = make([]string, length)
	copy(cache.list, val)
	cache.mutex.Unlock()
}

func (cache *fileCache) generate(args []string, formats *types.Types) {
	cache.mutex.Lock()
	cache.list = []string{}
	cache.mutex.Unlock()

	fileList(args, &filters{}, "", cache, formats)

	if Cache && CacheFile != "" {
		cache.Export(CacheFile)
	}
}

func (cache *fileCache) isEmpty() bool {
	cache.mutex.RLock()
	length := len(cache.list)
	cache.mutex.RUnlock()

	return length == 0
}

func (cache *fileCache) Export(path string) error {
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

	cache.mutex.RLock()
	enc.Encode(&cache.list)
	cache.mutex.RUnlock()

	return nil
}

func (cache *fileCache) Import(path string) error {
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

	cache.mutex.Lock()

	err = dec.Decode(&cache.list)

	cache.mutex.Unlock()

	if err != nil {
		return err
	}

	return nil
}

func serveCacheClear(args []string, cache *fileCache, formats *types.Types) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		cache.generate(args, formats)

		w.Header().Set("Content-Type", "text/plain")

		w.Write([]byte("Ok"))
	}
}

func registerCacheHandlers(mux *httprouter.Router, args []string, cache *fileCache, formats *types.Types) {
	skipIndex := false

	if CacheFile != "" {
		err := cache.Import(CacheFile)
		if err == nil {
			skipIndex = true
		}
	}

	if !skipIndex {
		cache.generate(args, formats)
	}

	register(mux, Prefix+"/clear_cache", serveCacheClear(args, cache, formats))
}
