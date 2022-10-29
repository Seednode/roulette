/*
Copyright © 2022 Seednode <seednode@seedno.de>
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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/h2non/filetype"
)

var (
	ErrNoImagesFound = fmt.Errorf("no supported image formats found")
	extensions       = [5]string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
)

type Files struct {
	Mutex sync.Mutex
	List  map[string][]string
}

type Stats struct {
	FilesMatched       uint64
	FilesSkipped       uint64
	DirectoriesMatched uint64
}

func (s *Stats) GetFilesTotal() uint64 {
	return atomic.LoadUint64(&s.FilesMatched) + atomic.LoadUint64(&s.FilesSkipped)
}

func (s *Stats) IncrementFilesMatched() {
	atomic.AddUint64(&s.FilesMatched, 1)
}

func (s *Stats) GetFilesMatched() uint64 {
	return atomic.LoadUint64(&s.FilesMatched)
}

func (s *Stats) IncrementFilesSkipped() {
	atomic.AddUint64(&s.FilesSkipped, 1)
}

func (s *Stats) GetFilesSkipped() uint64 {
	return atomic.LoadUint64(&s.FilesSkipped)
}

func (s *Stats) IncrementDirectoriesMatched() {
	atomic.AddUint64(&s.DirectoriesMatched, 1)
}

func (s *Stats) GetDirectoriesMatched() uint64 {
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

func getImageDimensions(path string) (string, error) {
	file, err := os.Open(path)
	defer file.Close()

	myImage, _, err := image.DecodeConfig(file)
	if errors.Is(err, image.ErrFormat) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	return fmt.Sprintf("%vx%v", myImage.Width, myImage.Height), nil
}

func preparePath(path string) string {
	if runtime.GOOS == "windows" {
		path = fmt.Sprintf("/%v", filepath.ToSlash(path))
	}

	return path
}

func appendPath(directory, path string, files *Files, stats *Stats) error {
	// If caching, check for valid images here, to speed up future pickFile() calls
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

func appendPaths(path string, files *Files, filters *Filters, stats *Stats) error {
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

func splitPath(path string, re regexp.Regexp) (*Path, error) {
	p := Path{}
	var err error

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
			time.Now().Format(LOGDATE),
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
	if err != nil {
		return false, err
	}
	defer file.Close()

	head := make([]byte, 261)
	file.Read(head)

	return filetype.IsImage(head), nil
}

func getFiles(path string, files *Files, filters *Filters, stats *Stats, concurrency *Concurrency) error {
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

func getFileList(paths []string, files *Files, filters *Filters, stats *Stats, concurrency *Concurrency) {
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

	if Cache && len(*fileCache) != 0 {
		fileList = *fileCache
	} else {
		files := Files{}
		files.List = make(map[string][]string)

		stats := Stats{}

		concurrency := Concurrency{}
		concurrency.DirectoryScans = make(chan int, maxDirectoryScans)
		concurrency.FileScans = make(chan int, maxFileScans)

		startTime := time.Now()
		getFileList(args, &files, filters, &stats, &concurrency)
		runTime := time.Since(startTime)

		if Verbose {
			fmt.Printf("%v | Scanned %v/%v files across %v directories in %v\n",
				time.Now().Format(LOGDATE),
				stats.GetFilesMatched(),
				stats.GetFilesTotal(),
				stats.GetDirectoriesMatched(),
				runTime,
			)
		}

		fileList = prepareDirectories(&files, sort)

		if Cache {
			*fileCache = append(*fileCache, fileList...)
		}
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(fileList), func(i, j int) { fileList[i], fileList[j] = fileList[j], fileList[i] })

	for i := 0; i < len(fileList); i++ {
		filePath := fileList[i]

		// If not caching, check for valid images just before serving, to speed up scanning
		if !Cache {
			isImage, err := isImage(filePath)
			if err != nil {
				return "", err
			}

			if isImage {
				return filePath, nil
			}
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
