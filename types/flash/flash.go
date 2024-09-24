/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package flash

import (
	"fmt"
	"strings"

	"seedno.de/seednode/roulette/types"
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
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<script src="https://unpkg.com/@ruffle-rs/ruffle"></script><script>window.RufflePlayer.config = {autoplay:"on"};</script><embed src="%s"></embed>`, fileUri))
	html.WriteString(`<br /><button id="next">Next</button>`)
	html.WriteString(fmt.Sprintf(`<script>window.addEventListener("load", function () { document.getElementById("next").addEventListener("click", function () { window.location.href = '%s'; }) }); </script>`, rootUrl))

	return html.String(), nil
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.swf`: `application/x-shockwave-flash`,
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
