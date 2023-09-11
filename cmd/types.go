/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
)

type SupportedType struct {
	title      func(queryParams, filePath, mime, fileName string, width, height int) string
	body       func(queryParams, filePath, mime, fileName string, width, height int) string
	extensions []string
	validator  func([]byte) bool
}

func (i *SupportedType) Extensions() []string {
	return i.extensions
}

type SupportedTypes struct {
	types []*SupportedType
}

func (s *SupportedTypes) Add(t *SupportedType) {
	s.types = append(s.types, t)
}

func (s *SupportedTypes) Extensions() []string {
	var r []string

	for _, t := range s.types {
		r = append(r, t.Extensions()...)
	}

	return r
}

func (s *SupportedTypes) Type(extension string) *SupportedType {
	for i := range s.types {
		for _, e := range s.types[i].Extensions() {
			if extension == e {
				return s.types[i]
			}
		}
	}

	return nil
}

func (s *SupportedTypes) IsSupported(head []byte) bool {
	r := false

	for i := range s.types {
		if s.types[i].validator(head) {
			r = true
		}
	}

	return r
}

func fileType(path string, types *SupportedTypes) (bool, *SupportedType, string, error) {
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

	extension := filepath.Ext(path)

	fileType := types.Type(extension)

	isSupported := types.IsSupported(head)
	if !isSupported {
		return false, nil, "", nil
	}

	mimeType := (filetype.GetType(strings.TrimPrefix(extension, "."))).MIME.Value

	return isSupported, fileType, mimeType, nil
}
