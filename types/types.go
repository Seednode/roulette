/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var SupportedFormats = &Types{
	Extensions: make(map[string]string),
	MimeTypes:  make(map[string]Type),
}

type Type interface {
	Css() string
	Title(queryParams, fileUri, filePath, fileName, mime string) string
	Body(queryParams, fileUri, filePath, fileName, mime string) string
	Extensions() map[string]string
	MimeTypes() []string
	Validate(filePath string) bool
}

type Types struct {
	Extensions map[string]string
	MimeTypes  map[string]Type
}

func (t *Types) Add(format Type) {
	for k, v := range format.Extensions() {
		_, exists := t.Extensions[k]
		if !exists {
			t.Extensions[k] = v
		}
	}

	for _, v := range format.MimeTypes() {
		_, exists := t.Extensions[v]
		if !exists {
			t.MimeTypes[v] = format
		}
	}
}

func (t *Types) FileType(path string) (bool, Type, string, error) {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil, "", nil
	case err != nil:
		return false, nil, "", err
	}
	defer file.Close()

	head := make([]byte, 512)
	file.Read(head)

	mimeType := http.DetectContentType(head)

	// try identifying files by mime types first
	fileType, exists := t.MimeTypes[mimeType]
	if exists {
		return fileType.Validate(path), fileType, mimeType, nil
	}

	// if mime type detection fails, use the file extension
	mimeType, exists = t.Extensions[filepath.Ext(path)]
	if exists {
		fileType, exists := t.MimeTypes[mimeType]

		if exists {
			return fileType.Validate(path), fileType, mimeType, nil
		}
	}

	return false, nil, "", nil
}

func (t *Types) Register(format Type) {
	t.Add(format)
}

func (t *Types) GetExtensions() string {
	var output strings.Builder

	extensions := make([]string, len(t.Extensions))

	i := 0

	for k := range t.Extensions {
		extensions[i] = k
		i++
	}

	slices.Sort(extensions)

	for _, v := range extensions {
		output.WriteString(v + "\n")
	}

	return output.String()
}

func (t *Types) GetMimeTypes() string {
	var output strings.Builder

	mimeTypes := make([]string, len(t.MimeTypes))

	i := 0

	for k := range t.MimeTypes {
		mimeTypes[i] = k
		i++
	}

	slices.Sort(mimeTypes)

	for _, v := range mimeTypes {
		output.WriteString(v + "\n")
	}

	return output.String()
}
