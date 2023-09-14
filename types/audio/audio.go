/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package audio

import (
	"fmt"
	"strings"

	"seedno.de/seednode/roulette/types"
)

type Format struct{}

func (t Format) Css() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) string {
	return fmt.Sprintf(`<a href="%s"><audio controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the audio tag.</audio></a>`,
		rootUrl,
		fileUri,
		mime,
		fileName)
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.mp3`: `audio/mpeg`,
		`.ogg`: `audio/ogg`,
		`.oga`: `audio/ogg`,
	}
}

func (t Format) MimeTypes() []string {
	return []string{
		`application/ogg`,
		`audio/mp3`,
		`audio/mpeg`,
		`audio/mpeg3`,
		`audio/ogg`,
		`audio/x-mpeg-3`,
	}
}

func (t Format) Validate(filePath string) bool {
	return true
}

func New() Format {
	return Format{}
}

func init() {
	types.SupportedFormats.Register(New())
}
