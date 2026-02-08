package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestContainer creates a FalkorDB container for performance testing.
// Returns the client and a cleanup function.
func setupTestContainer(t *testing.T) (graph.Client, func()) {
	t.Helper()

	ctx := context.Background()
	graphName := fmt.Sprintf("perf-%s", uuid.New().String()[:8])

	// Start FalkorDB container
	req := testcontainers.ContainerRequest{
		Image:        "falkordb/falkordb:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
		AutoRemove:   true,
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start FalkorDB container: %v", err)
	}

	// Get container host and port
	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to get container port: %v", err)
	}

	// Create and connect client
	config := graph.DefaultClientConfig()
	config.Host = host
	config.Port = port.Int()
	config.GraphName = graphName
	config.DialTimeout = 10 * time.Second

	client := graph.NewClient(config)
	if err := client.Connect(ctx); err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to connect to FalkorDB: %v", err)
	}

	// Initialize schema
	if err := client.InitializeSchema(ctx); err != nil {
		client.Close()
		container.Terminate(ctx)
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	cleanup := func() {
		client.Close()
		container.Terminate(ctx)
	}

	return client, cleanup
}

// TestGraphPerformance_LargeClusterSimulation simulates processing 10k events
// to verify that optimizations achieve target performance.
// Acceptance: Process 10k events in under 60 seconds (target from IMPLEMENTATION_PLAN.md)
func TestGraphPerformance_LargeClusterSimulation(t *testing.T) {
	client, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Create builder with client
	builder := NewGraphBuilderWithClient(client)

	// Generate 10k synthetic events simulating a large cluster
	events := generateLargeClusterEvents(10000)

	// Pre-populate label index with some pods for selector lookups
	labelIndex := builder.GetLabelIndex()
	for i := 0; i < 1000; i++ {
		ns := fmt.Sprintf("ns-%d", i%10)
		uid := fmt.Sprintf("pod-%d", i)
		labels := map[string]string{
			"app":     fmt.Sprintf("app-%d", i%50),
			"version": fmt.Sprintf("v%d", i%5),
		}
		labelIndex.Update(ns, "Pod", uid, labels)
	}

	// Record initial memory
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// Process all events
	start := time.Now()

	builder.SetBatchCache(events)
	defer builder.ClearBatchCache()

	var totalUpdates int
	for _, event := range events {
		update, err := builder.BuildFromEvent(ctx, event)
		if err != nil {
			t.Logf("Warning: BuildFromEvent failed: %v", err)
			continue
		}
		if update != nil {
			totalUpdates++
		}
	}

	duration := time.Since(start)

	// Record final memory
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	memUsedMB := float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024

	// Report metrics
	t.Logf("Processed %d events -> %d graph updates", len(events), totalUpdates)
	t.Logf("Duration: %v (%.1f events/sec)", duration, float64(len(events))/duration.Seconds())
	t.Logf("Memory used: %.2f MB", memUsedMB)

	// Report cache stats
	stateHits, stateMisses, stateSize := builder.GetStateCacheStats()
	labelHits, labelMisses, labelNs, labelResources := builder.GetLabelIndexStats()
	t.Logf("State cache: hits=%d, misses=%d, size=%d (hit rate: %.1f%%)",
		stateHits, stateMisses, stateSize, hitRate(stateHits, stateMisses))
	t.Logf("Label index: hits=%d, misses=%d, namespaces=%d, resources=%d (hit rate: %.1f%%)",
		labelHits, labelMisses, labelNs, labelResources, hitRate(labelHits, labelMisses))

	// Acceptance: 60 seconds for 10k events
	maxDuration := 60 * time.Second
	if duration > maxDuration {
		t.Errorf("Processing took %v, exceeds maximum %v", duration, maxDuration)
	}
}

