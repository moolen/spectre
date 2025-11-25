package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// WatcherConfig represents the configuration for watchers
type WatcherConfig struct {
	Resources []Resource `yaml:"resources"`
}

// Resource represents a single resource to watch
type Resource struct {
	Group     string `yaml:"group"`
	Version   string `yaml:"version"`
	Kind      string `yaml:"kind"`
	Namespace string `yaml:"namespace,omitempty"` // Optional, empty means cluster-wide
}

// LoadWatcherConfig loads the watcher configuration from a YAML file
func LoadWatcherConfig(path string) (*WatcherConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read watcher config file %s: %w", path, err)
	}

	var config WatcherConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse watcher config YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid watcher config: %w", err)
	}

	return &config, nil
}

// Validate checks that the watcher configuration is valid
func (wc *WatcherConfig) Validate() error {
	if len(wc.Resources) == 0 {
		return fmt.Errorf("at least one resource must be specified")
	}

	for i, resource := range wc.Resources {
		// Group can be empty for core resources (e.g., Pod, Service, Node)
		if resource.Version == "" {
			return fmt.Errorf("resource[%d]: version must not be empty", i)
		}
		if resource.Kind == "" {
			return fmt.Errorf("resource[%d]: kind must not be empty", i)
		}
	}

	return nil
}
