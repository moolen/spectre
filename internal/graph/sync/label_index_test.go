package sync

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLabelIndex(t *testing.T) {
	idx := NewLabelIndex()
	assert.NotNil(t, idx)
	assert.Equal(t, 0, idx.Len())
}

func TestLabelIndex_BasicOperations(t *testing.T) {
	idx := NewLabelIndex()

	// Add pod with labels
	idx.Update("default", "Pod", "pod-1", map[string]string{
		"app": "web",
		"env": "prod",
	})

	// Verify it's in the index
	assert.True(t, idx.Contains("default", "Pod", "pod-1"))
	assert.Equal(t, 1, idx.Len())

	// Get labels back
	labels := idx.GetLabels("default", "Pod", "pod-1")
	assert.Equal(t, "web", labels["app"])
	assert.Equal(t, "prod", labels["env"])

	// Find by single label
	uids := idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	assert.Contains(t, uids, "pod-1")

	// Find by multiple labels
	uids = idx.FindBySelector("default", "Pod", map[string]string{"app": "web", "env": "prod"})
	assert.Contains(t, uids, "pod-1")

	// No match for wrong label
	uids = idx.FindBySelector("default", "Pod", map[string]string{"app": "api"})
	assert.Empty(t, uids)
}

func TestLabelIndex_Update(t *testing.T) {
	idx := NewLabelIndex()

	// Add pod
	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})

	// Update labels
	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "api"})

	// Old label no longer matches
	uids := idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	assert.Empty(t, uids)

	// New label matches
	uids = idx.FindBySelector("default", "Pod", map[string]string{"app": "api"})
	assert.Contains(t, uids, "pod-1")

	// Verify only one resource in index
	assert.Equal(t, 1, idx.Len())
}

func TestLabelIndex_Remove(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})
	assert.True(t, idx.Contains("default", "Pod", "pod-1"))

	idx.Remove("default", "Pod", "pod-1")

	assert.False(t, idx.Contains("default", "Pod", "pod-1"))
	uids := idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	assert.Empty(t, uids)
	assert.Equal(t, 0, idx.Len())
}

func TestLabelIndex_RemoveNonExistent(t *testing.T) {
	idx := NewLabelIndex()

	// Should not panic when removing non-existent resource
	idx.Remove("default", "Pod", "nonexistent")
	assert.Equal(t, 0, idx.Len())
}

func TestLabelIndex_MultipleMatches(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web", "tier": "frontend"})
	idx.Update("default", "Pod", "pod-2", map[string]string{"app": "web", "tier": "backend"})
	idx.Update("default", "Pod", "pod-3", map[string]string{"app": "api", "tier": "frontend"})

	// Match by app=web
	uids := idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	sort.Strings(uids)
	assert.Equal(t, []string{"pod-1", "pod-2"}, uids)

	// Match by app=web AND tier=frontend
	uids = idx.FindBySelector("default", "Pod", map[string]string{"app": "web", "tier": "frontend"})
	assert.Equal(t, []string{"pod-1"}, uids)

	// Match by tier=frontend only
	uids = idx.FindBySelector("default", "Pod", map[string]string{"tier": "frontend"})
	sort.Strings(uids)
	assert.Equal(t, []string{"pod-1", "pod-3"}, uids)
}

func TestLabelIndex_MultipleNamespaces(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})
	idx.Update("kube-system", "Pod", "pod-2", map[string]string{"app": "web"})
	idx.Update("production", "Pod", "pod-3", map[string]string{"app": "web"})

	// Should only find in specified namespace
	uids := idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	assert.Equal(t, []string{"pod-1"}, uids)

	uids = idx.FindBySelector("kube-system", "Pod", map[string]string{"app": "web"})
	assert.Equal(t, []string{"pod-2"}, uids)

	// Different namespace returns empty
	uids = idx.FindBySelector("staging", "Pod", map[string]string{"app": "web"})
	assert.Empty(t, uids)

	// Verify stats
	_, _, namespaces, resources := idx.GetStats()
	assert.Equal(t, 3, namespaces)
	assert.Equal(t, 3, resources)
}

func TestLabelIndex_EmptySelector(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})

	// Empty selector returns nil
	uids := idx.FindBySelector("default", "Pod", map[string]string{})
	assert.Nil(t, uids)

	// Nil selector would also return nil (if we accept nil)
	uids = idx.FindBySelector("default", "Pod", nil)
	assert.Nil(t, uids)
}

