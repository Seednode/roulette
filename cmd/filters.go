/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
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
			result = slices.DeleteFunc(fileList, func(s string) bool {
				return strings.Contains(strings.ToLower(s), strings.ToLower(exclude))
			})
		}
	}

	if filters.hasIncludes() {
		for _, include := range filters.included {
			result = slices.DeleteFunc(fileList, func(s string) bool {
				return !strings.Contains(strings.ToLower(s), strings.ToLower(include))
			})
		}
	}

	return result
}
