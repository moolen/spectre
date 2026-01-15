package reconciler

import "time"

// Config holds configuration for the reconciler.
type Config struct {
	// Enabled controls whether reconciliation runs.
	Enabled bool

	// Interval between reconciliation cycles.
	Interval time.Duration

	// BatchSize limits resources checked per cycle per handler.
	BatchSize int
}

// DefaultConfig returns the default reconciler configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:   true,
		Interval:  5 * time.Minute,
		BatchSize: 100,
	}
}
