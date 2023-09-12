/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"fmt"
	"strings"
)

func RegisterFlashFormats() *SupportedFormat {
	return &SupportedFormat{
		Name: `flash`,
		Css:  ``,
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			var html strings.Builder

			html.WriteString(fmt.Sprintf(`<script src="https://unpkg.com/@ruffle-rs/ruffle"></script><script>window.RufflePlayer.config = {autoplay:"on"};</script><embed src="%s"></embed>`, fileUri))
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
