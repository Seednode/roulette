/*
Copyright © 2023 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.32.1"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  "Print the version number of roulette",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("roulette v" + Version)
	},
}
