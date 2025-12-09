package main

import (
	"os"

	"github.com/moolen/spectre/cmd/spectre/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
