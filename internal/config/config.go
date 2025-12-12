package config

// Config holds all configuration for the application
type Config struct {
	// DataDir is the directory where events are stored
	DataDir string

	// APIPort is the port the API server listens on
	APIPort int

	// LogLevel is the logging level (debug, info, warn, error)
	LogLevel string

	// WatcherConfigPath is the path to the YAML file containing watcher configuration
	WatcherConfigPath string

	// SegmentSize is the target size for compression segments in bytes
	SegmentSize int64

	// MaxConcurrentRequests is the maximum number of concurrent API requests
	MaxConcurrentRequests int

	// BlockCacheMaxMB is the maximum memory for block cache in MB
	BlockCacheMaxMB int64

	// BlockCacheEnabled indicates whether block caching is enabled
	BlockCacheEnabled bool

	// TracingEnabled indicates whether OpenTelemetry tracing is enabled
	TracingEnabled bool

	// TracingEndpoint is the OTLP gRPC endpoint for trace export
	TracingEndpoint string

	// TracingTLSCAPath is the path to the CA certificate for TLS verification
	TracingTLSCAPath string
}

// LoadConfig creates a Config with the provided values
func LoadConfig(dataDir string, apiPort int, logLevel, watcherConfigPath string, segmentSize int64, maxConcurrentRequests int, blockCacheMaxMB int64, blockCacheEnabled bool, tracingEnabled bool, tracingEndpoint string, tracingTLSCAPath string) *Config {
	cfg := &Config{
		DataDir:               dataDir,
		APIPort:               apiPort,
		LogLevel:              logLevel,
		WatcherConfigPath:     watcherConfigPath,
		SegmentSize:           segmentSize,
		MaxConcurrentRequests: maxConcurrentRequests,
		BlockCacheMaxMB:       blockCacheMaxMB,
		BlockCacheEnabled:     blockCacheEnabled,
		TracingEnabled:        tracingEnabled,
		TracingEndpoint:       tracingEndpoint,
		TracingTLSCAPath:      tracingTLSCAPath,
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

	if c.BlockCacheEnabled && c.BlockCacheMaxMB < 1 {
		return NewConfigError("BlockCacheMaxMB must be at least 1 when cache is enabled")
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
