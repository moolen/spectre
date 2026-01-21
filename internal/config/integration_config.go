package config

import (
	"fmt"
)

// IntegrationsFile represents the top-level structure of the integrations config file.
// This file defines integration instances with their configurations.
//
// Example YAML structure:
//
//	schema_version: v1
//	instances:
//	  - name: victorialogs-prod
//	    type: victorialogs
//	    enabled: true
//	    config:
//	      url: "http://victorialogs:9428"
//	  - name: victorialogs-staging
//	    type: victorialogs
//	    enabled: false
//	    config:
//	      url: "http://victorialogs-staging:9428"
type IntegrationsFile struct {
	// SchemaVersion is the explicit config schema version (e.g., "v1")
	// Used for in-memory migration when loading older config formats
	SchemaVersion string `yaml:"schema_version"`

	// Instances is the list of integration instances to manage
	Instances []IntegrationConfig `yaml:"instances"`
}

// IntegrationConfig represents a single integration instance configuration.
// Each instance has a unique name and type-specific configuration.
type IntegrationConfig struct {
	// Name is the unique instance name (e.g., "victorialogs-prod")
	// Must be unique across all instances in the file
	Name string `yaml:"name"`

	// Type is the integration type (e.g., "victorialogs")
	// Multiple instances can have the same Type with different Names
	Type string `yaml:"type"`

	// Enabled indicates whether this instance should be started
	// Disabled instances are skipped during initialization
	Enabled bool `yaml:"enabled"`

	// Config holds instance-specific configuration as a map
	// Each integration type interprets this differently
	// (e.g., VictoriaLogs expects {"url": "http://..."})
	Config map[string]interface{} `yaml:"config"`
}

// Validate checks that the IntegrationsFile is valid.
// Returns descriptive errors for validation failures.
func (f *IntegrationsFile) Validate() error {
	// Check schema version
	if f.SchemaVersion != "v1" {
		return NewConfigError(fmt.Sprintf(
			"unsupported schema_version: %q (expected \"v1\")",
			f.SchemaVersion,
		))
	}

	// Track instance names for uniqueness check
	seenNames := make(map[string]bool)

	for i, instance := range f.Instances {
		// Check required fields
		if instance.Name == "" {
			return NewConfigError(fmt.Sprintf(
				"instance[%d]: name is required",
				i,
			))
		}

		if instance.Type == "" {
			return NewConfigError(fmt.Sprintf(
				"instance[%d] (%s): type is required",
				i, instance.Name,
			))
		}

		// Check for duplicate names
		if seenNames[instance.Name] {
			return NewConfigError(fmt.Sprintf(
				"instance[%d]: duplicate instance name %q",
				i, instance.Name,
			))
		}
		seenNames[instance.Name] = true
	}

	return nil
}
