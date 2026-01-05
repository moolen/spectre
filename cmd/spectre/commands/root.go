package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/moolen/spectre/internal/logging"
	"github.com/spf13/cobra"
)

const Version = "0.1.0"

var (
	logLevelFlags []string // Supports multiple --log-level flags
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
	// Supports per-package log levels: --log-level debug --log-level graph.sync=debug
	rootCmd.PersistentFlags().StringSliceVar(&logLevelFlags, "log-level",
		[]string{"info"},
		"Log level for packages. Use 'default=level' for default, or 'package.name=level' for per-package.\n"+
			"Examples: --log-level debug (all), --log-level graph.sync=debug --log-level controller=warn")

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

// setupLog initializes the logging system with parsed log level flags
// Supports per-package log levels and environment variables
// Priority: CLI flags > Environment variables > Helm values > Initialize default
func setupLog(flags []string) error {
	defaultLevel, packageLevels, err := parseLogLevelFlags(flags)
	if err != nil {
		return err
	}

	// Initialize logging with default level and package-specific overrides
	if err := logging.Initialize(defaultLevel, packageLevels); err != nil {
		return err
	}
	return nil
}

// parseLogLevelFlags parses CLI flags and environment variables
// Priority: CLI flags > Environment variables
//
// CLI format: ["debug"], ["default=info", "graph.sync=debug"], or ["info"]
// Env vars: LOG_LEVEL_GRAPH_SYNC=debug (package name uppercased, dots to underscores)
//
// Returns: (defaultLevel, packageLevels map, error)
func parseLogLevelFlags(flags []string) (string, map[string]string, error) {
	result := make(map[string]string)

	// Step 1: Parse environment variables first (lower priority)
	// Look for LOG_LEVEL_* pattern
	for _, envPair := range os.Environ() {
		if strings.HasPrefix(envPair, "LOG_LEVEL_") {
			parts := strings.SplitN(envPair, "=", 2)
			if len(parts) != 2 {
				continue
			}
			// Convert back: LOG_LEVEL_GRAPH_SYNC=debug -> graph.sync
			packageName := convertEnvKeyToPackageName(parts[0])
			level := parts[1]
			result[packageName] = level
		}
	}

	// Step 2: Parse CLI flags (override env vars)
	for _, flag := range flags {
		if !strings.Contains(flag, "=") {
			// Simple format like "debug" or "info" means default level
			result["default"] = flag
		} else {
			// Format like "graph.sync=debug"
			parts := strings.SplitN(flag, "=", 2)
			if len(parts) == 2 {
				pkg, level := parts[0], parts[1]
				result[pkg] = level
			}
		}
	}

	// Step 3: Extract default level (special key "default")
	defaultLevel := "info"
	if level, exists := result["default"]; exists {
		defaultLevel = level
		delete(result, "default")
	}

	// Step 4: Validate default level
	if err := validateLogLevel(defaultLevel); err != nil {
		return "", nil, err
	}

	// Step 5: Validate all package levels
	for pkg, level := range result {
		if err := validateLogLevel(level); err != nil {
			return "", nil, fmt.Errorf("invalid log level for package %q: %v", pkg, err)
		}
	}

	return defaultLevel, result, nil
}

// convertEnvKeyToPackageName converts LOG_LEVEL_GRAPH_SYNC -> graph.sync
func convertEnvKeyToPackageName(envKey string) string {
	// Remove LOG_LEVEL_ prefix
	name := strings.TrimPrefix(envKey, "LOG_LEVEL_")
	// Convert underscores to dots, lowercase
	return strings.ToLower(strings.ReplaceAll(name, "_", "."))
}

// validateLogLevel checks if a level string is valid
func validateLogLevel(level string) error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if !validLevels[strings.ToLower(level)] {
		return fmt.Errorf("invalid log level: %s (must be one of: debug, info, warn, error, fatal)", level)
	}
	return nil
}

// GetLogLevel returns the default log level from the flags
// This is kept for backward compatibility
func GetLogLevel() string {
	level, _, err := parseLogLevelFlags(logLevelFlags)
	if err != nil {
		return "info"
	}
	return level
}
