/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package types

import (
	"path/filepath"
	"slices"
	"strings"
)

var SupportedFormats = make(Types)

type Type interface {
	// Returns either "inline" or "embed", depending on whether the file
	// should be displayed inline (e.g. code) or embedded (e.g. images)
	Type() string

	// Returns a CSS string used to format the corresponding page
	Css() string

	// Returns an HTML <title> element for the specified file
	Title(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error)

	// Returns an HTML <body> element used to display the specified file
	Body(rootUrl, fileUri, filePath, fileName, prefix, mime string) (string, error)

	// Returns a map of file extensions to MIME type strings.
	Extensions() map[string]string

	// Given a file extension, returns the corresponding media type,
	// if one exists. Otherwise, returns an empty string.
	MediaType(extension string) string

	// Optional function used to validate whether a given file matches this format.
	// If no validation checks are needed, this function should always return true.
	Validate(filePath string) bool
}

type Types map[string]Type

func (t Types) Add(format Type) {
	for k := range format.Extensions() {
		_, exists := t[k]
		if !exists {
			t[k] = format
		}
	}
}

func (t Types) FileType(path string) Type {
	fileType, exists := t[filepath.Ext(path)]
	if exists {
		return fileType
	}

	return nil
}

func (t Types) Register(format Type) {
	t.Add(format)
}

func (t Types) Validate(path string) bool {
	format, exists := t[filepath.Ext(path)]
	if !exists {
		return false
	}

	return format.Validate(path)
}

func (t Types) GetExtensions() string {
	var output strings.Builder

	extensions := make([]string, len(t))

	i := 0

	for k := range t {
		extensions[i] = k
		i++
	}

	slices.Sort(extensions)

	for _, v := range extensions {
		output.WriteString(v + "\n")
	}

	return output.String()
}

func (t Types) GetMediaTypes() string {
	var output strings.Builder

	var mediaTypes []string

	for _, j := range t {
		extensions := j.Extensions()

		for _, v := range extensions {
			if v != "" {
				mediaTypes = append(mediaTypes, v)
			}
		}
	}

	mediaTypes = removeDuplicateStr(mediaTypes)

	slices.Sort(mediaTypes)

	for _, v := range mediaTypes {
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
