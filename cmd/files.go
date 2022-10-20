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

type Stats struct {
	FilesMatched       uint64
	FilesSkipped       uint64
	DirectoriesMatched uint64
	DirectoriesSkipped uint64
}

type Path struct {
	Base      string
	Number    int
	Extension string
}

func (p *Path) Increment() {
	p.Number = p.Number + 1
}

func (p *Path) Decrement() {
	p.Number = p.Number - 1
}

var (
	ErrNoImagesFound = fmt.Errorf("no supported image formats found")
)

func appendPaths(m map[string][]string, path string, filters *Filters, stats *Stats) (map[string][]string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	directory, filename := filepath.Split(absolutePath)

	filename = strings.ToLower(filename)

	switch {
	case filters.HasIncludes() && !filters.HasExcludes():
		for i := 0; i < len(filters.Includes); i++ {
			if strings.Contains(
				filename,
				filters.Includes[i],
			) {
				m[directory] = append(m[directory], path)
				stats.FilesMatched += 1

				return m, nil
			}
		}

		stats.FilesSkipped += 1

		return m, nil
	case !filters.HasIncludes() && filters.HasExcludes():
		for i := 0; i < len(filters.Excludes); i++ {
			if strings.Contains(
				filename,
				filters.Excludes[i],
			) {
				stats.FilesSkipped += 1

				return m, nil
			}
		}

		m[directory] = append(m[directory], path)
		stats.FilesMatched += 1

		return m, nil
	case filters.HasIncludes() && filters.HasExcludes():
		for i := 0; i < len(filters.Excludes); i++ {
			if strings.Contains(
				filename,
				filters.Excludes[i],
			) {
				stats.FilesSkipped += 1

				return m, nil
			}
		}

		for i := 0; i < len(filters.Includes); i++ {
			if strings.Contains(
				filename,
				filters.Includes[i],
			) {
				m[directory] = append(m[directory], path)
				stats.FilesMatched += 1

				return m, nil
			}
		}

		stats.FilesSkipped += 1

		return m, nil
	default:
		m[directory] = append(m[directory], path)
		stats.FilesMatched += 1

		return m, nil
	}
}

func getFirstFile(p *Path) (string, error) {
	p.Number = 1

	fileName, err := tryExtensions(p)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func getLastFile(p *Path) (string, error) {
	var fileName string
	var err error

	p.Number = 1

	for {
		p.Increment()

		fileName, err = tryExtensions(p)
		if err != nil {
			return "", err
		}

		if fileName == "" {
			p.Decrement()

			fileName, err = tryExtensions(p)
			if err != nil {
				return "", err
			}

			break
		}
	}

	return fileName, nil
}

func getNextFile(p *Path) (string, error) {
	p.Increment()

	fileName, err := tryExtensions(p)
	if err != nil {
		return "", err
	}

	return fileName, err
}

func getPreviousFile(p *Path) (string, error) {
	p.Decrement()

	fileName, err := tryExtensions(p)
	if err != nil {
		return "", err
	}

	return fileName, err
}

func splitPath(path string) (*Path, error) {
	p := Path{}
	var err error

	re := regexp.MustCompile(`(.+)([0-9]{3})(\..+)`)

	split := re.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return &Path{}, nil
	}

	p.Base = split[0][1]

	p.Number, err = strconv.Atoi(split[0][2])
	if err != nil {
		return &Path{}, err
	}

	p.Extension = split[0][3]

	return &p, nil
}

func tryExtensions(p *Path) (string, error) {
	extensions := [6]string{p.Extension, ".jpg", ".jpeg", ".png", ".gif", ".webp"}

	var fileName string

	for _, i := range extensions {
		fileName = fmt.Sprintf("%v%.3d%v", p.Base, p.Number, i)

		exists, err := fileExists(fileName)
		if err != nil {
			return "", err
		}

		if exists {
			return fileName, nil
		}
	}

	return "", nil
}

func fileExists(path string) (bool, error) {
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

func getFiles(m map[string][]string, path string, filters *Filters, stats *Stats) (map[string][]string, error) {
	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && Filter && filters.HasExcludes() {
			for i := 0; i < len(filters.Excludes); i++ {
				if strings.Contains(
					strings.ToLower(p),
					strings.ToLower(filters.Excludes[i]),
				) {
					stats.DirectoriesSkipped += 1

					return filepath.SkipDir
				}
			}
		}

		switch {
		case !Recursive && info.IsDir() && p != path:
			stats.DirectoriesSkipped += 1

			return filepath.SkipDir
		case !info.IsDir():
			m, err = appendPaths(m, p, filters, stats)
			if err != nil {
				return err
			}
		case info.IsDir():
			stats.DirectoriesMatched += 1
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func getFileList(paths []string, filters *Filters, stats *Stats) (map[string][]string, error) {
	fileMap := map[string][]string{}
	var err error

	for i := 0; i < len(paths); i++ {
		fileMap, err = getFiles(fileMap, paths[i], filters, stats)
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
		return append([]string{}, directory[0])
	} else {
		return directory
	}
}

func prepareDirectories(m map[string][]string, sort string) []string {
	directories := []string{}

	keys := make([]string, len(m))

	i := 0
	for k := range m {
		keys[i] = k
		i++
	}

	if sort == "asc" || sort == "desc" {
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

func pickFile(args []string, filters *Filters, sort string) (string, error) {
	stats := Stats{}

	fileMap, err := getFileList(args, filters, &stats)
	if err != nil {
		return "", err
	}

	if Count {
		fmt.Printf("Choosing from %v files (skipped %v) out of %v directories (skipped %v)\n",
			stats.FilesMatched,
			stats.FilesSkipped,
			stats.DirectoriesMatched,
			stats.DirectoriesSkipped,
		)
	}

	fileList := prepareDirectories(fileMap, sort)

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
