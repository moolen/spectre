package api

import "testing"

func TestEmbeddedGoldenScenarios(t *testing.T) {
	runGoldenScenarios(t, newEmbeddedGoldenHarness)
}
