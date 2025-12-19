package graph

import (
	"sync"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// TestNewQueryCache tests cache creation
func TestNewQueryCache(t *testing.T) {
	tests := []struct {
		name      string
		config    QueryCacheConfig
		shouldErr bool
	}{
		{
			name: "Valid 64MB cache",
			config: QueryCacheConfig{
				MaxMemoryMB: 64,
				TTL:         2 * time.Minute,
				Enabled:     true,
			},
			shouldErr: false,
		},
		{
			name: "Valid 1MB cache",
			config: QueryCacheConfig{
				MaxMemoryMB: 1,
				TTL:         30 * time.Second,
				Enabled:     true,
			},
			shouldErr: false,
		},
		{
			name: "Invalid 0MB cache",
			config: QueryCacheConfig{
				MaxMemoryMB: 0,
				TTL:         2 * time.Minute,
				Enabled:     true,
			},
			shouldErr: true,
		},
		{
			name: "Invalid negative memory",
			config: QueryCacheConfig{
				MaxMemoryMB: -1,
				TTL:         2 * time.Minute,
				Enabled:     true,
			},
			shouldErr: true,
		},
		{
			name: "Invalid zero TTL",
			config: QueryCacheConfig{
				MaxMemoryMB: 64,
				TTL:         0,
				Enabled:     true,
			},
			shouldErr: true,
		},
		{
			name: "Invalid negative TTL",
			config: QueryCacheConfig{
				MaxMemoryMB: 64,
				TTL:         -1 * time.Second,
				Enabled:     true,
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewQueryCache(tt.config, logging.GetLogger("test"))
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if cache == nil {
					t.Errorf("expected cache, got nil")
				}
				expectedMaxMemory := tt.config.MaxMemoryMB * 1024 * 1024
				if cache.maxMemory != expectedMaxMemory {
					t.Errorf("expected maxMemory %d, got %d", expectedMaxMemory, cache.maxMemory)
				}
				if cache.ttl != tt.config.TTL {
					t.Errorf("expected TTL %v, got %v", tt.config.TTL, cache.ttl)
				}
			}
		})
	}
}

// TestQueryCacheGetPut tests basic get/put operations
func TestQueryCacheGetPut(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create test result
	result := &QueryResult{
		Columns: []string{"uid", "name", "kind"},
		Rows: [][]interface{}{
			{"uid-1", "test-pod-1", "Pod"},
			{"uid-2", "test-pod-2", "Pod"},
		},
		Stats: QueryStats{
			NodesCreated:  0,
			ExecutionTime: 100 * time.Millisecond,
		},
	}

	key := "test-query-key-12345678"

	// Put result in cache
	cache.Put(key, result)

	// Get result from cache
	retrieved, ok := cache.Get(key)
	if !ok {
		t.Error("expected to get result, got not found")
	}
	if retrieved == nil {
		t.Fatal("expected result, got nil")
	}
	if len(retrieved.Columns) != len(result.Columns) {
		t.Errorf("expected %d columns, got %d", len(result.Columns), len(retrieved.Columns))
	}
	if len(retrieved.Rows) != len(result.Rows) {
		t.Errorf("expected %d rows, got %d", len(result.Rows), len(retrieved.Rows))
	}
}

