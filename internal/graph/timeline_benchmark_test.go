package graph

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// BenchmarkTimelineQuery30Min benchmarks a 30-minute timeline query
func BenchmarkTimelineQuery30Min(b *testing.B) {
	benchmarkTimelineQuery(b, 30*time.Minute)
}

// BenchmarkTimelineQuery1Hour benchmarks a 1-hour timeline query
func BenchmarkTimelineQuery1Hour(b *testing.B) {
	benchmarkTimelineQuery(b, 1*time.Hour)
}

// BenchmarkTimelineQuery6Hours benchmarks a 6-hour timeline query
func BenchmarkTimelineQuery6Hours(b *testing.B) {
	benchmarkTimelineQuery(b, 6*time.Hour)
}

// BenchmarkTimelineQuery24Hours benchmarks a 24-hour timeline query
func BenchmarkTimelineQuery24Hours(b *testing.B) {
	benchmarkTimelineQuery(b, 24*time.Hour)
}

// benchmarkTimelineQuery is the core benchmark function
func benchmarkTimelineQuery(b *testing.B, timeRange time.Duration) {
	// Skip if no test database is available
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Setup test client (assumes FalkorDB is running)
	config := DefaultClientConfig()
	config.GraphName = "spectre_benchmark"
	client := NewClient(config)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		b.Skipf("FalkorDB not available: %v", err)
	}
	defer client.Close()

	executor := NewQueryExecutor(client)

	// Define query
	endTime := time.Now().Unix()
	startTime := endTime - int64(timeRange.Seconds())

	query := &models.QueryRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Filters: models.QueryFilters{
			Namespace: "default",
		},
	}

	// Record initial memory
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := executor.Execute(ctx, query)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}

		// Verify we got results
		if result.Count == 0 {
			b.Log("Warning: query returned 0 events")
		}
	}

	b.StopTimer()

	// Record final memory
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Report memory usage
	memUsed := memAfter.Alloc - memBefore.Alloc
	b.ReportMetric(float64(memUsed)/1024/1024, "MB/op")
}

// BenchmarkTimelineQueryWithKindFilter benchmarks filtered queries
func BenchmarkTimelineQueryWithKindFilter(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	config := DefaultClientConfig()
	config.GraphName = "spectre_benchmark"
	client := NewClient(config)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		b.Skipf("FalkorDB not available: %v", err)
	}
	defer client.Close()

	executor := NewQueryExecutor(client)

	endTime := time.Now().Unix()
	startTime := endTime - 3600 // 1 hour

	query := &models.QueryRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(ctx, query)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkStatusSegmentBuilding benchmarks the resource builder
func BenchmarkStatusSegmentBuilding(b *testing.B) {
	builder := NewResourceBuilder()

	// Create sample events
	events := make([]ChangeEvent, 100)
	baseTime := time.Now().UnixNano()

	for i := range events {
		status := "Ready"
		if i%10 == 0 {
			status = "Warning"
		}

		events[i] = ChangeEvent{
			ID:        string(rune(i)),
			Timestamp: baseTime + int64(i)*1e9,
			Status:    status,
		}
	}

	queryStart := baseTime
	queryEnd := baseTime + 100*1e9

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		segments := builder.BuildStatusSegments(events, queryStart, queryEnd)
		if len(segments) == 0 {
			b.Fatal("No segments built")
		}
	}
}

// TestTimelineQueryPerformance validates performance against acceptance criteria
func TestTimelineQueryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultClientConfig()
	config.GraphName = "spectre_test"
	client := NewClient(config)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}
	defer client.Close()

	// Verify connection is stable
	if err := client.Ping(ctx); err != nil {
		t.Skipf("FalkorDB connection unstable: %v", err)
	}

	executor := NewQueryExecutor(client)

	testCases := []struct {
		name           string
		timeRange      time.Duration
		maxLatencyMs   int64
		acceptableRatio float64 // vs storage baseline
	}{
		{"30min", 30 * time.Minute, 200, 2.0},
		{"1hour", 1 * time.Hour, 300, 2.0},
		{"6hours", 6 * time.Hour, 500, 2.0},
		{"24hours", 24 * time.Hour, 1000, 2.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endTime := time.Now().Unix()
			startTime := endTime - int64(tc.timeRange.Seconds())

			query := &models.QueryRequest{
				StartTimestamp: startTime,
				EndTimestamp:   endTime,
			}

			start := time.Now()
			result, err := executor.Execute(ctx, query)
			latency := time.Since(start)

			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			latencyMs := latency.Milliseconds()
			t.Logf("Query returned %d events in %dms", result.Count, latencyMs)

			if latencyMs > tc.maxLatencyMs {
				t.Errorf("Latency %dms exceeds maximum %dms", latencyMs, tc.maxLatencyMs)
			}
		})
	}
}

// TestMemoryUsage validates memory consumption
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	config := DefaultClientConfig()
	config.GraphName = "spectre_test"
	client := NewClient(config)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}
	defer client.Close()

	// Verify connection is stable
	if err := client.Ping(ctx); err != nil {
		t.Skipf("FalkorDB connection unstable: %v", err)
	}

	executor := NewQueryExecutor(client)

	// Query 7 days of data
	endTime := time.Now().Unix()
	startTime := endTime - 7*24*3600

	query := &models.QueryRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
	}

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	result, err := executor.Execute(ctx, query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	memUsedMB := float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024
	t.Logf("7-day query: %d events, %.2f MB memory used", result.Count, memUsedMB)

	// Acceptance: < 500MB for 7-day retention
	if memUsedMB > 500 {
		t.Errorf("Memory usage %.2fMB exceeds 500MB limit", memUsedMB)
	}
}
