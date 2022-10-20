/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Port uint16
var Recursive bool
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
	rootCmd.Flags().Uint16VarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files to stdout")
	rootCmd.Flags().SetInterspersed(true)
}
