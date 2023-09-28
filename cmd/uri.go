/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

func refreshInterval(r *http.Request) (int64, string) {
	interval := r.URL.Query().Get("refresh")

	duration, err := time.ParseDuration(interval)

	switch {
	case err != nil || duration == 0 || !Refresh:
		return 0, "0ms"
	case duration < 500*time.Millisecond:
		return 500, "500ms"
	default:
		return duration.Milliseconds(), interval
	}
}

func sortOrder(r *http.Request) string {
	sortOrder := r.URL.Query().Get("sort")
	if sortOrder == "asc" || sortOrder == "desc" {
		return sortOrder
	}

	return ""
}

func splitQueryParams(query string, regexes *regexes) []string {
	results := []string{}

	if query == "" {
		return results
	}

	params := strings.Split(query, ",")

	for i := 0; i < len(params); i++ {
		results = append(results, params[i])
	}

	return results
}

func generateQueryParams(filters *filters, sortOrder, refreshInterval string) string {
	var hasParams bool

	var queryParams strings.Builder

	queryParams.WriteString("?")

	if Filtering {
		queryParams.WriteString("include=")
		if filters.hasIncludes() {
			queryParams.WriteString(filters.includes())
		}

		queryParams.WriteString("&exclude=")
		if filters.hasExcludes() {
			queryParams.WriteString(filters.excludes())
		}

		hasParams = true
	}

	if Sorting {
		if hasParams {
			queryParams.WriteString("&")
		}

		queryParams.WriteString(fmt.Sprintf("sort=%s", sortOrder))

		hasParams = true
	}

	if Refresh {
		if hasParams {
			queryParams.WriteString("&")
		}
		queryParams.WriteString(fmt.Sprintf("refresh=%s", refreshInterval))

		hasParams = true
	}

	if hasParams {
		return queryParams.String()
	}

	return ""
}

func stripQueryParams(request string) (string, error) {
	uri, err := url.Parse(request)
	if err != nil {
		return "", err
	}

	uri.RawQuery = ""

	escapedUri, err := url.QueryUnescape(uri.String())
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		return strings.TrimPrefix(escapedUri, "/"), nil
	}

	return escapedUri, nil
}

func generateFileUri(path string) string {
	var uri strings.Builder

	uri.WriteString(sourcePrefix)
	if runtime.GOOS == "windows" {
		uri.WriteString(`/`)
	}
	uri.WriteString(path)

	return uri.String()
}

func refererToUri(referer string) string {
	parts := strings.SplitAfterN(referer, "/", 4)

	if len(parts) < 4 {
		return ""
	}

	return "/" + parts[3]
}

func realIP(r *http.Request) string {
	remoteAddr := strings.SplitAfter(r.RemoteAddr, ":")

	if len(remoteAddr) < 1 {
		return r.RemoteAddr
	}

	remotePort := remoteAddr[len(remoteAddr)-1]

	cfIP := r.Header.Get("Cf-Connecting-Ip")
	xRealIp := r.Header.Get("X-Real-Ip")

	switch {
	case cfIP != "":
		return cfIP + ":" + remotePort
	case xRealIp != "":
		return xRealIp + ":" + remotePort
	default:
		return r.RemoteAddr
	}
}
