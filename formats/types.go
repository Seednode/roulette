/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"errors"
	"net/http"
	"os"
)

type FormatFunction func(queryParams, fileUri, filePath, fileName, mime string) string

type SupportedFormat struct {
	Title      FormatFunction
	Body       FormatFunction
	Extensions []string
	MimeTypes  []string
}

type SupportedFormats struct {
	types []*SupportedFormat
}

func (s *SupportedFormats) Add(t *SupportedFormat) {
	s.types = append(s.types, t)
}

func (s *SupportedFormats) Extensions() []string {
	var extensions []string

	for _, t := range s.types {
		extensions = append(extensions, t.Extensions...)
	}

	return extensions
}

func (s *SupportedFormats) MimeTypes() []string {
	var mimeTypes []string

	for _, t := range s.types {
		mimeTypes = append(mimeTypes, t.MimeTypes...)
	}

	return mimeTypes
}

func (s *SupportedFormats) Type(mimeType string) *SupportedFormat {
	for i := range s.types {
		for _, m := range s.types[i].MimeTypes {
			if mimeType == m {
				return s.types[i]
			}
		}
	}

	return nil
}

func FileType(path string, types *SupportedFormats) (bool, *SupportedFormat, string, error) {
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

	for _, v := range types.MimeTypes() {
		if mimeType == v {
			fileType := types.Type(mimeType)

			return true, fileType, mimeType, nil
		}
	}

	return false, nil, "", nil
}
