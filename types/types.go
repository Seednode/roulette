/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package types

import (
	"path/filepath"
	"slices"
	"strings"
)

var SupportedFormats = &Types{
	Extensions: make(map[string]Type),
}

type Type interface {
	Css() string
	Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) string
	Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) string
	Extensions() map[string]string
	MimeType(string) string
	Validate(filePath string) bool
}

type Types struct {
	Extensions map[string]Type
}

func (t *Types) Add(format Type) {
	for k := range format.Extensions() {
		_, exists := t.Extensions[k]
		if !exists {
			t.Extensions[k] = format
		}
	}
}

func (t *Types) FileType(path string) Type {
	fileType, exists := t.Extensions[filepath.Ext(path)]
	if exists {
		return fileType
	}

	return nil
}

func (t *Types) Register(format Type) {
	t.Add(format)
}

func (t *Types) Validate(path string) bool {
	format, exists := t.Extensions[filepath.Ext(path)]
	if !exists {
		return false
	}

	return format.Validate(path)
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

	var mimeTypes []string

	for _, j := range t.Extensions {
		extensions := j.Extensions()
		for _, v := range extensions {
			mimeTypes = append(mimeTypes, v)
		}
	}

	mimeTypes = removeDuplicateStr(mimeTypes)

	slices.Sort(mimeTypes)

	for _, v := range mimeTypes {
		output.WriteString(v + "\n")
	}

	return output.String()
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
