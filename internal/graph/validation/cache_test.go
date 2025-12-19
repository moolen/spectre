package validation

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUCache_BasicOperations(t *testing.T) {
	cache := NewLRUCache(3, 1*time.Hour)
	
	// Test Put and Get
	cache.Put("key1", "value1")
	val, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
	
	// Test non-existent key
	val, ok = cache.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
	
	// Test Size
	cache.Put("key2", "value2")
	cache.Put("key3", "value3")
	assert.Equal(t, 3, cache.Size())
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(2, 1*time.Hour)
	
	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	
	// Adding third item should evict key1 (least recently used)
	cache.Put("key3", "value3")
	
	_, ok := cache.Get("key1")
	assert.False(t, ok, "key1 should have been evicted")
	
	_, ok = cache.Get("key2")
	assert.True(t, ok, "key2 should still exist")
	
	_, ok = cache.Get("key3")
	assert.True(t, ok, "key3 should exist")
}

func TestLRUCache_LRUOrdering(t *testing.T) {
	cache := NewLRUCache(2, 1*time.Hour)
	
	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	
	// Access key1 to make it most recently used
	cache.Get("key1")
	
	// Adding key3 should evict key2 (least recently used)
	cache.Put("key3", "value3")
	
	_, ok := cache.Get("key2")
	assert.False(t, ok, "key2 should have been evicted")
	
	_, ok = cache.Get("key1")
	assert.True(t, ok, "key1 should still exist")
	
	_, ok = cache.Get("key3")
	assert.True(t, ok, "key3 should exist")
}

func TestLRUCache_TTL(t *testing.T) {
	cache := NewLRUCache(10, 50*time.Millisecond)
	
	cache.Put("key1", "value1")
	
	// Should be available immediately
	val, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
	
	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)
	
	// Should be expired
	val, ok = cache.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(10, 1*time.Hour)
	
	cache.Put("key1", "value1")
	assert.Equal(t, 1, cache.Size())
	
	cache.Delete("key1")
	assert.Equal(t, 0, cache.Size())
	
	_, ok := cache.Get("key1")
	assert.False(t, ok)
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(10, 1*time.Hour)
	
	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	cache.Put("key3", "value3")
	assert.Equal(t, 3, cache.Size())
	
	cache.Clear()
	assert.Equal(t, 0, cache.Size())
	
	_, ok := cache.Get("key1")
	assert.False(t, ok)
}

func TestCachedResourceLookup_FindByUID(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultCacheConfig()
	lookup := NewCachedResourceLookup(client, config)
	
	// Mock query result
	client.queryResults[`
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
			LIMIT 1
		`] = &graph.QueryResult{
		Rows: [][]interface{}{
			{
				map[string]interface{}{
					"uid":       "test-uid",
					"kind":      "Pod",
					"namespace": "default",
					"name":      "test-pod",
					"deleted":   false,
				},
			},
		},
	}
	
	// First call - should miss cache
	resource, err := lookup.FindResourceByUID(context.Background(), "test-uid")
	require.NoError(t, err)
	require.NotNil(t, resource)
	assert.Equal(t, "test-uid", resource.UID)
	assert.Equal(t, "Pod", resource.Kind)
	
	stats := lookup.GetStats()
	assert.Equal(t, int64(0), stats["hits"])
	assert.Equal(t, int64(1), stats["misses"])
	
	// Second call - should hit cache
	resource2, err := lookup.FindResourceByUID(context.Background(), "test-uid")
	require.NoError(t, err)
	require.NotNil(t, resource2)
	assert.Equal(t, "test-uid", resource2.UID)
	
	stats = lookup.GetStats()
	assert.Equal(t, int64(1), stats["hits"])
	assert.Equal(t, int64(1), stats["misses"])
	
	// Verify only one query was executed
	assert.Equal(t, 1, len(client.queries))
}

