package embedded

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestStore_FluxFixture_ManagersOwnershipAndNamespaceGraph(t *testing.T) {
	ctx := context.Background()
	fixturePath := fixturePath(t, "testrootcause-fluxhelmreleasevaluesfrom-.jsonl")
	events := loadFixtureEvents(t, fixturePath)
	failureTimestamp, podUID, namespace := extractFixtureContext(t, events)

	store, err := New(events)
	require.NoError(t, err)

	chain, err := store.GetOwnershipChain(ctx, podUID, failureTimestamp, 5)
	require.NoError(t, err)
	require.NotEmpty(t, chain)
	require.Equal(t, podUID, chain[0].Resource.UID)
	require.True(t, chainContainsKind(chain, "ReplicaSet"))
	require.True(t, chainContainsKind(chain, "Deployment"))

	deploymentUID := firstUIDByKind(chain, "Deployment")
	require.NotEmpty(t, deploymentUID)

	managers, err := store.GetManagers(ctx, []string{deploymentUID}, 0.5)
	require.NoError(t, err)
	require.Contains(t, managers, deploymentUID)
	require.Equal(t, "HelmRelease", managers[deploymentUID].Manager.Kind)

	namespaceGraph, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   namespace,
		TimestampNs: failureTimestamp,
		LookbackNs:  int64(10 * time.Minute),
		MaxDepth:    5,
		Limit:       200,
	})
	require.NoError(t, err)
	require.True(t, namespaceGraphHasNodeKind(namespaceGraph, "HelmRelease"))
	require.True(t, namespaceGraphHasNodeKind(namespaceGraph, "ConfigMap"))
	require.True(t, namespaceGraphHasEdge(namespaceGraph, "HelmRelease", "MANAGES", "Deployment"))
	require.True(t, namespaceGraphHasEdge(namespaceGraph, "HelmRelease", "REFERENCES_SPEC", "ConfigMap"))
}

func TestStore_FluxEndpointFixture_OwnershipChain(t *testing.T) {
	ctx := context.Background()
	fixturePath := fixturePath(t, "testrootcause-fluxhelmrelease-endpoint-e.jsonl")
	events := loadFixtureEvents(t, fixturePath)
	failureTimestamp, podUID, _ := extractFixtureContext(t, events)

	store, err := New(events)
	require.NoError(t, err)

	chain, err := store.GetOwnershipChain(ctx, podUID, failureTimestamp, 5)
	require.NoError(t, err)
	require.NotEmpty(t, chain)
	require.Equal(t, podUID, chain[0].Resource.UID)
}

func TestStore_IngressFixture_RelatedResourcesFromPod(t *testing.T) {
	ctx := context.Background()
	fixturePath := fixturePath(t, "testrootcause-ingress-samenamespace-endp.jsonl")
	events := loadFixtureEvents(t, fixturePath)
	failureTimestamp, podUID, _ := extractFixtureContext(t, events)

	store, err := New(events)
	require.NoError(t, err)

	related, err := store.GetRelatedResources(ctx, []string{podUID}, analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         int64(10 * time.Minute),
	})
	require.NoError(t, err)

	podRelated := related[podUID]
	require.True(t, hasRelatedKind(podRelated, "Node", "SCHEDULED_ON"))
	require.True(t, hasRelatedKind(podRelated, "ServiceAccount", "USES_SERVICE_ACCOUNT"))
	require.True(t, hasRelatedKind(podRelated, "Service", "SELECTS"))
	require.True(t, hasRelatedKind(podRelated, "Ingress", "INGRESS_REF"))
}

func TestStore_RBACFixture_RelatedResourcesFromPodIncludeBindingAndRole(t *testing.T) {
	ctx := context.Background()
	fixturePath := fixturePath(t, "golden/rbac-violation.jsonl")
	events := loadFixtureEvents(t, fixturePath)
	failureTimestamp, podUID, _ := extractFixtureContext(t, events)

	store, err := New(events)
	require.NoError(t, err)

	related, err := store.GetRelatedResources(ctx, []string{podUID}, analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         int64(10 * time.Minute),
	})
	require.NoError(t, err)

	podRelated := related[podUID]
	require.True(t, hasRelatedKind(podRelated, "ServiceAccount", "USES_SERVICE_ACCOUNT"))
	require.True(t, hasRelatedKind(podRelated, "RoleBinding", "GRANTS_TO"))
	require.True(t, hasRelatedKind(podRelated, "Role", "BINDS_ROLE"))
}

