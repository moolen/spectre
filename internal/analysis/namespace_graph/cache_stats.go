package namespacegraph

import "time"

// GetStats returns cache statistics.
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		EntryCount:     len(c.cache),
		UsedMemoryMB:   c.usedMemory / (1024 * 1024),
		MaxMemoryMB:    c.config.MaxMemoryMB,
		RefreshTTLSecs: int64(c.config.RefreshTTL.Seconds()),
	}

	for ns, entry := range c.cache {
		stats.Entries = append(stats.Entries, CacheEntryStats{
			Namespace: ns,
			NodeCount: entry.Response.Metadata.NodeCount,
			EdgeCount: entry.Response.Metadata.EdgeCount,
			SizeKB:    entry.Size / 1024,
			Age:       time.Since(entry.UpdatedAt).Milliseconds(),
			ExpiresIn: time.Until(entry.ExpiresAt).Milliseconds(),
		})
	}

	return stats
}

// CacheStats contains cache statistics
type CacheStats struct {
	EntryCount     int               `json:"entryCount"`
	UsedMemoryMB   int64             `json:"usedMemoryMB"`
	MaxMemoryMB    int64             `json:"maxMemoryMB"`
	RefreshTTLSecs int64             `json:"refreshTTLSecs"`
	Entries        []CacheEntryStats `json:"entries,omitempty"`
}

// CacheEntryStats contains statistics for a single cache entry
type CacheEntryStats struct {
	Namespace string `json:"namespace"`
	NodeCount int    `json:"nodeCount"`
	EdgeCount int    `json:"edgeCount"`
	SizeKB    int64  `json:"sizeKB"`
	Age       int64  `json:"ageMs"`
	ExpiresIn int64  `json:"expiresInMs"`
}

// estimateResponseSize estimates the memory size of a response.
func estimateResponseSize(response *NamespaceGraphResponse) int64 {
	if response == nil {
		return 0
	}

	size := int64(1024)
	size += int64(len(response.Graph.Nodes)) * 500
	size += int64(len(response.Graph.Edges)) * 100
	size += int64(len(response.Anomalies)) * 200
	size += int64(len(response.CausalPaths)) * 500

	return size
}
