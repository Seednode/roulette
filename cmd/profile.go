/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"net/http/pprof"

	"github.com/julienschmidt/httprouter"
)

func registerProfileHandlers(mux *httprouter.Router) {
	mux.HandlerFunc("GET", "/debug/pprof/", pprof.Index)
	mux.HandlerFunc("GET", "/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandlerFunc("GET", "/debug/pprof/profile", pprof.Profile)
	mux.HandlerFunc("GET", "/debug/pprof/symbol", pprof.Symbol)
	mux.HandlerFunc("GET", "/debug/pprof/trace", pprof.Trace)
}
