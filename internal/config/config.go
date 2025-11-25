package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application
type Config struct {
	// DataDir is the directory where events are stored
	DataDir string

	// APIPort is the port the API server listens on
	APIPort int

	// LogLevel is the logging level (debug, info, warn, error)
	LogLevel string

	// WatchResources is a comma-separated list of resource types to watch
	WatchResources []string

	// SegmentSize is the target size for compression segments in bytes
	SegmentSize int64

	// MaxConcurrentRequests is the maximum number of concurrent API requests
	MaxConcurrentRequests int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		DataDir:                getEnvOrDefault("RPK_DATA_DIR", "./data"),
		APIPort:                getEnvOrDefaultInt("RPK_API_PORT", 8080),
		LogLevel:               getEnvOrDefault("RPK_LOG_LEVEL", "info"),
		SegmentSize:            getEnvOrDefaultInt64("RPK_SEGMENT_SIZE", 1048576), // 1MB
		MaxConcurrentRequests:  getEnvOrDefaultInt("RPK_MAX_CONCURRENT_REQUESTS", 100),
	}

	// Parse watch resources
	watchResourcesStr := getEnvOrDefault("RPK_WATCH_RESOURCES", "Pod,Deployment,Service,Node,StatefulSet")
	cfg.WatchResources = strings.Split(watchResourcesStr, ",")
	for i := range cfg.WatchResources {
		cfg.WatchResources[i] = strings.TrimSpace(cfg.WatchResources[i])
	}

	return cfg
}

// Validate checks that the configuration is valid
func (c *Config) Validate() error {
	if c.DataDir == "" {
		return NewConfigError("DataDir must not be empty")
	}

	if c.APIPort < 1 || c.APIPort > 65535 {
		return NewConfigError("APIPort must be between 1 and 65535")
	}

	if c.SegmentSize < 1024 {
		return NewConfigError("SegmentSize must be at least 1024 bytes (1KB)")
	}

	if c.SegmentSize > 1073741824 {
		return NewConfigError("SegmentSize must be at most 1GB")
	}

	if c.MaxConcurrentRequests < 1 {
		return NewConfigError("MaxConcurrentRequests must be at least 1")
	}

	if len(c.WatchResources) == 0 {
		return NewConfigError("WatchResources must contain at least one resource type")
	}

	return nil
}

// Helper functions for environment variable loading

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvOrDefaultInt returns the value of an environment variable as int or a default value
func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvOrDefaultInt64 returns the value of an environment variable as int64 or a default value
func getEnvOrDefaultInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// ConfigError represents a configuration error
type ConfigError struct {
	message string
}

// NewConfigError creates a new configuration error
func NewConfigError(message string) *ConfigError {
	return &ConfigError{message: message}
}

// Error returns the error message
func (e *ConfigError) Error() string {
	return e.message
}
