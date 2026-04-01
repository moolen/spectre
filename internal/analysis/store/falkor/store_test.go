package falkor

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	itgraph "github.com/moolen/spectre/tests/integration/graph"
	"github.com/stretchr/testify/require"
)

func TestStore_GetOwnershipChainAndManagers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	harness, err := itgraph.NewTestHarness(t)
	require.NoError(t, err)

	fixturePath := fixturePath(t, "testrootcause-fluxhelmreleasevaluesfrom-.jsonl")
	require.NoError(t, harness.SeedEventsFromAuditLog(ctx, fixturePath))

	failureTimestamp, podUID, _ := extractFixtureContext(t, fixturePath)
	store := New(harness.GetClient())

	chain, err := store.GetOwnershipChain(ctx, podUID, failureTimestamp, 5)
	require.NoError(t, err)
	require.NotEmpty(t, chain)
	require.Equal(t, podUID, chain[0].Resource.UID)
	require.Equal(t, 0, chain[0].Distance)

	containsKind := func(kind string) bool {
		for _, entry := range chain {
			if entry.Resource.Kind == kind {
				return true
			}
		}
		return false
	}
	require.True(t, containsKind("ReplicaSet"))
	require.True(t, containsKind("Deployment"))

	deploymentUID := firstUIDByKind(chain, "Deployment")
	require.NotEmpty(t, deploymentUID)

	managers, err := store.GetManagers(ctx, []string{deploymentUID, "missing-uid"}, 0.5)
	require.NoError(t, err)

	managerData, ok := managers[deploymentUID]
	require.True(t, ok, "expected manager for deployment UID")
	require.Equal(t, "HelmRelease", managerData.Manager.Kind)
	_, missingFound := managers["missing-uid"]
	require.False(t, missingFound, "missing UID should be omitted from map")
}

func TestStore_GetRelatedResources_IncludesDeletedWithinWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	harness, err := itgraph.NewTestHarness(t)
	require.NoError(t, err)

	client := harness.GetClient()
	require.NoError(t, createRelatedResourcesDeletionScenario(ctx, client))

	store := New(client)
	failureTimestamp := int64(1_000_000_000_000)

	shortWindow := analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         int64(10 * time.Second),
	}
	shortRelated, err := store.GetRelatedResources(ctx, []string{"pod-test"}, shortWindow)
	require.NoError(t, err)
	require.True(t, hasRelatedUID(shortRelated["pod-test"], "cm-deleted-recent", "REFERENCES_SPEC"))
	require.False(t, hasRelatedUID(shortRelated["pod-test"], "cm-deleted-old", "REFERENCES_SPEC"))

	longWindow := analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         int64(5 * time.Minute),
	}
	longRelated, err := store.GetRelatedResources(ctx, []string{"pod-test"}, longWindow)
	require.NoError(t, err)
	require.True(t, hasRelatedUID(longRelated["pod-test"], "cm-deleted-recent", "REFERENCES_SPEC"))
	require.True(t, hasRelatedUID(longRelated["pod-test"], "cm-deleted-old", "REFERENCES_SPEC"))
}

func TestStore_GetNamespaceGraph_Pagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	harness, err := itgraph.NewTestHarness(t)
	require.NoError(t, err)

	store := New(harness.GetClient())
	require.NoError(t, createNamespacePaginationScenario(ctx, harness.GetClient()))

	firstPage, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   "team-a",
		TimestampNs: 2000,
		LookbackNs:  int64(10 * time.Minute),
		MaxDepth:    1,
		Limit:       2,
	})
	require.NoError(t, err)
	require.NotNil(t, firstPage)
	require.Len(t, firstPage.Graph.Nodes, 2)
	require.True(t, firstPage.Metadata.HasMore)
	require.NotEmpty(t, firstPage.Metadata.NextCursor)

	secondPage, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   "team-a",
		TimestampNs: 2000,
		LookbackNs:  int64(10 * time.Minute),
		MaxDepth:    1,
		Limit:       2,
		Cursor:      firstPage.Metadata.NextCursor,
	})
	require.NoError(t, err)
	require.NotNil(t, secondPage)
	require.NotEmpty(t, secondPage.Graph.Nodes)

	firstUIDs := make(map[string]struct{}, len(firstPage.Graph.Nodes))
	for _, node := range firstPage.Graph.Nodes {
		firstUIDs[node.UID] = struct{}{}
	}
	for _, node := range secondPage.Graph.Nodes {
		_, seen := firstUIDs[node.UID]
		require.False(t, seen, "expected pagination cursor to avoid duplicates across pages")
	}
}

func fixturePath(t *testing.T, baseName string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "tests", "integration", "fixtures", baseName)
}

