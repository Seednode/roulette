/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"fmt"
	"strings"
)

type Flash struct{}

func (t Flash) Css() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)

	return css.String()
}

func (t Flash) Title(queryParams, fileUri, filePath, fileName, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Flash) Body(queryParams, fileUri, filePath, fileName, mime string) string {
	var html strings.Builder

	html.WriteString(fmt.Sprintf(`<script src="https://unpkg.com/@ruffle-rs/ruffle"></script><script>window.RufflePlayer.config = {autoplay:"on"};</script><embed src="%s"></embed>`, fileUri))
	html.WriteString(fmt.Sprintf(`<br /><button onclick="window.location.href = '/%s';">Next</button>`, queryParams))

	return html.String()
}

func (t Flash) Extensions() map[string]string {
	return map[string]string{
		`.swf`: `application/x-shockwave-flash`,
	}
}

func (t Flash) MimeTypes() []string {
	return []string{
		`application/x-shockwave-flash`,
	}
}

func (t Flash) Validate(filePath string) bool {
	return true
}
