package config

import "testing"

func TestLoadConfigPreservesScrubSensitiveData(t *testing.T) {
	cfg := LoadConfig(
		"/data",
		8080,
		"info",
		"watcher.yaml",
		1024,
		1,
		1,
		true,
		false,
		"",
		"",
		false,
		true,
	)

	if !cfg.ScrubSensitiveData {
		t.Fatalf("expected ScrubSensitiveData to be true")
	}
}
