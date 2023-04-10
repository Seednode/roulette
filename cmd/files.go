/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"github.com/h2non/filetype"
)

type maxConcurrency int

const (
	// avoid hitting default open file descriptor limits (1024)
	maxDirectoryScans maxConcurrency = 32
	maxFileScans      maxConcurrency = 256
)

type Concurrency struct {
	directoryScans chan int
	fileScans      chan int
}

var (
	ErrNoImagesFound = fmt.Errorf("no supported image formats found which match all criteria")
	Extensions       = [6]string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}
)

type Dimensions struct {
	width  int
	height int
}

type Files struct {
	mutex sync.RWMutex
	list  map[string][]string
}

func (f *Files) Append(directory, path string) {
	f.mutex.Lock()
	f.list[directory] = append(f.list[directory], path)
	f.mutex.Unlock()
}

type ScanStats struct {
	filesMatched       uint64
	filesSkipped       uint64
	directoriesMatched uint64
}

func (s *ScanStats) FilesTotal() uint64 {
	return atomic.LoadUint64(&s.filesMatched) + atomic.LoadUint64(&s.filesSkipped)
}

func (s *ScanStats) incrementFilesMatched() {
	atomic.AddUint64(&s.filesMatched, 1)
}

func (s *ScanStats) FilesMatched() uint64 {
	return atomic.LoadUint64(&s.filesMatched)
}

func (s *ScanStats) incrementFilesSkipped() {
	atomic.AddUint64(&s.filesSkipped, 1)
}

func (s *ScanStats) FilesSkipped() uint64 {
	return atomic.LoadUint64(&s.filesSkipped)
}

func (s *ScanStats) incrementDirectoriesMatched() {
	atomic.AddUint64(&s.directoriesMatched, 1)
}

func (s *ScanStats) DirectoriesMatched() uint64 {
	return atomic.LoadUint64(&s.directoriesMatched)
}

type Path struct {
	base      string
	number    int
	extension string
}

func (p *Path) increment() {
	p.number = p.number + 1
}

func (p *Path) decrement() {
	p.number = p.number - 1
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
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

func imageDimensions(path string) (*Dimensions, error) {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return &Dimensions{}, nil
	case err != nil:
		return &Dimensions{}, err
	}
	defer file.Close()

	myImage, _, err := image.DecodeConfig(file)
	switch {
	case errors.Is(err, image.ErrFormat):
		return &Dimensions{width: 0, height: 0}, nil
	case err != nil:
		return &Dimensions{}, err
	}

	return &Dimensions{width: myImage.Width, height: myImage.Height}, nil
}

func preparePath(path string) string {
	if runtime.GOOS == "windows" {
		path = fmt.Sprintf("/%s", filepath.ToSlash(path))
	}

	return path
}

func appendPath(directory, path string, files *Files, stats *ScanStats, shouldCache bool) error {
	if shouldCache {
		image, err := isImage(path)
		if err != nil {
			return err
		}

		if !image {
			return nil
		}
	}

	files.Append(directory, path)

	stats.incrementFilesMatched()

	return nil
}

func appendPaths(path string, files *Files, filters *Filters, stats *ScanStats) error {
	shouldCache := cache && filters.IsEmpty()

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	directory, filename := filepath.Split(absolutePath)

	filename = strings.ToLower(filename)

	if filters.HasExcludes() {
		for i := 0; i < len(filters.excludes); i++ {
			if strings.Contains(
				filename,
				filters.excludes[i],
			) {
				stats.incrementFilesSkipped()

				return nil
			}
		}
	}

	if filters.HasIncludes() {
		for i := 0; i < len(filters.includes); i++ {
			if strings.Contains(
				filename,
				filters.includes[i],
			) {
				err := appendPath(directory, path, files, stats, shouldCache)
				if err != nil {
					return err
				}

				return nil
			}
		}

		stats.incrementFilesSkipped()

		return nil
	}

	err = appendPath(directory, path, files, stats, shouldCache)
	if err != nil {
		return err
	}

	return nil
}

func newFile(paths []string, filters *Filters, sortOrder string, Regexes *Regexes, index *Index) (string, error) {
	filePath, err := pickFile(paths, filters, sortOrder, index)
	if err != nil {
		return "", nil
	}

	path, err := splitPath(filePath, Regexes)
	if err != nil {
		return "", err
	}

	path.number = 1

	switch {
	case sortOrder == "asc":
		filePath, err = tryExtensions(path)
		if err != nil {
			return "", err
		}
	case sortOrder == "desc":
		for {
			path.increment()

			filePath, err = tryExtensions(path)
			if err != nil {
				return "", err
			}

			if filePath == "" {
				path.decrement()

				filePath, err = tryExtensions(path)
				if err != nil {
					return "", err
				}

				break
			}
		}
	}

	return filePath, nil
}

