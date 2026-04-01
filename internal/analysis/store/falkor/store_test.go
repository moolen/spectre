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
	"github.com/stretchr/testify/require"
)

func TestStore_GetOwnershipChainAndManagers(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
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
	ctx := context.Background()
	harness, err := newTestHarness(t)
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

func TestStore_GetRelatedResources_UsesRawStartWithoutClamp(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
	require.NoError(t, err)

	client := harness.GetClient()
	require.NoError(t, createRelatedResourcesNegativeDeletedAtScenario(ctx, client))

	store := New(client)
	window := analysisstore.ResourceWindow{
		FailureTimestampNs: 50,
		LookbackNs:         100, // raw start = -50; clamped start would be 0
	}

	related, err := store.GetRelatedResources(ctx, []string{"pod-negative"}, window)
	require.NoError(t, err)
	require.True(
		t,
		hasRelatedUID(related["pod-negative"], "cm-deleted-negative", "REFERENCES_SPEC"),
		"deleted resource within raw negative lookback should be included",
	)
}

func TestStore_GetNamespaceGraph_Pagination(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
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

func TestStore_GetNamespaceGraph_CapsLookbackAt24Hours(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
	require.NoError(t, err)

	require.NoError(t, createNamespaceLookbackCapScenario(ctx, harness.GetClient()))
	store := New(harness.GetClient())

	// far larger than 24h; adapter should cap to 24h and exclude the older event from diff window
	result, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
		Namespace:   "team-cap",
		TimestampNs: int64(30 * time.Hour),
		LookbackNs:  int64(7 * 24 * time.Hour),
		MaxDepth:    1,
		Limit:       10,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Graph.Nodes)

	var workloadNode *analysisstore.NamespaceGraphNode
	for i := range result.Graph.Nodes {
		if result.Graph.Nodes[i].UID == "uid-cap-deploy" {
			workloadNode = &result.Graph.Nodes[i]
			break
		}
	}
	require.NotNil(t, workloadNode)
	require.NotNil(t, workloadNode.LatestEvent)
	require.Empty(
		t,
		workloadNode.LatestEvent.SpecChanges,
		"spec diff should be empty when lookback is capped to 24h and older event is out of window",
	)
}

func TestStore_GetResource(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
	require.NoError(t, err)

	require.NoError(t, createGetResourceScenario(ctx, harness.GetClient()))
	s := New(harness.GetClient())

	resource, err := s.GetResource(ctx, "uid-resource-test")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "uid-resource-test", resource.UID)
	require.Equal(t, "Pod", resource.Kind)

	missing, err := s.GetResource(ctx, "uid-does-not-exist")
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestStore_GetChangeEvents(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
	require.NoError(t, err)

	require.NoError(t, createGetEventsScenario(ctx, harness.GetClient()))
	s := New(harness.GetClient())

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: 1000,
		LookbackNs:         200,
	}
	events, err := s.GetChangeEvents(ctx, []string{"uid-events-resource"}, window)
	require.NoError(t, err)

	resourceEvents, ok := events["uid-events-resource"]
	require.True(t, ok)
	require.Len(t, resourceEvents, 2)
	ids := map[string]bool{}
	var configEvent *analysisstore.ChangeEventInfo
	for i := range resourceEvents {
		ids[resourceEvents[i].EventID] = true
		if resourceEvents[i].ConfigChanged {
			configEvent = &resourceEvents[i]
		}
	}
	require.True(t, ids["evt-inside-config"])
	require.True(t, ids["evt-inside-status"])
	require.False(t, ids["evt-outside"])
	require.NotNil(t, configEvent)
	require.Equal(t, time.Unix(0, 900), configEvent.Timestamp)
}

func TestStore_GetK8sEvents(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
	require.NoError(t, err)

	require.NoError(t, createGetEventsScenario(ctx, harness.GetClient()))
	s := New(harness.GetClient())

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: 1000,
		LookbackNs:         200,
	}
	events, err := s.GetK8sEvents(ctx, []string{"uid-events-resource"}, window)
	require.NoError(t, err)

	resourceEvents, ok := events["uid-events-resource"]
	require.True(t, ok)
	require.Len(t, resourceEvents, 1)
	require.Equal(t, "kevt-inside", resourceEvents[0].EventID)
	require.Equal(t, "BackOff", resourceEvents[0].Reason)
	require.Equal(t, time.Unix(0, 920), resourceEvents[0].Timestamp)
}