func TestCachedResourceLookup_FindByNamespace(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultCacheConfig()
	lookup := NewCachedResourceLookup(client, config)
	
	// Mock query result
	client.queryResults[`
			MATCH (r:ResourceIdentity {namespace: $namespace, kind: $kind, name: $name})
			WHERE r.deleted = false
			RETURN r
			LIMIT 1
		`] = &graph.QueryResult{
		Rows: [][]interface{}{
			{
				map[string]interface{}{
					"uid":       "test-uid",
					"kind":      "Service",
					"namespace": "default",
					"name":      "test-service",
					"deleted":   false,
				},
			},
		},
	}
	
	// First call - should miss cache
	resource, err := lookup.FindResourceByNamespace(context.Background(), "default", "Service", "test-service")
	require.NoError(t, err)
	require.NotNil(t, resource)
	assert.Equal(t, "Service", resource.Kind)
	
	// Second call - should hit cache
	resource2, err := lookup.FindResourceByNamespace(context.Background(), "default", "Service", "test-service")
	require.NoError(t, err)
	require.NotNil(t, resource2)
	
	stats := lookup.GetStats()
	assert.Equal(t, int64(1), stats["hits"])
	assert.Equal(t, int64(1), stats["misses"])
	assert.Greater(t, stats["hitRate"].(float64), 0.0)
}

func TestCachedResourceLookup_Invalidation(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultCacheConfig()
	lookup := NewCachedResourceLookup(client, config)
	
	// Mock query result
	client.queryResults[`
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
			LIMIT 1
		`] = &graph.QueryResult{
		Rows: [][]interface{}{
			{
				map[string]interface{}{
					"uid":  "test-uid",
					"kind": "Pod",
				},
			},
		},
	}
	
	// Populate cache
	lookup.FindResourceByUID(context.Background(), "test-uid")
	
	stats := lookup.GetStats()
	assert.Equal(t, int64(1), stats["misses"])
	
	// Invalidate
	lookup.InvalidateUID("test-uid")
	
	// Next call should miss cache again
	lookup.FindResourceByUID(context.Background(), "test-uid")
	
	stats = lookup.GetStats()
	assert.Equal(t, int64(2), stats["misses"])
}

func TestCachedResourceLookup_Clear(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultCacheConfig()
	lookup := NewCachedResourceLookup(client, config)
	
	// Mock query result
	client.queryResults[`
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
			LIMIT 1
		`] = &graph.QueryResult{
		Rows: [][]interface{}{
			{
				map[string]interface{}{
					"uid": "test-uid",
				},
			},
		},
	}
	
	// Populate cache
	lookup.FindResourceByUID(context.Background(), "test-uid")
	
	stats := lookup.GetStats()
	assert.Greater(t, stats["size"].(int), 0)
	
	// Clear cache
	lookup.Clear()
	
	stats = lookup.GetStats()
	assert.Equal(t, 0, stats["size"])
	assert.Equal(t, int64(0), stats["hits"])
	assert.Equal(t, int64(0), stats["misses"])
}

func TestCachedResourceLookup_HitRate(t *testing.T) {
	client := NewMockGraphClient()
	config := DefaultCacheConfig()
	lookup := NewCachedResourceLookup(client, config)
	
	// Mock query result
	client.queryResults[`
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
			LIMIT 1
		`] = &graph.QueryResult{
		Rows: [][]interface{}{
			{
				map[string]interface{}{
					"uid": "test-uid",
				},
			},
		},
	}
	
	// 1 miss, 4 hits
	lookup.FindResourceByUID(context.Background(), "test-uid")
	lookup.FindResourceByUID(context.Background(), "test-uid")
	lookup.FindResourceByUID(context.Background(), "test-uid")
	lookup.FindResourceByUID(context.Background(), "test-uid")
	lookup.FindResourceByUID(context.Background(), "test-uid")
	
	stats := lookup.GetStats()
	assert.Equal(t, int64(4), stats["hits"])
	assert.Equal(t, int64(1), stats["misses"])
	assert.InDelta(t, 0.8, stats["hitRate"].(float64), 0.01) // 4/5 = 0.8
}
