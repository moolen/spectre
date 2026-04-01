package sync

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateCache(t *testing.T) {
	t.Run("creates cache with specified size", func(t *testing.T) {
		cache, err := NewStateCache(100)
		require.NoError(t, err)
		assert.NotNil(t, cache)
	})

	t.Run("uses default size when zero", func(t *testing.T) {
		cache, err := NewStateCache(0)
		require.NoError(t, err)
		assert.NotNil(t, cache)
	})

	t.Run("uses default size when negative", func(t *testing.T) {
		cache, err := NewStateCache(-10)
		require.NoError(t, err)
		assert.NotNil(t, cache)
	})
}

func TestStateCache_PutAndGet(t *testing.T) {
	cache, _ := NewStateCache(100)

	t.Run("returns nil for missing key", func(t *testing.T) {
		state := cache.Get("nonexistent")
		assert.Nil(t, state)
	})

	t.Run("stores and retrieves state", func(t *testing.T) {
		data := []byte(`{"spec":{"replicas":1}}`)
		cache.Put("uid-1", data, 1000, "CREATE")

		state := cache.Get("uid-1")
		require.NotNil(t, state)
		assert.Equal(t, data, state.Data)
		assert.Equal(t, int64(1000), state.Timestamp)
		assert.Equal(t, "CREATE", state.EventType)
	})

	t.Run("updates existing state", func(t *testing.T) {
		data1 := []byte(`{"spec":{"replicas":1}}`)
		data2 := []byte(`{"spec":{"replicas":2}}`)

		cache.Put("uid-2", data1, 1000, "CREATE")
		cache.Put("uid-2", data2, 2000, "UPDATE")

		state := cache.Get("uid-2")
		require.NotNil(t, state)
		assert.Equal(t, data2, state.Data)
		assert.Equal(t, int64(2000), state.Timestamp)
		assert.Equal(t, "UPDATE", state.EventType)
	})

	t.Run("stores copy of data", func(t *testing.T) {
		data := []byte(`{"original":true}`)
		cache.Put("uid-3", data, 1000, "CREATE")

		// Modify original data
		data[0] = 'X'

		// Cached data should be unchanged
		state := cache.Get("uid-3")
		require.NotNil(t, state)
		assert.Equal(t, byte('{'), state.Data[0])
	})
}

func TestStateCache_Remove(t *testing.T) {
	cache, _ := NewStateCache(100)

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	assert.NotNil(t, cache.Get("uid-1"))

	cache.Remove("uid-1")
	assert.Nil(t, cache.Get("uid-1"))
}

func TestStateCache_Contains(t *testing.T) {
	cache, _ := NewStateCache(100)

	assert.False(t, cache.Contains("uid-1"))

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	assert.True(t, cache.Contains("uid-1"))

	cache.Remove("uid-1")
	assert.False(t, cache.Contains("uid-1"))
}

func TestStateCache_Len(t *testing.T) {
	cache, _ := NewStateCache(100)

	assert.Equal(t, 0, cache.Len())

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	assert.Equal(t, 1, cache.Len())

	cache.Put("uid-2", []byte(`{}`), 2000, "CREATE")
	assert.Equal(t, 2, cache.Len())

	cache.Remove("uid-1")
	assert.Equal(t, 1, cache.Len())
}

func TestStateCache_LRUEviction(t *testing.T) {
	// Create tiny cache that can only hold 3 items
	cache, _ := NewStateCache(3)

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	cache.Put("uid-2", []byte(`{}`), 2000, "CREATE")
	cache.Put("uid-3", []byte(`{}`), 3000, "CREATE")

	// All three should exist
	assert.NotNil(t, cache.Get("uid-1"))
	assert.NotNil(t, cache.Get("uid-2"))
	assert.NotNil(t, cache.Get("uid-3"))

	// Adding a 4th item should evict the least recently used (uid-1 in original order,
	// but we just accessed all of them, so it depends on access order)
	// Access uid-1 to make it recently used
	cache.Get("uid-1")

	// Now add uid-4, which should evict uid-2 (least recently used)
	cache.Put("uid-4", []byte(`{}`), 4000, "CREATE")

	assert.NotNil(t, cache.Get("uid-1"), "uid-1 was recently accessed, should not be evicted")
	assert.Nil(t, cache.Get("uid-2"), "uid-2 should have been evicted")
	assert.NotNil(t, cache.Get("uid-3"), "uid-3 should still exist")
	assert.NotNil(t, cache.Get("uid-4"), "uid-4 was just added")
}

func TestStateCache_Stats(t *testing.T) {
	cache, _ := NewStateCache(100)

	// Initial stats should be zero
	hits, misses, size := cache.GetStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 0, size)

	// Add some items
	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	cache.Put("uid-2", []byte(`{}`), 2000, "CREATE")

	// Get existing item (hit)
	cache.Get("uid-1")
	hits, misses, size = cache.GetStats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 2, size)

	// Get non-existing item (miss)
	cache.Get("uid-999")
	hits, misses, size = cache.GetStats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)

	// Multiple accesses
	cache.Get("uid-1") // hit
	cache.Get("uid-2") // hit
	cache.Get("uid-3") // miss
	hits, misses, size = cache.GetStats()
	assert.Equal(t, int64(3), hits)
	assert.Equal(t, int64(2), misses)
}

