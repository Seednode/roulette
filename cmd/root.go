/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Port int
var Recursive bool

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
		fmt.Println(err)
		panic(Exit{1})
	}
}

func init() {
	rootCmd.Flags().IntVarP(&Port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	rootCmd.Flags().SetInterspersed(false)
}
