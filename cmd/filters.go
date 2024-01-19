/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"path/filepath"
	"slices"
	"strings"
)

type filters struct {
	included []string
	excluded []string
}

func (filters *filters) isEmpty() bool {
	return !(filters.hasIncludes() || filters.hasExcludes())
}

func (filters *filters) hasIncludes() bool {
	return len(filters.included) != 0 && Filtering
}

func (filters *filters) includes() string {
	return strings.Join(filters.included, ",")
}

func (filters *filters) hasExcludes() bool {
	return len(filters.excluded) != 0 && Filtering
}

func (filters *filters) excludes() string {
	return strings.Join(filters.excluded, ",")
}

func (filters *filters) apply(fileList []string) []string {
	result := make([]string, len(fileList))

	copy(result, fileList)

	if filters.hasExcludes() {
		result = slices.DeleteFunc(result, func(s string) bool {
			p := filepath.Base(s)

			for _, exclude := range filters.excluded {
				if (!CaseInsensitive && strings.Contains(p, exclude)) || (CaseInsensitive && strings.Contains(strings.ToLower(p), strings.ToLower(exclude))) {
					return true
				}
			}

			return false
		})
	}

	if filters.hasIncludes() {
		result = slices.DeleteFunc(result, func(s string) bool {
			p := filepath.Base(s)

			for _, include := range filters.included {
				if (!CaseInsensitive && strings.Contains(p, include)) || (CaseInsensitive && strings.Contains(strings.ToLower(p), strings.ToLower(include))) {
					return false
				}
			}

			return true
		})
	}

	return result
}