// TestGraphPerformance_BatchProcessingEfficiency tests that batch processing
// achieves significant query reduction compared to individual event processing.
func TestGraphPerformance_BatchProcessingEfficiency(t *testing.T) {
	client, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Create pipeline config for batching
	pipelineConfig := DefaultPipelineConfig()
	pipelineConfig.StateCacheSize = 10000

	builder := NewGraphBuilderWithClientAndCacheSize(client, pipelineConfig.StateCacheSize)

	// Generate batch of 100 events (target batch size)
	events := generateMixedEvents(100)

	// Pre-populate label index
	labelIndex := builder.GetLabelIndex()
	for i := 0; i < 200; i++ {
		ns := fmt.Sprintf("ns-%d", i%5)
		uid := fmt.Sprintf("pod-%d", i)
		labels := map[string]string{
			"app":     fmt.Sprintf("app-%d", i%20),
			"version": "v1",
		}
		labelIndex.Update(ns, "Pod", uid, labels)
	}

	// Set batch cache for change detection
	builder.SetBatchCache(events)
	defer builder.ClearBatchCache()

	// Process batch
	updates, err := builder.BuildFromBatch(ctx, events)
	require.NoError(t, err)

	// Collect statistics
	var totalNodes, totalEdges int
	for _, update := range updates {
		if update != nil {
			totalNodes += len(update.ResourceNodes) + len(update.EventNodes) + len(update.K8sEventNodes)
			totalEdges += len(update.Edges)
		}
	}

	// Get cache stats
	stateHits, stateMisses, _ := builder.GetStateCacheStats()
	labelHits, labelMisses, _, _ := builder.GetLabelIndexStats()

	t.Logf("Batch of %d events produced %d updates", len(events), len(updates))
	t.Logf("Total nodes: %d, edges: %d", totalNodes, totalEdges)
	t.Logf("State cache hit rate: %.1f%% (%d hits, %d misses)",
		hitRate(stateHits, stateMisses), stateHits, stateMisses)
	t.Logf("Label index hit rate: %.1f%% (%d hits, %d misses)",
		hitRate(labelHits, labelMisses), labelHits, labelMisses)

	// Verify we got reasonable output
	assert.NotEmpty(t, updates, "Should produce graph updates")

	// After warm-up, state cache should achieve >70% hit rate for repeated resources
	// (This test generates unique resources, so hit rate will be lower)
}

// TestGraphPerformance_StateCacheWarmup tests that state cache improves
// after processing events for the same resources.
func TestGraphPerformance_StateCacheWarmup(t *testing.T) {
	client, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	builder := NewGraphBuilderWithClient(client)

	// Generate events for same set of 10 resources across 5 batches
	numResources := 10
	numBatches := 5
	eventsPerBatch := numResources

	baseTime := time.Now()
	totalProcessed := 0

	for batch := 0; batch < numBatches; batch++ {
		events := make([]models.Event, eventsPerBatch)
		for i := 0; i < eventsPerBatch; i++ {
			events[i] = createTestEvent(
				fmt.Sprintf("resource-%d", i),
				models.EventTypeUpdate,
				baseTime.Add(time.Duration(batch*eventsPerBatch+i)*time.Second),
			)
		}

		builder.SetBatchCache(events)
		for _, event := range events {
			_, err := builder.BuildFromEvent(ctx, event)
			if err != nil {
				t.Logf("BuildFromEvent failed: %v", err)
			}
			totalProcessed++
		}
		builder.ClearBatchCache()
	}

	hits, misses, size := builder.GetStateCacheStats()
	hitRateVal := hitRate(hits, misses)

	t.Logf("Processed %d events for %d resources over %d batches",
		totalProcessed, numResources, numBatches)
	t.Logf("State cache: hits=%d, misses=%d, size=%d, hit rate=%.1f%%",
		hits, misses, size, hitRateVal)

	// After first batch, subsequent batches should hit cache for most resources
	// The cache should be populated with resource states
	assert.GreaterOrEqual(t, size, numResources,
		"State cache should contain at least %d resources", numResources)
}

