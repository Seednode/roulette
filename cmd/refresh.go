/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"net/http"
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

func refreshFunction(rootUrl string, refreshTimer int64, nonce string) string {
	var htmlBody strings.Builder

	htmlBody.WriteString(fmt.Sprintf(`<script nonce=%q>window.addEventListener("load", function(){ clear = setInterval(function() {window.location.href = '%s';}, %d)});`,
		nonce,
		rootUrl,
		refreshTimer))
	htmlBody.WriteString("document.body.onkeyup = function(e) { ")
	htmlBody.WriteString(`if (e.key == " " || e.code == "Space" || e.keyCode == 32) { `)
	htmlBody.WriteString(`if (typeof clear !== 'undefined') {`)
	htmlBody.WriteString(`clearInterval(clear); delete clear;`)
	htmlBody.WriteString(`} else {`)
	htmlBody.WriteString(fmt.Sprintf("clear = setInterval(function(){window.location.href = '%s';}, %d);}}}",
		rootUrl,
		refreshTimer))
	htmlBody.WriteString(`</script>`)

	return htmlBody.String()
}
