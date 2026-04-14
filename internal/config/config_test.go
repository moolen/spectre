package config

import "testing"

func TestLoadConfigPreservesScrubSensitiveData(t *testing.T) {
	cfg := LoadConfig(
		8080,
		[]string{"info"},
		"watcher.yaml",
		1,
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
