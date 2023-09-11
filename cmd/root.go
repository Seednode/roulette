/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"errors"
	"log"
	"time"

	"github.com/spf13/cobra"
)

var (
	ErrIncorrectRefreshInterval = errors.New("refresh interval must be a duration string >= 500ms")
)

const (
	Version string = "0.63.1"
)

var (
	audio            bool
	bind             string
	cache            bool
	cacheFile        string
	debug            bool
	filtering        bool
	images           bool
	maximumFileCount uint32
	minimumFileCount uint32
	pageLength       uint16
	port             uint16
	profile          bool
	recursive        bool
	refreshInterval  string
	russian          bool
	sorting          bool
	statistics       bool
	statisticsFile   string
	verbose          bool
	version          bool
	videos           bool

	rootCmd = &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random media from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				cmd.MarkFlagRequired("cache")
			}

			if refreshInterval != "" {
				interval, err := time.ParseDuration(refreshInterval)
				if err != nil || interval < 500*time.Millisecond {
					log.Fatal(ErrIncorrectRefreshInterval)
				}
			}
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
	rootCmd.Flags().BoolVar(&audio, "audio", false, "enable support for audio files")
	rootCmd.Flags().StringVarP(&bind, "bind", "b", "0.0.0.0", "address to bind to")
	rootCmd.Flags().BoolVarP(&cache, "cache", "c", false, "generate directory cache at startup")
	rootCmd.Flags().StringVar(&cacheFile, "cache-file", "", "path to optional persistent cache file")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "expose debug endpoint")
	rootCmd.Flags().BoolVarP(&filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().BoolVar(&images, "images", true, "enable support for image files")
	rootCmd.Flags().Uint32Var(&maximumFileCount, "maximum-files", 1<<32-1, "skip directories with file counts over this value")
	rootCmd.Flags().Uint32Var(&minimumFileCount, "minimum-files", 0, "skip directories with file counts under this value")
	rootCmd.Flags().Uint16Var(&pageLength, "page-length", 0, "pagination length for statistics and debug pages")
	rootCmd.Flags().Uint16VarP(&port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVar(&profile, "profile", false, "register net/http/pprof handlers")
	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().StringVar(&refreshInterval, "refresh-interval", "", "force refresh interval equal to this duration (minimum 500ms)")
	rootCmd.Flags().BoolVar(&russian, "russian", false, "remove selected images after serving")
	rootCmd.Flags().BoolVarP(&sorting, "sort", "s", false, "enable sorting")
	rootCmd.Flags().BoolVar(&statistics, "stats", false, "expose stats endpoint")
	rootCmd.Flags().StringVar(&statisticsFile, "stats-file", "", "path to optional persistent stats file")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "log accessed files to stdout")
	rootCmd.Flags().BoolVarP(&version, "version", "V", false, "display version and exit")
	rootCmd.Flags().BoolVar(&videos, "video", false, "enable support for video files")

	rootCmd.Flags().SetInterspersed(true)

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.SilenceErrors = true
	rootCmd.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})

	rootCmd.SetVersionTemplate("roulette v{{.Version}}\n")
	rootCmd.Version = Version
}
