/*
Copyright © 2025 Seednode <seednode@seedno.de>
*/

package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

func sortOrder(r *http.Request) string {
	sortOrder := r.URL.Query().Get("sort")
	if sortOrder == "asc" || sortOrder == "desc" {
		return sortOrder
	}

	return ""
}

func generateQueryParams(sortOrder, refreshInterval string) string {
	var hasParams bool

	var queryParams strings.Builder

	queryParams.WriteString("?")

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

	cfIp := r.Header.Get("Cf-Connecting-Ip")
	xRealIp := r.Header.Get("X-Real-Ip")

	requestor := ""

	switch {
	case cfIp != "":
		if net.ParseIP(cfIp).To4() == nil {
			cfIp = "[" + cfIp + "]"
		}

		requestor = cfIp + ":" + remotePort
	case xRealIp != "":
		if net.ParseIP(xRealIp).To4() == nil {
			xRealIp = "[" + xRealIp + "]"
		}

		requestor = xRealIp + ":" + remotePort
	default:
		requestor = r.RemoteAddr
	}

	return requestor
}
