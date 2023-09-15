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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"seedno.de/seednode/roulette/types"
)

type maxConcurrency int

const (
	// avoid hitting default open file descriptor limits (1024)
	maxDirectoryScans maxConcurrency = 32
	maxFileScans      maxConcurrency = 256
)

type regexes struct {
	alphanumeric *regexp.Regexp
	filename     *regexp.Regexp
}

type concurrency struct {
	directoryScans chan int
	fileScans      chan int
}

type files struct {
	mutex sync.RWMutex
	list  []string
}

func (f *files) append(path string) {
	f.mutex.Lock()
	f.list = append(f.list, path)
	f.mutex.Unlock()
}

type scanStats struct {
	filesMatched       atomic.Uint32
	filesSkipped       atomic.Uint32
	directoriesMatched atomic.Uint32
	directoriesSkipped atomic.Uint32
}

type splitPath struct {
	base      string
	number    int
	extension string
}

func (splitPath *splitPath) increment() {
	splitPath.number = splitPath.number + 1
}

func (splitPath *splitPath) decrement() {
	splitPath.number = splitPath.number - 1
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

func preparePath(path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s/%s", mediaPrefix, filepath.ToSlash(path))
	}

	return mediaPrefix + path
}

func newFile(paths []string, filters *filters, sortOrder string, regexes *regexes, cache *fileCache, formats *types.Types) (string, error) {
	path, err := pickFile(paths, filters, sortOrder, cache, formats)
	if err != nil {
		return "", nil
	}

	splitPath, err := split(path, regexes)
	if err != nil {
		return "", err
	}

	splitPath.number = 1

	switch {
	case sortOrder == "asc":
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

	return path, nil
}

func nextFile(path, sortOrder string, regexes *regexes, formats *types.Types) (string, error) {
	splitPath, err := split(path, regexes)
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

	fileName, err := tryExtensions(splitPath, formats)
	if err != nil {
		return "", err
	}

	return fileName, err
}

func split(path string, regexes *regexes) (*splitPath, error) {
	p := splitPath{}
	var err error

	split := regexes.filename.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return &splitPath{}, nil
	}

	p.base = split[0][1]

	p.number, err = strconv.Atoi(split[0][2])
	if err != nil {
		return &splitPath{}, err
	}

	p.extension = split[0][3]

	return &p, nil
}

func tryExtensions(splitPath *splitPath, formats *types.Types) (string, error) {
	var fileName string

	for extension := range formats.Extensions {
		fileName = fmt.Sprintf("%s%.3d%s", splitPath.base, splitPath.number, extension)

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

func pathIsValid(path string, paths []string) bool {
	var matchesPrefix = false

	for i := 0; i < len(paths); i++ {
		if strings.HasPrefix(path, paths[i]) {
			matchesPrefix = true
		}
	}

	switch {
	case Verbose && !matchesPrefix:
		fmt.Printf("%s | Error: Failed to serve file outside specified path(s): %s\n",
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

func pathHasSupportedFiles(path string, formats *types.Types) (bool, error) {
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

func pathCount(path string) (uint32, uint32, error) {
	var directories uint32 = 0
	var files uint32 = 0

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

func scanPath(path string, files *files, stats *scanStats, concurrency *concurrency, formats *types.Types) error {
	var wg sync.WaitGroup

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch {
		case !Recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case !info.IsDir():
			wg.Add(1)
			concurrency.fileScans <- 1

			go func() {
				defer func() {
					<-concurrency.fileScans

					wg.Done()
				}()

				path, err := normalizePath(p)
				if err != nil {
					fmt.Println(err)
				}

				if !formats.Validate(path) {
					stats.filesSkipped.Add(1)

					return
				}

				files.append(path)

				stats.filesMatched.Add(1)
			}()
		case info.IsDir():
			files, directories, err := pathCount(p)
			if err != nil {
				fmt.Println(err)
			}

			if files > 0 && (files < MinimumFileCount) || (files > MaximumFileCount) {
				// This count will not otherwise include the parent directory itself, so increment by one
				stats.directoriesSkipped.Add(directories + 1)
				stats.filesSkipped.Add(files)

				return filepath.SkipDir
			}

			stats.directoriesMatched.Add(1)
		}

		return err
	})

	wg.Wait()

	if err != nil {
		return err
	}

	return nil
}

func scanPaths(paths []string, sort string, cache *fileCache, formats *types.Types) []string {
	files := &files{
		mutex: sync.RWMutex{},
		list:  []string{},
	}

	stats := &scanStats{
		filesMatched:       atomic.Uint32{},
		filesSkipped:       atomic.Uint32{},
		directoriesMatched: atomic.Uint32{},
		directoriesSkipped: atomic.Uint32{},
	}

	concurrency := &concurrency{
		directoryScans: make(chan int, maxDirectoryScans),
		fileScans:      make(chan int, maxFileScans),
	}

	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < len(paths); i++ {
		wg.Add(1)
		concurrency.directoryScans <- 1

		go func(i int) {
			defer func() {
				<-concurrency.directoryScans

				wg.Done()
			}()

			err := scanPath(paths[i], files, stats, concurrency, formats)
			if err != nil {
				fmt.Println(err)
			}
		}(i)
	}

	wg.Wait()

	if stats.filesMatched.Load() < 1 {
		fmt.Println("No files matched")
		return []string{}
	}

	if Verbose {
		fmt.Printf("%s | Indexed %d/%d files across %d/%d directories in %s\n",
			time.Now().Format(logDate),
			stats.filesMatched.Load(),
			stats.filesMatched.Load()+stats.filesSkipped.Load(),
			stats.directoriesMatched.Load(),
			stats.directoriesMatched.Load()+stats.directoriesSkipped.Load(),
			time.Since(startTime),
		)
	}

	return files.list
}

func fileList(paths []string, filters *filters, sort string, cache *fileCache, formats *types.Types) []string {
	switch {
	case Cache && !cache.isEmpty() && filters.isEmpty():
		return cache.List()
	case Cache && !cache.isEmpty() && !filters.isEmpty():
		return filters.apply(cache.List())
	case Cache && cache.isEmpty() && !filters.isEmpty():
		cache.set(scanPaths(paths, sort, cache, formats))
		return filters.apply(cache.List())
	case Cache && cache.isEmpty() && filters.isEmpty():
		cache.set(scanPaths(paths, sort, cache, formats))
		return cache.List()
	case !Cache && !filters.isEmpty():
		return filters.apply(scanPaths(paths, sort, cache, formats))
	default:
		return scanPaths(paths, sort, cache, formats)
	}
}

func pickFile(args []string, filters *filters, sort string, cache *fileCache, formats *types.Types) (string, error) {
	list := fileList(args, filters, sort, cache, formats)

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

		hasSupportedFiles, err := pathHasSupportedFiles(path, formats)
		if err != nil {
			return nil, err
		}

		var addPath bool = false

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
