package namespacegraph

import (
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// MetadataProvider interface for getting namespace list from metadata cache
type MetadataProvider interface {
	Get() (*models.MetadataResponse, error)
}

// CacheInvalidator interface for event-driven cache invalidation
// Implemented by Cache to allow the NamespaceChangeDetector to mark namespaces as stale
type CacheInvalidator interface {
	// InvalidateNamespaces marks the given namespaces as stale
	// The cache will recompute these on next request
	InvalidateNamespaces(namespaces []string)

	// InvalidateAll marks all cached entries as stale
	// Used for cluster-scoped resource changes that may affect all namespaces
	InvalidateAll()
}

// CacheConfig contains configuration for the namespace graph cache
type CacheConfig struct {
	// RefreshTTL is how long cached entries are valid before periodic refresh kicks in
	// With event-driven invalidation, this can be much longer (default 5 minutes)
	// Event-driven invalidation will mark namespaces dirty when changes occur
	RefreshTTL time.Duration

	// MaxMemoryMB is the maximum memory to use for cache (default 256MB)
	MaxMemoryMB int64

	// PeriodicSyncPeriod is how often to sync with metadata for namespace discovery
	// This is independent of RefreshTTL and handles new/deleted namespace detection
	// Default: 5 minutes (same as RefreshTTL)
	PeriodicSyncPeriod time.Duration
}

// DefaultCacheConfig returns the default cache configuration
// With event-driven invalidation enabled, the default TTL is much longer
// since cache freshness is maintained by the NamespaceChangeDetector
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		RefreshTTL:         5 * time.Minute, // Was 20s - now event-driven handles freshness
		MaxMemoryMB:        256,
		PeriodicSyncPeriod: 5 * time.Minute,
	}
}

// CachedEntry holds a cached namespace graph response
type CachedEntry struct {
	Response  *NamespaceGraphResponse
	Size      int64
	ExpiresAt time.Time
	UpdatedAt time.Time
}

// Cache provides fast in-memory access to namespace graph responses
// It caches one "view" per namespace and refreshes in the background
// Implements CacheInvalidator for event-driven invalidation
type Cache struct {
	mu         sync.RWMutex
	cache      map[string]*CachedEntry
	dirtySet   map[string]bool // Namespaces marked for recomputation by event-driven invalidation
	usedMemory int64

	config        CacheConfig
	analyzer      *Analyzer
	metadataCache MetadataProvider // For discovering namespaces to pre-warm
	logger        *logging.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}
