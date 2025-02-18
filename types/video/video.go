/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package video

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Seednode/roulette/types"
)

type Format struct{}

func (t Format) CSS() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)
	css.WriteString(`table{margin-left:auto;margin-right:auto;}`)
	css.WriteString(`video{margin:auto;display:block;max-width:97%;max-height:97%;`)
	css.WriteString(`object-fit:scale-down;position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}`)

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	return fmt.Sprintf(`<title>%s</title>`, fileName), nil
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	return fmt.Sprintf(`<a href="%s"><video controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the video tag.</video></a>`,
		rootUrl,
		fileUri,
		mime,
		fileName), nil
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.mp4`:  `video/mp4`,
		`.ogm`:  `video/ogg`,
		`.ogv`:  `video/ogg`,
		`.webm`: `video/webm`,
	}
}

func (t Format) MediaType(path string) string {
	extensions := t.Extensions()

	extension := filepath.Ext(path)

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
