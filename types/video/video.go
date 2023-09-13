/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package video

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
	css.WriteString(`video{margin:auto;display:block;max-width:97%;max-height:97%;`)
	css.WriteString(`object-fit:scale-down;position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}`)

	return css.String()
}

func (t Format) Title(queryParams, fileUri, filePath, fileName, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Format) Body(queryParams, fileUri, filePath, fileName, mime string) string {
	return fmt.Sprintf(`<a href="/%s"><video controls autoplay loop preload="auto"><source src="%s" type="%s" alt="Roulette selected: %s">Your browser does not support the video tag.</video></a>`,
		queryParams,
		fileUri,
		mime,
		fileName)
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.mp4`:  `video/mp4`,
		`.ogm`:  `video/ogg`,
		`.ogv`:  `video/ogg`,
		`.webm`: `video/webm`,
	}
}

func (t Format) MimeTypes() []string {
	return []string{
		`video/mp4`,
		`video/ogg`,
		`video/webm`,
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