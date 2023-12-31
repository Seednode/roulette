/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"io/fs"
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

const ignoreFilePattern string = `^[A-z0-9.-_]+$`

type regexes struct {
	alphanumeric *regexp.Regexp
	filename     *regexp.Regexp
}

type scanStatsChannels struct {
	filesMatched       chan int
	filesSkipped       chan int
	directoriesMatched chan int
	directoriesSkipped chan int
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

func kill(path string, index *fileIndex) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}

	if Index {
		index.remove(path)
	}

	return nil
}

func newFile(list []string, sortOrder string, regexes *regexes, formats types.Types) (string, error) {
	path, err := pickFile(list)
	if err != nil {
		return "", err
	}

	if sortOrder == "asc" || sortOrder == "desc" {
		splitPath, err := split(path, regexes)
		if err != nil {
			return "", err
		}

		switch {
		case sortOrder == "asc":
			splitPath.number = fmt.Sprintf("%0*d", len(splitPath.number), 1)

			path, err = tryExtensions(splitPath, formats)
			if err != nil {
				return "", err
			}
		case sortOrder == "desc":
			for {
				splitPath.number = splitPath.increment()

				path, err = tryExtensions(splitPath, formats)
				if err != nil {
					return "", err
				}

				if path == "" {
					splitPath.number = splitPath.decrement()

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

func nextFile(filePath, sortOrder string, regexes *regexes, formats types.Types) (string, error) {
	splitPath, err := split(filePath, regexes)
	if err != nil {
		return "", err
	}

	switch {
	case sortOrder == "asc":
		splitPath.number = splitPath.increment()
	case sortOrder == "desc":
		splitPath.number = splitPath.decrement()
	default:
		return "", nil
	}

	path, err := tryExtensions(splitPath, formats)
	if err != nil {
		return "", err
	}

	return path, err
}

func tryExtensions(splitPath *splitPath, formats types.Types) (string, error) {
	var path string

	for extension := range formats {
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

			break
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

func hasSupportedFiles(path string, formats types.Types) (bool, error) {
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

func walkPath(path string, fileChannel chan<- string, stats *scanStatsChannels, limit chan int, formats types.Types) error {
	limit <- 1

	defer func() {
		<-limit
	}()

	errorChannel := make(chan error)
	done := make(chan bool, 1)

	nodes, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	var directories, files = 0, 0

	var skipDir = false

	for _, node := range nodes {
		if node.IsDir() {
			directories++
		} else {
			if Ignore && node.Name() == IgnoreFile {
				skipDir = true
			}

			files++
		}
	}

	var skipFiles = false

	if files <= MaxFileCount && files >= MinFileCount && !skipDir {
		stats.directoriesMatched <- 1
	} else {
		stats.filesSkipped <- files
		stats.directoriesSkipped <- 1

		skipFiles = true
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, node := range nodes {
			wg.Add(1)

			go func(node fs.DirEntry) {
				defer wg.Done()

				fullPath := filepath.Join(path, node.Name())

				switch {
				case node.IsDir() && Recursive:
					err := walkPath(fullPath, fileChannel, stats, limit, formats)
					if err != nil {
						errorChannel <- err

						return
					}
				case !node.IsDir() && !skipFiles:
					path, err := normalizePath(fullPath)
					if err != nil {
						errorChannel <- err

						return
					}

					if formats.Validate(path) || Fallback {
						fileChannel <- path

						stats.filesMatched <- 1

						return
					}

					stats.filesSkipped <- 1
				}
			}(node)
		}
	}()

	go func() {
		wg.Wait()

		done <- true
	}()

Poll:
	for {
		select {
		case err := <-errorChannel:
			return err
		case <-done:
			break Poll
		}
	}

	return nil
}

func scanPaths(paths []string, sort string, index *fileIndex, formats types.Types) ([]string, error) {
	startTime := time.Now()

	var filesMatched, filesSkipped int
	var directoriesMatched, directoriesSkipped int

	fileChannel := make(chan string)
	errorChannel := make(chan error)
	done := make(chan bool)

	statsChannels := &scanStatsChannels{
		filesMatched:       make(chan int),
		filesSkipped:       make(chan int),
		directoriesMatched: make(chan int),
		directoriesSkipped: make(chan int),
	}

	var list []string

	go func() {
		for {
			select {
			case path := <-fileChannel:
				list = append(list, path)
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case stat := <-statsChannels.filesMatched:
				filesMatched += stat
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case stat := <-statsChannels.filesSkipped:
				filesSkipped += stat
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case stat := <-statsChannels.directoriesMatched:
				directoriesMatched += stat
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case stat := <-statsChannels.directoriesSkipped:
				directoriesSkipped += stat
			case <-done:
				return
			}
		}
	}()

	limit := make(chan int, Concurrency)

	var wg sync.WaitGroup

	for i := 0; i < len(paths); i++ {
		wg.Add(1)

		go func(i int) {
			defer func() {
				wg.Done()
			}()

			err := walkPath(paths[i], fileChannel, statsChannels, limit, formats)

			if err != nil {
				errorChannel <- err

				return
			}
		}(i)
	}

	go func() {
		wg.Wait()

		close(done)
	}()

Poll:
	for {
		select {
		case err := <-errorChannel:
			return []string{}, err
		case <-done:
			break Poll
		}
	}

	if Verbose {
		fmt.Printf("%s | INDEX: Selected %d/%d files across %d/%d directories in %s\n",
			time.Now().Format(logDate),
			filesMatched,
			filesMatched+filesSkipped,
			directoriesMatched,
			directoriesMatched+directoriesSkipped,
			time.Since(startTime),
		)
	}

	return list, nil
}

func fileList(paths []string, filters *filters, sort string, index *fileIndex, formats types.Types) ([]string, error) {
	switch {
	case Index && !index.isEmpty() && filters.isEmpty():
		return index.List(), nil
	case Index && !index.isEmpty() && !filters.isEmpty():
		return filters.apply(index.List()), nil
	case Index && index.isEmpty() && !filters.isEmpty():
		list, err := scanPaths(paths, sort, index, formats)
		if err != nil {
			return []string{}, err
		}
		index.set(list)

		return filters.apply(index.List()), nil
	case Index && index.isEmpty() && filters.isEmpty():
		list, err := scanPaths(paths, sort, index, formats)
		if err != nil {
			return []string{}, err
		}
		index.set(list)

		return index.List(), nil
	case !Index && !filters.isEmpty():
		list, err := scanPaths(paths, sort, index, formats)
		if err != nil {
			return []string{}, err
		}

		return filters.apply(list), nil
	default:
		list, err := scanPaths(paths, sort, index, formats)
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

func validatePaths(args []string, formats types.Types) ([]string, error) {
	var paths []string

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

		switch {
		case pathMatches && hasSupportedFiles:
			fmt.Printf("%s | PATHS: Added %s\n",
				time.Now().Format(logDate),
				args[i],
			)

			paths = append(paths, path)
		case !pathMatches && hasSupportedFiles:
			fmt.Printf("%s | PATHS: Added %s [resolved to %s]\n",
				time.Now().Format(logDate),
				args[i],
				path,
			)

			paths = append(paths, path)
		case pathMatches && !hasSupportedFiles:
			fmt.Printf("%s | PATHS: Skipped %s (No supported files found)\n",
				time.Now().Format(logDate),
				args[i],
			)
		case !pathMatches && !hasSupportedFiles:
			fmt.Printf("%s | PATHS: Skipped %s [resolved to %s] (No supported files found)\n",
				time.Now().Format(logDate),
				args[i],
				path,
			)
		}
	}

	return paths, nil
}
