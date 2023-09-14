/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package text

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"seedno.de/seednode/roulette/types"
)

type Format struct{}

func (t Format) Css() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;overflow:hidden;}`)
	css.WriteString(`textarea{border:none;caret-color:transparent;outline:none;margin:.5rem;`)
	css.WriteString(`height:99%;width:99%;white-space:pre;overflow:auto;}`)

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) string {
	body, err := os.ReadFile(filePath)
	if err != nil {
		body = []byte{}
	}

	return fmt.Sprintf(`<a href="%s"><textarea autofocus readonly>%s</textarea></a>`,
		rootUrl,
		body)
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.css`:  `text/css`,
		`.csv`:  `text/csv`,
		`.js`:   `text/javascript`,
		`.json`: `application/json`,
		`.md`:   `text/markdown`,
		`.ps1`:  `text/plain`,
		`.sh`:   `application./x-sh`,
		`.toml`: `application/toml`,
		`.txt`:  `text/plain`,
		`.xml`:  `application/xml`,
		`.yml`:  `application/yaml`,
		`.yaml`: `application/yaml`,
	}
}

func (t Format) MimeTypes() []string {
	return []string{
		`application/json`,
		`application/toml`,
		`application/xml`,
		`application/yaml`,
		`text/css`,
		`text/csv`,
		`text/javascript`,
		`text/plain`,
		`text/plain; charset=utf-8`,
	}
}

func (t Format) Validate(filePath string) bool {
	file, err := os.Open(filePath)
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
}

func New() Format {
	return Format{}
}

func init() {
	types.SupportedFormats.Register(New())
}
