/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
)

type SupportedFormat struct {
	Css        string
	Title      func(queryParams, fileUri, filePath, fileName, mime string) string
	Body       func(queryParams, fileUri, filePath, fileName, mime string) string
	Extensions []string
	MimeTypes  []string
	Validate   func(filePath string) bool
}

type SupportedFormats struct {
	Extensions map[string]*SupportedFormat
	MimeTypes  map[string]*SupportedFormat
}

func (s *SupportedFormats) Add(t *SupportedFormat) {
	for _, v := range t.Extensions {
		_, exists := s.Extensions[v]
		if !exists {
			s.Extensions[v] = t
		}
	}

	for _, v := range t.MimeTypes {
		_, exists := s.Extensions[v]
		if !exists {
			s.MimeTypes[v] = t
		}
	}
}

func FileType(path string, registeredFormats *SupportedFormats) (bool, *SupportedFormat, string, error) {
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
	fileType, exists := registeredFormats.MimeTypes[mimeType]
	if exists {
		return fileType.Validate(path), fileType, mimeType, nil
	}

	// if mime type detection fails, use the file extension
	fileType, exists = registeredFormats.Extensions[filepath.Ext(path)]
	if exists {
		return fileType.Validate(path), fileType, mimeType, nil
	}

	return false, nil, "", nil
}
