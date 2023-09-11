/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/h2non/filetype"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

func RegisterImageFormats() *SupportedType {
	return &SupportedType{
		title: func(queryParams, filePath, mime, fileName string, width, height int) string {
			return fmt.Sprintf(`<title>%s (%dx%d)</title>`,
				fileName,
				width,
				height)
		},
		body: func(queryParams, filePath, mime, fileName string, width, height int) string {
			return fmt.Sprintf(`<a href="/%s"><img src="%s" width="%d" height="%d" type="%s" alt="Roulette selected: %s"></a>`,
				queryParams,
				filePath,
				width,
				height,
				mime,
				fileName)
		},
		extensions: []string{
			`.bmp`,
			`.gif`,
			`.jpeg`,
			`.jpg`,
			`.png`,
			`.webp`,
		},
		validator: func(head []byte) bool {
			return filetype.IsImage(head)
		},
	}
}