func extractFixtureContext(t *testing.T, jsonlPath string) (int64, string, string) {
	t.Helper()

	events, err := itgraph.LoadAuditLog(jsonlPath)
	require.NoError(t, err)
	require.NotEmpty(t, events)

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

func firstUIDByKind(chain []analysisstore.ResourceWithDistance, kind string) string {
	for _, entry := range chain {
		if entry.Resource.Kind == kind {
			return entry.Resource.UID
		}
	}
	return ""
}

func hasRelatedUID(items []analysisstore.RelatedResourceData, uid, relType string) bool {
	for _, item := range items {
		if item.Resource.UID == uid && item.RelationshipType == relType {
			return true
		}
	}
	return false
}

func createRelatedResourcesDeletionScenario(ctx context.Context, client graph.Client) error {
	queries := []graph.GraphQuery{
		{
			Query: `
				CREATE (pod:ResourceIdentity {
					uid: 'pod-test',
					kind: 'Pod',
					apiGroup: '',
					version: 'v1',
					namespace: 'default',
					name: 'pod-test',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
			`,
		},
		{
			Query: `
				CREATE (cmRecent:ResourceIdentity {
					uid: 'cm-deleted-recent',
					kind: 'ConfigMap',
					apiGroup: '',
					version: 'v1',
					namespace: 'default',
					name: 'cm-deleted-recent',
					firstSeen: 1,
					lastSeen: 1,
					deleted: true,
					deletedAt: 999995000000
				})
			`,
		},
		{
			Query: `
				CREATE (cmOld:ResourceIdentity {
					uid: 'cm-deleted-old',
					kind: 'ConfigMap',
					apiGroup: '',
					version: 'v1',
					namespace: 'default',
					name: 'cm-deleted-old',
					firstSeen: 1,
					lastSeen: 1,
					deleted: true,
					deletedAt: 900000000000
				})
			`,
		},
		{
			Query: `
				MATCH (pod:ResourceIdentity {uid: 'pod-test'})
				MATCH (cmRecent:ResourceIdentity {uid: 'cm-deleted-recent'})
				MERGE (pod)-[:REFERENCES_SPEC]->(cmRecent)
			`,
		},
		{
			Query: `
				MATCH (pod:ResourceIdentity {uid: 'pod-test'})
				MATCH (cmOld:ResourceIdentity {uid: 'cm-deleted-old'})
				MERGE (pod)-[:REFERENCES_SPEC]->(cmOld)
			`,
		},
	}

	for _, query := range queries {
		if _, err := client.ExecuteQuery(ctx, query); err != nil {
			return fmt.Errorf("seed query failed: %w", err)
		}
	}
	return nil
}

func createNamespacePaginationScenario(ctx context.Context, client graph.Client) error {
	query := graph.GraphQuery{
		Query: `
			CREATE (r1:ResourceIdentity {
				uid: 'uid-configmap',
				kind: 'ConfigMap',
				apiGroup: '',
				version: 'v1',
				namespace: 'team-a',
				name: 'a-config',
				firstSeen: 1,
				lastSeen: 1,
				deleted: false
			})
			CREATE (r2:ResourceIdentity {
				uid: 'uid-deployment',
				kind: 'Deployment',
				apiGroup: 'apps',
				version: 'v1',
				namespace: 'team-a',
				name: 'b-deploy',
				firstSeen: 1,
				lastSeen: 1,
				deleted: false
			})
			CREATE (r3:ResourceIdentity {
				uid: 'uid-pod',
				kind: 'Pod',
				apiGroup: '',
				version: 'v1',
				namespace: 'team-a',
				name: 'c-pod',
				firstSeen: 1,
				lastSeen: 1,
				deleted: false
			})
			CREATE (eventKind:ResourceIdentity {
				uid: 'uid-event-kind',
				kind: 'Event',
				apiGroup: '',
				version: 'v1',
				namespace: 'team-a',
				name: 'ignored-event-kind',
				firstSeen: 1,
				lastSeen: 1,
				deleted: false
			})
			CREATE (otherNs:ResourceIdentity {
				uid: 'uid-other-ns',
				kind: 'Service',
				apiGroup: '',
				version: 'v1',
				namespace: 'team-b',
				name: 'z-other',
				firstSeen: 1,
				lastSeen: 1,
				deleted: false
			})

			CREATE (e1:ChangeEvent {id: 'e1', timestamp: 1000, eventType: 'CREATE', status: 'Ready', data: '{}', configChanged: false, statusChanged: false})
			CREATE (e2:ChangeEvent {id: 'e2', timestamp: 1000, eventType: 'CREATE', status: 'Ready', data: '{}', configChanged: false, statusChanged: false})
			CREATE (e3:ChangeEvent {id: 'e3', timestamp: 1000, eventType: 'CREATE', status: 'Ready', data: '{}', configChanged: false, statusChanged: false})
			CREATE (e4:ChangeEvent {id: 'e4', timestamp: 1000, eventType: 'CREATE', status: 'Ready', data: '{}', configChanged: false, statusChanged: false})
			CREATE (e5:ChangeEvent {id: 'e5', timestamp: 1000, eventType: 'CREATE', status: 'Ready', data: '{}', configChanged: false, statusChanged: false})

			CREATE (r1)-[:CHANGED]->(e1)
			CREATE (r2)-[:CHANGED]->(e2)
			CREATE (r3)-[:CHANGED]->(e3)
			CREATE (eventKind)-[:CHANGED]->(e4)
			CREATE (otherNs)-[:CHANGED]->(e5)
		`,
	}
	if _, err := client.ExecuteQuery(ctx, query); err != nil {
		return fmt.Errorf("seed namespace pagination scenario failed: %w", err)
	}
	return nil
}
