/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type MaxConcurrency int

const (
	// avoid hitting default open file descriptor limits (1024)
	maxDirectoryScans MaxConcurrency = 32
	maxFileScans      MaxConcurrency = 256

	// number of times pickFile() will check for a valid file
	maxRetries int = 10
)

type Concurrency struct {
	DirectoryScans chan int
	FileScans      chan int
}

var Cache bool
var Filter bool
var Port uint16
var Recursive bool
var Sort bool
var Verbose bool

var rootCmd = &cobra.Command{
	Use:   "roulette <path> [path2]...",
	Short: "Serves random images from the specified directories.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := ServePage(args)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&Cache, "cache", "c", false, "only scan directories once, at startup (incompatible with --filter)")
	rootCmd.Flags().BoolVarP(&Filter, "filter", "f", false, "enable filtering via query parameters (incompatible with --cache)")
	rootCmd.MarkFlagsMutuallyExclusive("cache", "filter")
	rootCmd.Flags().Uint16VarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().BoolVarP(&Sort, "sort", "s", false, "enable sorting via query parameters")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files to stdout")
	rootCmd.Flags().SetInterspersed(true)
}
