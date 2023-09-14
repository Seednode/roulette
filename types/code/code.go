/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package code

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"seedno.de/seednode/roulette/types"
)

type Format struct{}

func (t Format) Css() string {
	var css strings.Builder

	formatter := html.New(
		html.LineNumbersInTable(true),
		html.TabWidth(4),
		html.WithClasses(true),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, ""),
		html.WrapLongLines(true))

	style := styles.Get("solarized-dark256")
	if style == nil {
		style = styles.Fallback
	}

	var response bytes.Buffer
	w := bufio.NewWriter(&response)
	r := bufio.NewReader(&response)

	err := formatter.WriteCSS(w, style)
	if err != nil {
		return ""
	}

	w.Flush()

	b, err := io.ReadAll(r)
	if err != nil {
		return ""
	}

	css.Write(b)

	css.WriteString("a{margin:0;padding:0;height:100%;width:100%;color:inherit;text-decoration:none;}\n")
	css.WriteString("html{background-color:#1c1c1c;}\n")

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) string {
	return fmt.Sprintf(`<title>%s</title>`, fileName)
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) string {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	contentString := string(contents)

	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Analyse(contentString)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	var response bytes.Buffer
	w := bufio.NewWriter(&response)
	r := bufio.NewReader(&response)

	style := styles.Get("solarized-dark256")
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(
		html.LineNumbersInTable(true),
		html.TabWidth(4),
		html.WithClasses(true),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, ""),
		html.WrapLongLines(true))

	iterator, err := lexer.Tokenise(nil, contentString)
	if err != nil {
		return ""
	}

	err = formatter.Format(w, style, iterator)
	if err != nil {
		return ""
	}

	w.Flush()

	b, err := io.ReadAll(r)
	if err != nil {
		return ""
	}

	return fmt.Sprintf(`<a href="%s">%s</a>`,
		rootUrl,
		string(b))
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.go`: `text/plain`,
		`.sh`: `application/x-sh`,
	}
}

func (t Format) MimeType(extension string) string {
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

func New() Format {
	return Format{}
}

func init() {
	types.SupportedFormats.Register(New())
}