func TestStateCache_HitRate(t *testing.T) {
	cache, _ := NewStateCache(100)

	// No lookups = 0% hit rate
	assert.Equal(t, 0.0, cache.HitRate())

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")

	// 1 hit out of 1 = 100%
	cache.Get("uid-1")
	assert.Equal(t, 100.0, cache.HitRate())

	// 1 hit out of 2 = 50%
	cache.Get("uid-missing")
	assert.Equal(t, 50.0, cache.HitRate())

	// 2 hits out of 3 = 66.67%
	cache.Get("uid-1")
	hitRate := cache.HitRate()
	assert.InDelta(t, 66.67, hitRate, 0.1)
}

func TestStateCache_ResetStats(t *testing.T) {
	cache, _ := NewStateCache(100)

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	cache.Get("uid-1")
	cache.Get("uid-missing")

	hits, misses, _ := cache.GetStats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)

	cache.ResetStats()

	hits, misses, size := cache.GetStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 1, size) // Size should still be 1
}

func TestStateCache_Clear(t *testing.T) {
	cache, _ := NewStateCache(100)

	cache.Put("uid-1", []byte(`{}`), 1000, "CREATE")
	cache.Put("uid-2", []byte(`{}`), 2000, "CREATE")
	cache.Get("uid-1") // Generate some stats

	cache.Clear()

	// Cache should be empty
	assert.Equal(t, 0, cache.Len())
	assert.Nil(t, cache.Get("uid-1"))
	assert.Nil(t, cache.Get("uid-2"))

	// Stats should be reset
	hits, misses, _ := cache.GetStats()
	// Note: The Get calls above after Clear will register as misses
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(2), misses) // Two misses from the Get calls after Clear
}

func TestStateCache_ConcurrentAccess(t *testing.T) {
	cache, _ := NewStateCache(1000)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				uid := fmt.Sprintf("uid-%d-%d", id, j)
				data := []byte(fmt.Sprintf(`{"id":%d}`, j))
				cache.Put(uid, data, int64(j), "UPDATE")
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				uid := fmt.Sprintf("uid-%d-%d", id, j)
				cache.Get(uid)
			}
		}(i)
	}

	// Concurrent removers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				uid := fmt.Sprintf("uid-%d-%d", id, j)
				cache.Remove(uid)
			}
		}(i)
	}

	// Should complete without race conditions
	wg.Wait()

	// Verify cache is in a consistent state
	hits, misses, size := cache.GetStats()
	t.Logf("After concurrent access: hits=%d, misses=%d, size=%d", hits, misses, size)
	assert.True(t, size >= 0 && size <= 1000)
}

func TestStateCache_TypicalUsage(t *testing.T) {
	// Simulate typical usage pattern: create, update, update, update, delete
	cache, _ := NewStateCache(100)

	uid := "pod-12345"

	// CREATE event
	createData := []byte(`{"metadata":{"name":"test"},"spec":{"replicas":1}}`)
	cache.Put(uid, createData, 1000, "CREATE")

	// First UPDATE - check previous state exists
	state := cache.Get(uid)
	require.NotNil(t, state)
	assert.Equal(t, int64(1000), state.Timestamp)
	assert.Equal(t, "CREATE", state.EventType)

	// Store UPDATE
	updateData1 := []byte(`{"metadata":{"name":"test"},"spec":{"replicas":2}}`)
	cache.Put(uid, updateData1, 2000, "UPDATE")

	// Second UPDATE - check previous state
	state = cache.Get(uid)
	require.NotNil(t, state)
	assert.Equal(t, int64(2000), state.Timestamp)
	assert.Equal(t, "UPDATE", state.EventType)

	// DELETE - remove from cache
	cache.Remove(uid)

	// After delete, state should be gone
	state = cache.Get(uid)
	assert.Nil(t, state)
}

func BenchmarkStateCache_Put(b *testing.B) {
	cache, _ := NewStateCache(10000)
	data := []byte(`{"metadata":{"name":"test","namespace":"default"},"spec":{"replicas":1}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uid := fmt.Sprintf("uid-%d", i%10000)
		cache.Put(uid, data, int64(i), "UPDATE")
	}
}

func BenchmarkStateCache_Get_Hit(b *testing.B) {
	cache, _ := NewStateCache(10000)
	data := []byte(`{"metadata":{"name":"test"}}`)

	// Pre-populate cache
	for i := 0; i < 10000; i++ {
		uid := fmt.Sprintf("uid-%d", i)
		cache.Put(uid, data, int64(i), "CREATE")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uid := fmt.Sprintf("uid-%d", i%10000)
		cache.Get(uid)
	}
}

func BenchmarkStateCache_Get_Miss(b *testing.B) {
	cache, _ := NewStateCache(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uid := fmt.Sprintf("uid-%d", i)
		cache.Get(uid)
	}
}

func BenchmarkStateCache_ConcurrentReadWrite(b *testing.B) {
	cache, _ := NewStateCache(10000)
	data := []byte(`{"metadata":{"name":"test"}}`)

	// Pre-populate half the cache
	for i := 0; i < 5000; i++ {
		uid := fmt.Sprintf("uid-%d", i)
		cache.Put(uid, data, int64(i), "CREATE")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			uid := fmt.Sprintf("uid-%d", i%10000)
			if i%2 == 0 {
				cache.Get(uid)
			} else {
				cache.Put(uid, data, int64(i), "UPDATE")
			}
			i++
		}
	})
}
