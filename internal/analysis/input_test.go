package analysis

import "testing"

func TestPrepareAnalyzeInput_AppliesDefaults(t *testing.T) {
	input := PrepareAnalyzeInput(AnalyzeInput{})

	if input.LookbackNs != DefaultLookbackNs {
		t.Fatalf("expected default lookback %d, got %d", DefaultLookbackNs, input.LookbackNs)
	}
	if input.Format != FormatDiff {
		t.Fatalf("expected default format %q, got %q", FormatDiff, input.Format)
	}
}

func TestPrepareAnalyzeInput_PreservesExplicitValues(t *testing.T) {
	input := PrepareAnalyzeInput(AnalyzeInput{
		LookbackNs:       123,
		MaxDepth:         7,
		MinConfidence:    0.9,
		Format:           FormatLegacy,
		ResourceUID:      "pod-123",
		FailureTimestamp: 456,
	})

	if input.LookbackNs != 123 {
		t.Fatalf("expected lookback 123, got %d", input.LookbackNs)
	}
	if input.Format != FormatLegacy {
		t.Fatalf("expected format %q, got %q", FormatLegacy, input.Format)
	}
	if input.MaxDepth != 7 {
		t.Fatalf("expected max depth 7, got %d", input.MaxDepth)
	}
	if input.MinConfidence != 0.9 {
		t.Fatalf("expected min confidence 0.9, got %f", input.MinConfidence)
	}
}
