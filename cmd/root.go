/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

const (
	ReleaseVersion string = "0.91.0"
)

var (
	All              bool
	Audio            bool
	Bind             string
	Cache            bool
	CacheFile        string
	Code             bool
	CodeTheme        string
	ExitOnError      bool
	Filtering        bool
	Flash            bool
	Handlers         bool
	Images           bool
	Info             bool
	MaximumFileCount uint
	MinimumFileCount uint
	PageLength       uint32
	Port             uint16
	Prefix           string
	Profile          bool
	Recursive        bool
	Refresh          bool
	Russian          bool
	Sorting          bool
	Text             bool
	Verbose          bool
	Version          bool
	Videos           bool

	rootCmd = &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random media from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
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
	rootCmd.Flags().BoolVar(&Code, "code", false, "enable support for source code files")
	rootCmd.Flags().StringVar(&CodeTheme, "code-theme", "solarized-dark256", "theme for source code syntax highlighting")
	rootCmd.Flags().BoolVar(&ExitOnError, "exit-on-error", false, "shut down webserver on error, instead of just printing the error")
	rootCmd.Flags().BoolVarP(&Filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().BoolVar(&Flash, "flash", false, "enable support for shockwave flash files (via ruffle.rs)")
	rootCmd.Flags().BoolVar(&Handlers, "handlers", false, "display registered handlers (for debugging)")
	rootCmd.Flags().BoolVar(&Images, "images", false, "enable support for image files")
	rootCmd.Flags().BoolVarP(&Info, "info", "i", false, "expose informational endpoints")
	rootCmd.Flags().UintVar(&MaximumFileCount, "maximum-files", 1<<32-1, "skip directories with file counts above this value")
	rootCmd.Flags().UintVar(&MinimumFileCount, "minimum-files", 1, "skip directories with file counts below this value")
	rootCmd.Flags().Uint32Var(&PageLength, "page-length", 0, "pagination length for info pages")
	rootCmd.Flags().Uint16VarP(&Port, "port", "p", 8080, "port to listen on")
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
