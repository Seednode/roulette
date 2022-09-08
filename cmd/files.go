/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

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

func getFileList(args []string) ([]string, error) {
	fileList := []string{}

	for i := 0; i < len(args); i++ {
		if Recursive {
			f, err := getFilesRecursive(args[i])
			if err != nil {
				return nil, err
			}

			fileList = append(fileList, f...)
		} else {
			f, err := getFiles(args[i])
			if err != nil {
				return nil, err
			}

			fileList = append(fileList, f...)
		}
	}

	return fileList, nil
}

func pickFile(fileList []string) (string, string) {
	rand.Seed(time.Now().UnixMicro())

	filePath := fileList[rand.Intn(len(fileList))]
	fileName := filepath.Base(filePath)

	return fileName, filePath
}

func normalizePaths(args []string) []string {
	var paths []string

	for i := 0; i < len(args); i++ {
		absolutePath, err := filepath.Abs(args[i])
		if err != nil {
			panic(err)
		}

		paths = append(paths, absolutePath)
	}

	return paths
}
