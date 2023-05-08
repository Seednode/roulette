/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	bind           string
	cache          bool
	cacheFile      string
	debug          bool
	filtering      bool
	port           uint16
	recursive      bool
	sorting        bool
	statistics     bool
	statisticsFile string
	verbose        bool

	rootCmd = &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random images from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				cmd.MarkFlagRequired("cache")
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
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&bind, "bind", "b", "0.0.0.0", "address to bind to")
	rootCmd.Flags().BoolVarP(&cache, "cache", "c", false, "generate directory cache at startup")
	rootCmd.Flags().StringVar(&cacheFile, "cache-file", "", "path to optional persistent cache file")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "expose debug endpoint")
	rootCmd.Flags().BoolVarP(&filtering, "filter", "f", false, "enable filtering")
	rootCmd.Flags().Uint16VarP(&port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().BoolVarP(&sorting, "sort", "s", false, "enable sorting")
	rootCmd.Flags().BoolVar(&statistics, "stats", false, "expose stats endpoint")
	rootCmd.Flags().StringVar(&statisticsFile, "stats-file", "", "path to optional persistent stats file")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "log accessed files to stdout")
	rootCmd.Flags().SetInterspersed(true)
}
