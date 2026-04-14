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
