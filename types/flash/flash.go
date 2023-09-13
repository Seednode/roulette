/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package flash

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

func (t Format) Title(queryParams, fileUri, filePath, fileName, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Format) Body(queryParams, fileUri, filePath, fileName, mime string) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<script src="https://unpkg.com/@ruffle-rs/ruffle"></script><script>window.RufflePlayer.config = {autoplay:"on"};</script><embed src="%s"></embed>`, fileUri))
	html.WriteString(fmt.Sprintf(`<br /><button onclick="window.location.href = '/%s';">Next</button>`, queryParams))

	return html.String()
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.swf`: `application/x-shockwave-flash`,
	}
}

func (t Format) MimeTypes() []string {
	return []string{
		`application/x-shockwave-flash`,
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