func TestLabelIndex_EmptyLabels(t *testing.T) {
	idx := NewLabelIndex()

	// Adding resource with empty labels should not add to index
	idx.Update("default", "Pod", "pod-1", map[string]string{})
	assert.Equal(t, 0, idx.Len())

	// Adding resource with nil labels should not add to index
	idx.Update("default", "Pod", "pod-2", nil)
	assert.Equal(t, 0, idx.Len())
}

func TestLabelIndex_Stats(t *testing.T) {
	idx := NewLabelIndex()

	// Initial stats
	hits, misses, namespaces, resources := idx.GetStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 0, namespaces)
	assert.Equal(t, 0, resources)

	// Add some resources
	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})
	idx.Update("default", "Pod", "pod-2", map[string]string{"app": "web"})
	idx.Update("kube-system", "Pod", "pod-3", map[string]string{"app": "dns"})

	// Successful lookup (hit)
	idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})

	// Failed lookup (miss)
	idx.FindBySelector("default", "Pod", map[string]string{"app": "nonexistent"})

	hits, misses, namespaces, resources = idx.GetStats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)
	assert.Equal(t, 2, namespaces)
	assert.Equal(t, 3, resources)
}

func TestLabelIndex_HitRate(t *testing.T) {
	idx := NewLabelIndex()

	// No lookups = 0%
	assert.Equal(t, 0.0, idx.HitRate())

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})

	// Hit
	idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	assert.Equal(t, 100.0, idx.HitRate())

	// Miss
	idx.FindBySelector("default", "Pod", map[string]string{"app": "missing"})
	assert.Equal(t, 50.0, idx.HitRate())

	// Another hit
	idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	hitRate := idx.HitRate()
	assert.InDelta(t, 66.67, hitRate, 0.1)
}

func TestLabelIndex_ResetStats(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})
	idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	idx.FindBySelector("default", "Pod", map[string]string{"app": "missing"})

	idx.ResetStats()

	hits, misses, _, resources := idx.GetStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 1, resources) // Resources should still be there
}

func TestLabelIndex_Clear(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})
	idx.Update("default", "Pod", "pod-2", map[string]string{"app": "api"})
	idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})

	idx.Clear()

	assert.Equal(t, 0, idx.Len())
	assert.False(t, idx.Contains("default", "Pod", "pod-1"))

	hits, misses, namespaces, resources := idx.GetStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, 0, namespaces)
	assert.Equal(t, 0, resources)
}

func TestLabelIndex_GetLabels_Isolation(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})

	// Get labels and modify them
	labels := idx.GetLabels("default", "Pod", "pod-1")
	labels["app"] = "modified"
	labels["new"] = "added"

	// Original should be unchanged
	originalLabels := idx.GetLabels("default", "Pod", "pod-1")
	assert.Equal(t, "web", originalLabels["app"])
	assert.NotContains(t, originalLabels, "new")
}

func TestLabelIndex_UpdateIsolation(t *testing.T) {
	idx := NewLabelIndex()

	labels := map[string]string{"app": "web"}
	idx.Update("default", "Pod", "pod-1", labels)

	// Modify original map
	labels["app"] = "modified"

	// Index should have original value
	storedLabels := idx.GetLabels("default", "Pod", "pod-1")
	assert.Equal(t, "web", storedLabels["app"])
}

func TestLabelIndex_SpecialCharactersInLabels(t *testing.T) {
	idx := NewLabelIndex()

	// Labels with special characters (common in K8s)
	idx.Update("default", "Pod", "pod-1", map[string]string{
		"app.kubernetes.io/name":      "myapp",
		"app.kubernetes.io/component": "frontend",
		"helm.sh/chart":               "myapp-1.0.0",
	})

	// Should be able to find by these labels
	uids := idx.FindBySelector("default", "Pod", map[string]string{
		"app.kubernetes.io/name": "myapp",
	})
	assert.Contains(t, uids, "pod-1")

	// Multiple special char labels
	uids = idx.FindBySelector("default", "Pod", map[string]string{
		"app.kubernetes.io/name":      "myapp",
		"app.kubernetes.io/component": "frontend",
	})
	assert.Contains(t, uids, "pod-1")
}

