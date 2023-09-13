/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import "strings"

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
