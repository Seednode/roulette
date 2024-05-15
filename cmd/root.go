/*
Copyright © 2024 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"time"

	"github.com/spf13/cobra"
)

const (
	AllowedCharacters string = `^[A-z0-9.\-_]+$`
	ReleaseVersion    string = "8.8.2"
)

var (
	AdminPrefix     string
	All             bool
	AllowEmpty      bool
	API             bool
	Audio           bool
	Bind            string
	CaseInsensitive bool
	Code            bool
	CodeTheme       string
	Concurrency     int
	Debug           bool
	ErrorExit       bool
	Fallback        bool
	Filtering       bool
	Flash           bool
	Fun             bool
	Ignore          string
	Images          bool
	Index           bool
	IndexFile       string
	IndexInterval   string
	MaxFiles        int
	MinFiles        int
	NoButtons       bool
	Port            int
	Prefix          string
	Profile         bool
	Recursive       bool
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
			case MaxFiles < 0 || MinFiles < 0 || MaxFiles > math.MaxInt32 || MinFiles > math.MaxInt32:
				return ErrInvalidFileCountValue
			case MinFiles > MaxFiles:
				return ErrInvalidFileCountRange
			case Port < 1 || Port > 65535:
				return ErrInvalidPort
			case Concurrency < 1:
				return ErrInvalidConcurrency
			case Ignore != "" && !regexp.MustCompile(AllowedCharacters).MatchString(Ignore):
				return ErrInvalidIgnoreFile
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
	rootCmd.Flags().BoolVar(&API, "api", false, "expose REST API")
	rootCmd.Flags().BoolVar(&Audio, "audio", false, "enable support for audio files")
	rootCmd.Flags().StringVarP(&Bind, "bind", "b", "0.0.0.0", "address to bind to")
	rootCmd.Flags().BoolVar(&CaseInsensitive, "case-insensitive", false, "use case-insensitive matching for filters")
	rootCmd.Flags().BoolVar(&Code, "code", false, "enable support for source code files")
	rootCmd.Flags().StringVar(&CodeTheme, "code-theme", "solarized-dark256", "theme for source code syntax highlighting")
	rootCmd.Flags().IntVar(&Concurrency, "concurrency", 1024, "maximum concurrency for scan threads")
	rootCmd.Flags().BoolVarP(&Debug, "debug", "d", false, "log file permission errors instead of simply skipping the files")
	rootCmd.Flags().BoolVar(&ErrorExit, "error-exit", false, "shut down webserver on error, instead of just printing error")
	rootCmd.Flags().BoolVar(&Fallback, "fallback", false, "serve files as application/octet-stream if no matching format is registered")
	rootCmd.Flags().BoolVarP(&Filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().BoolVar(&Flash, "flash", false, "enable support for shockwave flash files (via ruffle.rs)")
	rootCmd.Flags().BoolVar(&Fun, "fun", false, "add a bit of excitement to your day")
	rootCmd.Flags().StringVar(&Ignore, "ignore", "", "filename used to indicate directory should be skipped")
	rootCmd.Flags().BoolVar(&Images, "images", false, "enable support for image files")
	rootCmd.Flags().BoolVarP(&Index, "index", "i", false, "generate index of supported file paths at startup")
	rootCmd.Flags().StringVar(&IndexFile, "index-file", "", "path to optional persistent index file")
	rootCmd.Flags().StringVar(&IndexInterval, "index-interval", "", "interval at which to regenerate index (e.g. \"5m\" or \"1h\")")
	rootCmd.Flags().IntVar(&MaxFiles, "max-files", math.MaxInt32, "skip directories with file counts above this value")
	rootCmd.Flags().IntVar(&MinFiles, "min-files", 0, "skip directories with file counts below this value")
	rootCmd.Flags().BoolVar(&NoButtons, "no-buttons", false, "disable first/prev/next/last buttons")
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

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.Flags().SetInterspersed(true)

	rootCmd.MarkFlagsOneRequired(RequiredArgs...)

	rootCmd.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})

	rootCmd.SetVersionTemplate("roulette v{{.Version}}\n")

	rootCmd.SilenceErrors = true

	rootCmd.Version = ReleaseVersion

	log.SetFlags(0)
}
