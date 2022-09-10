/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var Port int
var Recursive bool
var Verbose bool

var rootCmd = &cobra.Command{
	Use:   "roulette <path1> [path2] ... [pathN]",
	Short: "Serves random images from the specified directories.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ServePage(args)
	},
	Version: Version,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files to stdout")
	rootCmd.Flags().SetInterspersed(true)
}
