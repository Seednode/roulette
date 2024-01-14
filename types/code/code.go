/*
Copyright © 2024 Seednode <seednode@seedno.de>
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

type Format struct {
	Fun   bool
	Theme string
}

func (t Format) Css() string {
	var css strings.Builder

	formatter := html.New(
		html.TabWidth(4),
		html.WithClasses(true),
		html.WrapLongLines(true))

	style := styles.Get(t.Theme)
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

	css.WriteString("html{height:100%;width:100%;}\n")
	css.WriteString("a{bottom:0;left:0;position:absolute;right:0;top:0;margin:1rem;padding:0;height:99%;width:99%;color:inherit;text-decoration:none;}\n")
	if t.Fun {
		css.WriteString("body{font-family: \"Comic Sans MS\", cursive, \"Brush Script MT\", sans-serif;}\n")
	}

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	return fmt.Sprintf(`<title>%s</title>`, fileName), nil
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
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

	style := styles.Get(t.Theme)
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(
		html.TabWidth(4),
		html.WithClasses(true),
		html.WrapLongLines(true))

	iterator, err := lexer.Tokenise(nil, contentString)
	if err != nil {
		return "", err
	}

	err = formatter.Format(w, style, iterator)
	if err != nil {
		return "", err
	}

	w.Flush()

	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`<a href="%s">%s</a>`,
		rootUrl,
		string(b)), nil
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.4th`:     ``,
		`.abap`:    ``,
		`.ada`:     ``,
		`.ahk`:     ``,
		`.as`:      ``,
		`.awk`:     ``,
		`.bat`:     ``,
		`.bib`:     ``,
		`.bibtex`:  ``,
		`.c`:       ``,
		`.clj`:     ``,
		`.cljc`:    ``,
		`.cljs`:    ``,
		`.coffee`:  ``,
		`.conf`:    ``,
		`.cpp`:     ``,
		`.cr`:      ``,
		`.cs`:      ``,
		`.css`:     ``,
		`.d`:       ``,
		`.elm`:     ``,
		`.erl`:     ``,
		`.ex`:      ``,
		`.exs`:     ``,
		`.f03`:     ``,
		`.f90`:     ``,
		`.f95`:     ``,
		`.fs`:      ``,
		`.go`:      ``,
		`.graphql`: ``,
		`.groovy`:  ``,
		`.gsh`:     ``,
		`.gvy`:     ``,
		`.gy`:      ``,
		`.hc`:      ``,
		`.hcl`:     ``,
		`.hs`:      ``,
		`.java`:    ``,
		`.jinja`:   ``,
		`.jl`:      ``,
		`.js`:      ``,
		`.json`:    ``,
		`.kt`:      ``,
		`.lisp`:    ``,
		`.lsp`:     ``,
		`.lua`:     ``,
		`.m`:       ``,
		`.md`:      ``,
		`.ml`:      ``,
		`.nb`:      ``,
		`.nim`:     ``,
		`.nix`:     ``,
		`.php`:     ``,
		`.pl`:      ``,
		`.pp`:      ``,
		`.proto`:   ``,
		`.ps`:      ``,
		`.ps1`:     ``,
		`.psl`:     ``,
		`.py`:      ``,
		`.r`:       ``,
		`.raku`:    ``,
		`.rb`:      ``,
		`.rs`:      ``,
		`.sass`:    ``,
		`.sc`:      ``,
		`.scm`:     ``,
		`.scss`:    ``,
		`.scpt`:    ``,
		`.sh`:      ``,
		`.sql`:     ``,
		`.swift`:   ``,
		`.tcl`:     ``,
		`.tex`:     ``,
		`.tf`:      ``,
		`.toml`:    ``,
		`.ts`:      ``,
		`.unit`:    ``,
		`.v`:       ``,
		`.vb`:      ``,
		`.xml`:     ``,
		`.yaml`:    ``,
		`.yml`:     ``,
		`.zig`:     ``,
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
	return "inline"
}

func init() {
	types.SupportedFormats.Register(Format{})
}
