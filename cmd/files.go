/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func checkIfImage(path string) (bool, error) {
	magicNumber := make([]byte, 3)

	file, err := os.Open(path)
	if err != nil {
		return false, err
	}

	_, err = io.ReadFull(file, magicNumber)
	if err != nil {
		return false, err
	}

	switch {
	case bytes.Compare(magicNumber, []byte{0xFF, 0xD8, 0xFF}) == 0: // JPG
		return true, nil
	case bytes.Compare(magicNumber, []byte{0x89, 0x50, 0x4E}) == 0: // PNG
		return true, nil
	case bytes.Compare(magicNumber, []byte{0x47, 0x49, 0x46}) == 0: // GIF
		return true, nil
	default:
		return false, nil
	}
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

func pickFile(fileList []string) (string, string, error) {
	rand.Seed(time.Now().UnixNano())

	rand.Shuffle(len(fileList), func(i, j int) { fileList[i], fileList[j] = fileList[j], fileList[i] })

	var filePath string
	var fileName string

	for i := 0; i < len(fileList); i++ {
		filePath = fileList[i]
		fileName = filepath.Base(filePath)
		isImage, err := checkIfImage(filePath)
		if err != nil {
			return "", "", err
		}
		if isImage {
			return fileName, filePath, nil
		}
	}

	err := errors.New("no images found")

	return "", "", err
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