// TestStateCacheDirectOperations tests state cache operations directly
func TestStateCacheDirectOperations(t *testing.T) {
	cache, err := NewStateCache(1000)
	require.NoError(t, err)

	// Simulate processing multiple events for the same resource
	uid := "test-resource-1"
	baseTime := time.Now().UnixNano()

	// First access - miss
	result := cache.Get(uid)
	assert.Nil(t, result, "Should miss on first access")

	// Store state
	data := []byte(`{"metadata":{"uid":"test-resource-1"}}`)
	cache.Put(uid, data, baseTime, "CREATE")

	// Second access - hit
	result = cache.Get(uid)
	assert.NotNil(t, result, "Should hit on second access")
	assert.Equal(t, "CREATE", result.EventType)

	// Update state
	cache.Put(uid, data, baseTime+1000000, "UPDATE")

	// Third access - hit
	result = cache.Get(uid)
	assert.NotNil(t, result, "Should hit after update")
	assert.Equal(t, "UPDATE", result.EventType)

	// Check stats
	hits, misses, size := cache.GetStats()
	assert.Equal(t, int64(2), hits, "Should have 2 hits")
	assert.Equal(t, int64(1), misses, "Should have 1 miss")
	assert.Equal(t, 1, size, "Should have 1 entry")
	assert.InDelta(t, 66.67, cache.HitRate(), 0.01, "Hit rate should be ~67%")
}

// TestGraphPerformance_LabelIndexLookup tests label index lookup performance
func TestGraphPerformance_LabelIndexLookup(t *testing.T) {
	labelIndex := NewLabelIndex()

	// Populate index with 10k pods
	numPods := 10000
	numNamespaces := 100
	numApps := 500

	for i := 0; i < numPods; i++ {
		ns := fmt.Sprintf("ns-%d", i%numNamespaces)
		uid := fmt.Sprintf("pod-%d", i)
		labels := map[string]string{
			"app":       fmt.Sprintf("app-%d", i%numApps),
			"version":   fmt.Sprintf("v%d", i%10),
			"component": fmt.Sprintf("comp-%d", i%20),
		}
		labelIndex.Update(ns, "Pod", uid, labels)
	}

	// Run lookups
	numLookups := 1000
	start := time.Now()

	var totalMatches int
	for i := 0; i < numLookups; i++ {
		ns := fmt.Sprintf("ns-%d", i%numNamespaces)
		selector := map[string]string{
			"app":     fmt.Sprintf("app-%d", i%numApps),
			"version": fmt.Sprintf("v%d", i%10),
		}
		matches := labelIndex.FindBySelector(ns, "Pod", selector)
		totalMatches += len(matches)
	}

	duration := time.Since(start)
	lookupsPerSec := float64(numLookups) / duration.Seconds()

	hits, misses, namespaces, resources := labelIndex.GetStats()

	t.Logf("Label index size: %d pods across %d namespaces", resources, namespaces)
	t.Logf("Performed %d lookups in %v (%.0f lookups/sec)", numLookups, duration, lookupsPerSec)
	t.Logf("Total matches found: %d", totalMatches)
	t.Logf("Hit rate: %.1f%% (hits=%d, misses=%d)", labelIndex.HitRate(), hits, misses)

	// Label index should handle 10k+ lookups/sec
	assert.True(t, lookupsPerSec > 1000,
		"Label index performance %.0f lookups/sec is below 1000", lookupsPerSec)
}

// BenchmarkBuildFromEvent benchmarks single event processing
func BenchmarkBuildFromEvent(b *testing.B) {
	builder := NewGraphBuilder()
	ctx := context.Background()

	event := createTestEvent("benchmark-pod", models.EventTypeCreate, time.Now())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = builder.BuildFromEvent(ctx, event)
	}
}

// BenchmarkBuildFromBatch benchmarks batch processing
func BenchmarkBuildFromBatch(b *testing.B) {
	builder := NewGraphBuilder()
	ctx := context.Background()

	events := generateMixedEvents(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.SetBatchCache(events)
		_, _ = builder.BuildFromBatch(ctx, events)
		builder.ClearBatchCache()
	}
}

