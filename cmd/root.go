/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
)

const (
	ReleaseVersion string = "0.73.1"
)

var (
	All              bool
	Audio            bool
	Bind             string
	Cache            bool
	CacheFile        string
	Filtering        bool
	Flash            bool
	Images           bool
	Index            bool
	MaximumFileCount uint32
	MinimumFileCount uint32
	PageLength       uint32
	Port             uint16
	Profile          bool
	Recursive        bool
	RefreshInterval  string
	Russian          bool
	Sorting          bool
	Statistics       bool
	StatisticsFile   string
	Text             bool
	Verbose          bool
	Version          bool
	Videos           bool

	rootCmd = &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random media from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// enable image support if no other flags are passed, to retain backwards compatibility
			// to be replaced with MarkFlagsOneRequired on next spf13/cobra update
			if !(All || Audio || Flash || Images || Text || Videos) {
				Images = true
			}

			if Index {
				cmd.MarkFlagRequired("cache")
			}

			if RefreshInterval != "" {
				interval, err := time.ParseDuration(RefreshInterval)
				if err != nil || interval < 500*time.Millisecond {
					return ErrIncorrectRefreshInterval
				}
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
	rootCmd.Flags().BoolVar(&All, "all", false, "enable all supported file types")
	rootCmd.Flags().BoolVar(&Audio, "audio", false, "enable support for audio files")
	rootCmd.Flags().StringVarP(&Bind, "bind", "b", "0.0.0.0", "address to bind to")
	rootCmd.Flags().BoolVarP(&Cache, "cache", "c", false, "generate directory cache at startup")
	rootCmd.Flags().StringVar(&CacheFile, "cache-file", "", "path to optional persistent cache file")
	rootCmd.Flags().BoolVarP(&Filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().BoolVar(&Flash, "flash", false, "enable support for shockwave flash files (via ruffle.rs)")
	rootCmd.Flags().BoolVar(&Images, "images", false, "enable support for image files")
	rootCmd.Flags().BoolVarP(&Index, "index", "i", false, "expose index endpoints")
	rootCmd.Flags().Uint32Var(&MaximumFileCount, "maximum-files", 1<<32-1, "skip directories with file counts above this value")
	rootCmd.Flags().Uint32Var(&MinimumFileCount, "minimum-files", 1, "skip directories with file counts below this value")
	rootCmd.Flags().Uint32Var(&PageLength, "page-length", 0, "pagination length for statistics and debug pages")
	rootCmd.Flags().Uint16VarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVar(&Profile, "profile", false, "register net/http/pprof handlers")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().StringVar(&RefreshInterval, "refresh-interval", "", "force refresh interval equal to this duration (minimum 500ms)")
	rootCmd.Flags().BoolVar(&Russian, "russian", false, "remove selected images after serving")
	rootCmd.Flags().BoolVarP(&Sorting, "sort", "s", false, "enable sorting")
	rootCmd.Flags().BoolVar(&Statistics, "stats", false, "expose stats endpoint")
	rootCmd.Flags().StringVar(&StatisticsFile, "stats-file", "", "path to optional persistent stats file")
	rootCmd.Flags().BoolVar(&Text, "text", false, "enable support for text files")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files to stdout")
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
