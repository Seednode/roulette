/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/klauspost/compress/zstd"
	"seedno.de/seednode/roulette/types"
)

func serveExtensions(formats types.Types, available bool, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		w.Header().Add("Content-Security-Policy", "default-src 'self';")

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

		var extensions string

		if available {
			extensions = types.SupportedFormats.GetExtensions()
		} else {
			extensions = formats.GetExtensions()
		}

		written, err := w.Write([]byte(extensions))
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Registered extension list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond))
		}
	}
}

func serveMediaTypes(formats types.Types, available bool, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		w.Header().Add("Content-Security-Policy", "default-src 'self';")

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")

		var mediaTypes string

		if available {
			mediaTypes = types.SupportedFormats.GetMediaTypes()
		} else {
			mediaTypes = formats.GetMediaTypes()
		}

		written, err := w.Write([]byte(mediaTypes))
		if err != nil {
			errorChannel <- err
		}

		if Verbose {
			fmt.Printf("%s | SERVE: Available media type list (%s) to %s in %s\n",
				startTime.Format(logDate),
				humanReadableSize(written),
				realIP(r),
				time.Since(startTime).Round(time.Microsecond))
		}
	}
}

func registerAPIHandlers(mux *httprouter.Router, paths []string, index *fileIndex, formats types.Types, encoder *zstd.Encoder, errorChannel chan<- error) {
	if Index {
		mux.GET(Prefix+AdminPrefix+"/index", serveIndex(index, errorChannel))
		mux.POST(Prefix+AdminPrefix+"/index/rebuild", serveIndexRebuild(paths, index, formats, encoder, errorChannel))
	}

	mux.GET(Prefix+AdminPrefix+"/extensions/available", serveExtensions(formats, true, errorChannel))
	mux.GET(Prefix+AdminPrefix+"/extensions/enabled", serveExtensions(formats, false, errorChannel))
	mux.GET(Prefix+AdminPrefix+"/types/available", serveMediaTypes(formats, true, errorChannel))
	mux.GET(Prefix+AdminPrefix+"/types/enabled", serveMediaTypes(formats, false, errorChannel))
}
