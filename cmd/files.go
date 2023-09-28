/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"

	"crypto/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"seedno.de/seednode/roulette/types"
)

type regexes struct {
	alphanumeric *regexp.Regexp
	filename     *regexp.Regexp
}

type scanStats struct {
	filesMatched       int
	filesSkipped       int
	directoriesMatched int
	directoriesSkipped int
}

type scanStatsChannels struct {
	filesMatched       chan int
	filesSkipped       chan int
	directoriesMatched chan int
	directoriesSkipped chan int
}

type splitPath struct {
	base      string
	number    string
	extension string
}

func (splitPath *splitPath) increment() {
	length := len(splitPath.number)

	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return
	}

	splitPath.number = fmt.Sprintf("%0*d", length, asInt+1)
}

func (splitPath *splitPath) decrement() {
	length := len(splitPath.number)

	asInt, err := strconv.Atoi(splitPath.number)
	if err != nil {
		return
	}

	splitPath.number = fmt.Sprintf("%0*d", length, asInt-1)
}

func humanReadableSize(bytes int) string {
	const unit = 1000

	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0

	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}

func kill(path string, cache *fileCache) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}

	if Cache {
		cache.remove(path)
	}

	return nil
}

func split(path string, regexes *regexes) (*splitPath, int, error) {
	p := splitPath{}

	split := regexes.filename.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return &splitPath{}, 0, nil
	}

	p.base = split[0][1]

	p.number = split[0][2]

	p.extension = split[0][3]

	return &p, len(p.number), nil
}

func newFile(list []string, sortOrder string, regexes *regexes, formats *types.Types) (string, error) {
	path, err := pickFile(list)
	if err != nil {
		return "", err
	}

	if sortOrder == "asc" || sortOrder == "desc" {
		splitPath, length, err := split(path, regexes)
		if err != nil {
			return "", err
		}

		switch {
		case sortOrder == "asc":
			splitPath.number = fmt.Sprintf("%0*d", length, 1)

			path, err = tryExtensions(splitPath, formats)
			if err != nil {
				return "", err
			}
		case sortOrder == "desc":
			for {
				splitPath.increment()

				path, err = tryExtensions(splitPath, formats)
				if err != nil {
					return "", err
				}

				if path == "" {
					splitPath.decrement()

					path, err = tryExtensions(splitPath, formats)
					if err != nil {
						return "", err
					}

					break
				}
			}
		}
	}

	return path, nil
}

func nextFile(filePath, sortOrder string, regexes *regexes, formats *types.Types) (string, error) {
	splitPath, _, err := split(filePath, regexes)
	if err != nil {
		return "", err
	}

	switch {
	case sortOrder == "asc":
		splitPath.increment()
	case sortOrder == "desc":
		splitPath.decrement()
	default:
		return "", nil
	}

	path, err := tryExtensions(splitPath, formats)
	if err != nil {
		return "", err
	}

	return path, err
}

func tryExtensions(splitPath *splitPath, formats *types.Types) (string, error) {
	var path string

	for extension := range formats.Extensions {
		path = fmt.Sprintf("%s%s%s", splitPath.base, splitPath.number, extension)

		exists, err := fileExists(path)
		if err != nil {
			return "", err
		}

		if exists {
			return path, nil
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

func pathIsValid(path string, paths []string) bool {
	var matchesPrefix = false

	for i := 0; i < len(paths); i++ {
		if strings.HasPrefix(path, paths[i]) {
			matchesPrefix = true
		}
	}

	switch {
	case Verbose && !matchesPrefix:
		fmt.Printf("%s | ERROR: File outside specified path(s): %s\n",
			time.Now().Format(logDate),
			path,
		)

		return false
	case !matchesPrefix:
		return false
	default:
		return true
	}
}

func hasSupportedFiles(path string, formats *types.Types) (bool, error) {
	hasRegisteredFiles := make(chan bool, 1)

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch {
		case !Recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case !info.IsDir() && formats.Validate(p):
			hasRegisteredFiles <- true

			return filepath.SkipAll
		}

		return err
	})
	if err != nil {
		return false, err
	}

	select {
	case <-hasRegisteredFiles:
		return true, nil
	default:
		return false, nil
	}
}

func pathCount(path string) (int, int, error) {
	var directories = 0
	var files = 0

	nodes, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}

	for _, node := range nodes {
		if node.IsDir() {
			directories++
		} else {
			files++
		}
	}

	return files, directories, nil
}

func walkPath(path string, fileChannel chan<- string, fileScans chan int, stats *scanStatsChannels, formats *types.Types) error {
	var wg sync.WaitGroup

	errorChannel := make(chan error)
	done := make(chan bool, 1)

	filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		switch {
		case !Recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case !info.IsDir():
			wg.Add(1)
			fileScans <- 1

			go func() {
				defer func() {
					wg.Done()
					<-fileScans
				}()

				path, err := normalizePath(p)
				if err != nil {
					errorChannel <- err

					return
				}

				if !formats.Validate(path) {
					stats.filesSkipped <- 1

					return
				}

				fileChannel <- path

				stats.filesMatched <- 1
			}()
		case info.IsDir():
			files, directories, err := pathCount(p)
			if err != nil {
				errorChannel <- err
			}

			if files > 0 && (files < MinFileCount) || (files > MaxFileCount) {
				// This count will not otherwise include the parent directory itself, so increment by one
				stats.directoriesSkipped <- directories + 1
				stats.filesSkipped <- files

				return filepath.SkipDir
			}

			stats.directoriesMatched <- 1
		}

		return nil
	})

	go func() {
		wg.Wait()
		done <- true
	}()

