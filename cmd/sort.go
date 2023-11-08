/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"sort"
	"strings"

	"strconv"

	"seedno.de/seednode/roulette/types"
)

type splitPath struct {
	base      string
	number    string
	extension string
}

func (splitPath *splitPath) increment() string {
	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%0*d", len(splitPath.number), asInt+1)
}

func (splitPath *splitPath) decrement() string {
	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%0*d", len(splitPath.number), asInt-1)
}

func split(path string, regexes *regexes) (*splitPath, error) {
	split := regexes.filename.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return &splitPath{}, nil
	}

	p := &splitPath{
		base:      split[0][1],
		number:    split[0][2],
		extension: split[0][3],
	}

	return p, nil
}

func getRange(path string, regexes *regexes, index *fileIndex) (string, string, error) {
	splitPath, err := split(path, regexes)
	if err != nil {
		return "", "", err
	}

	list := index.List()

	sort.Slice(list, func(i, j int) bool {
		return list[i] <= list[j]
	})

	var first, last, previous string

Loop:
	for _, val := range list {
		splitVal, err := split(val, regexes)
		if err != nil {
			return "", "", err
		}

		switch {
		case splitVal.base == splitPath.base && first == "":
			first = val
		case splitVal.base != splitPath.base && first != "":
			last = previous

			break Loop
		}

		previous = val
	}

	return first, last, nil
}

func paginateSorted(path, first, last, queryParams string, regexes *regexes, formats types.Types) (string, error) {
	split, err := split(path, regexes)
	if err != nil {
		return "", err
	}

	var firstStatus, prevStatus, nextStatus, lastStatus string = "", "", "", ""

	if path <= first {
		firstStatus = " disabled"
		prevStatus = " disabled"
	}

	if path >= last {
		nextStatus = " disabled"
		lastStatus = " disabled"
	}

	prevPath := &splitPath{
		base:      split.base,
		number:    split.decrement(),
		extension: split.extension,
	}

	prevPage, err := tryExtensions(prevPath, formats)
	switch {
	case err != nil:
		return "", err
	case prevPage == "":
		prevStatus = " disabled"
	case prevPage < first:
		prevPage = first
	}

	nextPath := &splitPath{
		base:      split.base,
		number:    split.increment(),
		extension: split.extension,
	}

	nextPage, err := tryExtensions(nextPath, formats)
	switch {
	case err != nil:
		return "", err
	case nextPage == "":
		nextStatus = " disabled"
	case nextPage > last:
		nextPage = last
	}

	var html strings.Builder

	html.WriteString(`<table style="margin-left:auto;margin-right:auto;"><tr><td>`)

	html.WriteString(fmt.Sprintf(`<button onclick='window.location.href = "%s%s%s%s";'%s>First</button>`,
		Prefix,
		mediaPrefix,
		first,
		queryParams,
		firstStatus))

	html.WriteString(fmt.Sprintf(`<button onclick='window.location.href = "%s%s%s%s";'%s>Prev</button>`,
		Prefix,
		mediaPrefix,
		prevPage,
		queryParams,
		prevStatus))

	html.WriteString(fmt.Sprintf(`<button onclick='window.location.href = "%s%s%s%s";'%s>Next</button>`,
		Prefix,
		mediaPrefix,
		nextPage,
		queryParams,
		nextStatus))

	html.WriteString(fmt.Sprintf(`<button onclick='window.location.href = "%s%s%s%s";'%s>Last</button>`,
		Prefix,
		mediaPrefix,
		last,
		queryParams,
		lastStatus))

	html.WriteString("</td></tr></table>")

	return html.String(), nil
}
