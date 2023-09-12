/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"fmt"
)

func RegisterVideoFormats() *SupportedFormat {
	return &SupportedFormat{
		Css: ``,
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<a href="/%s"><video controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the video tag.</video></a>`,
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
		Validate: func(filePath string) bool {
			return true
		},
	}
}
