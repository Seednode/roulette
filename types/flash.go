/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"fmt"
	"strings"
)

func RegisterFlash() *Type {
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
			var html strings.Builder

			html.WriteString(fmt.Sprintf(`<script src="https://unpkg.com/@ruffle-rs/ruffle"></script><script>window.RufflePlayer.config = {autoplay:"on"};</script><embed src="%s"></embed>`, fileUri))
			html.WriteString(fmt.Sprintf(`<br /><button onclick="window.location.href = '/%s';">Next</button>`, queryParams))

			return html.String()
		},
		Extensions: []string{
			`.swf`,
		},
		MimeTypes: []string{
			`application/x-shockwave-flash`,
		},
		Validate: func(filePath string) bool {
			return true
		},
	}
}
