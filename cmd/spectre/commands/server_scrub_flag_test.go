package commands

import "testing"

func TestServerScrubSensitiveDataFlagDefaultsToFalse(t *testing.T) {
	flag := serverCmd.Flags().Lookup("scrub-sensitive-data")
	if flag == nil {
		t.Fatalf("expected scrub-sensitive-data flag to be registered")
	}

	if flag.DefValue != "false" {
		t.Fatalf("expected default false, got %q", flag.DefValue)
	}
}

func TestServerScrubSensitiveDataFlagSetsVariable(t *testing.T) {
	flag := serverCmd.Flags().Lookup("scrub-sensitive-data")
	if flag == nil {
		t.Fatalf("expected scrub-sensitive-data flag to be registered")
	}

	originalScrubSensitiveData := scrubSensitiveData
	originalValue := flag.Value.String()

	t.Cleanup(func() {
		scrubSensitiveData = originalScrubSensitiveData
		_ = serverCmd.Flags().Set("scrub-sensitive-data", originalValue)
	})

	if err := serverCmd.Flags().Set("scrub-sensitive-data", "true"); err != nil {
		t.Fatalf("failed to set scrub-sensitive-data flag: %v", err)
	}

	if !scrubSensitiveData {
		t.Fatalf("expected scrubSensitiveData to be true")
	}
}