// TestQueryCacheMiss tests cache miss behavior
func TestQueryCacheMiss(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Try to get non-existent key
	result, ok := cache.Get("non-existent-key-12345678")
	if ok {
		t.Error("expected cache miss, got hit")
	}
	if result != nil {
		t.Error("expected nil result, got non-nil")
	}

	stats := cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

// TestQueryCacheTTLExpiration tests TTL expiration behavior
func TestQueryCacheTTLExpiration(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         50 * time.Millisecond, // Very short TTL for testing
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	result := &QueryResult{
		Columns: []string{"uid"},
		Rows:    [][]interface{}{{"uid-1"}},
	}

	key := "test-ttl-key-12345678"
	cache.Put(key, result)

	// Should be in cache immediately
	retrieved, ok := cache.Get(key)
	if !ok {
		t.Error("expected cache hit immediately after put")
	}
	if retrieved == nil {
		t.Error("expected result immediately after put")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	retrieved, ok = cache.Get(key)
	if ok {
		t.Error("expected cache miss after TTL expiration")
	}
	if retrieved != nil {
		t.Error("expected nil result after TTL expiration")
	}

	stats := cache.Stats()
	if stats.Expired != 1 {
		t.Errorf("expected 1 expired entry, got %d", stats.Expired)
	}
}

// TestQueryCacheStats tests cache statistics
func TestQueryCacheStats(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	result1 := &QueryResult{
		Columns: []string{"uid"},
		Rows:    [][]interface{}{{"uid-1"}},
	}
	result2 := &QueryResult{
		Columns: []string{"uid", "name"},
		Rows:    [][]interface{}{{"uid-2", "name-2"}},
	}

	cache.Put("key1-1234567890123456", result1)
	cache.Put("key2-1234567890123456", result2)

	// 2 hits
	cache.Get("key1-1234567890123456")
	cache.Get("key1-1234567890123456")

	// 1 miss
	cache.Get("key3-1234567890123456")

	stats := cache.Stats()

	if stats.Items != 2 {
		t.Errorf("expected 2 items, got %d", stats.Items)
	}
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.UsedMemory <= 0 {
		t.Errorf("expected positive used memory, got %d", stats.UsedMemory)
	}

	expectedHitRate := 2.0 / 3.0
	if stats.HitRate < expectedHitRate-0.01 || stats.HitRate > expectedHitRate+0.01 {
		t.Errorf("expected hit rate ~%.3f, got %.3f", expectedHitRate, stats.HitRate)
	}
}

// TestQueryCacheLRUEviction tests LRU eviction under memory pressure
func TestQueryCacheLRUEviction(t *testing.T) {
	// Use 1MB cache (minimum allowed)
	// Create results that are ~400KB each, so 2 fit but 3 don't
	config := QueryCacheConfig{
		MaxMemoryMB: 1, // 1MB = 1048576 bytes
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create results that are ~450KB each (3 won't fit in 1MB)
	largeResult1 := createLargeResult(450)
	largeResult2 := createLargeResult(450)
	largeResult3 := createLargeResult(450)

	size1 := estimateResultSize(largeResult1)
	size2 := estimateResultSize(largeResult2)
	size3 := estimateResultSize(largeResult3)

	t.Logf("Result sizes: size1=%d, size2=%d, size3=%d, total=%d, maxMemory=%d",
		size1, size2, size3, size1+size2+size3, config.MaxMemoryMB*1024*1024)

	cache.Put("key1-1234567890123456", largeResult1)
	cache.Put("key2-1234567890123456", largeResult2)

	// Access key1 to make it more recently used
	cache.Get("key1-1234567890123456")

	// Check stats before third put
	statsBefore := cache.Stats()
	t.Logf("Before third put: items=%d, usedMemory=%d", statsBefore.Items, statsBefore.UsedMemory)

	// Put third result - should evict at least one entry
	cache.Put("key3-1234567890123456", largeResult3)

	// Check stats after third put
	statsAfter := cache.Stats()
	t.Logf("After third put: items=%d, usedMemory=%d, evictions=%d", statsAfter.Items, statsAfter.UsedMemory, statsAfter.Evictions)

	// At least one item should have been evicted
	if statsAfter.Evictions < 1 {
		t.Errorf("expected at least 1 eviction, got %d", statsAfter.Evictions)
	}

	// key3 should be in cache (most recently added)
	_, ok := cache.Get("key3-1234567890123456")
	if !ok {
		t.Error("expected key3 to be in cache")
	}
}

// TestQueryCacheClear tests cache clearing
func TestQueryCacheClear(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	result1 := &QueryResult{Columns: []string{"uid"}, Rows: [][]interface{}{{"uid-1"}}}
	result2 := &QueryResult{Columns: []string{"uid"}, Rows: [][]interface{}{{"uid-2"}}}

	cache.Put("key1-1234567890123456", result1)
	cache.Put("key2-1234567890123456", result2)

	stats := cache.Stats()
	if stats.Items != 2 {
		t.Errorf("expected 2 items before clear, got %d", stats.Items)
	}

	// Clear cache
	cache.Clear()

	stats = cache.Stats()
	if stats.Items != 0 {
		t.Errorf("expected 0 items after clear, got %d", stats.Items)
	}
	if stats.UsedMemory != 0 {
		t.Errorf("expected 0 used memory after clear, got %d", stats.UsedMemory)
	}
}

// TestQueryCacheInvalidate tests entry invalidation
func TestQueryCacheInvalidate(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	result := &QueryResult{Columns: []string{"uid"}, Rows: [][]interface{}{{"uid-1"}}}
	key := "test-invalidate-key-1234"

	cache.Put(key, result)

	// Verify it's in cache
	_, ok := cache.Get(key)
	if !ok {
		t.Error("expected key to be in cache before invalidation")
	}

	// Invalidate
	cache.Invalidate(key)

	// Verify it's gone
	_, ok = cache.Get(key)
	if ok {
		t.Error("expected key to be removed after invalidation")
	}
}

// TestMakeQueryKey tests deterministic key generation
func TestMakeQueryKey(t *testing.T) {
	query1 := GraphQuery{
		Query: "MATCH (n:ResourceIdentity) WHERE n.uid = $uid RETURN n",
		Parameters: map[string]interface{}{
			"uid": "test-uid-123",
		},
	}

	query2 := GraphQuery{
		Query: "MATCH (n:ResourceIdentity) WHERE n.uid = $uid RETURN n",
		Parameters: map[string]interface{}{
			"uid": "test-uid-123",
		},
	}

	query3 := GraphQuery{
		Query: "MATCH (n:ResourceIdentity) WHERE n.uid = $uid RETURN n",
		Parameters: map[string]interface{}{
			"uid": "different-uid",
		},
	}

	key1 := MakeQueryKey(query1)
	key2 := MakeQueryKey(query2)
	key3 := MakeQueryKey(query3)

	// Same query should produce same key
	if key1 != key2 {
		t.Errorf("expected same key for identical queries, got %s and %s", key1, key2)
	}

	// Different parameters should produce different key
	if key1 == key3 {
		t.Errorf("expected different keys for different parameters")
	}

	// Key should be 64 chars (SHA256 hex)
	if len(key1) != 64 {
		t.Errorf("expected key length 64, got %d", len(key1))
	}
}

// TestMakeQueryKeyParameterOrder tests that parameter order doesn't affect key
func TestMakeQueryKeyParameterOrder(t *testing.T) {
	query1 := GraphQuery{
		Query: "MATCH (n) WHERE n.a = $a AND n.b = $b RETURN n",
		Parameters: map[string]interface{}{
			"a": "value-a",
			"b": "value-b",
		},
	}

	query2 := GraphQuery{
		Query: "MATCH (n) WHERE n.a = $a AND n.b = $b RETURN n",
		Parameters: map[string]interface{}{
			"b": "value-b",
			"a": "value-a",
		},
	}

	key1 := MakeQueryKey(query1)
	key2 := MakeQueryKey(query2)

	if key1 != key2 {
		t.Errorf("expected same key regardless of parameter order, got %s and %s", key1, key2)
	}
}

// TestIsWriteQuery tests write query detection
func TestIsWriteQuery(t *testing.T) {
	tests := []struct {
		query   string
		isWrite bool
	}{
		{"MATCH (n) RETURN n", false},
		{"match (n) return n", false},
		{"MATCH (n:Pod) WHERE n.uid = $uid RETURN n", false},
		{"CREATE (n:Pod {uid: $uid})", true},
		{"create (n:Pod {uid: $uid})", true},
		{"MERGE (n:Pod {uid: $uid})", true},
		{"MATCH (n) DELETE n", true},
		{"MATCH (n) DETACH DELETE n", true},
		{"MATCH (n) SET n.name = $name", true},
		{"MATCH (n) REMOVE n.name", true},
		{"MATCH (n) WHERE n.uid = $uid CREATE (m:Pod)", true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := isWriteQuery(tt.query)
			if result != tt.isWrite {
				t.Errorf("isWriteQuery(%q) = %v, want %v", tt.query, result, tt.isWrite)
			}
		})
	}
}

// TestQueryCacheConcurrent tests concurrent access
func TestQueryCacheConcurrent(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 100,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create multiple results
	results := make([]*QueryResult, 10)
	for i := 0; i < 10; i++ {
		results[i] = &QueryResult{
			Columns: []string{"uid"},
			Rows:    [][]interface{}{{i}},
		}
	}

	// Put results concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := MakeQueryKey(GraphQuery{
				Query:      "MATCH (n) RETURN n",
				Parameters: map[string]interface{}{"idx": idx},
			})
			cache.Put(key, results[idx])
		}(i)
	}
	wg.Wait()

	// Get results concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := MakeQueryKey(GraphQuery{
				Query:      "MATCH (n) RETURN n",
				Parameters: map[string]interface{}{"idx": idx},
			})
			result, ok := cache.Get(key)
			if !ok {
				t.Errorf("expected to find result for idx %d", idx)
			}
			if result == nil {
				t.Errorf("expected non-nil result for idx %d", idx)
			}
		}(i)
	}
	wg.Wait()

	stats := cache.Stats()
	if stats.Items != 10 {
		t.Errorf("expected 10 items, got %d", stats.Items)
	}
}

