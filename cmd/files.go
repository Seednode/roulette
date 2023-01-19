/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"encoding/json"
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

var (
	ErrNoImagesFound = fmt.Errorf("no supported image formats found which match all criteria")
	extensions       = [6]string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}
)

type Dimensions struct {
	Width  int
	Height int
}

type Files struct {
	Mutex sync.Mutex
	List  map[string][]string
}

type ScanStats struct {
	FilesMatched       uint64
	FilesSkipped       uint64
	DirectoriesMatched uint64
}

type TimesServed struct {
	file  string
	count uint64
}

type ServeStats struct {
	ImagesServed uint64
	ImageList    []string
	ImageCount   map[string]uint64
}

func (s *ServeStats) GetFilesTotal() uint64 {
	return atomic.LoadUint64(&s.ImagesServed)
}

func (s *ServeStats) IncrementCounter(image string) {
	s.ImageCount[image] += 1

	if !contains(s.ImageList, image) {
		s.ImageList = append(s.ImageList, image)
	}
}

func (s *ServeStats) ListImages() ([]byte, error) {
	a := []TimesServed{}

	for _, image := range s.ImageList {
		a = append(a, TimesServed{image, s.ImageCount[image]})
	}

	r, err := json.Marshal(a)
	if err != nil {
		return []byte{}, err
	}

	return r, nil
}

func (s *ScanStats) GetFilesTotal() uint64 {
	return atomic.LoadUint64(&s.FilesMatched) + atomic.LoadUint64(&s.FilesSkipped)
}

func (s *ScanStats) IncrementFilesMatched() {
	atomic.AddUint64(&s.FilesMatched, 1)
}

func (s *ScanStats) GetFilesMatched() uint64 {
	return atomic.LoadUint64(&s.FilesMatched)
}

func (s *ScanStats) IncrementFilesSkipped() {
	atomic.AddUint64(&s.FilesSkipped, 1)
}

func (s *ScanStats) GetFilesSkipped() uint64 {
	return atomic.LoadUint64(&s.FilesSkipped)
}

func (s *ScanStats) IncrementDirectoriesMatched() {
	atomic.AddUint64(&s.DirectoriesMatched, 1)
}

func (s *ScanStats) GetDirectoriesMatched() uint64 {
	return atomic.LoadUint64(&s.DirectoriesMatched)
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

func getImageDimensions(path string) (*Dimensions, error) {
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
		return &Dimensions{Width: 0, Height: 0}, nil
	case err != nil:
		return &Dimensions{}, err
	}

	return &Dimensions{Width: myImage.Width, Height: myImage.Height}, nil
}

func preparePath(path string) string {
	if runtime.GOOS == "windows" {
		path = fmt.Sprintf("/%v", filepath.ToSlash(path))
	}

	return path
}

func appendPath(directory, path string, files *Files, stats *ScanStats) error {
	// If caching, only check image types once, during the initial scan, to speed up future pickFile() calls
	if Cache {
		image, err := isImage(path)
		if err != nil {
			return err
		}

		if !image {
			return nil
		}
	}

	files.Mutex.Lock()
	files.List[directory] = append(files.List[directory], path)
	files.Mutex.Unlock()

	stats.IncrementFilesMatched()

	return nil
}

func appendPaths(path string, files *Files, filters *Filters, stats *ScanStats) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	directory, filename := filepath.Split(absolutePath)

	filename = strings.ToLower(filename)

	if filters.HasExcludes() {
		for i := 0; i < len(filters.Excludes); i++ {
			if strings.Contains(
				filename,
				filters.Excludes[i],
			) {
				stats.IncrementFilesSkipped()

				return nil
			}
		}
	}

	if filters.HasIncludes() {
		for i := 0; i < len(filters.Includes); i++ {
			if strings.Contains(
				filename,
				filters.Includes[i],
			) {
				err := appendPath(directory, path, files, stats)
				if err != nil {
					return err
				}

				return nil
			}
		}

		stats.IncrementFilesSkipped()

		return nil
	}

	err = appendPath(directory, path, files, stats)
	if err != nil {
		return err
	}

	return nil
}

