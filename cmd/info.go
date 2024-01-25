/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"seedno.de/seednode/roulette/types"
)

func serveIndex(args []string, index *fileIndex, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

		indexDump := index.List()

		sort.SliceStable(indexDump, func(p, q int) bool {
			return strings.ToLower(indexDump[p]) < strings.ToLower(indexDump[q])
		})

		w.Header().Set("Content-Type", "application/json;charset=UTF-8")

		response, err := json.MarshalIndent(indexDump, "", "    ")
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

func serveExtensions(formats types.Types, available bool, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

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
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func serveMediaTypes(formats types.Types, available bool, errorChannel chan<- error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		startTime := time.Now()

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
				time.Since(startTime).Round(time.Microsecond),
			)
		}
	}
}

func registerInfoHandlers(mux *httprouter.Router, args []string, index *fileIndex, formats types.Types, errorChannel chan<- error) {
	if Index {
		mux.GET(Prefix+AdminPrefix+"/index", serveIndex(args, index, errorChannel))
	}

	mux.GET(Prefix+AdminPrefix+"/extensions/available", serveExtensions(formats, true, errorChannel))
	mux.GET(Prefix+AdminPrefix+"/extensions/enabled", serveExtensions(formats, false, errorChannel))
	mux.GET(Prefix+AdminPrefix+"/types/available", serveMediaTypes(formats, true, errorChannel))
	mux.GET(Prefix+AdminPrefix+"/types/enabled", serveMediaTypes(formats, false, errorChannel))
}
