/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package audio

import (
	"fmt"
	"strings"

	"github.com/Seednode/roulette/types"
)

type Format struct{}

func (t Format) CSS() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)
	css.WriteString(`table{margin-left:auto;margin-right:auto;}`)

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	return fmt.Sprintf(`<title>%s</title>`, fileName), nil
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	return fmt.Sprintf(`<a href="%s"><audio controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the audio tag.</audio></a>`,
		rootUrl,
		fileUri,
		mime,
		fileName), nil
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.mp3`: `audio/mpeg`,
		`.ogg`: `audio/ogg`,
		`.oga`: `audio/ogg`,
	}
}

func (t Format) MediaType(extension string) string {
	extensions := t.Extensions()

	value, exists := extensions[extension]
	if exists {
		return value
	}

	return ""
}

func (t Format) Validate(filePath string) bool {
	return true
}

func (t Format) Type() string {
	return "embed"
}

func init() {
	types.SupportedFormats.Register(Format{})
}