// TestEstimateResultSize tests size estimation
func TestEstimateResultSize(t *testing.T) {
	// Empty result
	emptyResult := &QueryResult{}
	emptySize := estimateResultSize(emptyResult)
	if emptySize <= 0 {
		t.Errorf("expected positive size for empty result, got %d", emptySize)
	}

	// Result with data
	result := &QueryResult{
		Columns: []string{"uid", "name", "kind", "namespace"},
		Rows: [][]interface{}{
			{"uid-1", "pod-1", "Pod", "default"},
			{"uid-2", "pod-2", "Pod", "kube-system"},
		},
	}
	size := estimateResultSize(result)
	if size <= emptySize {
		t.Errorf("expected larger size for result with data, got %d (empty: %d)", size, emptySize)
	}

	// Nil result
	nilSize := estimateResultSize(nil)
	if nilSize != 0 {
		t.Errorf("expected 0 size for nil result, got %d", nilSize)
	}
}

// TestQueryCacheZeroHitRate tests hit rate when no accesses
func TestQueryCacheZeroHitRate(t *testing.T) {
	config := QueryCacheConfig{
		MaxMemoryMB: 10,
		TTL:         5 * time.Minute,
		Enabled:     true,
	}
	cache, err := NewQueryCache(config, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	result := &QueryResult{Columns: []string{"uid"}, Rows: [][]interface{}{{"uid-1"}}}
	cache.Put("test-key-12345678901234", result)

	stats := cache.Stats()
	if stats.HitRate != 0.0 {
		t.Errorf("expected hit rate 0.0 with no accesses, got %.3f", stats.HitRate)
	}
}

// TestDefaultQueryCacheConfig tests default configuration
func TestDefaultQueryCacheConfig(t *testing.T) {
	config := DefaultQueryCacheConfig()

	if config.MaxMemoryMB != 64 {
		t.Errorf("expected default MaxMemoryMB 64, got %d", config.MaxMemoryMB)
	}
	if config.TTL != 2*time.Minute {
		t.Errorf("expected default TTL 2m, got %v", config.TTL)
	}
	if config.Enabled != false {
		t.Errorf("expected default Enabled false, got %v", config.Enabled)
	}
}

// createLargeResult creates a result with approximately the specified size in KB
func createLargeResult(sizeKB int) *QueryResult {
	// Create rows with enough data to reach target size
	// Each row when JSON serialized is roughly 130 bytes
	targetBytes := sizeKB * 1024
	rowCount := targetBytes / 130
	if rowCount < 1 {
		rowCount = 1
	}

	// Create a long padding string to increase row size
	padding := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	rows := make([][]interface{}, rowCount)
	for i := 0; i < rowCount; i++ {
		// Create a unique string for each row
		rows[i] = []interface{}{
			"uid-1234567890abcdef-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			"some-very-long-name-value-that-takes-up-space-" + padding + "-" + string(rune('a'+i%26)),
			"ResourceIdentity",
			"default-namespace-with-extra-padding-" + padding,
			i,
			true,
		}
	}
	return &QueryResult{
		Columns: []string{"uid", "name", "kind", "namespace", "index", "active"},
		Rows:    rows,
	}
}