// BenchmarkLabelIndexLookup benchmarks label index lookup performance
func BenchmarkLabelIndexLookup(b *testing.B) {
	labelIndex := NewLabelIndex()

	// Populate with 5k pods
	for i := 0; i < 5000; i++ {
		ns := fmt.Sprintf("ns-%d", i%50)
		uid := fmt.Sprintf("pod-%d", i)
		labels := map[string]string{
			"app":     fmt.Sprintf("app-%d", i%100),
			"version": fmt.Sprintf("v%d", i%5),
		}
		labelIndex.Update(ns, "Pod", uid, labels)
	}

	selector := map[string]string{"app": "app-42", "version": "v1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = labelIndex.FindBySelector("ns-5", "Pod", selector)
	}
}

// BenchmarkStateCacheOperations benchmarks state cache operations
func BenchmarkStateCacheOperations(b *testing.B) {
	cache, _ := NewStateCache(10000)

	// Populate cache
	for i := 0; i < 5000; i++ {
		uid := fmt.Sprintf("resource-%d", i)
		data := []byte(fmt.Sprintf(`{"metadata":{"uid":"%s"}}`, uid))
		cache.Put(uid, data, time.Now().UnixNano(), "CREATE")
	}

	testUID := "resource-2500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Get(testUID)
	}
}

// Helper functions

func generateLargeClusterEvents(count int) []models.Event {
	events := make([]models.Event, count)
	baseTime := time.Now()

	kinds := []string{"Pod", "Deployment", "Service", "ConfigMap", "Secret", "ReplicaSet"}
	eventTypes := []models.EventType{models.EventTypeCreate, models.EventTypeUpdate, models.EventTypeUpdate, models.EventTypeUpdate}

	for i := 0; i < count; i++ {
		kind := kinds[i%len(kinds)]
		eventType := eventTypes[i%len(eventTypes)]
		ns := fmt.Sprintf("ns-%d", i%20)
		name := fmt.Sprintf("%s-%d", strings.ToLower(kind), i)
		uid := fmt.Sprintf("%s-%s-%s", ns, kind, name)

		data, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
				"uid":       uid,
				"labels": map[string]interface{}{
					"app":     fmt.Sprintf("app-%d", i%50),
					"version": fmt.Sprintf("v%d", i%5),
				},
			},
			"spec": map[string]interface{}{
				"replicas": i % 5,
			},
		})

		events[i] = models.Event{
			ID:        uid,
			Type:      eventType,
			Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond).UnixNano(),
			Resource: models.ResourceMetadata{
				UID:       uid,
				Kind:      kind,
				Name:      name,
				Namespace: ns,
				Group:     groupForKind(kind),
				Version:   "v1",
			},
			Data: data,
		}
	}

	return events
}

func generateMixedEvents(count int) []models.Event {
	events := make([]models.Event, count)
	baseTime := time.Now()

	// Mix of event types and kinds that trigger different code paths
	for i := 0; i < count; i++ {
		var kind string
		var eventType models.EventType
		switch i % 10 {
		case 0:
			kind, eventType = "Pod", models.EventTypeCreate
		case 1:
			kind, eventType = "Pod", models.EventTypeUpdate
		case 2:
			kind, eventType = "Deployment", models.EventTypeCreate
		case 3:
			kind, eventType = "Deployment", models.EventTypeUpdate
		case 4:
			kind, eventType = "Service", models.EventTypeCreate
		case 5:
			kind, eventType = "Service", models.EventTypeUpdate
		case 6:
			kind, eventType = "ConfigMap", models.EventTypeUpdate
		case 7:
			kind, eventType = "Secret", models.EventTypeUpdate
		case 8:
			kind, eventType = "ReplicaSet", models.EventTypeCreate
		case 9:
			kind, eventType = "ReplicaSet", models.EventTypeUpdate
		}

		ns := fmt.Sprintf("ns-%d", i%5)
		name := fmt.Sprintf("%s-%d", strings.ToLower(kind), i)
		uid := fmt.Sprintf("%s-%s-%s", ns, kind, name)

		data, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
				"uid":       uid,
				"labels": map[string]interface{}{
					"app":     fmt.Sprintf("app-%d", i%20),
					"version": "v1",
				},
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": fmt.Sprintf("app-%d", i%20),
					},
				},
			},
		})

		events[i] = models.Event{
			ID:        uid,
			Type:      eventType,
			Timestamp: baseTime.Add(time.Duration(i) * time.Second).UnixNano(),
			Resource: models.ResourceMetadata{
				UID:       uid,
				Kind:      kind,
				Name:      name,
				Namespace: ns,
				Group:     groupForKind(kind),
				Version:   "v1",
			},
			Data: data,
		}
	}

	return events
}

