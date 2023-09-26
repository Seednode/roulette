/*
Copyright © 2023 Seednode <seednode@seedno.de>
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
	return len(filters.included) != 0
}

func (filters *filters) includes() string {
	return strings.Join(filters.included, ",")
}

func (filters *filters) hasExcludes() bool {
	return len(filters.excluded) != 0
}

func (filters *filters) excludes() string {
	return strings.Join(filters.excluded, ",")
}

func (filters *filters) apply(fileList []string) []string {
	result := make([]string, len(fileList))

	copy(result, fileList)

	if filters.hasExcludes() {
		for _, exclude := range filters.excluded {
			result = slices.DeleteFunc(result, func(s string) bool {
				if CaseSensitive {
					return strings.Contains(s, filepath.Base(exclude))
				} else {
					return strings.Contains(strings.ToLower(s), strings.ToLower(filepath.Base(exclude)))
				}
			})
		}
	}

	if filters.hasIncludes() {
		result = slices.DeleteFunc(result, func(s string) bool {
			var delete bool = true

			p := filepath.Base(s)

			for _, include := range filters.included {
				if (CaseSensitive && strings.Contains(p, include)) || (!CaseSensitive && strings.Contains(strings.ToLower(p), strings.ToLower(include))) {
					delete = false
				}
			}

			return delete
		})
	}

	return result
}
