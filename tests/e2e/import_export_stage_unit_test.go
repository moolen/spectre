package e2e

import (
	"strings"
	"testing"
)

func TestNewCLIClusterName_UniqueForRapidCalls(t *testing.T) {
	first := newCLIClusterName()
	second := newCLIClusterName()

	if first == second {
		t.Fatalf("expected unique cluster names for rapid calls, got %q", first)
	}

	if !strings.HasPrefix(first, "cli-test-") {
		t.Fatalf("expected cli cluster name prefix, got %q", first)
	}

	if !strings.HasPrefix(second, "cli-test-") {
		t.Fatalf("expected cli cluster name prefix, got %q", second)
	}
}
