package store

import "testing"

func TestResourceWindowStartUsesRawSubtraction(t *testing.T) {
	t.Parallel()

	window := ResourceWindow{
		FailureTimestampNs: 1_000,
		LookbackNs:         150,
	}

	if got, want := window.Start(), int64(850); got != want {
		t.Fatalf("unexpected window start: got %d want %d", got, want)
	}
}

func TestResourceWindowStartAllowsNegativeValues(t *testing.T) {
	t.Parallel()

	window := ResourceWindow{
		FailureTimestampNs: 5,
		LookbackNs:         10,
	}

	if got, want := window.Start(), int64(-5); got != want {
		t.Fatalf("expected negative start: got %d want %d", got, want)
	}
}
