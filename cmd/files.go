/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/filetype"
)

var (
	ErrNoImagesFound = fmt.Errorf("no supported image formats found")
)

func appendPaths(paths []string, path, filter string) ([]string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return paths, err
	}

	switch {
	case filter != "" && strings.Contains(path, filter):
		paths = append(paths, absolutePath)
	case filter == "":
		paths = append(paths, absolutePath)
	}

	return paths, nil
}

func getFirstFile(path string) (string, error) {
	re := regexp.MustCompile(`(.+)([0-9]{3})(\..+)`)

	split := re.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return "", nil
	}

	base := split[0][1]
	number := 1
	extension := split[0][3]

	fileName := fmt.Sprintf("%v%.3d%v", base, number, extension)

	nextFile, err := checkNextFile(fileName)
	if err != nil {
		return "", err
	}

	if !nextFile {
		return "", nil
	}

	return fileName, nil
}

func getNextFile(path string) (string, error) {
	re := regexp.MustCompile(`(.+)([0-9]{3})(\..+)`)

	split := re.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return "", nil
	}

	base := split[0][1]
	number, err := strconv.Atoi(split[0][2])
	if err != nil {
		return "", err
	}
	extension := split[0][3]

	incremented := number + 1

	fileName := fmt.Sprintf("%v%.3d%v", base, incremented, extension)

	nextFile, err := checkNextFile(fileName)
	if err != nil {
		return "", err
	}

	if !nextFile {
		return "", nil
	}

	return fileName, nil
}

func checkNextFile(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

func checkIfImage(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	head := make([]byte, 261)
	file.Read(head)

	if filetype.IsImage(head) {
		return true, nil
	}

	return false, nil
}

func getFiles(path, filter string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch {
		case !Recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case Filter != "" && !info.IsDir():
			paths, err = appendPaths(paths, p, Filter)
			if err != nil {
				return err
			}
		case filter != "" && !info.IsDir():
			paths, err = appendPaths(paths, p, filter)
			if err != nil {
				return err
			}
		default:
			paths, err = appendPaths(paths, p, "")
			if err != nil {
				return err
			}
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func getFileList(paths []string, filter string) ([]string, error) {
	fileList := []string{}

	for i := 0; i < len(paths); i++ {

		f, err := getFiles(paths[i], filter)
		if err != nil {
			return nil, err
		}

		fileList = append(fileList, f...)
	}

	return fileList, nil
}

func pickFile(args []string, filter string) (string, error) {
	fileList, err := getFileList(args, filter)
	if err != nil {
		return "", err
	}

	rand.Seed(time.Now().UnixNano())

	rand.Shuffle(len(fileList), func(i, j int) { fileList[i], fileList[j] = fileList[j], fileList[i] })

	for i := 0; i < len(fileList); i++ {
		filePath := fileList[i]
		isImage, err := checkIfImage(filePath)
		if err != nil {
			return "", err
		}
		if isImage {
			return filePath, nil
		}
	}

	return "", ErrNoImagesFound
}

func normalizePaths(args []string) ([]string, error) {
	var paths []string

	for i := 0; i < len(args); i++ {
		absolutePath, err := filepath.Abs(args[i])
		if err != nil {
			return nil, err
		}

		paths = append(paths, absolutePath)
	}

	return paths, nil
}
