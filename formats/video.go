/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"fmt"
)

func RegisterVideoFormats() *SupportedFormat {
	return &SupportedFormat{
		Css: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return ``
		},
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<a href="/%s"><video controls autoplay><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the video tag.</video></a>`,
				queryParams,
				fileUri,
				mime,
				fileName)
		},
		Extensions: []string{
			`.mp4`,
			`.ogv`,
			`.webm`,
		},
		MimeTypes: []string{
			`video/mp4`,
			`video/ogg`,
			`video/webm`,
		},
	}
}
