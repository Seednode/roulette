/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"

	"strconv"
)

type splitPath struct {
	base      string
	number    string
	extension string
}

func (splitPath *splitPath) increment() {
	length := len(splitPath.number)

	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return
	}

	splitPath.number = fmt.Sprintf("%0*d", length, asInt+1)
}

func (splitPath *splitPath) decrement() {
	length := len(splitPath.number)

	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return
	}

	splitPath.number = fmt.Sprintf("%0*d", length, asInt-1)
}

func split(path string, regexes *regexes) (*splitPath, int, error) {
	p := splitPath{}

	split := regexes.filename.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return &splitPath{}, 0, nil
	}

	p.base = split[0][1]

	p.number = split[0][2]

	p.extension = split[0][3]

	return &p, len(p.number), nil
}
