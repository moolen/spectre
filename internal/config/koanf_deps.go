package config

// This file ensures Koanf dependencies are added to go.mod for Phase 2 config loader implementation.
// The imports below are intentionally unused until the config loader is implemented.

import (
	_ "github.com/knadh/koanf/parsers/yaml" // YAML parser for Koanf
	_ "github.com/knadh/koanf/providers/file" // File provider with fsnotify support
	_ "github.com/knadh/koanf/v2" // Koanf v2 core library
)
