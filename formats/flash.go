/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"bytes"
	"embed"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

//go:embed ruffle/*
var ruffle embed.FS

func RegisterFlashFormats() *SupportedFormat {
	return &SupportedFormat{
		Name: `flash`,
		Css:  ``,
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			var html strings.Builder

			html.WriteString(fmt.Sprintf(`<script src="/ruffle/ruffle.js"></script><script>window.RufflePlayer.config = {autoplay:"on"};</script><embed src="%s"></embed>`, fileUri))
			html.WriteString(fmt.Sprintf(`<br /><button onclick=\"window.location.href = '/%s';\">Next</button>`, queryParams))

			return html.String()
		},
		Extensions: []string{
			`.swf`,
		},
		MimeTypes: []string{
			`application/x-shockwave-flash`,
		},
		Validate: func(filePath string) bool {
			return true
		},
	}
}

func ServeRuffle() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fname := strings.TrimPrefix(r.URL.Path, "/")

		data, err := ruffle.ReadFile(fname)
		if err != nil {
			return
		}

		w.Header().Write(bytes.NewBufferString("Content-Length: " + strconv.Itoa(len(data))))

		w.Write(data)
	}
}