func nextFile(filePath, sortOrder string, Regexes *Regexes) (string, error) {
	path, err := splitPath(filePath, Regexes)
	if err != nil {
		return "", err
	}

	switch {
	case sortOrder == "asc":
		path.increment()
	case sortOrder == "desc":
		path.decrement()
	default:
		return "", nil
	}

	fileName, err := tryExtensions(path)
	if err != nil {
		return "", err
	}

	return fileName, err
}

func splitPath(path string, Regexes *Regexes) (*Path, error) {
	p := Path{}
	var err error

	split := Regexes.filename.FindAllStringSubmatch(path, -1)

	if len(split) < 1 || len(split[0]) < 3 {
		return &Path{}, nil
	}

	p.base = split[0][1]

	p.number, err = strconv.Atoi(split[0][2])
	if err != nil {
		return &Path{}, err
	}

	p.extension = split[0][3]

	return &p, nil
}

func tryExtensions(p *Path) (string, error) {
	var fileName string

	for _, extension := range Extensions {
		fileName = fmt.Sprintf("%s%.3d%s", p.base, p.number, extension)

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

	switch {
	case verbose && !matchesPrefix:
		fmt.Printf("%s | Error: Failed to serve file outside specified path(s): %s\n",
			time.Now().Format(LogDate),
			filePath,
		)

		return false
	case !matchesPrefix:
		return false
	default:
		return true
	}
}

func isImage(path string) (bool, error) {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}
	defer file.Close()

	head := make([]byte, 261)
	file.Read(head)

	return filetype.IsImage(head), nil
}

func scanPath(path string, files *Files, filters *Filters, stats *ScanStats, concurrency *Concurrency) error {
	var wg sync.WaitGroup

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch {
		case !recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case !info.IsDir():
			wg.Add(1)
			concurrency.fileScans <- 1

			go func() {
				defer func() {
					<-concurrency.fileScans
					wg.Done()
				}()

				err = appendPaths(p, files, filters, stats)
				if err != nil {
					fmt.Println(err)
				}
			}()
		case info.IsDir():
			stats.incrementDirectoriesMatched()
		}

		return err
	})

	wg.Wait()

	if err != nil {
		return err
	}

	return nil
}

func fileList(paths []string, filters *Filters, sort string, index *Index) ([]string, bool) {
	if cache && filters.IsEmpty() && !index.IsEmpty() {
		return index.Index(), true
	}

	var fileList []string

	files := &Files{
		mutex: sync.RWMutex{},
		list:  make(map[string][]string),
	}

	stats := &ScanStats{
		filesMatched:       0,
		filesSkipped:       0,
		directoriesMatched: 0,
	}

	concurrency := &Concurrency{
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

			err := scanPath(paths[i], files, filters, stats, concurrency)
			if err != nil {
				fmt.Println(err)
			}
		}(i)
	}

	wg.Wait()

	fileList = prepareDirectories(files, sort)

	if verbose {
		fmt.Printf("%s | Indexed %d/%d files across %d directories in %s\n",
			time.Now().Format(LogDate),
			stats.FilesMatched(),
			stats.FilesTotal(),
			stats.DirectoriesMatched(),
			time.Since(startTime),
		)
	}

	if cache && filters.IsEmpty() {
		index.setIndex(fileList)
	}

	return fileList, false
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
		return []string{directory[0]}
	} else {
		return directory
	}
}

func prepareDirectories(files *Files, sort string) []string {
	directories := []string{}

	keys := make([]string, len(files.list))

	i := 0
	for k := range files.list {
		keys[i] = k
		i++
	}

	if sort == "asc" || sort == "desc" {
		for i := 0; i < len(keys); i++ {
			directories = append(directories, prepareDirectory(files.list[keys[i]])...)
		}
	} else {
		for i := 0; i < len(keys); i++ {
			directories = append(directories, files.list[keys[i]]...)
		}
	}

	return directories
}

func pickFile(args []string, filters *Filters, sort string, index *Index) (string, error) {
	fileList, fromCache := fileList(args, filters, sort, index)

	fileCount := len(fileList)
	if fileCount < 1 {
		return "", ErrNoImagesFound
	}

	r := rand.Intn(fileCount - 1)

	for i := 0; i < fileCount; i++ {
		if r >= (fileCount - 1) {
			r = 0
		} else {
			r++
		}

		filePath := fileList[r]

		if !fromCache {
			isImage, err := isImage(filePath)
			if err != nil {
				return "", err
			}

			if isImage {
				return filePath, nil
			}

			continue
		}

		return filePath, nil
	}

	return "", ErrNoImagesFound
}

func normalizePaths(args []string) ([]string, error) {
	paths := make([]string, len(args))

	fmt.Println("Paths:")

	for i := 0; i < len(args); i++ {
		path, err := filepath.EvalSymlinks(args[i])
		if err != nil {
			return nil, err
		}

		absolutePath, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}

		if (args[i]) != absolutePath {
			fmt.Printf("%s (resolved to %s)\n", args[i], absolutePath)
		} else {
			fmt.Printf("%s\n", args[i])
		}

		paths[i] = absolutePath
	}

	fmt.Println()

	return paths, nil
}
