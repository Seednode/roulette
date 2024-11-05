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
)

var (
	ErrInvalidAdminPrefix    = errors.New("admin path must match the pattern " + AllowedCharacters)
	ErrInvalidConcurrency    = errors.New("concurrency limit must be a positive integer")
	ErrInvalidFileCountRange = errors.New("maximum file count limit must be greater than or equal to minimum file count limit")
	ErrInvalidFileCountValue = errors.New("file count limits must be non-negative integers no greater than 2147483647")
	ErrInvalidIgnoreFile     = errors.New("ignore filename must match the pattern " + AllowedCharacters)
	ErrInvalidOverrideFile   = errors.New("override filename must match the pattern " + AllowedCharacters)
	ErrInvalidPort           = errors.New("listen port must be an integer between 1 and 65535 inclusive")
	ErrNoMediaFound          = errors.New("no supported media formats found which match all criteria")
)

func notFound(w http.ResponseWriter, r *http.Request, path string) error {
	if Verbose {
		fmt.Printf("%s | ERROR: Unavailable file %s requested by %s\n",
			time.Now().Format(logDate),
			path,
			r.RemoteAddr,
		)
	}

	w.WriteHeader(http.StatusNotFound)

	w.Header().Add("Content-Type", "text/html")

	_, err := io.WriteString(w, newPage("Not Found", "404 Page not found"))
	if err != nil {
		return err
	}

	return nil
}

func serverError(w http.ResponseWriter, r *http.Request, i interface{}) {
	if Verbose {
		fmt.Printf("%s | ERROR: Invalid request for %s from %s\n",
			time.Now().Format(logDate),
			r.URL.Path,
			r.RemoteAddr)
	}

	w.Header().Add("Content-Type", "text/html")

	io.WriteString(w, newPage("Server Error", "An error has occurred. Please try again."))
}

func serverErrorHandler() func(http.ResponseWriter, *http.Request, interface{}) {
	return serverError
}