func getNewFile(paths []string, filters *Filters, sortOrder string, regexes *Regexes, fileCache *[]string) (string, error) {
	filePath, err := pickFile(paths, filters, sortOrder, fileCache)
	if err != nil {
		return "", nil
	}

	path, err := splitPath(filePath, regexes)
	if err != nil {
		return "", err
	}

	path.Number = 1

	switch {
	case sortOrder == "asc":
		filePath, err = tryExtensions(path)
		if err != nil {
			return "", err
		}
	case sortOrder == "desc":
		for {
			path.Increment()

			filePath, err = tryExtensions(path)
			if err != nil {
				return "", err
			}

			if filePath == "" {
				path.Decrement()

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

func getNextFile(filePath, sortOrder string, regexes *Regexes) (string, error) {
	path, err := splitPath(filePath, regexes)
	if err != nil {
		return "", err
	}

	switch {
	case sortOrder == "asc":
		path.Increment()
	case sortOrder == "desc":
		path.Decrement()
	default:
		return "", nil
	}

	fileName, err := tryExtensions(path)
	if err != nil {
		return "", err
	}

	return fileName, err
}

func splitPath(path string, regexes *Regexes) (*Path, error) {
	p := Path{}
	var err error

	split := regexes.Filename.FindAllStringSubmatch(path, -1)

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
	var fileName string

	for _, extension := range extensions {
		fileName = fmt.Sprintf("%v%.3d%v", p.Base, p.Number, extension)

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
	case Verbose && !matchesPrefix:
		fmt.Printf("%v | Error: Failed to serve file outside specified path(s): %v\n",
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

func getFiles(path string, files *Files, filters *Filters, stats *ScanStats, concurrency *Concurrency) error {
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
			concurrency.FileScans <- 1

			go func() {
				defer func() {
					<-concurrency.FileScans
					wg.Done()
				}()

				err = appendPaths(p, files, filters, stats)
				if err != nil {
					fmt.Println(err)
				}
			}()
		case info.IsDir():
			stats.IncrementDirectoriesMatched()
		}

		return err
	})

	wg.Wait()

	if err != nil {
		return err
	}

	return nil
}

func getFileList(paths []string, files *Files, filters *Filters, stats *ScanStats, concurrency *Concurrency) {
	var wg sync.WaitGroup

	for i := 0; i < len(paths); i++ {
		wg.Add(1)
		concurrency.DirectoryScans <- 1

		go func(i int) {
			defer func() {
				<-concurrency.DirectoryScans
				wg.Done()
			}()

			err := getFiles(paths[i], files, filters, stats, concurrency)
			if err != nil {
				fmt.Println(err)
			}
		}(i)
	}

	wg.Wait()
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

func prepareDirectories(files *Files, sort string) []string {
	directories := []string{}

	keys := make([]string, len(files.List))

	i := 0
	for k := range files.List {
		keys[i] = k
		i++
	}

	if sort == "asc" || sort == "desc" {
		for i := 0; i < len(keys); i++ {
			directories = append(directories, prepareDirectory(files.List[keys[i]])...)
		}
	} else {
		for i := 0; i < len(keys); i++ {
			directories = append(directories, files.List[keys[i]]...)
		}
	}

	return directories
}

func pickFile(args []string, filters *Filters, sort string, fileCache *[]string) (string, error) {
	var fileList []string

	if Cache && filters.IsEmpty() && len(*fileCache) != 0 {
		fileList = *fileCache
	} else {
		files := &Files{
			List: make(map[string][]string),
		}

		stats := &ScanStats{
			FilesMatched:       0,
			FilesSkipped:       0,
			DirectoriesMatched: 0,
		}

		concurrency := &Concurrency{
			DirectoryScans: make(chan int, maxDirectoryScans),
			FileScans:      make(chan int, maxFileScans),
		}

		startTime := time.Now()
		getFileList(args, files, filters, stats, concurrency)
		runTime := time.Since(startTime)

		if Verbose {
			fmt.Printf("%v | Scanned %v/%v files across %v directories in %v\n",
				time.Now().Format(LogDate),
				stats.GetFilesMatched(),
				stats.GetFilesTotal(),
				stats.GetDirectoriesMatched(),
				runTime,
			)
		}

		fileList = prepareDirectories(files, sort)

		if Cache {
			*fileCache = append(*fileCache, fileList...)
		}
	}

	fileCount := len(fileList)
	if fileCount == 0 {
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

		if !Cache {
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
	var paths []string

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
			fmt.Printf("%v (resolved to %v)\n", args[i], absolutePath)
		} else {
			fmt.Printf("%v\n", args[i])
		}

		paths = append(paths, absolutePath)
	}

	fmt.Println()

	return paths, nil
}
