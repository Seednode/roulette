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
	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return
	}

	splitPath.number = fmt.Sprintf("%0*d", len(splitPath.number), asInt+1)
}

func (splitPath *splitPath) decrement() {
	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return
	}

	splitPath.number = fmt.Sprintf("%0*d", len(splitPath.number), asInt-1)
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
