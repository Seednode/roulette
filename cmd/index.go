/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"compress/flate"
	"compress/gzip"
	"compress/lzw"
	"compress/zlib"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang/snappy"
	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
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

func getReader(format string, file io.Reader) (io.Reader, error) {
	switch format {
	case "flate":
		return flate.NewReader(file), nil
	case "gzip":
		return gzip.NewReader(file)
	case "lz4":
		return lz4.NewReader(file), nil
	case "lzw":
		return lzw.NewReader(file, lzw.LSB, 8), nil
	case "none":
		return io.NopCloser(file), nil
	case "snappy":
		return snappy.NewReader(file), nil
	case "zlib":
		return zlib.NewReader(file)
	case "zstd":
		decoder, err := zstd.NewReader(file)

		return decoder.IOReadCloser(), err
	}

	return io.NopCloser(file), ErrInvalidCompression
}

func getWriter(format string, file io.WriteCloser) (io.WriteCloser, error) {
	switch {
	case format == "flate" && CompressionFast:
		return flate.NewWriter(file, flate.BestCompression)
	case format == "flate":
		return flate.NewWriter(file, flate.BestSpeed)
	case format == "gzip" && CompressionFast:
		return gzip.NewWriterLevel(file, gzip.BestSpeed)
	case format == "gzip":
		return gzip.NewWriterLevel(file, gzip.BestCompression)
	case format == "lz4" && CompressionFast:
		encoder := lz4.NewWriter(file)

		err := encoder.Apply(lz4.CompressionLevelOption(lz4.Fast))
		if err != nil {
			return file, err
		}

		return encoder, nil
	case format == "lz4":
		encoder := lz4.NewWriter(file)

		err := encoder.Apply(lz4.CompressionLevelOption(lz4.Level9))
		if err != nil {
			return file, err
		}

		return encoder, nil
	case format == "lzw":
		return lzw.NewWriter(file, lzw.LSB, 8), nil
	case format == "none":
		return file, nil
	case format == "snappy":
		return snappy.NewBufferedWriter(file), nil
	case format == "zlib" && CompressionFast:
		return zlib.NewWriterLevel(file, zlib.BestSpeed)
	case format == "zlib":
		return zlib.NewWriterLevel(file, zlib.BestCompression)
	case format == "zstd" && CompressionFast:
		return zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.SpeedFastest))
	case format == "zstd":
		return zstd.NewWriter(file, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	}

	return file, ErrInvalidCompression
}

func (index *fileIndex) Export(path string) error {
	startTime := time.Now()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder, err := getWriter(Compression, file)
	if err != nil {
		return err
	}
	defer encoder.Close()

	enc := gob.NewEncoder(encoder)

	index.mutex.RLock()
	err = enc.Encode(&index.list)
	if err != nil {
		index.mutex.RUnlock()

		return err
	}
	length := len(index.list)
	index.mutex.RUnlock()

	// Close encoder prior to checking file size,
	// to ensure the correct value is returned.
	// If no compression is used, skip this step,
	// as the encoder is just the file itself.
	if Compression != "none" {
		encoder.Close()
	}

	stats, err := file.Stat()
	if err != nil {
		return err
	}

	if Verbose {
		fmt.Printf("%s | INDEX: Exported %d entries to %s (%s) in %s\n",
			time.Now().Format(logDate),
			length,
			path,
			humanReadableSize(int(stats.Size())),
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

	stats, err := file.Stat()
	if err != nil {
		return err
	}

	reader, err := getReader(Compression, file)
	if err != nil {
		return err
	}

	dec := gob.NewDecoder(reader)

	index.mutex.Lock()
	err = dec.Decode(&index.list)
	if err != nil {
		index.mutex.Unlock()

		return err
	}
	length := len(index.list)
	index.mutex.Unlock()

	if Verbose {
		fmt.Printf("%s | INDEX: Imported %d entries from %s (%s) in %s\n",
			time.Now().Format(logDate),
			length,
			path,
			humanReadableSize(int(stats.Size())),
			time.Since(startTime),
		)
	}

	return nil
}

func serveIndexRebuild(args []string, index *fileIndex, formats types.Types, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		index.clear()

		fileList(args, &filters{}, "", index, formats, errorChannel)

		w.Header().Set("Content-Type", "text/plain")

		w.Write([]byte("Ok\n"))
	}
}

func importIndex(args []string, index *fileIndex, formats types.Types, errorChannel chan<- error) {
	if IndexFile != "" {
		err := index.Import(IndexFile)
		if err == nil {
			return
		}
	}

	fileList(args, &filters{}, "", index, formats, errorChannel)
}
