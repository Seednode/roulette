/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"fmt"
	"strings"
)

func RegisterVideos() *Type {
	return &Type{
		Css: func() string {
			var css strings.Builder

			css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
			css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)
			css.WriteString(`video{margin:auto;display:block;max-width:97%;max-height:97%;`)
			css.WriteString(`object-fit:scale-down;position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}`)

			return css.String()
		},
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
			`.ogm`,
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
