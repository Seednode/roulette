/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"fmt"
)

func RegisterAudioFormats() *SupportedFormat {
	return &SupportedFormat{
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<a href="/%s"><audio controls autoplay><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the audio tag.</audio></a>`,
				queryParams,
				fileUri,
				mime,
				fileName)
		},
		Extensions: []string{
			`.mp3`,
			`.ogg`,
			`.oga`,
			`.wav`,
		},
		MimeTypes: []string{
			`audio/mpeg`,
			`audio/ogg`,
			`audio/wav`,
		},
	}
}
