package commands

import (
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug utilities for Spectre",
	Long:  `Various debugging and inspection tools for Spectre internals.`,
}

func init() {
	// Storage debug command removed - storage package no longer exists
	// Future debug commands for graph can be added here
}