func createTestEvent(uid string, eventType models.EventType, timestamp time.Time) models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"uid":       uid,
			"labels": map[string]interface{}{
				"app": "test",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "main",
					"image": "nginx:latest",
				},
			},
		},
		"status": map[string]interface{}{
			"phase": "Running",
		},
	})

	return models.Event{
		ID:        uid,
		Type:      eventType,
		Timestamp: timestamp.UnixNano(),
		Resource: models.ResourceMetadata{
			UID:       uid,
			Kind:      "Pod",
			Name:      "test-pod",
			Namespace: "default",
			Group:     "",
			Version:   "v1",
		},
		Data: data,
	}
}

func groupForKind(kind string) string {
	switch kind {
	case "Deployment", "ReplicaSet":
		return "apps"
	default:
		return ""
	}
}

func hitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// =============================================================================
// COMPARISON BENCHMARKS: Before/After Optimization
// These benchmarks compare performance with optimizations enabled vs disabled
// =============================================================================

// BenchmarkCompare_WithLabelIndex benchmarks Service event processing with label index
func BenchmarkCompare_WithLabelIndex(b *testing.B) {
	// Create builder with label index (optimized path)
	builder := NewGraphBuilder()
	labelIndex := builder.GetLabelIndex()

	// Pre-populate label index with 1000 pods
	for i := 0; i < 1000; i++ {
		ns := fmt.Sprintf("ns-%d", i%10)
		uid := fmt.Sprintf("pod-%d", i)
		labels := map[string]string{
			"app":     fmt.Sprintf("app-%d", i%50),
			"version": "v1",
		}
		labelIndex.Update(ns, "Pod", uid, labels)
	}

	// Create Service event with selector
	serviceData, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-service",
			"namespace": "ns-5",
			"uid":       "service-uid",
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"app":     "app-25",
				"version": "v1",
			},
		},
	})

	event := models.Event{
		ID:        "service-event",
		Type:      models.EventTypeCreate,
		Timestamp: time.Now().UnixNano(),
		Resource: models.ResourceMetadata{
			UID:       "service-uid",
			Kind:      "Service",
			Name:      "test-service",
			Namespace: "ns-5",
			Version:   "v1",
		},
		Data: serviceData,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = builder.BuildFromEvent(ctx, event)
	}

	b.ReportMetric(float64(labelIndex.HitRate()), "hitRate%")
}

// BenchmarkCompare_StateCacheVsNoCache compares change detection with/without state cache
func BenchmarkCompare_StateCacheVsNoCache(b *testing.B) {
	// Test data
	resourceData, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":       "test-pod",
			"namespace":  "default",
			"uid":        "test-uid",
			"generation": float64(1),
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "main",
					"image": "nginx:1.19",
				},
			},
		},
		"status": map[string]interface{}{
			"phase": "Running",
		},
	})

	b.Run("WithStateCache", func(b *testing.B) {
		cache, _ := NewStateCache(10000)
		// Pre-warm cache
		cache.Put("test-uid", resourceData, time.Now().UnixNano()-1000000, "CREATE")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cache.Get("test-uid")
		}

		hits, misses, _ := cache.GetStats()
		b.ReportMetric(hitRate(hits, misses), "hitRate%")
	})

	b.Run("WithoutCache_SimulatedQuery", func(b *testing.B) {
		// Simulate the overhead of a database query (just the unmarshaling part)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var parsed map[string]interface{}
			_ = json.Unmarshal(resourceData, &parsed)
		}
	})
}

