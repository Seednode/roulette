/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package images

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"seedno.de/seednode/roulette/types"
)

type dimensions struct {
	width  int
	height int
}

type Format struct{}

func (t Format) Css() string {
	var css strings.Builder

	css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
	css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)
	css.WriteString(`img{margin:auto;display:block;max-width:97%;max-height:97%;`)
	css.WriteString(`object-fit:scale-down;position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}`)

	return css.String()
}

func (t Format) Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	dimensions, err := ImageDimensions(filePath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`<title>%s (%dx%d)</title>`,
		fileName,
		dimensions.width,
		dimensions.height), nil
}

func (t Format) Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error) {
	dimensions, err := ImageDimensions(filePath)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`<a href="%s"><img src="%s" width="%d" height="%d" type="%s" alt="Roulette selected: %s"></a>`,
		rootUrl,
		fileUri,
		dimensions.width,
		dimensions.height,
		mime,
		fileName), nil
}

func (t Format) Extensions() map[string]string {
	return map[string]string{
		`.apng`:  `image/apng`,
		`.avif`:  `image/avif`,
		`.bmp`:   `image/bmp`,
		`.gif`:   `image/gif`,
		`.jpg`:   `image/jpeg`,
		`.jpeg`:  `image/jpeg`,
		`.jfif`:  `image/jpeg`,
		`.pjp`:   `image/jpeg`,
		`.pjpeg`: `image/jpeg`,
		`.png`:   `image/png`,
		`.webp`:  `image/webp`,
	}
}

func (t Format) Validate(filePath string) bool {
	return true
}

func (t Format) MimeType(extension string) string {
	extensions := t.Extensions()

	value, exists := extensions[extension]
	if exists {
		return value
	}

	return ""
}

func ImageDimensions(path string) (*dimensions, error) {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Printf("File %s does not exist\n", path)
		return &dimensions{}, nil
	case err != nil:
		fmt.Printf("File %s open returned error: %s\n", path, err)
		return &dimensions{}, err
	}
	defer file.Close()

	decodedConfig, _, err := image.DecodeConfig(file)
	switch {
	case errors.Is(err, image.ErrFormat):
		fmt.Printf("File %s has invalid image format\n", path)
		return &dimensions{width: 0, height: 0}, nil
	case err != nil:
		fmt.Printf("File %s decode returned error: %s\n", path, err)
		return &dimensions{}, err
	}

	return &dimensions{width: decodedConfig.Width, height: decodedConfig.Height}, nil
}

func New() Format {
	return Format{}
}

func init() {
	types.SupportedFormats.Register(New())
}
