/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"log"
	"math"

	"github.com/spf13/cobra"
)

const (
	ReleaseVersion string = "1.0.1"
)

var (
	All           bool
	Audio         bool
	Bind          string
	Cache         bool
	CacheFile     string
	CaseSensitive bool
	Code          bool
	CodeTheme     string
	ExitOnError   bool
	Filtering     bool
	Flash         bool
	Handlers      bool
	Images        bool
	Info          bool
	MaxDirScans   int
	MaxFileScans  int
	MaxFileCount  int
	MinFileCount  int
	PageLength    int
	Port          int
	Prefix        string
	Profile       bool
	Recursive     bool
	Refresh       bool
	Russian       bool
	Sorting       bool
	Text          bool
	Verbose       bool
	Version       bool
	Videos        bool

	rootCmd = &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random media from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case MaxDirScans < 1 || MaxFileScans < 1 || MaxDirScans > math.MaxInt32 || MaxFileScans > math.MaxInt32:
				return ErrInvalidScanCount
			case MaxFileCount < 1 || MinFileCount < 1 || MaxFileCount > math.MaxInt32 || MinFileCount > math.MaxInt32:
				return ErrInvalidFileCountValue
			case MinFileCount > MaxFileCount:
				return ErrInvalidFileCountRange
			case Port < 1 || Port > 65535:
				return ErrInvalidPort
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ServePage(args)
			if err != nil {
				return err
			}

			return nil
		},
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&All, "all", "a", false, "enable all supported file types")
	rootCmd.Flags().BoolVar(&Audio, "audio", false, "enable support for audio files")
	rootCmd.Flags().StringVarP(&Bind, "bind", "b", "0.0.0.0", "address to bind to")
	rootCmd.Flags().BoolVarP(&Cache, "cache", "c", false, "generate directory cache at startup")
	rootCmd.Flags().StringVar(&CacheFile, "cache-file", "", "path to optional persistent cache file")
	rootCmd.Flags().BoolVar(&CaseSensitive, "case-sensitive", false, "use case-sensitive matching for filters")
	rootCmd.Flags().BoolVar(&Code, "code", false, "enable support for source code files")
	rootCmd.Flags().StringVar(&CodeTheme, "code-theme", "solarized-dark256", "theme for source code syntax highlighting")
	rootCmd.Flags().BoolVar(&ExitOnError, "exit-on-error", false, "shut down webserver on error, instead of just printing the error")
	rootCmd.Flags().BoolVarP(&Filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().BoolVar(&Flash, "flash", false, "enable support for shockwave flash files (via ruffle.rs)")
	rootCmd.Flags().BoolVar(&Handlers, "handlers", false, "display registered handlers (for debugging)")
	rootCmd.Flags().BoolVar(&Images, "images", false, "enable support for image files")
	rootCmd.Flags().BoolVarP(&Info, "info", "i", false, "expose informational endpoints")
	rootCmd.Flags().IntVar(&MaxDirScans, "max-directory-scans", 32, "number of directories to scan at once")
	rootCmd.Flags().IntVar(&MaxFileScans, "max-file-scans", 256, "number of files to scan at once")
	rootCmd.Flags().IntVar(&MaxFileCount, "max-file-count", math.MaxInt32, "skip directories with file counts above this value")
	rootCmd.Flags().IntVar(&MinFileCount, "min-file-count", 1, "skip directories with file counts below this value")
	rootCmd.Flags().IntVar(&PageLength, "page-length", 0, "pagination length for info pages")
	rootCmd.Flags().IntVarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().StringVar(&Prefix, "prefix", "/", "root path for http handlers (for reverse proxying)")
	rootCmd.Flags().BoolVar(&Profile, "profile", false, "register net/http/pprof handlers")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().BoolVar(&Refresh, "refresh", false, "enable automatic page refresh via query parameter")
	rootCmd.Flags().BoolVar(&Russian, "russian", false, "remove selected images after serving")
	rootCmd.Flags().BoolVarP(&Sorting, "sort", "s", false, "enable sorting")
	rootCmd.Flags().BoolVar(&Text, "text", false, "enable support for text files")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files and other information to stdout")
	rootCmd.Flags().BoolVarP(&Version, "version", "V", false, "display version and exit")
	rootCmd.Flags().BoolVar(&Videos, "video", false, "enable support for video files")

	rootCmd.Flags().SetInterspersed(true)

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.SilenceErrors = true
	rootCmd.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})

	rootCmd.SetVersionTemplate("roulette v{{.Version}}\n")
	rootCmd.Version = ReleaseVersion
}
