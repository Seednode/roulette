/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"fmt"
	"os"
	"unicode/utf8"
)

func RegisterTextFormats() *SupportedFormat {
	return &SupportedFormat{
		Css: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return `pre{margin:.5rem;}`
		},
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			body, err := os.ReadFile(filePath)
			if err != nil {
				body = []byte{}
			}

			if !utf8.Valid(body) {
				body = []byte(`Unable to parse binary file as text.`)
			}

			return fmt.Sprintf(`<a href="/%s"><pre>%s</pre></a>`,
				queryParams,
				body)
		},
		Extensions: []string{
			`.css`,
			`.csv`,
			`.html`,
			`.js`,
			`.json`,
			`.md`,
			`.txt`,
			`.xml`,
		},
		MimeTypes: []string{
			`application/json`,
			`application/octet-stream`,
			`application/xml`,
			`text/css`,
			`text/csv`,
			`text/javascript`,
			`text/plain`,
			`text/plain; charset=utf-8`,
		},
	}
}
