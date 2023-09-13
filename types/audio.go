/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"fmt"
	"strings"
)

type Audio struct{}

func (t Audio) Css() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)

	return css.String()
}

func (t Audio) Title(queryParams, fileUri, filePath, fileName, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Audio) Body(queryParams, fileUri, filePath, fileName, mime string) string {
	return fmt.Sprintf(`<a href="/%s"><audio controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the audio tag.</audio></a>`,
		queryParams,
		fileUri,
		mime,
		fileName)
}

func (t Audio) Extensions() map[string]string {
	return map[string]string{
		`.mp3`: `audio/mpeg`,
		`.ogg`: `audio/ogg`,
		`.oga`: `audio/ogg`,
	}
}

func (t Audio) MimeTypes() []string {
	return []string{
		`application/ogg`,
		`audio/mp3`,
		`audio/mpeg`,
		`audio/mpeg3`,
		`audio/ogg`,
		`audio/x-mpeg-3`,
	}
}

func (t Audio) Validate(filePath string) bool {
	return true
}
