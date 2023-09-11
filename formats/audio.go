/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"fmt"

	"github.com/h2non/filetype"
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
			`.wav`,
		},
		validator: func(head []byte) bool {
			return filetype.IsAudio(head)
		},
	}
}
