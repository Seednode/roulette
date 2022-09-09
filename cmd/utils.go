/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"os"
)

type Exit struct{ Code int }

func HandleExit() {
	if e := recover(); e != nil {
		if exit, ok := e.(Exit); ok == true {
			os.Exit(exit.Code)
		}
		panic(e)
	}
}
