/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"

	"github.com/h2non/filetype"
)

func RegisterAudioFormats() *SupportedType {
	return &SupportedType{
		title: func(queryParams, filePath, mime, fileName string, width, height int) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		body: func(queryParams, filePath, mime, fileName string, width, height int) string {
			return fmt.Sprintf(`<a href="/%s"><audio controls autoplay><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the audio tag.</audio></a>`,
				queryParams,
				filePath,
				mime,
				fileName)
		},
		extensions: []string{
			`.mp3`,
			`.ogg`,
			`.wav`,
		},
		validator: func(head []byte) bool {
			return filetype.IsAudio(head)
		},
	}
}
