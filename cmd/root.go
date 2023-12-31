/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/spf13/cobra"
)

const (
	AllowedCharacters string = `^[A-z0-9.\-_]+$`
	ReleaseVersion    string = "5.0.0"
)

var (
	AdminPrefix     string
	All             bool
	AllowEmpty      bool
	Audio           bool
	BinaryPrefix    bool
	Bind            string
	CaseSensitive   bool
	Code            bool
	CodeTheme       string
	Compression     string
	CompressionFast bool
	Concurrency     int
	Debug           bool
	DisableButtons  bool
	ExitOnError     bool
	Fallback        bool
	Filtering       bool
	Flash           bool
	Fun             bool
	Handlers        bool
	Ignore          bool
	IgnoreFile      string
	Images          bool
	Index           bool
	IndexFile       string
	Info            bool
	MaxFileCount    int
	MinFileCount    int
	PageLength      int
	Port            int
	Prefix          string
	Profile         bool
	Recursive       bool
	Redact          bool
	Refresh         bool
	Russian         bool
	Sorting         bool
	Text            bool
	Verbose         bool
	Version         bool
	Videos          bool

	RequiredArgs = []string{
		"all",
		"audio",
		"code",
		"fallback",
		"flash",
		"images",
		"text",
		"video",
	}

	rootCmd = &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random media from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case MaxFileCount < 0 || MinFileCount < 0 || MaxFileCount > math.MaxInt32 || MinFileCount > math.MaxInt32:
				return ErrInvalidFileCountValue
			case MinFileCount > MaxFileCount:
				return ErrInvalidFileCountRange
			case Port < 1 || Port > 65535:
				return ErrInvalidPort
			case Concurrency < 1:
				return ErrInvalidConcurrency
			case Ignore && !regexp.MustCompile(AllowedCharacters).MatchString(IgnoreFile):
				return ErrInvalidIgnoreFile
			case !slices.Contains(CompressionFormats, Compression):
				return ErrInvalidCompression
			case AdminPrefix != "" && !regexp.MustCompile(AllowedCharacters).MatchString(AdminPrefix):
				return ErrInvalidAdminPrefix
			case AdminPrefix != "":
				AdminPrefix = "/" + AdminPrefix
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
		fmt.Printf("%s | ERROR: %v\n", time.Now().Format(logDate), err)

		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&AdminPrefix, "admin-prefix", "", "string to prepend to administrative paths")
	rootCmd.Flags().BoolVarP(&All, "all", "a", false, "enable all supported file types")
	rootCmd.Flags().BoolVar(&AllowEmpty, "allow-empty", false, "allow specifying paths containing no supported files")
	rootCmd.Flags().BoolVar(&Audio, "audio", false, "enable support for audio files")
	rootCmd.Flags().BoolVar(&BinaryPrefix, "binary-prefix", false, "use IEC binary prefixes instead of SI decimal prefixes")
	rootCmd.Flags().StringVarP(&Bind, "bind", "b", "0.0.0.0", "address to bind to")
	rootCmd.Flags().BoolVar(&CaseSensitive, "case-sensitive", false, "use case-sensitive matching for filters")
	rootCmd.Flags().BoolVar(&Code, "code", false, "enable support for source code files")
	rootCmd.Flags().StringVar(&CodeTheme, "code-theme", "solarized-dark256", "theme for source code syntax highlighting")
	rootCmd.Flags().StringVar(&Compression, "compression", "zstd", "compression format to use for index (none, zlib, zstd)")
	rootCmd.Flags().BoolVar(&CompressionFast, "compression-fast", false, "use fastest compression level (default is best)")
	rootCmd.Flags().IntVar(&Concurrency, "concurrency", math.MaxInt32, "maximum concurrency for scan threads")
	rootCmd.Flags().BoolVarP(&Debug, "debug", "d", false, "display even more verbose logs")
	rootCmd.Flags().BoolVar(&DisableButtons, "disable-buttons", false, "disable first/prev/next/last buttons")
	rootCmd.Flags().BoolVar(&ExitOnError, "exit-on-error", false, "shut down webserver on error, instead of just printing error")
	rootCmd.Flags().BoolVar(&Fallback, "fallback", false, "serve files as application/octet-stream if no matching format is registered")
	rootCmd.Flags().BoolVarP(&Filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().BoolVar(&Flash, "flash", false, "enable support for shockwave flash files (via ruffle.rs)")
	rootCmd.Flags().BoolVar(&Fun, "fun", false, "add a bit of excitement to your day")
	rootCmd.Flags().BoolVar(&Handlers, "handlers", false, "display registered handlers (for debugging)")
	rootCmd.Flags().BoolVar(&Ignore, "ignore", false, "skip all directories containing a specified filename")
	rootCmd.Flags().StringVar(&IgnoreFile, "ignore-file", ".roulette-ignore", "filename used to indicate directory to be skipped")
	rootCmd.Flags().BoolVar(&Images, "images", false, "enable support for image files")
	rootCmd.Flags().BoolVar(&Index, "index", false, "generate index of supported file paths at startup")
	rootCmd.Flags().StringVar(&IndexFile, "index-file", "", "path to optional persistent index file")
	rootCmd.Flags().BoolVarP(&Info, "info", "i", false, "expose informational endpoints")
	rootCmd.Flags().IntVar(&MaxFileCount, "max-file-count", math.MaxInt32, "skip directories with file counts above this value")
	rootCmd.Flags().IntVar(&MinFileCount, "min-file-count", 0, "skip directories with file counts below this value")
	rootCmd.Flags().IntVar(&PageLength, "page-length", 0, "pagination length for info pages")
	rootCmd.Flags().IntVarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().StringVar(&Prefix, "prefix", "/", "root path for http handlers (for reverse proxying)")
	rootCmd.Flags().BoolVar(&Profile, "profile", false, "register net/http/pprof handlers")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().BoolVar(&Redact, "redact", false, "redact admin prefix in log output")
	rootCmd.Flags().BoolVar(&Refresh, "refresh", false, "enable automatic page refresh via query parameter")
	rootCmd.Flags().BoolVar(&Russian, "russian", false, "remove selected images after serving")
	rootCmd.Flags().BoolVarP(&Sorting, "sort", "s", false, "enable sorting")
	rootCmd.Flags().BoolVar(&Text, "text", false, "enable support for text files")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files and other information to stdout")
	rootCmd.Flags().BoolVarP(&Version, "version", "V", false, "display version and exit")
	rootCmd.Flags().BoolVar(&Videos, "video", false, "enable support for video files")

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.Flags().SetInterspersed(true)

	rootCmd.MarkFlagsMutuallyExclusive("debug", "exit-on-error")

	rootCmd.MarkFlagsOneRequired(RequiredArgs...)

	rootCmd.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})

	rootCmd.SetVersionTemplate("roulette v{{.Version}}\n")

	rootCmd.SilenceErrors = true

	rootCmd.Version = ReleaseVersion
}
