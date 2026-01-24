package config

import (
	"fmt"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// LoadIntegrationsFile loads and validates an integrations configuration file using Koanf.
// Returns the parsed and validated IntegrationsFile or an error.
//
// Error cases:
//   - File not found or cannot be read
//   - Invalid YAML syntax
//   - Schema validation failure (unsupported version, missing required fields, duplicate names)
//
// This loader performs synchronous loading - file watching for hot-reload
// will be implemented in a later plan.
func LoadIntegrationsFile(filepath string) (*IntegrationsFile, error) {
	// Create new Koanf instance with dot delimiter
	k := koanf.New(".")

	// Load file using file provider with YAML parser
	if err := k.Load(file.Provider(filepath), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("failed to load integrations config from %q: %w", filepath, err)
	}

	// Unmarshal into IntegrationsFile struct
	// Use UnmarshalWithConf to specify the yaml tag
	var config IntegrationsFile
	if err := k.UnmarshalWithConf("", &config, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return nil, fmt.Errorf("failed to parse integrations config from %q: %w", filepath, err)
	}

	// Validate schema version and structure
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("integrations config validation failed for %q: %w", filepath, err)
	}

	return &config, nil
}
