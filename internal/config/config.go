package config

// Config holds all configuration for the application
type Config struct {
	// APIPort is the port the API server listens on
	APIPort int

	// LogLevelFlags are the per-package log level configurations
	// Format: ["debug"], ["default=info", "graph.sync=debug"], or ["info"]
	// Kept for backward compatibility, but can hold multiple entries for per-package levels
	LogLevelFlags []string

	// WatcherConfigPath is the path to the YAML file containing watcher configuration
	WatcherConfigPath string

	// MaxConcurrentRequests is the maximum number of concurrent API requests
	MaxConcurrentRequests int

	// TracingEnabled indicates whether OpenTelemetry tracing is enabled
	TracingEnabled bool

	// TracingEndpoint is the OTLP gRPC endpoint for trace export
	TracingEndpoint string

	// TracingTLSCAPath is the path to the CA certificate for TLS verification
	TracingTLSCAPath string

	// TracingTLSInsecure allows insecure TLS connections (skip verification)
	TracingTLSInsecure bool
}

// LoadConfig creates a Config with the provided values
func LoadConfig(apiPort int, logLevelFlags []string, watcherConfigPath string, maxConcurrentRequests int, tracingEnabled bool, tracingEndpoint, tracingTLSCAPath string, tracingTLSInsecure bool) *Config {
	cfg := &Config{
		APIPort:               apiPort,
		LogLevelFlags:         logLevelFlags,
		WatcherConfigPath:     watcherConfigPath,
		MaxConcurrentRequests: maxConcurrentRequests,
		TracingEnabled:        tracingEnabled,
		TracingEndpoint:       tracingEndpoint,
		TracingTLSCAPath:      tracingTLSCAPath,
		TracingTLSInsecure:    tracingTLSInsecure,
	}

	return cfg
}

// Validate checks that the configuration is valid
func (c *Config) Validate() error {
	if c.APIPort < 1 || c.APIPort > 65535 {
		return NewConfigError("APIPort must be between 1 and 65535")
	}

	if c.MaxConcurrentRequests < 1 {
		return NewConfigError("MaxConcurrentRequests must be at least 1")
	}

	if c.TracingEnabled && c.TracingEndpoint == "" {
		return NewConfigError("TracingEndpoint must be set when tracing is enabled")
	}

	return nil
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