Poll:
	for {
		select {
		case e := <-errorChannel:
			return e
		case <-done:
			break Poll
		}
	}

	return nil
}

func scanPaths(paths []string, sort string, cache *fileCache, formats *types.Types) ([]string, error) {
	var list []string

	fileChannel := make(chan string)
	errorChannel := make(chan error)
	directoryScans := make(chan int, MaxDirScans)
	fileScans := make(chan int, MaxFileScans)
	done := make(chan bool, 1)

	stats := &scanStats{
		filesMatched:       0,
		filesSkipped:       0,
		directoriesMatched: 0,
		directoriesSkipped: 0,
	}

	statsChannels := &scanStatsChannels{
		filesMatched:       make(chan int),
		filesSkipped:       make(chan int),
		directoriesMatched: make(chan int),
		directoriesSkipped: make(chan int),
	}

	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < len(paths); i++ {
		wg.Add(1)
		directoryScans <- 1

		go func(i int) {
			defer func() {
				wg.Done()
				<-directoryScans
			}()

			err := walkPath(paths[i], fileChannel, fileScans, statsChannels, formats)
			if err != nil {
				errorChannel <- err

				return
			}
		}(i)
	}

	go func() {
		wg.Wait()
		done <- true
	}()

Poll:
	for {
		select {
		case path := <-fileChannel:
			list = append(list, path)
		case stat := <-statsChannels.filesMatched:
			stats.filesMatched = stats.filesMatched + stat
		case stat := <-statsChannels.filesSkipped:
			stats.filesSkipped = stats.filesSkipped + stat
		case stat := <-statsChannels.directoriesMatched:
			stats.directoriesMatched = stats.directoriesMatched + stat
		case stat := <-statsChannels.directoriesSkipped:
			stats.directoriesSkipped = stats.directoriesSkipped + stat
		case err := <-errorChannel:
			return []string{}, err
		case <-done:
			break Poll
		}
	}

	if stats.filesMatched < 1 {
		fmt.Println("No files matched")
		return []string{}, nil
	}

	if Verbose {
		fmt.Printf("%s | INDEX: %d/%d files across %d/%d directories in %s\n",
			time.Now().Format(logDate),
			stats.filesMatched,
			stats.filesMatched+stats.filesSkipped,
			stats.directoriesMatched,
			stats.directoriesMatched+stats.directoriesSkipped,
			time.Since(startTime),
		)
	}

	return list, nil
}

func fileList(paths []string, filters *filters, sort string, cache *fileCache, formats *types.Types) ([]string, error) {
	switch {
	case Cache && !cache.isEmpty() && filters.isEmpty():
		return cache.List(), nil
	case Cache && !cache.isEmpty() && !filters.isEmpty():
		return filters.apply(cache.List()), nil
	case Cache && cache.isEmpty() && !filters.isEmpty():
		list, err := scanPaths(paths, sort, cache, formats)
		if err != nil {
			return []string{}, err
		}
		cache.set(list)

		return filters.apply(cache.List()), nil
	case Cache && cache.isEmpty() && filters.isEmpty():
		list, err := scanPaths(paths, sort, cache, formats)
		if err != nil {
			return []string{}, err
		}
		cache.set(list)

		return cache.List(), nil
	case !Cache && !filters.isEmpty():
		list, err := scanPaths(paths, sort, cache, formats)
		if err != nil {
			return []string{}, err
		}

		return filters.apply(list), nil
	default:
		list, err := scanPaths(paths, sort, cache, formats)
		if err != nil {
			return []string{}, err
		}

		return list, nil
	}
}

func pickFile(list []string) (string, error) {
	fileCount := len(list)

	if fileCount < 1 {
		return "", ErrNoMediaFound
	}

	r, err := rand.Int(rand.Reader, big.NewInt(int64(fileCount)))
	if err != nil {
		return "", err
	}

	val, err := strconv.Atoi(strconv.FormatInt(r.Int64(), 10))
	if err != nil {
		return "", err
	}

	return list[val], nil
}

func normalizePath(path string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absolutePath, nil
}

func validatePaths(args []string, formats *types.Types) ([]string, error) {
	var paths []string

	var pathList strings.Builder
	pathList.WriteString("Paths:\n")

	for i := 0; i < len(args); i++ {
		path, err := normalizePath(args[i])
		if err != nil {
			return nil, err
		}

		pathMatches := (args[i] == path)

		hasSupportedFiles, err := hasSupportedFiles(path, formats)
		if err != nil {
			return nil, err
		}

		var addPath = false

		switch {
		case pathMatches && hasSupportedFiles:
			pathList.WriteString(fmt.Sprintf("%s\n", args[i]))
			addPath = true
		case !pathMatches && hasSupportedFiles:
			pathList.WriteString(fmt.Sprintf("%s (resolved to %s)\n", args[i], path))
			addPath = true
		case pathMatches && !hasSupportedFiles:
			pathList.WriteString(fmt.Sprintf("%s [No supported files found]\n", args[i]))
		case !pathMatches && !hasSupportedFiles:
			pathList.WriteString(fmt.Sprintf("%s (resolved to %s) [No supported files found]\n", args[i], path))
		}

		if addPath {
			paths = append(paths, path)
		}
	}

	if len(paths) > 0 {
		fmt.Println(pathList.String())
	}

	return paths, nil
}
