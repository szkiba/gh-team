package main

import (
	"os"

	"github.com/szkiba/gh-team/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