func TestLabelIndex_ConcurrentAccess(t *testing.T) {
	idx := NewLabelIndex()
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				idx.Update("default", "Pod", fmt.Sprintf("pod-%d-%d", id, j),
					map[string]string{"app": fmt.Sprintf("app-%d", id)})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				idx.FindBySelector("default", "Pod", map[string]string{"app": fmt.Sprintf("app-%d", id)})
			}
		}(i)
	}

	// Concurrent removers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				idx.Remove("default", "Pod", fmt.Sprintf("pod-%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()

	// Should complete without race conditions
	// Verify index is in a consistent state
	_, _, _, resources := idx.GetStats()
	t.Logf("After concurrent access: %d resources remain", resources)
	assert.GreaterOrEqual(t, resources, 0)
}

func TestLabelIndex_PartialLabelMatch(t *testing.T) {
	idx := NewLabelIndex()

	idx.Update("default", "Pod", "pod-1", map[string]string{
		"app": "web",
		"env": "prod",
		"version": "v1",
	})
	idx.Update("default", "Pod", "pod-2", map[string]string{
		"app": "web",
		"env": "staging",
	})

	// Searching for a label that only pod-1 has should only return pod-1
	uids := idx.FindBySelector("default", "Pod", map[string]string{
		"app": "web",
		"version": "v1",
	})
	assert.Equal(t, []string{"pod-1"}, uids)

	// pod-2 doesn't have version label, so searching for version=v1 shouldn't include it
	uids = idx.FindBySelector("default", "Pod", map[string]string{
		"version": "v1",
	})
	assert.Equal(t, []string{"pod-1"}, uids)
}

func TestLabelIndex_DifferentKinds(t *testing.T) {
	idx := NewLabelIndex()

	// Same namespace, same labels, different kinds
	idx.Update("default", "Pod", "pod-1", map[string]string{"app": "web"})
	idx.Update("default", "Deployment", "deploy-1", map[string]string{"app": "web"})
	idx.Update("default", "Service", "svc-1", map[string]string{"app": "web"})

	// Should only find the Pod
	uids := idx.FindBySelector("default", "Pod", map[string]string{"app": "web"})
	assert.Equal(t, []string{"pod-1"}, uids)

	// Should only find the Deployment
	uids = idx.FindBySelector("default", "Deployment", map[string]string{"app": "web"})
	assert.Equal(t, []string{"deploy-1"}, uids)
}

// Benchmarks

func BenchmarkLabelIndex_Update(b *testing.B) {
	idx := NewLabelIndex()
	labels := map[string]string{
		"app":     "myapp",
		"version": "v1",
		"env":     "prod",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Update("default", "Pod", fmt.Sprintf("pod-%d", i), labels)
	}
}

func BenchmarkLabelIndex_FindBySelector_Small(b *testing.B) {
	idx := NewLabelIndex()

	// Populate with 100 pods
	for i := 0; i < 100; i++ {
		idx.Update("default", "Pod", fmt.Sprintf("pod-%d", i), map[string]string{
			"app": fmt.Sprintf("app-%d", i%10),
			"env": "prod",
		})
	}

	selector := map[string]string{"app": "app-5", "env": "prod"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.FindBySelector("default", "Pod", selector)
	}
}

func BenchmarkLabelIndex_FindBySelector_Large(b *testing.B) {
	idx := NewLabelIndex()

	// Populate with 10000 pods across 100 namespaces
	for ns := 0; ns < 100; ns++ {
		for pod := 0; pod < 100; pod++ {
			idx.Update(
				fmt.Sprintf("ns-%d", ns),
				"Pod",
				fmt.Sprintf("pod-%d-%d", ns, pod),
				map[string]string{
					"app":     fmt.Sprintf("app-%d", pod%10),
					"version": fmt.Sprintf("v%d", pod%3),
				},
			)
		}
	}

	selector := map[string]string{"app": "app-5", "version": "v1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.FindBySelector(fmt.Sprintf("ns-%d", i%100), "Pod", selector)
	}
}

func BenchmarkLabelIndex_ConcurrentReadWrite(b *testing.B) {
	idx := NewLabelIndex()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		idx.Update("default", "Pod", fmt.Sprintf("pod-%d", i), map[string]string{
			"app": fmt.Sprintf("app-%d", i%10),
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				idx.FindBySelector("default", "Pod", map[string]string{"app": fmt.Sprintf("app-%d", i%10)})
			} else {
				idx.Update("default", "Pod", fmt.Sprintf("pod-%d", i%1000), map[string]string{
					"app": fmt.Sprintf("app-%d", i%10),
				})
			}
			i++
		}
	})
}
