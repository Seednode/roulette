/*
Copyright © 2025 Seednode <seednode@seedno.de>
*/

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Seednode/roulette/types"
)

type scanStats struct {
	filesMatched       chan int
	filesSkipped       chan int
	directoriesMatched chan int
	directoriesSkipped chan int
}

func humanReadableSize(bytes int) string {
	unit := 1000

	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := unit, 0

	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div),
		"kMGTPE"[exp])
}

func kill(path string, index *fileIndex) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}

	if Index {
		index.remove(path)
		index.generate()
	}

	return nil
}

func newFile(list []string, sortOrder string, filename *regexp.Regexp, formats types.Types) (string, error) {
	path, err := pickFile(list)
	if err != nil {
		return "", err
	}

	if sortOrder == "asc" || sortOrder == "desc" {
		splitPath, err := split(path, filename)
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

func nextFile(filePath, sortOrder string, filename *regexp.Regexp, formats types.Types) (string, error) {
	splitPath, err := split(filePath, filename)
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
		path = fmt.Sprintf("%s%s%s",
			splitPath.base,
			splitPath.number,
			extension)

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

	for i := range paths {
		if strings.HasPrefix(path, paths[i]) {
			matchesPrefix = true

			break
		}
	}

	switch {
	case Verbose && !matchesPrefix:
		fmt.Printf("%s | ERROR: File outside specified path(s): %s\n",
			time.Now().Format(logDate),
			path)

		return false
	case !matchesPrefix:
		return false
	default:
		return true
	}
}

func hasSupportedFiles(path string, formats types.Types) (bool, error) {
	if AllowEmpty {
		return true, nil
	}

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

func walkPath(path string, fileChannel chan<- string, wg1 *sync.WaitGroup, stats *scanStats, limit chan struct{}, formats types.Types, errorChannel chan<- error) {
	limit <- struct{}{}

	defer func() {
		<-limit
	}()

	nodes, err := os.ReadDir(path)
	if err != nil {
		stats.directoriesSkipped <- 1

		errorChannel <- err

		return
	}

	var files = 0

	var skipDir = false
	var overrideDir = false

	for _, node := range nodes {
		if !node.IsDir() {
			files++

			if Ignore != "" && node.Name() == Ignore {
				skipDir = true
			}

			if Override != "" && node.Name() == Override {
				overrideDir = true
			}
		}
	}

	var skipFiles = false

	if !overrideDir && (files > MaxFiles || files < MinFiles || skipDir) {
		stats.filesSkipped <- files
		stats.directoriesSkipped <- 1

		skipFiles = true
	} else {
		stats.directoriesMatched <- 1
	}

	var wg2 sync.WaitGroup

	for _, node := range nodes {
		wg2.Add(1)

		go func(node fs.DirEntry) {
			defer wg2.Done()

			fullPath := filepath.Join(path, node.Name())

			switch {
			case node.IsDir() && Recursive:
				wg1.Add(1)

				go func() {
					defer wg1.Done()

					walkPath(fullPath, fileChannel, wg1, stats, limit, formats, errorChannel)
				}()

			case !node.IsDir() && !skipFiles:
				path, err := normalizePath(fullPath)

				switch {
				case err != nil:
					errorChannel <- err
				case formats.Validate(path) || Fallback:
					fileChannel <- path

					stats.filesMatched <- 1

					return
				}

				stats.filesSkipped <- 1
			}
		}(node)
	}

	wg2.Wait()
}

func scanPaths(paths []string, formats types.Types, errorChannel chan<- error) []string {
	startTime := time.Now()

	var filesMatched, filesSkipped int
	var directoriesMatched, directoriesSkipped int

	fileChannel := make(chan string)
	done := make(chan bool)

	stats := &scanStats{
		filesMatched:       make(chan int),
		filesSkipped:       make(chan int),
		directoriesMatched: make(chan int),
		directoriesSkipped: make(chan int),
	}

	var list []string

	var wg0 sync.WaitGroup

	wg0.Add(1)
	go func() {
		defer wg0.Done()
		for {
			select {
			case path := <-fileChannel:
				list = append(list, path)
			case <-done:
				return
			}
		}
	}()

	wg0.Add(1)
	go func() {
		defer wg0.Done()

		for {
			select {
			case stat := <-stats.filesMatched:
				filesMatched += stat
			case <-done:
				return
			}
		}
	}()

	wg0.Add(1)
	go func() {
		defer wg0.Done()

		for {
			select {
			case stat := <-stats.filesSkipped:
				filesSkipped += stat
			case <-done:
				return
			}
		}
	}()

	wg0.Add(1)
	go func() {
		defer wg0.Done()

		for {
			select {
			case stat := <-stats.directoriesMatched:
				directoriesMatched += stat
			case <-done:
				return
			}
		}
	}()

	wg0.Add(1)
	go func() {
		defer wg0.Done()

		for {
			select {
			case stat := <-stats.directoriesSkipped:
				directoriesSkipped += stat
			case <-done:
				return
			}
		}
	}()

	limit := make(chan struct{}, Concurrency)

	var wg1 sync.WaitGroup

	for i := 0; i < len(paths); i++ {
		wg1.Add(1)

		go func(i int) {
			defer wg1.Done()

			walkPath(paths[i], fileChannel, &wg1, stats, limit, formats, errorChannel)
		}(i)
	}

	wg1.Wait()

	close(done)

	wg0.Wait()

	if Verbose {
		fmt.Printf("%s | INDEX: Selected %d/%d files across %d/%d directories in %s\n",
			time.Now().Format(logDate),
			filesMatched,
			filesMatched+filesSkipped,
			directoriesMatched,
			directoriesMatched+directoriesSkipped,
			time.Since(startTime).Round(time.Microsecond))
	}

	slices.Sort(list)

	return list
}

func fileList(paths []string, index *fileIndex, formats types.Types, errorChannel chan<- error) []string {
	switch {
	case Index && !index.isEmpty():
		return index.pathMap[index.getDirectory()]
	case Index && index.isEmpty():
		index.set(scanPaths(paths, formats, errorChannel), errorChannel)

		return index.pathMap[index.getDirectory()]
	default:
		return scanPaths(paths, formats, errorChannel)
	}
}

func pickFile(list []string) (string, error) {
	fileCount := len(list)

	switch {
	case fileCount < 1 && AllowEmpty:
		return "", nil
	case fileCount < 1:
		return "", ErrNoMediaFound
	}

	return list[rand.IntN(fileCount)], nil
}

func preparePath(prefix, path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s/%s",
			prefix,
			filepath.ToSlash(path))
	}

	return prefix + path
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

		pathMatches := args[i] == path

		hasSupportedFiles, err := hasSupportedFiles(path, formats)
		if err != nil {
			return nil, err
		}

		switch {
		case pathMatches && hasSupportedFiles:
			if Verbose {
				fmt.Printf("%s | PATHS: Added %s\n",
					time.Now().Format(logDate),
					args[i])
			}

			paths = append(paths, path)
		case !pathMatches && hasSupportedFiles:
			if Verbose {
				fmt.Printf("%s | PATHS: Added %s [resolved to %s]\n",
					time.Now().Format(logDate),
					args[i],
					path)
			}

			paths = append(paths, path)
		case pathMatches && !hasSupportedFiles:
			if Verbose {
				fmt.Printf("%s | PATHS: Skipped %s (No supported files found)\n",
					time.Now().Format(logDate),
					args[i])
			}
		case !pathMatches && !hasSupportedFiles:
			if Verbose {
				fmt.Printf("%s | PATHS: Skipped %s [resolved to %s] (No supported files found)\n",
					time.Now().Format(logDate),
					args[i],
					path)
			}
		}
	}

	return paths, nil
}
