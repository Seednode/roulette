/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func pickFile(fileList []fs.DirEntry) string {
	rand.Seed(time.Now().Unix())

	isFile := false

	var fileName string

	for isFile == false {
		file := fileList[rand.Intn(len(fileList))]

		if file.IsDir() == false {
			isFile = true
			fileName = file.Name()
		}
	}

	return fileName
}

func getRandomFile(fileList []os.DirEntry) string {
	rand.Seed(time.Now().Unix())

	file := fileList[rand.Intn(len(fileList))].Name()

	absolutePath, err := filepath.Abs(file)
	if err != nil {
		panic(err)
	}

	return absolutePath
}

func getFiles(path string) []fs.DirEntry {
	fileList, err := os.ReadDir(path)
	if err != nil {
		panic(err)
	}

	return fileList
}

func getFile(args []string) (string, string) {
	fileList := []fs.DirEntry{}

	for i := 0; i < len(args); i++ {
		f := getFiles(args[i])
		fileList = append(fileList, f...)
	}

	fileName := pickFile(fileList)

	filePath, err := filepath.Abs(fileName)
	if err != nil {
		panic(err)
	}

	return fileName, filePath
}