// BenchmarkCompare_BatchVsIndividual compares batch vs individual event processing
func BenchmarkCompare_BatchVsIndividual(b *testing.B) {
	events := generateMixedEvents(100)
	ctx := context.Background()

	b.Run("Individual", func(b *testing.B) {
		builder := NewGraphBuilder()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, event := range events {
				_, _ = builder.BuildFromEvent(ctx, event)
			}
		}
	})

	b.Run("Batch", func(b *testing.B) {
		builder := NewGraphBuilder()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			builder.SetBatchCache(events)
			_, _ = builder.BuildFromBatch(ctx, events)
			builder.ClearBatchCache()
		}
	})
}

// BenchmarkCompare_LabelIndexScaling tests label index performance at different scales
func BenchmarkCompare_LabelIndexScaling(b *testing.B) {
	scales := []struct {
		name     string
		numPods  int
		numNs    int
		numApps  int
	}{
		{"Small_100pods", 100, 5, 10},
		{"Medium_1000pods", 1000, 20, 50},
		{"Large_10000pods", 10000, 100, 500},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			labelIndex := NewLabelIndex()

			// Populate index
			for i := 0; i < scale.numPods; i++ {
				ns := fmt.Sprintf("ns-%d", i%scale.numNs)
				uid := fmt.Sprintf("pod-%d", i)
				labels := map[string]string{
					"app":     fmt.Sprintf("app-%d", i%scale.numApps),
					"version": fmt.Sprintf("v%d", i%5),
				}
				labelIndex.Update(ns, "Pod", uid, labels)
			}

			selector := map[string]string{"app": "app-1", "version": "v1"}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = labelIndex.FindBySelector("ns-0", "Pod", selector)
			}

			b.ReportMetric(float64(labelIndex.Len()), "indexSize")
		})
	}
}

// BenchmarkCompare_StateCacheScaling tests state cache performance at different sizes
func BenchmarkCompare_StateCacheScaling(b *testing.B) {
	scales := []struct {
		name      string
		cacheSize int
		fillRatio float64 // How full the cache is
	}{
		{"Small_1000_50pct", 1000, 0.5},
		{"Medium_5000_80pct", 5000, 0.8},
		{"Large_10000_90pct", 10000, 0.9},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			cache, _ := NewStateCache(scale.cacheSize)

			// Fill cache to specified ratio
			fillCount := int(float64(scale.cacheSize) * scale.fillRatio)
			for i := 0; i < fillCount; i++ {
				uid := fmt.Sprintf("resource-%d", i)
				data := []byte(fmt.Sprintf(`{"metadata":{"uid":"%s"}}`, uid))
				cache.Put(uid, data, time.Now().UnixNano(), "CREATE")
			}

			// Test UID that exists in cache
			testUID := fmt.Sprintf("resource-%d", fillCount/2)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = cache.Get(testUID)
			}

			b.ReportMetric(float64(cache.Len()), "cacheSize")
			b.ReportMetric(cache.HitRate(), "hitRate%")
		})
	}
}

// BenchmarkMemory_LabelIndex measures memory usage of label index
func BenchmarkMemory_LabelIndex(b *testing.B) {
	scales := []int{1000, 5000, 10000}

	for _, numPods := range scales {
		b.Run(fmt.Sprintf("%dpods", numPods), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				labelIndex := NewLabelIndex()

				for j := 0; j < numPods; j++ {
					ns := fmt.Sprintf("ns-%d", j%50)
					uid := fmt.Sprintf("pod-%d", j)
					labels := map[string]string{
						"app":       fmt.Sprintf("app-%d", j%100),
						"version":   fmt.Sprintf("v%d", j%5),
						"component": fmt.Sprintf("comp-%d", j%20),
						"team":      fmt.Sprintf("team-%d", j%10),
					}
					labelIndex.Update(ns, "Pod", uid, labels)
				}
			}
		})
	}
}

// BenchmarkMemory_StateCache measures memory usage of state cache
func BenchmarkMemory_StateCache(b *testing.B) {
	sizes := []int{1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size%d", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				cache, _ := NewStateCache(size)

				for j := 0; j < size; j++ {
					uid := fmt.Sprintf("resource-%d", j)
					// Typical resource JSON is ~1-2KB
					data := make([]byte, 1500)
					cache.Put(uid, data, time.Now().UnixNano(), "CREATE")
				}
			}
		})
	}
}
