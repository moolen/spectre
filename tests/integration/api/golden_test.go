package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const kindPod = "Pod"

// GoldenMetadata represents the expected test metadata from .meta.json files
type GoldenMetadata struct {
	Scenario  ScenarioInfo    `json:"scenario"`
	Expected  ExpectedResults `json:"expected"`
	Timestamp int64           `json:"timestamp"`
}

// ScenarioInfo describes the test scenario
type ScenarioInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ExpectedResults contains expected anomalies and causal paths
type ExpectedResults struct {
	Anomalies  []ExpectedAnomaly   `json:"anomalies"`
	CausalPath *ExpectedCausalPath `json:"causal_path"`
}

// ExpectedAnomaly represents an expected anomaly detection result
type ExpectedAnomaly struct {
	NodeKind     string `json:"node_kind"`
	Category     string `json:"category"`
	Type         string `json:"type"`
	MinSeverity  string `json:"min_severity"`
	SummaryMatch string `json:"summary_match,omitempty"`
}

// ExpectedCausalPath represents expected causal path discovery results
type ExpectedCausalPath struct {
	RootKind          string   `json:"root_kind"`
	IntermediateKinds []string `json:"intermediate_kinds"`
	SymptomKind       string   `json:"symptom_kind"`
	MinConfidence     float64  `json:"min_confidence"`
}

// TestGoldenScenarios runs tests against all golden fixtures generated from real scenarios
func TestGoldenScenarios(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get caller information")
	}
	testDir := filepath.Dir(testFile)
	goldenDir := filepath.Join(testDir, "..", "fixtures", "golden")

	if _, err := os.Stat(goldenDir); os.IsNotExist(err) {
		t.Skipf("golden fixtures directory not found at %s - run golden-generator to create fixtures", goldenDir)
	}

	// List all fixture files
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("failed to read golden fixtures directory: %v", err)
	}

	// Collect scenarios (JSONL files with matching meta.json)
	scenarios := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) == ".jsonl" {
			scenarioName := name[:len(name)-6] // Remove .jsonl extension
			metaFile := filepath.Join(goldenDir, scenarioName+".meta.json")
			if fileExists(metaFile) {
				scenarios[scenarioName] = metaFile
			}
		}
	}

	if len(scenarios) == 0 {
		t.Skipf("no golden fixtures found in %s - run golden-generator to create fixtures", goldenDir)
	}

	t.Logf("Found %d golden scenarios to test", len(scenarios))

	// Run tests for each scenario
	for scenarioName, metaFile := range scenarios {
		t.Run(scenarioName, func(t *testing.T) {
			t.Parallel() // Run scenarios in parallel
			runGoldenScenario(t, goldenDir, scenarioName, metaFile)
		})
	}
}

