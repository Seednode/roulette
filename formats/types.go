/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package formats

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
)

type FormatFunction func(queryParams, fileUri, filePath, fileName, mime string) string
type ValidatorFunction func([]byte) bool

type SupportedFormat struct {
	Title      FormatFunction
	Body       FormatFunction
	Extensions []string
	validator  ValidatorFunction
}

type SupportedFormats struct {
	types []*SupportedFormat
}

func (s *SupportedFormats) Add(t *SupportedFormat) {
	s.types = append(s.types, t)
}

func (s *SupportedFormats) Extensions() []string {
	var r []string

	for _, t := range s.types {
		r = append(r, t.Extensions...)
	}

	return r
}

func (s *SupportedFormats) Type(extension string) *SupportedFormat {
	for i := range s.types {
		for _, e := range s.types[i].Extensions {
			if extension == e {
				return s.types[i]
			}
		}
	}

	return nil
}

func (s *SupportedFormats) IsSupported(head []byte) bool {
	r := false

	for i := range s.types {
		if s.types[i].validator(head) {
			r = true
		}
	}

	return r
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

	head := make([]byte, 261)
	file.Read(head)

	if types.IsSupported(head) {
		extension := filepath.Ext(path)

		for _, v := range types.Extensions() {
			if extension == v {
				fileType := types.Type(extension)

				mimeType := (filetype.GetType(strings.TrimPrefix(extension, "."))).MIME.Value

				return true, fileType, mimeType, nil
			}
		}
	}

	return false, nil, "", nil
}