func TestStore_GetChangeEvents_PreservesConfigChangesOutsideRecentLimit(t *testing.T) {
	ctx := context.Background()
	events := make([]models.Event, 0, 13)

	events = append(events, newDeploymentEvent(t, "evt-create", 100, "demo:v1", 0))
	events = append(events, newDeploymentEvent(t, "evt-config", 200, "demo:v2", 0))
	for i := 0; i < 11; i++ {
		events = append(events, newDeploymentEvent(t, fmt.Sprintf("evt-status-%02d", i), int64(210+i), "demo:v2", i+1))
	}

	store, err := New(events)
	require.NoError(t, err)

	changeEvents, err := store.GetChangeEvents(ctx, []string{"deploy-uid"}, analysisstore.ResourceWindow{
		FailureTimestampNs: 400,
		LookbackNs:         500,
	})
	require.NoError(t, err)

	deploymentEvents := changeEvents["deploy-uid"]
	require.Len(t, deploymentEvents, 11)
	require.True(t, containsChangeEvent(deploymentEvents, "evt-config", true))
	require.True(t, containsChangeEvent(deploymentEvents, "evt-status-10", false))
	require.False(t, containsChangeEvent(deploymentEvents, "evt-create", false))
}

func fixturePath(t *testing.T, baseName string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "tests", "integration", "fixtures", baseName)
}

func loadFixtureEvents(t *testing.T, jsonlPath string) []models.Event {
	t.Helper()
	events, err := loadAuditLog(jsonlPath)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	return events
}

func extractFixtureContext(t *testing.T, events []models.Event) (int64, string, string) {
	t.Helper()
	var lastTimestamp int64
	var podUID string
	var namespace string
	for _, event := range events {
		if event.Timestamp > lastTimestamp {
			lastTimestamp = event.Timestamp
		}
		if event.Resource.Kind == "Pod" {
			podUID = event.Resource.UID
			namespace = event.Resource.Namespace
		}
	}
	require.NotZero(t, lastTimestamp)
	require.NotEmpty(t, podUID)
	require.NotEmpty(t, namespace)
	return lastTimestamp, podUID, namespace
}

func chainContainsKind(chain []analysisstore.ResourceWithDistance, kind string) bool {
	for _, entry := range chain {
		if entry.Resource.Kind == kind {
			return true
		}
	}
	return false
}

func firstUIDByKind(chain []analysisstore.ResourceWithDistance, kind string) string {
	for _, entry := range chain {
		if entry.Resource.Kind == kind {
			return entry.Resource.UID
		}
	}
	return ""
}

func namespaceGraphHasNodeKind(graph *analysisstore.NamespaceGraphData, kind string) bool {
	for _, node := range graph.Graph.Nodes {
		if node.Kind == kind {
			return true
		}
	}
	return false
}

func namespaceGraphHasEdge(graph *analysisstore.NamespaceGraphData, fromKind, relType, toKind string) bool {
	nodes := make(map[string]string, len(graph.Graph.Nodes))
	for _, node := range graph.Graph.Nodes {
		nodes[node.UID] = node.Kind
	}
	for _, edge := range graph.Graph.Edges {
		if nodes[edge.Source] == fromKind && edge.RelationshipType == relType && nodes[edge.Target] == toKind {
			return true
		}
	}
	return false
}

func hasRelatedKind(items []analysisstore.RelatedResourceData, kind, relType string) bool {
	for _, item := range items {
		if item.Resource.Kind == kind && item.RelationshipType == relType {
			return true
		}
	}
	return false
}

func containsChangeEvent(items []analysisstore.ChangeEventInfo, id string, configChanged bool) bool {
	for _, item := range items {
		if item.EventID == id && item.ConfigChanged == configChanged {
			return true
		}
	}
	return false
}

func newDeploymentEvent(t *testing.T, id string, timestamp int64, image string, unavailableReplicas int) models.Event {
	t.Helper()

	object := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "default",
			"uid":       "deploy-uid",
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app",
							"image": image,
						},
					},
				},
			},
		},
		"status": map[string]any{
			"unavailableReplicas": unavailableReplicas,
		},
	}

	data, err := json.Marshal(object)
	require.NoError(t, err)

	eventType := models.EventTypeUpdate
	if id == "evt-create" {
		eventType = models.EventTypeCreate
	}

	return models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "demo",
			UID:       "deploy-uid",
		},
		Data: data,
	}
}

func loadAuditLog(filePath string) ([]models.Event, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	var events []models.Event
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if err := event.Validate(); err != nil {
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
