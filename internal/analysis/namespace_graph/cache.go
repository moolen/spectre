package namespacegraph

import (
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// CacheInvalidator interface for event-driven cache invalidation
type CacheInvalidator interface {
	// InvalidateNamespaces drops the cached responses of the given namespaces
	InvalidateNamespaces(namespaces []string)

	// InvalidateAll drops every cached response
	InvalidateAll()
}

// CacheConfig contains configuration for the namespace graph cache
type CacheConfig struct {
	// RefreshTTL is how long a cached response is served. Requests for "now" are
	// bucketed to TimestampBucketSize, so a live view is at most this much older than
	// a freshly computed one.
	RefreshTTL time.Duration

	// MaxMemoryMB is the maximum memory to use for cache (default 256MB)
	MaxMemoryMB int64
}

// DefaultCacheConfig returns the default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		RefreshTTL:  2 * time.Minute,
		MaxMemoryMB: 256,
	}
}

// cacheKey identifies a request. Inputs are normalized by PrepareAnalyzeInput first, so
// requests for "now" within one timestamp bucket share a key.
type cacheKey struct {
	namespace          string
	timestamp          int64
	includeAnomalies   bool
	includeCausalPaths bool
	lookback           time.Duration
	maxDepth           int
	limit              int
	cursor             string
}

func newCacheKey(input AnalyzeInput) cacheKey {
	return cacheKey{
		namespace:          input.Namespace,
		timestamp:          input.Timestamp,
		includeAnomalies:   input.IncludeAnomalies,
		includeCausalPaths: input.IncludeCausalPaths,
		lookback:           input.Lookback,
		maxDepth:           input.MaxDepth,
		limit:              input.Limit,
		cursor:             input.Cursor,
	}
}

// CachedEntry holds a cached namespace graph response
type CachedEntry struct {
	Response  *NamespaceGraphResponse
	Size      int64
	ExpiresAt time.Time
	UpdatedAt time.Time
}

// Cache keeps recent namespace graph responses, keyed by the full request, so repeated
// loads of the same view (page reloads, several users) are computed once per TTL.
// Implements CacheInvalidator for event-driven invalidation.
type Cache struct {
	mu         sync.RWMutex
	cache      map[cacheKey]*CachedEntry
	usedMemory int64

	config   CacheConfig
	analyzer *Analyzer
	logger   *logging.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}
