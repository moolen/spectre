package causalpaths

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/anomaly"
	analysisembedded "github.com/moolen/spectre/internal/analysis/store/embedded"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestPathDiscoverer_IdentifyFirstFailurePrefersNonChangeAnomalies(t *testing.T) {
	discoverer := &PathDiscoverer{}
	changeTs := time.Unix(100, 0)
	failureTs := time.Unix(200, 0)
	fallbackTs := int64(300)

	actual := discoverer.identifyFirstFailure([]anomaly.Anomaly{
		{
			Timestamp: changeTs,
			Category:  anomaly.CategoryChange,
			Type:      "SpecModified",
		},
		{
			Timestamp: failureTs,
			Category:  anomaly.CategoryState,
			Type:      "CrashLoopBackOff",
		},
	}, fallbackTs)

	require.Equal(t, failureTs, actual)
}

func TestPathDiscoverer_IdentifyFirstFailureFallsBackWhenSymptomOnlyHasChangeAnomalies(t *testing.T) {
	discoverer := &PathDiscoverer{}
	fallbackTs := int64(300)

	actual := discoverer.identifyFirstFailure([]anomaly.Anomaly{
		{
			Timestamp: time.Unix(100, 0),
			Category:  anomaly.CategoryChange,
			Type:      "SpecModified",
		},
	}, fallbackTs)

	require.Equal(t, time.Unix(0, fallbackTs), actual)
}

func TestPathDiscoverer_EmbeddedGoldenHelmReleaseValuesFromReturnsHelmReleaseRoot(t *testing.T) {
	ctx := context.Background()
	events := loadFixtureEvents(t, "golden/helmrelease-valuesfrom-failure.jsonl")
	failureTimestamp, podUID := extractFixtureFailureAndPod(t, events)

	store, err := analysisembedded.New(events)
	require.NoError(t, err)

	analyzer := analysis.NewRootCauseAnalyzer(store)
	graph, err := analyzer.Analyze(ctx, analysis.AnalyzeInput{
		ResourceUID:      podUID,
		FailureTimestamp: failureTimestamp,
		LookbackNs:       int64((10 * time.Minute).Nanoseconds()),
		MaxDepth:         5,
		MinConfidence:    0.5,
		Format:           analysis.FormatDiff,
	})
	require.NoError(t, err)
	require.True(t, graphHasEdgeBetweenKinds(graph, "HelmRelease", "MANAGES", "Deployment"))
	require.True(t, graphHasEdgeBetweenKinds(graph, "HelmRelease", "REFERENCES_SPEC", "ConfigMap"))

	discoverer := NewPathDiscoverer(store)
	response, err := discoverer.DiscoverCausalPaths(ctx, CausalPathsInput{
		ResourceUID:      podUID,
		FailureTimestamp: failureTimestamp,
		LookbackNs:       int64((10 * time.Minute).Nanoseconds()),
		MaxDepth:         5,
		MaxPaths:         10,
	})
	require.NoError(t, err)

	require.True(t, responseHasRootKind(response, "HelmRelease"), "expected a HelmRelease-rooted causal path")
}

func TestPathDiscoverer_EmbeddedGoldenHelmReleaseUpgradeMeetsConfidenceFloor(t *testing.T) {
	ctx := context.Background()
	events := loadFixtureEvents(t, "golden/helmrelease-upgrade-failure.jsonl")
	failureTimestamp, podUID := extractFixtureFailureAndPod(t, events)

	store, err := analysisembedded.New(events)
	require.NoError(t, err)

	discoverer := NewPathDiscoverer(store)
	response, err := discoverer.DiscoverCausalPaths(ctx, CausalPathsInput{
		ResourceUID:      podUID,
		FailureTimestamp: failureTimestamp,
		LookbackNs:       int64((10 * time.Minute).Nanoseconds()),
		MaxDepth:         5,
		MaxPaths:         10,
	})
	require.NoError(t, err)

	path := firstPathByRootKind(response, "HelmRelease")
	require.NotNil(t, path)
	require.GreaterOrEqual(t, path.ConfidenceScore, 0.85)
}

func loadFixtureEvents(t *testing.T, relativePath string) []models.Event {
	t.Helper()

	path := fixturePath(t, relativePath)
	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, file.Close())
	}()

	var events []models.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event models.Event
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &event))
		events = append(events, event)
	}
	require.NoError(t, scanner.Err())
	require.NotEmpty(t, events)

	return events
}

func fixturePath(t *testing.T, relativePath string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "tests", "integration", "fixtures", relativePath)
}

func extractFixtureFailureAndPod(t *testing.T, events []models.Event) (int64, string) {
	t.Helper()

	var failureTimestamp int64
	var podUID string
	for _, event := range events {
		if event.Timestamp > failureTimestamp {
			failureTimestamp = event.Timestamp
		}
		if event.Resource.Kind == "Pod" {
			podUID = event.Resource.UID
		}
	}

	require.NotZero(t, failureTimestamp)
	require.NotEmpty(t, podUID)
	return failureTimestamp, podUID
}

func graphHasEdgeBetweenKinds(result *analysis.RootCauseAnalysisV2, fromKind, relType, toKind string) bool {
	nodeKinds := make(map[string]string, len(result.Incident.Graph.Nodes))
	for _, node := range result.Incident.Graph.Nodes {
		nodeKinds[node.ID] = node.Resource.Kind
	}
	for _, edge := range result.Incident.Graph.Edges {
		if nodeKinds[edge.From] == fromKind && edge.RelationshipType == relType && nodeKinds[edge.To] == toKind {
			return true
		}
	}
	return false
}

func responseHasRootKind(response *CausalPathsResponse, kind string) bool {
	return firstPathByRootKind(response, kind) != nil
}

func firstPathByRootKind(response *CausalPathsResponse, kind string) *CausalPath {
	for _, path := range response.Paths {
		if path.CandidateRoot.Resource.Kind == kind {
			pathCopy := path
			return &pathCopy
		}
	}
	return nil
}
