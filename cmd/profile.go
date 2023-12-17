/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/julienschmidt/httprouter"
)

func registerProfileHandler(mux *httprouter.Router, verb, path string, handler http.HandlerFunc) {
	mux.HandlerFunc(verb, path, handler)

	if Handlers {
		fmt.Printf("%s | SERVE: Registered handler for %s\n",
			time.Now().Format(logDate),
			path,
		)
	}
}

func registerProfileHandlers(mux *httprouter.Router) {
	registerProfileHandler(mux, "GET", Prefix+AdminPrefix+"/debug/pprof/", pprof.Index)
	registerProfileHandler(mux, "GET", Prefix+AdminPrefix+"/debug/pprof/cmdline", pprof.Cmdline)
	registerProfileHandler(mux, "GET", Prefix+AdminPrefix+"/debug/pprof/profile", pprof.Profile)
	registerProfileHandler(mux, "GET", Prefix+AdminPrefix+"/debug/pprof/symbol", pprof.Symbol)
	registerProfileHandler(mux, "GET", Prefix+AdminPrefix+"/debug/pprof/trace", pprof.Trace)
}
