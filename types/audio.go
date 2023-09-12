/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"fmt"
	"strings"
)

func RegisterAudio() *Type {
	return &Type{
		Css: func() string {
			var css strings.Builder

			css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
			css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)

			return css.String()
		},
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<a href="/%s"><audio controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the audio tag.</audio></a>`,
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
		Validate: func(filePath string) bool {
			return true
		},
	}
}
