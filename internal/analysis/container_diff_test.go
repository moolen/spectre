package analysis_test

import (
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/analysis"
)

func TestContainerDiff(t *testing.T) {
	old := `{
		"spec": {
			"template": {
				"spec": {
					"containers": [
						{"name": "app", "image": "nginx:1.19", "env": [{"name": "FOO", "value": "bar"}]}
					]
				}
			}
		}
	}`

	newJSON := `{
		"spec": {
			"template": {
				"spec": {
					"containers": [
						{"name": "app", "image": "nginx:1.20", "env": [{"name": "FOO", "value": "baz"}]}
					]
				}
			}
		}
	}`

	diffs, err := analysis.ComputeJSONDiff([]byte(old), []byte(newJSON))
	if err != nil {
		t.Fatal(err)
	}

	// Filter to spec-only changes
	specDiffs := analysis.FilterSpecOnly(diffs)

	if len(specDiffs) == 0 {
		t.Fatal("Expected diffs but got none")
	}

	// Check that we got container-level diffs with proper path format
	foundImageDiff := false
	foundEnvDiff := false
	for _, d := range specDiffs {
		if strings.Contains(d.Path, "containers[name=app].image") {
			foundImageDiff = true
			if d.OldValue != "nginx:1.19" || d.NewValue != "nginx:1.20" {
				t.Errorf("Image diff has wrong values: old=%v new=%v", d.OldValue, d.NewValue)
			}
		}
		if strings.Contains(d.Path, "env[name=FOO].value") {
			foundEnvDiff = true
			if d.OldValue != "bar" || d.NewValue != "baz" {
				t.Errorf("Env diff has wrong values: old=%v new=%v", d.OldValue, d.NewValue)
			}
		}
	}

	if !foundImageDiff {
		t.Error("Expected container image diff with path containing 'containers[name=app].image'")
	}
	if !foundEnvDiff {
		t.Error("Expected env diff with path containing 'env[name=FOO].value'")
	}
}

func TestContainerAddedAndRemoved(t *testing.T) {
	old := `{
		"spec": {
			"containers": [
				{"name": "app", "image": "nginx:1.19"},
				{"name": "sidecar", "image": "envoy:1.0"}
			]
		}
	}`

	newJSON := `{
		"spec": {
			"containers": [
				{"name": "app", "image": "nginx:1.20"},
				{"name": "init", "image": "busybox:latest"}
			]
		}
	}`

	diffs, err := analysis.ComputeJSONDiff([]byte(old), []byte(newJSON))
	if err != nil {
		t.Fatal(err)
	}

	specDiffs := analysis.FilterSpecOnly(diffs)

	foundImageChange := false
	foundSidecarRemoved := false
	foundInitAdded := false

	for _, d := range specDiffs {
		switch {
		case strings.Contains(d.Path, "containers[name=app].image"):
			foundImageChange = true
			if d.Op != "replace" {
				t.Errorf("Expected replace op for image change, got %s", d.Op)
			}
		case strings.Contains(d.Path, "containers[name=sidecar]") && d.Op == "remove":
			foundSidecarRemoved = true
		case strings.Contains(d.Path, "containers[name=init]") && d.Op == "add":
			foundInitAdded = true
		}
	}

	if !foundImageChange {
		t.Error("Expected image change diff")
	}
	if !foundSidecarRemoved {
		t.Error("Expected sidecar container removal diff")
	}
	if !foundInitAdded {
		t.Error("Expected init container addition diff")
	}
}

func TestArrayDiffByIndex(t *testing.T) {
	// Test arrays without key fields (should diff by index)
	old := `{
		"spec": {
			"args": ["--verbose", "--port=8080"]
		}
	}`

	newJSON := `{
		"spec": {
			"args": ["--verbose", "--port=9090", "--debug"]
		}
	}`

	diffs, err := analysis.ComputeJSONDiff([]byte(old), []byte(newJSON))
	if err != nil {
		t.Fatal(err)
	}

	specDiffs := analysis.FilterSpecOnly(diffs)

	if len(specDiffs) == 0 {
		t.Fatal("Expected diffs but got none")
	}

	// Should have diffs for the changed and added elements
	foundPortChange := false
	foundDebugAdded := false

	for _, d := range specDiffs {
		if strings.Contains(d.Path, "args[1]") && d.Op == "replace" {
			foundPortChange = true
		}
		if strings.Contains(d.Path, "args[2]") && d.Op == "add" {
			foundDebugAdded = true
		}
	}

	if !foundPortChange {
		t.Error("Expected port change diff at args[1]")
	}
	if !foundDebugAdded {
		t.Error("Expected debug flag addition at args[2]")
	}
}
