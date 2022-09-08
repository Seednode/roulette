/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func getRandomFile(fileList []os.DirEntry) string {
	rand.Seed(time.Now().Unix())

	file := fileList[rand.Intn(len(fileList))].Name()

	absolutePath, err := filepath.Abs(file)
	if err != nil {
		panic(err)
	}

	return absolutePath
}

func getFiles(path string) ([]string, error) {
	fileList, err := os.ReadDir(path)

	return fileList, err
}

func getFilesRecursive(path string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		//		if strings.HasPrefix(info.Name(), ".") {
		//			if info.IsDir() {
		//				return filepath.SkipDir
		//			}
		//			return err
		//		}
		if !info.IsDir() {
			paths = append(paths, p)
		}
		return err
	})

	rand.Seed(time.Now().Unix())

	file := paths[rand.Intn(len(paths))]
	fmt.Println(file)
	return paths, err
}

func getFileList(args []string) []string {
	fileList := []string{}

	for i := 0; i < len(args); i++ {
		if Recursive {
			f, err := getFilesRecursive(args[i])
			if err != nil {
				panic(err)
			}
			fileList = append(fileList, f...)
		} else {
			f, err := getFiles(args[i])
			if err != nil {
				panic(err)
			}
			fileList = append(fileList, f...)
		}
	}

	return fileList
}

func pickFile(fileList []string) (string, string) {
	rand.Seed(time.Now().Unix())

	filePath := fileList[rand.Intn(len(fileList))]
	fileName := filepath.Base(filePath)

	return filePath, fileName
}
