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
	"math/big"

	"crypto/rand"
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
	ErrNoMediaFound = errors.New("no supported media formats found which match all criteria")
	Extensions      = [12]string{
		".bmp",
		".gif",
		".jpeg",
		".jpg",
		".mp3",
		".mp4",
		".ogg",
		".ogv",
		".png",
		".wav",
		".webm",
		".webp",
	}
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
	filesMatched       atomic.Uint32
	filesSkipped       atomic.Uint32
	directoriesMatched atomic.Uint32
	directoriesSkipped atomic.Uint32
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
		return fmt.Sprintf("%s/%s", MediaPrefix, filepath.ToSlash(path))
	}

	return MediaPrefix + path
}

func appendPath(directory, path string, files *Files, stats *ScanStats, types *SupportedTypes, shouldCache bool) error {
	if shouldCache {
		supported, _, _, err := fileType(path, types)
		if err != nil {
			return err
		}

		if !supported {
			return nil
		}
	}

	files.Append(directory, path)

	stats.filesMatched.Add(1)

	return nil
}

func appendPaths(path string, files *Files, filters *Filters, stats *ScanStats, types *SupportedTypes) error {
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
				stats.filesSkipped.Add(1)

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
				err := appendPath(directory, path, files, stats, types, shouldCache)
				if err != nil {
					return err
				}

				return nil
			}
		}

		stats.filesSkipped.Add(1)

		return nil
	}

	err = appendPath(directory, path, files, stats, types, shouldCache)
	if err != nil {
		return err
	}

	return nil
}

func newFile(paths []string, filters *Filters, sortOrder string, Regexes *Regexes, index *Index, types *SupportedTypes) (string, error) {
	filePath, err := pickFile(paths, filters, sortOrder, index, types)
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

func fileType(path string, types *SupportedTypes) (bool, *SupportedType, string, error) {
	file, err := os.Open(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil, "", nil
	case err != nil:
		return false, nil, "", err
	}
	defer file.Close()

	head := make([]byte, 261)
	file.Read(head)

	extension := filepath.Ext(path)

	fileType := types.Type(extension)

	isSupported := types.IsSupported(head)
	if !isSupported {
		return false, nil, "", nil
	}

	mimeType := (filetype.GetType(strings.TrimPrefix(extension, "."))).MIME.Value

	return isSupported, fileType, mimeType, nil
}

func pathHasSupportedFiles(path string, types *SupportedTypes) (bool, error) {
	hasSupportedFiles := make(chan bool, 1)

	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		switch {
		case !recursive && info.IsDir() && p != path:
			return filepath.SkipDir
		case !info.IsDir():
			supported, _, _, err := fileType(p, types)
			if err != nil {
				return err
			}

			if supported {
				hasSupportedFiles <- true
				return filepath.SkipAll
			}
		}

		return err
	})
	if err != nil {
		return false, err
	}

	select {
	case <-hasSupportedFiles:
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

func scanPath(path string, files *Files, filters *Filters, stats *ScanStats, concurrency *Concurrency, types *SupportedTypes) error {
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

				path, err := normalizePath(p)
				if err != nil {
					fmt.Println(err)
				}

				err = appendPaths(path, files, filters, stats, types)
				if err != nil {
					fmt.Println(err)
				}
			}()
		case info.IsDir():
			files, directories, err := pathCount(p)
			if err != nil {
				fmt.Println(err)
			}

			if files > 0 && (files < minimumFileCount) || (files > maximumFileCount) {
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

func fileList(paths []string, filters *Filters, sort string, index *Index, types *SupportedTypes) ([]string, bool) {
	if cache && filters.IsEmpty() && !index.IsEmpty() {
		return index.Index(), true
	}

	var fileList []string

	files := &Files{
		mutex: sync.RWMutex{},
		list:  make(map[string][]string),
	}

	stats := &ScanStats{
		filesMatched:       atomic.Uint32{},
		filesSkipped:       atomic.Uint32{},
		directoriesMatched: atomic.Uint32{},
		directoriesSkipped: atomic.Uint32{},
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

			err := scanPath(paths[i], files, filters, stats, concurrency, types)
			if err != nil {
				fmt.Println(err)
			}
		}(i)
	}

	wg.Wait()

	fileList = prepareDirectories(files, sort)

	if stats.filesMatched.Load() < 1 {
		return []string{}, false
	}

	if verbose {
		fmt.Printf("%s | Indexed %d/%d files across %d/%d directories in %s\n",
			time.Now().Format(LogDate),
			stats.filesMatched.Load(),
			stats.filesMatched.Load()+stats.filesSkipped.Load(),
			stats.directoriesMatched.Load(),
			stats.directoriesMatched.Load()+stats.directoriesSkipped.Load(),
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

func pickFile(args []string, filters *Filters, sort string, index *Index, types *SupportedTypes) (string, error) {
	fileList, fromCache := fileList(args, filters, sort, index, types)

	fileCount := len(fileList)
	if fileCount < 1 {
		return "", ErrNoMediaFound
	}

	r, err := rand.Int(rand.Reader, big.NewInt(int64(fileCount-2)))
	if err != nil {
		return "", err
	}

	val, err := strconv.Atoi(strconv.FormatInt(r.Int64(), 10))
	if err != nil {
		return "", err
	}

	for i := 0; i < fileCount; i++ {
		if val >= fileCount {
			val = 0
		} else {
			val++
		}

		filePath := fileList[val]

		if !fromCache {
			supported, _, _, err := fileType(filePath, types)
			if err != nil {
				return "", err
			}

			if supported {
				return filePath, nil
			}

			continue
		}

		return filePath, nil
	}

	return "", ErrNoMediaFound
}

func normalizePath(path string) (string, error) {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absolutePath, nil
}

func normalizePaths(args []string, types *SupportedTypes) ([]string, error) {
	var paths []string

	var pathList strings.Builder
	pathList.WriteString("Paths:\n")

	for i := 0; i < len(args); i++ {
		path, err := normalizePath(args[i])
		if err != nil {
			return nil, err
		}

		pathMatches := (args[i] == path)

		hasSupportedFiles, err := pathHasSupportedFiles(path, types)
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
