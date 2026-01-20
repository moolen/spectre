//go:build integration
// +build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
)

// TestAnomalyHandler_FluxHelmRelease tests the anomaly detection handler
// for a Flux-managed HelmRelease scenario.
func TestAnomalyHandler_FluxHelmRelease(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-fluxhelmrelease-endpoint-e.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Convert timestamp to seconds (start = timestamp - 10 minutes, end = timestamp)
	endSec := timestamp / 1_000_000_000
	startSec := endSec - 600 // 10 minutes

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewAnomalyHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("start", strconv.FormatInt(startSec, 10))
	q.Set("end", strconv.FormatInt(endSec, 10))
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result anomaly.AnomalyResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response: %s", rr.Body.String())

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Equal(t, podUID, result.Metadata.ResourceUID, "Resource UID should match")
	require.Greater(t, result.Metadata.NodesAnalyzed, 0, "Should have analyzed at least one node")

	t.Logf("Anomaly detection found %d anomalies across %d nodes",
		len(result.Anomalies), result.Metadata.NodesAnalyzed)

	// Log anomaly details for debugging
	for i, a := range result.Anomalies {
		t.Logf("  Anomaly %d: %s (%s/%s) - %s: %s",
			i+1, a.Node.Kind, a.Node.Namespace, a.Node.Name, a.Category, a.Summary)
	}
}

// TestAnomalyHandler_FluxKustomization tests the anomaly detection handler
// for a Flux Kustomization scenario.
func TestAnomalyHandler_FluxKustomization(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-fluxkustomization-endpoint.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Convert timestamp to seconds (start = timestamp - 10 minutes, end = timestamp)
	endSec := timestamp / 1_000_000_000
	startSec := endSec - 600 // 10 minutes

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewAnomalyHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("start", strconv.FormatInt(startSec, 10))
	q.Set("end", strconv.FormatInt(endSec, 10))
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result anomaly.AnomalyResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Equal(t, podUID, result.Metadata.ResourceUID, "Resource UID should match")
	require.Greater(t, result.Metadata.NodesAnalyzed, 0, "Should have analyzed at least one node")

	t.Logf("Anomaly detection found %d anomalies across %d nodes",
		len(result.Anomalies), result.Metadata.NodesAnalyzed)

	// Log anomaly details for debugging
	for i, a := range result.Anomalies {
		t.Logf("  Anomaly %d: %s (%s/%s) - %s: %s",
			i+1, a.Node.Kind, a.Node.Namespace, a.Node.Name, a.Category, a.Summary)
	}
}

// TestAnomalyHandler_StatefulSet tests the anomaly detection handler
// for a StatefulSet scenario.
func TestAnomalyHandler_StatefulSet(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-statefulsetimagechange-end.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Convert timestamp to seconds (start = timestamp - 10 minutes, end = timestamp)
	endSec := timestamp / 1_000_000_000
	startSec := endSec - 600 // 10 minutes

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewAnomalyHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("start", strconv.FormatInt(startSec, 10))
	q.Set("end", strconv.FormatInt(endSec, 10))
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result anomaly.AnomalyResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Equal(t, podUID, result.Metadata.ResourceUID, "Resource UID should match")
	require.Greater(t, result.Metadata.NodesAnalyzed, 0, "Should have analyzed at least one node")

	t.Logf("Anomaly detection found %d anomalies across %d nodes",
		len(result.Anomalies), result.Metadata.NodesAnalyzed)

	// Log anomaly details for debugging
	for i, a := range result.Anomalies {
		t.Logf("  Anomaly %d: %s (%s/%s) - %s: %s",
			i+1, a.Node.Kind, a.Node.Namespace, a.Node.Name, a.Category, a.Summary)
	}
}

// TestAnomalyHandler_NetworkPolicy tests the anomaly detection handler
// for a NetworkPolicy scenario.
func TestAnomalyHandler_NetworkPolicy(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-networkpolicy-samenamespac.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Convert timestamp to seconds (start = timestamp - 10 minutes, end = timestamp)
	endSec := timestamp / 1_000_000_000
	startSec := endSec - 600 // 10 minutes

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewAnomalyHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("start", strconv.FormatInt(startSec, 10))
	q.Set("end", strconv.FormatInt(endSec, 10))
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result anomaly.AnomalyResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Equal(t, podUID, result.Metadata.ResourceUID, "Resource UID should match")
	require.Greater(t, result.Metadata.NodesAnalyzed, 0, "Should have analyzed at least one node")

	t.Logf("Anomaly detection found %d anomalies across %d nodes",
		len(result.Anomalies), result.Metadata.NodesAnalyzed)

	// Log anomaly details for debugging
	for i, a := range result.Anomalies {
		t.Logf("  Anomaly %d: %s (%s/%s) - %s: %s",
			i+1, a.Node.Kind, a.Node.Namespace, a.Node.Name, a.Category, a.Summary)
	}
}

// TestAnomalyHandler_Ingress tests the anomaly detection handler
// for an Ingress scenario.
func TestAnomalyHandler_Ingress(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-ingress-samenamespace-endp.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Convert timestamp to seconds (start = timestamp - 10 minutes, end = timestamp)
	endSec := timestamp / 1_000_000_000
	startSec := endSec - 600 // 10 minutes

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewAnomalyHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/anomalies", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("start", strconv.FormatInt(startSec, 10))
	q.Set("end", strconv.FormatInt(endSec, 10))
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result anomaly.AnomalyResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Equal(t, podUID, result.Metadata.ResourceUID, "Resource UID should match")
	require.Greater(t, result.Metadata.NodesAnalyzed, 0, "Should have analyzed at least one node")

	t.Logf("Anomaly detection found %d anomalies across %d nodes",
		len(result.Anomalies), result.Metadata.NodesAnalyzed)

	// Log anomaly details for debugging
	for i, a := range result.Anomalies {
		t.Logf("  Anomaly %d: %s (%s/%s) - %s: %s",
			i+1, a.Node.Kind, a.Node.Namespace, a.Node.Name, a.Category, a.Summary)
	}
}