func runGoldenScenario(t *testing.T, goldenDir, scenarioName, metaFile string) {
	ctx := context.Background()
	metadata, err := loadGoldenMetadata(metaFile)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	t.Logf("Testing scenario: %s - %s", metadata.Scenario.Name, metadata.Scenario.Description)

	fixtureFile := filepath.Join(goldenDir, scenarioName+".jsonl")
	if _, err := os.Stat(fixtureFile); os.IsNotExist(err) {
		t.Fatalf("fixture file not found: %s", fixtureFile)
	}
	events, err := LoadAuditLog(fixtureFile)
	if err != nil {
		t.Fatalf("failed to load fixture events: %v", err)
	}
	if len(events) == 0 {
		t.Skipf("fixture %s has no events - skipping test", scenarioName)
	}

	t.Logf("Loaded %d events from fixture", len(events))

	// Determine which resource kind to use as symptom
	// If expected causal path has symptom_kind != "Pod", use that kind
	var timestamp int64
	var resourceUID string
	symptomKind := kindPod // Default
	if metadata.Expected.CausalPath != nil && metadata.Expected.CausalPath.SymptomKind != "" && metadata.Expected.CausalPath.SymptomKind != kindPod {
		symptomKind = metadata.Expected.CausalPath.SymptomKind
	}

	if symptomKind != kindPod {
		// Extract UID for the specific symptom kind (e.g., Service)
		var err error
		timestamp, resourceUID, err = ExtractTimestampAndResourceUIDFromFile(fixtureFile, symptomKind)
		if err != nil {
			t.Fatalf("failed to extract timestamp and %s UID: %v", symptomKind, err)
		}
	} else {
		// Default behavior: extract Pod UID
		var err error
		timestamp, resourceUID, err = ExtractTimestampAndPodUIDFromFile(fixtureFile)
		if err != nil {
			t.Fatalf("failed to extract timestamp and UID: %v", err)
		}
	}

	if resourceUID == "" {
		t.Skipf("no %s UID found in fixture - skipping test", symptomKind)
	}
	t.Logf("Using %s resource UID: %s, timestamp: %d", symptomKind, resourceUID, timestamp)

	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("failed to create test harness: %v", err)
	}
	defer harness.Cleanup(ctx)
	if err := harness.SeedEventsFromAuditLog(ctx, fixtureFile); err != nil {
		t.Fatalf("failed to seed events from fixture: %v", err)
	}

	if len(metadata.Expected.Anomalies) > 0 {
		t.Run("Anomalies", func(t *testing.T) {
			testGoldenAnomalies(t, harness, resourceUID, timestamp, metadata.Expected.Anomalies)
		})
	}

	if metadata.Expected.CausalPath != nil {
		t.Run("CausalPaths", func(t *testing.T) {
			testGoldenCausalPaths(t, harness, resourceUID, timestamp, metadata.Expected.CausalPath)
		})
	}
}

func testGoldenAnomalies(t *testing.T, harness *TestHarness, resourceUID string, timestamp int64, expectedAnomalies []ExpectedAnomaly) {
	logger := logging.GetLogger("test")
	handler := handlers.NewAnomalyHandler(harness.GetClient(), logger, nil)

	// Convert nanoseconds to seconds, rounding up to ensure we include events in the same second
	endSec := (timestamp / 1_000_000_000) + 1
	startSec := endSec - 600 // 10 minutes

	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies", http.NoBody)
	q := req.URL.Query()
	q.Set("resourceUID", resourceUID)
	q.Set("start", strconv.FormatInt(startSec, 10))
	q.Set("end", strconv.FormatInt(endSec, 10))
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	var result anomaly.AnomalyResponse
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response: %s", rr.Body.String())
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Equal(t, resourceUID, result.Metadata.ResourceUID, "Resource UID should match")

	t.Logf("Anomaly detection found %d anomalies across %d nodes",
		len(result.Anomalies), result.Metadata.NodesAnalyzed)

	for i, a := range result.Anomalies {
		t.Logf("  Anomaly %d: Kind=%s, Category=%s, Type=%s, Summary=%s",
			i+1, a.Node.Kind, a.Category, a.Type, a.Summary)
	}
	assert.Greater(t, len(result.Anomalies), 0, "Expected at least one anomaly, got %d", len(result.Anomalies))

	for _, expected := range expectedAnomalies {
		found := findMatchingAnomaly(result.Anomalies, expected)
		assert.True(t, found, "Expected anomaly not found: Kind=%s, Category=%s, Type=%s",
			expected.NodeKind, expected.Category, expected.Type)
	}
}

