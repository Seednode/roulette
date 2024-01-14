/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"net/http/pprof"

	"github.com/julienschmidt/httprouter"
)

func registerProfileHandlers(mux *httprouter.Router) {
	mux.Handler("GET", Prefix+AdminPrefix+"/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handler("GET", Prefix+AdminPrefix+"/debug/pprof/block", pprof.Handler("block"))
	mux.Handler("GET", Prefix+AdminPrefix+"/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handler("GET", Prefix+AdminPrefix+"/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handler("GET", Prefix+AdminPrefix+"/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handler("GET", Prefix+AdminPrefix+"/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.HandlerFunc("GET", Prefix+AdminPrefix+"/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandlerFunc("GET", Prefix+AdminPrefix+"/debug/pprof/profile", pprof.Profile)
	mux.HandlerFunc("GET", Prefix+AdminPrefix+"/debug/pprof/symbol", pprof.Symbol)
	mux.HandlerFunc("GET", Prefix+AdminPrefix+"/debug/pprof/trace", pprof.Trace)
}
