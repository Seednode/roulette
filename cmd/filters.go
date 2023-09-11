/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import "strings"

type Filters struct {
	includes []string
	excludes []string
}

func (f *Filters) IsEmpty() bool {
	return !(f.HasIncludes() || f.HasExcludes())
}

func (f *Filters) HasIncludes() bool {
	return len(f.includes) != 0
}

func (f *Filters) Includes() string {
	return strings.Join(f.includes, ",")
}

func (f *Filters) HasExcludes() bool {
	return len(f.excludes) != 0
}

func (f *Filters) Excludes() string {
	return strings.Join(f.excludes, ",")
}
