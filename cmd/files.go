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
			paths = append(paths, p)
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
			paths = append(paths, p)
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
	rand.Seed(time.Now().Unix())

	filePath := fileList[rand.Intn(len(fileList))]
	fileName := filepath.Base(filePath)

	return filePath, fileName
}
