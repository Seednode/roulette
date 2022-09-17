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
	"time"

	"github.com/h2non/filetype"
)

func getNextFile(path string) (string, error) {
	re := regexp.MustCompile("(.+)([0-9]{3})(\\..+)")

	split := re.FindAllStringSubmatch(path, -1)

	base := split[0][1]
	number, err := strconv.Atoi(split[0][2])
	if err != nil {
		return "", nil
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

func getFiles(path string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if info.IsDir() && p != path {
			return filepath.SkipDir
		} else {
			absolutePath, err := filepath.Abs(p)
			if err != nil {
				return err
			}
			paths = append(paths, absolutePath)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func getFilesRecursive(path string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if !info.IsDir() {
			absolutePath, err := filepath.Abs(p)
			if err != nil {
				return err
			}
			paths = append(paths, absolutePath)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func getFileList(paths []string) ([]string, error) {
	fileList := []string{}

	for i := 0; i < len(paths); i++ {
		if Recursive {
			f, err := getFilesRecursive(paths[i])
			if err != nil {
				return nil, err
			}

			fileList = append(fileList, f...)
		} else {
			f, err := getFiles(paths[i])
			if err != nil {
				return nil, err
			}

			fileList = append(fileList, f...)
		}
	}

	return fileList, nil
}

func pickFile(args []string) (string, error) {
	fileList, err := getFileList(args)
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

	err = errors.New("no images found")

	return "", err
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
