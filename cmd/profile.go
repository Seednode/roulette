/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"net/http/pprof"

	"github.com/julienschmidt/httprouter"
)

func registerProfileHandlers(mux *httprouter.Router) {
	mux.HandlerFunc("GET", Prefix+"/debug/pprof/", pprof.Index)
	mux.HandlerFunc("GET", Prefix+"/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandlerFunc("GET", Prefix+"/debug/pprof/profile", pprof.Profile)
	mux.HandlerFunc("GET", Prefix+"/debug/pprof/symbol", pprof.Symbol)
	mux.HandlerFunc("GET", Prefix+"/debug/pprof/trace", pprof.Trace)
}
