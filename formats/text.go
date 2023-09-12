/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"errors"
	"fmt"
	"os"
	"unicode/utf8"
)

func RegisterTextFormats() *SupportedFormat {
	return &SupportedFormat{
		Css: `pre{margin:.5rem;}`,
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			body, err := os.ReadFile(filePath)
			if err != nil {
				body = []byte{}
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
		Validate: func(path string) bool {
			file, err := os.Open(path)
			switch {
			case errors.Is(err, os.ErrNotExist):
				return false
			case err != nil:
				return false
			}
			defer file.Close()

			head := make([]byte, 512)
			file.Read(head)

			return utf8.Valid(head)
		},
	}
}
