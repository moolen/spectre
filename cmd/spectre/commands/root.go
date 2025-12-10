package commands

import (
	"fmt"
	"os"

	"github.com/moolen/spectre/internal/logging"
	"github.com/spf13/cobra"
)

const Version = "0.1.0"

var (
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "spectre",
	Short: "Spectre - Kubernetes Event Monitoring and Analysis",
	Long: `Spectre is a Kubernetes event monitoring system that captures, stores,
and provides analysis capabilities for cluster events and resource state changes.`,
	Version: Version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Logging level (debug, info, warn, error)")

	// Add subcommands
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(debugCmd)
}

// HandleError prints error and exits
func HandleError(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
		os.Exit(1)
	}
}

// setupLog initializes the logging system with the specified level
func setupLog(level string) error {
	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s (must be one of: debug, info, warn, error)", level)
	}

	// Initialize logging
	logging.Initialize(level)
	return nil
}

// GetLogLevel returns the current log level from the flag
func GetLogLevel() string {
	return logLevel
}
