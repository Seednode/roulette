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

func appendPaths(m map[string][]string, path, filter string) (map[string][]string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	directory, _ := filepath.Split(absolutePath)

	if filter != "" && strings.Contains(path, filter) {
		m[directory] = append(m[directory], path)
	} else if filter == "" {
		m[directory] = append(m[directory], path)
	}

	return m, nil
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

	nextFile, err := nextFileExists(fileName)
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

	nextFile, err := nextFileExists(fileName)
	if err != nil {
		return "", err
	}

	if !nextFile {
		return "", nil
	}

	return fileName, nil
}

func pathIsValid(filePath string, paths []string) bool {
	var matchesPrefix = false
	for i := 0; i < len(paths); i++ {
		if strings.HasPrefix(filePath, paths[i]) {
			matchesPrefix = true
		}
	}
	if !matchesPrefix {
		if Verbose {
			fmt.Printf("%v Failed to serve file outside specified path(s): %v\n", time.Now().Format(LOGDATE), filePath)
		}

		return false
	}

	return true
}

func fileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		if Verbose {
			fmt.Printf("%v Failed to serve non-existent file: %v\n", time.Now().Format(LOGDATE), filePath)
		}

		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) && err != nil {
		return false, err
	}

	return true, nil
}

func nextFileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func isImage(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	head := make([]byte, 261)
	file.Read(head)

	return filetype.IsImage(head), nil
}

func getFiles(m map[string][]string, path, filter string) (map[string][]string, error) {
	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch {
		case !Recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case filter != "" && !info.IsDir():
			m, err = appendPaths(m, p, filter)
			if err != nil {
				return err
			}
		case !info.IsDir():
			m, err = appendPaths(m, p, "")
			if err != nil {
				return err
			}
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func getFileList(paths []string, filter string) (map[string][]string, error) {
	fileMap := map[string][]string{}
	var err error

	for i := 0; i < len(paths); i++ {
		fileMap, err = getFiles(fileMap, paths[i], filter)
		if err != nil {
			return nil, err
		}
	}

	return fileMap, nil
}

func cleanFilename(filename string) string {
	return filename[:len(filename)-(len(filepath.Ext(filename))+3)]
}

func prepareDirectory(directory []string) []string {
	_, first := filepath.Split(directory[0])
	first = cleanFilename(first)

	_, last := filepath.Split(directory[len(directory)-1])
	last = cleanFilename(last)

	if first == last {
		d := append([]string{}, directory[0])
		return d
	} else {
		return directory
	}
}

func prepareDirectories(m map[string][]string, successive string) []string {
	directories := []string{}

	keys := make([]string, len(m))

	i := 0
	for k := range m {
		keys[i] = k
		i++
	}

	if successive != "" {
		for i := 0; i < len(keys); i++ {
			directories = append(directories, prepareDirectory(m[keys[i]])...)
		}
	} else {
		for i := 0; i < len(keys); i++ {
			directories = append(directories, m[keys[i]]...)
		}
	}

	return directories
}

func pickFile(args []string, filter, successive string) (string, error) {
	fileMap, err := getFileList(args, filter)
	if err != nil {
		return "", err
	}

	fileList := prepareDirectories(fileMap, successive)

	rand.Seed(time.Now().UnixNano())

	rand.Shuffle(len(fileList), func(i, j int) { fileList[i], fileList[j] = fileList[j], fileList[i] })

	for i := 0; i < len(fileList); i++ {
		filePath := fileList[i]
		isImage, err := isImage(filePath)
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
