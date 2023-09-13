/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

func RegisterText() *Type {
	return &Type{
		Css: func() string {
			var css strings.Builder

			css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
			css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;overflow:hidden;}`)
			css.WriteString(`textarea{border:none;caret-color:transparent;outline:none;margin:0 .5rem 0 .5rem;height:100%;width:99%;overflow:auto;}`)

			return css.String()
		},
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			return fmt.Sprintf(`<title>%s</title>`, fileName)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			body, err := os.ReadFile(filePath)
			if err != nil {
				body = []byte{}
			}

			return fmt.Sprintf(`<a href="/%s"><textarea autofocus readonly>%s</textarea></a>`,
				queryParams,
				body)
		},
		Extensions: map[string]string{
			`.css`:  `text/css`,
			`.csv`:  `text/csv`,
			`.html`: `text/html`,
			`.js`:   `text/javascript`,
			`.json`: `application/json`,
			`.md`:   `text/markdown`,
			`.txt`:  `text/plain`,
			`.xml`:  `application/xml`,
		},
		MimeTypes: []string{
			`application/json`,
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