func testGoldenCausalPaths(t *testing.T, harness *TestHarness, resourceUID string, timestamp int64, expected *ExpectedCausalPath) {
	logger := logging.GetLogger("test")
	handler := handlers.NewCausalPathsHandler(harness.GetClient(), logger, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/causal-paths", http.NoBody)
	q := req.URL.Query()
	q.Set("resourceUID", resourceUID)
	// Use timestamp+1ns to ensure we capture events at the exact failure timestamp
	q.Set("failureTimestamp", strconv.FormatInt(timestamp+1, 10))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "10")
	q.Set("maxPaths", "10")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	var result causalpaths.CausalPathsResponse
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response: %s", rr.Body.String())
	require.NotNil(t, result.Metadata, "Metadata should not be nil")

	t.Logf("Causal paths discovery found %d paths, explored %d nodes",
		len(result.Paths), result.Metadata.NodesExplored)
	for i, path := range result.Paths {
		t.Logf("  Path %d: confidence=%.2f, root=%s (%s)",
			i+1, path.ConfidenceScore,
			path.CandidateRoot.Resource.Name, path.CandidateRoot.Resource.Kind)
		for j, step := range path.Steps {
			t.Logf("    Step %d: %s (%s)", j+1, step.Node.Resource.Name, step.Node.Resource.Kind)
		}
	}

	// Skip causal path validation if no expected path is specified (empty root_kind)
	if expected.RootKind == "" && expected.SymptomKind == "" {
		t.Logf("No expected causal path specified - skipping causal path validation")
		return
	}

	assert.Greater(t, len(result.Paths), 0, "Expected at least one causal path, got %d", len(result.Paths))

	found := false
	for _, path := range result.Paths {
		if matchesCausalPath(path, expected) {
			found = true
			t.Logf("Found matching causal path: root=%s, confidence=%.2f (min expected: %.2f)",
				path.CandidateRoot.Resource.Kind, path.ConfidenceScore, expected.MinConfidence)

			assert.GreaterOrEqual(t, path.ConfidenceScore, expected.MinConfidence,
				"Path confidence %.2f should be >= expected minimum %.2f",
				path.ConfidenceScore, expected.MinConfidence)
			break
		}
	}

	assert.True(t, found, "No causal path found matching expected structure: root=%s, symptom=%s",
		expected.RootKind, expected.SymptomKind)
}

// findMatchingAnomaly checks if an anomaly matching the expected criteria exists
func findMatchingAnomaly(anomalies []anomaly.Anomaly, expected ExpectedAnomaly) bool {
	for _, a := range anomalies {
		// Match by Kind
		if !strings.EqualFold(a.Node.Kind, expected.NodeKind) {
			continue
		}

		// Match by Category (case insensitive)
		if !strings.EqualFold(string(a.Category), expected.Category) {
			continue
		}

		// Match by Type if specified (case insensitive, partial match)
		if expected.Type != "" {
			if !strings.Contains(strings.ToLower(a.Type), strings.ToLower(expected.Type)) {
				continue
			}
		}

		// Match by Summary if specified (partial match)
		if expected.SummaryMatch != "" {
			if !strings.Contains(strings.ToLower(a.Summary), strings.ToLower(expected.SummaryMatch)) {
				continue
			}
		}

		return true
	}
	return false
}

// matchesCausalPath checks if a causal path matches expected structure
func matchesCausalPath(path causalpaths.CausalPath, expected *ExpectedCausalPath) bool {
	// Check root kind
	if !strings.EqualFold(path.CandidateRoot.Resource.Kind, expected.RootKind) {
		return false
	}

	// Check symptom kind (should be at the end of the path or in steps)
	symptomFound := false
	for _, step := range path.Steps {
		if strings.EqualFold(step.Node.Resource.Kind, expected.SymptomKind) {
			symptomFound = true
			break
		}
	}

	if !symptomFound {
		return false
	}

	// Check intermediate kinds if specified
	if len(expected.IntermediateKinds) > 0 {
		for _, intermediateKind := range expected.IntermediateKinds {
			found := false
			for _, step := range path.Steps {
				if strings.EqualFold(step.Node.Resource.Kind, intermediateKind) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// loadGoldenMetadata loads metadata from a .meta.json file
func loadGoldenMetadata(metaFile string) (*GoldenMetadata, error) {
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, err
	}

	var metadata GoldenMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
