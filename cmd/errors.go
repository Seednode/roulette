/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yosssi/gohtml"
	"seedno.de/seednode/roulette/types"
)

var (
	ErrInvalidAdminPrefix    = errors.New("admin path must match the pattern " + AllowedCharacters)
	ErrInvalidConcurrency    = errors.New("concurrency limit must be a positive integer")
	ErrInvalidFileCountRange = errors.New("maximum file count limit must be greater than or equal to minimum file count limit")
	ErrInvalidFileCountValue = errors.New("file count limits must be non-negative integers no greater than 2147483647")
	ErrInvalidIgnoreFile     = errors.New("ignore filename must match the pattern " + AllowedCharacters)
	ErrInvalidPort           = errors.New("listen port must be an integer between 1 and 65535 inclusive")
	ErrNoMediaFound          = errors.New("no supported media formats found which match all criteria")
)

func notFound(w http.ResponseWriter, r *http.Request, path string) error {
	startTime := time.Now()

	w.WriteHeader(http.StatusNotFound)
	w.Header().Add("Content-Type", "text/html")

	nonce := types.GetNonce(6)

	w.Header().Add("Content-Security-Policy", fmt.Sprintf("default-src 'self' 'nonce-%s';", nonce))

	_, err := io.WriteString(w, gohtml.Format(newPage("Not Found", "404 Page not found", nonce)))
	if err != nil {
		return err
	}

	if Verbose {
		fmt.Printf("%s | ERROR: Unavailable file %s requested by %s\n",
			startTime.Format(logDate),
			path,
			r.RemoteAddr,
		)
	}

	return nil
}

func serverError(w http.ResponseWriter, r *http.Request, i interface{}) {
	startTime := time.Now()

	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Add("Content-Type", "text/html")

	nonce := types.GetNonce(6)

	w.Header().Add("Content-Security-Policy", fmt.Sprintf("default-src 'self' 'nonce-%s';", nonce))

	io.WriteString(w, gohtml.Format(newPage("Server Error", "An error has occurred. Please try again.", nonce)))

	if Verbose {
		fmt.Printf("%s | ERROR: Invalid request for %s from %s\n",
			startTime.Format(logDate),
			r.URL.Path,
			r.RemoteAddr,
		)
	}
}

func serverErrorHandler() func(http.ResponseWriter, *http.Request, interface{}) {
	return serverError
}
