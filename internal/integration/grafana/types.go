package grafana

import (
	"fmt"
	"strings"
)

// SecretRef references a Kubernetes Secret for sensitive values
type SecretRef struct {
	// SecretName is the name of the Kubernetes Secret in the same namespace as Spectre
	SecretName string `json:"secretName" yaml:"secretName"`

	// Key is the key within the Secret's Data map
	Key string `json:"key" yaml:"key"`
}

// Config represents the Grafana integration configuration
type Config struct {
	// URL is the base URL for the Grafana instance (Cloud or self-hosted)
	// Examples: https://myorg.grafana.net or https://grafana.internal:3000
	URL string `json:"url" yaml:"url"`

	// APITokenRef references a Kubernetes Secret containing the API token
	APITokenRef *SecretRef `json:"apiTokenRef,omitempty" yaml:"apiTokenRef,omitempty"`
}

// Validate checks config for common errors
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("url is required")
	}

	// Normalize URL: remove trailing slash for consistency
	c.URL = strings.TrimSuffix(c.URL, "/")

	// Validate SecretRef if present
	if c.APITokenRef != nil && c.APITokenRef.SecretName != "" {
		if c.APITokenRef.Key == "" {
			return fmt.Errorf("apiTokenRef.key is required when apiTokenRef is specified")
		}
	}

	return nil
}

// UsesSecretRef returns true if config uses Kubernetes Secret for authentication
func (c *Config) UsesSecretRef() bool {
	return c.APITokenRef != nil && c.APITokenRef.SecretName != ""
}
