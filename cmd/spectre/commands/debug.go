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
	debugCmd.AddCommand(debugGenerateImportDataCmd)
}
