/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

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
)

type Dimensions struct {
	Width  int
	Height int
}

func RegisterImages() *Type {
	return &Type{
		Css: func() string {
			var css strings.Builder

			css.WriteString(`html,body{margin:0;padding:0;height:100%;}`)
			css.WriteString(`a{color:inherit;display:block;height:100%;width:100%;text-decoration:none;}`)
			css.WriteString(`img{margin:auto;display:block;max-width:97%;max-height:97%;`)
			css.WriteString(`object-fit:scale-down;position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);}`)

			return css.String()
		},
		Title: func(queryParams, fileUri, filePath, fileName, mime string) string {
			dimensions, err := ImageDimensions(filePath)
			if err != nil {
				fmt.Println(err)
			}

			return fmt.Sprintf(`<title>%s (%dx%d)</title>`,
				fileName,
				dimensions.Width,
				dimensions.Height)
		},
		Body: func(queryParams, fileUri, filePath, fileName, mime string) string {
			dimensions, err := ImageDimensions(filePath)
			if err != nil {
				fmt.Println(err)
			}

			return fmt.Sprintf(`<a href="/%s"><img src="%s" width="%d" height="%d" type="%s" alt="Roulette selected: %s"></a>`,
				queryParams,
				fileUri,
				dimensions.Width,
				dimensions.Height,
				mime,
				fileName)
		},
		Extensions: map[string]string{
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
			`.svg`:   `image/svg+xml`,
			`.webp`:  `image/webp`,
		},
		MimeTypes: []string{
			`image/apng`,
			`image/avif`,
			`image/bmp`,
			`image/gif`,
			`image/jpeg`,
			`image/png`,
			`image/svg+xml`,
			`image/webp`,
		},
		Validate: func(filePath string) bool {
			return true
		},
	}
}

func ImageDimensions(path string) (*Dimensions, error) {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Printf("File %s does not exist\n", path)
		return &Dimensions{}, nil
	case err != nil:
		fmt.Printf("File %s open returned error: %s\n", path, err)
		return &Dimensions{}, err
	}
	defer file.Close()

	decodedConfig, _, err := image.DecodeConfig(file)
	switch {
	case errors.Is(err, image.ErrFormat):
		fmt.Printf("File %s has invalid image format\n", path)
		return &Dimensions{Width: 0, Height: 0}, nil
	case err != nil:
		fmt.Printf("File %s decode returned error: %s\n", path, err)
		return &Dimensions{}, err
	}

	return &Dimensions{Width: decodedConfig.Width, Height: decodedConfig.Height}, nil
}
