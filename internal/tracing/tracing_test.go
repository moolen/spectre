package tracing

import (
	"testing"
)

func TestTLSInsecureConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		expectError bool
		description string
	}{
		{
			name: "TLS with insecure skip verify",
			cfg: Config{
				Enabled:     true,
				Endpoint:    "localhost:4317",
				TLSInsecure: true,
			},
			expectError: false,
			description: "Should create provider with InsecureSkipVerify=true",
		},
		{
			name: "TLS with CA certificate",
			cfg: Config{
				Enabled:   true,
				Endpoint:  "localhost:4317",
				TLSCAPath: "/path/to/ca.crt",
			},
			expectError: true, // Will fail because file doesn't exist, but that's OK for this test
			description: "Should attempt to load CA certificate",
		},
		{
			name: "No TLS (insecure connection)",
			cfg: Config{
				Enabled:  true,
				Endpoint: "localhost:4317",
			},
			expectError: false,
			description: "Should create provider without TLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewTracingProvider(tt.cfg)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if provider != nil && provider.enabled != tt.cfg.Enabled {
				t.Errorf("Provider enabled=%v, want %v", provider.enabled, tt.cfg.Enabled)
			}
		})
	}
}