func TestStore_GetManagers_DeterministicHighestConfidenceThenUID(t *testing.T) {
	ctx := context.Background()
	harness, err := newTestHarness(t)
	require.NoError(t, err)

	require.NoError(t, createManagersDeterministicScenario(ctx, harness.GetClient()))
	s := New(harness.GetClient())

	managers, err := s.GetManagers(ctx, []string{"uid-managed"}, 0.5)
	require.NoError(t, err)
	require.Contains(t, managers, "uid-managed")
	require.Equal(t, "uid-manager-a", managers["uid-managed"].Manager.UID)
	require.Equal(t, 0.9, managers["uid-managed"].ManagesEdge.Confidence)
}

func fixturePath(t *testing.T, baseName string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "tests", "integration", "fixtures", baseName)
}

func extractFixtureContext(t *testing.T, jsonlPath string) (int64, string, string) {
	t.Helper()

	events, err := loadAuditLog(jsonlPath)
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

func createRelatedResourcesNegativeDeletedAtScenario(ctx context.Context, client graph.Client) error {
	queries := []graph.GraphQuery{
		{
			Query: `
				CREATE (pod:ResourceIdentity {
					uid: 'pod-negative',
					kind: 'Pod',
					apiGroup: '',
					version: 'v1',
					namespace: 'default',
					name: 'pod-negative',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
			`,
		},
		{
			Query: `
				CREATE (cm:ResourceIdentity {
					uid: 'cm-deleted-negative',
					kind: 'ConfigMap',
					apiGroup: '',
					version: 'v1',
					namespace: 'default',
					name: 'cm-deleted-negative',
					firstSeen: 1,
					lastSeen: 1,
					deleted: true,
					deletedAt: -10
				})
			`,
		},
		{
			Query: `
				MATCH (pod:ResourceIdentity {uid: 'pod-negative'})
				MATCH (cm:ResourceIdentity {uid: 'cm-deleted-negative'})
				MERGE (pod)-[:REFERENCES_SPEC]->(cm)
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

func createNamespaceLookbackCapScenario(ctx context.Context, client graph.Client) error {
	query := graph.GraphQuery{
		Query: `
			CREATE (r:ResourceIdentity {
				uid: 'uid-cap-deploy',
				kind: 'Deployment',
				apiGroup: 'apps',
				version: 'v1',
				namespace: 'team-cap',
				name: 'cap-deploy',
				firstSeen: 0,
				lastSeen: 108000000000000,
				deleted: false
			})
			CREATE (old:ChangeEvent {
				id: 'cap-old',
				timestamp: 0,
				eventType: 'UPDATE',
				status: 'Ready',
				data: '{"apiVersion":"apps/v1","kind":"Deployment","spec":{"replicas":1}}',
				configChanged: true,
				statusChanged: false
			})
			CREATE (latest:ChangeEvent {
				id: 'cap-latest',
				timestamp: 108000000000000,
				eventType: 'UPDATE',
				status: 'Ready',
				data: '{"apiVersion":"apps/v1","kind":"Deployment","spec":{"replicas":3}}',
				configChanged: true,
				statusChanged: false
			})
			CREATE (r)-[:CHANGED]->(old)
			CREATE (r)-[:CHANGED]->(latest)
		`,
	}
	if _, err := client.ExecuteQuery(ctx, query); err != nil {
		return fmt.Errorf("seed lookback-cap scenario failed: %w", err)
	}
	return nil
}

func createGetResourceScenario(ctx context.Context, client graph.Client) error {
	return runQueries(ctx, client, []graph.GraphQuery{
		{
			Query: `
				CREATE (:ResourceIdentity {
					uid: 'uid-resource-test',
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
	})
}

func createGetEventsScenario(ctx context.Context, client graph.Client) error {
	return runQueries(ctx, client, []graph.GraphQuery{
		{
			Query: `
				CREATE (r:ResourceIdentity {
					uid: 'uid-events-resource',
					kind: 'Pod',
					apiGroup: '',
					version: 'v1',
					namespace: 'default',
					name: 'events-pod',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
				CREATE (evtOld:ChangeEvent {
					id: 'evt-outside',
					timestamp: 600,
					eventType: 'UPDATE',
					status: 'Ready',
					data: '{"spec":{"replicas":1}}',
					configChanged: true,
					statusChanged: false
				})
				CREATE (evtConfig:ChangeEvent {
					id: 'evt-inside-config',
					timestamp: 900,
					eventType: 'UPDATE',
					status: 'Ready',
					data: '{"spec":{"replicas":2}}',
					configChanged: true,
					statusChanged: false
				})
				CREATE (evtStatus:ChangeEvent {
					id: 'evt-inside-status',
					timestamp: 950,
					eventType: 'UPDATE',
					status: 'Warning',
					data: '{"status":{"phase":"Pending"}}',
					configChanged: false,
					statusChanged: true
				})
				CREATE (kOld:K8sEvent {
					id: 'kevt-outside',
					timestamp: 500,
					reason: 'FailedScheduling',
					message: 'old',
					type: 'Warning',
					count: 1,
					source: 'scheduler'
				})
				CREATE (kIn:K8sEvent {
					id: 'kevt-inside',
					timestamp: 920,
					reason: 'BackOff',
					message: 'inside',
					type: 'Warning',
					count: 2,
					source: 'kubelet'
				})
				CREATE (r)-[:CHANGED]->(evtOld)
				CREATE (r)-[:CHANGED]->(evtConfig)
				CREATE (r)-[:CHANGED]->(evtStatus)
				CREATE (r)-[:EMITTED_EVENT]->(kOld)
				CREATE (r)-[:EMITTED_EVENT]->(kIn)
			`,
		},
	})
}

func createManagersDeterministicScenario(ctx context.Context, client graph.Client) error {
	return runQueries(ctx, client, []graph.GraphQuery{
		{
			Query: `
				CREATE (res:ResourceIdentity {
					uid: 'uid-managed',
					kind: 'Deployment',
					apiGroup: 'apps',
					version: 'v1',
					namespace: 'default',
					name: 'managed',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
				CREATE (mgrA:ResourceIdentity {
					uid: 'uid-manager-a',
					kind: 'HelmRelease',
					apiGroup: 'helm.toolkit.fluxcd.io',
					version: 'v2beta1',
					namespace: 'default',
					name: 'a',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
				CREATE (mgrZ:ResourceIdentity {
					uid: 'uid-manager-z',
					kind: 'HelmRelease',
					apiGroup: 'helm.toolkit.fluxcd.io',
					version: 'v2beta1',
					namespace: 'default',
					name: 'z',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
				CREATE (mgrLow:ResourceIdentity {
					uid: 'uid-manager-low',
					kind: 'HelmRelease',
					apiGroup: 'helm.toolkit.fluxcd.io',
					version: 'v2beta1',
					namespace: 'default',
					name: 'low',
					firstSeen: 1,
					lastSeen: 1,
					deleted: false
				})
				CREATE (mgrA)-[:MANAGES {confidence: 0.9, firstObserved: 1, lastValidated: 1, validationState: 'confirmed'}]->(res)
				CREATE (mgrZ)-[:MANAGES {confidence: 0.9, firstObserved: 1, lastValidated: 1, validationState: 'confirmed'}]->(res)
				CREATE (mgrLow)-[:MANAGES {confidence: 0.8, firstObserved: 1, lastValidated: 1, validationState: 'confirmed'}]->(res)
			`,
		},
	})
}

func runQueries(ctx context.Context, client graph.Client, queries []graph.GraphQuery) error {
	for _, query := range queries {
		if _, err := client.ExecuteQuery(ctx, query); err != nil {
			return fmt.Errorf("seed query failed: %w", err)
		}
	}
	return nil
}
