package api

import (
	"context"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// MetadataCache provides fast in-memory access to cluster metadata
// It periodically refreshes the data from the query executor in the background
type MetadataCache struct {
	executor      QueryExecutor
	logger        *logging.Logger
	refreshPeriod time.Duration

	mu       sync.RWMutex
	metadata *models.MetadataResponse
	lastErr  error

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewMetadataCache creates a new metadata cache
// refreshPeriod: how often to refresh the cache (e.g., 30 seconds)
func NewMetadataCache(executor QueryExecutor, logger *logging.Logger, refreshPeriod time.Duration) *MetadataCache {
	return &MetadataCache{
		executor:      executor,
		logger:        logger,
		refreshPeriod: refreshPeriod,
		stopCh:        make(chan struct{}),
	}
}

// Start initializes the cache and starts the background refresh loop
// It performs an initial synchronous load before returning
func (mc *MetadataCache) Start(ctx context.Context) error {
	mc.logger.Info("Starting metadata cache with refresh period: %v", mc.refreshPeriod)

	// Perform initial load synchronously
	if err := mc.refresh(ctx); err != nil {
		mc.logger.Error("Failed initial metadata cache load: %v", err)
		return err
	}

	mc.logger.Info("Metadata cache initialized successfully")

	// Start background refresh loop
	mc.wg.Add(1)
	go mc.refreshLoop()

	return nil
}

// Stop gracefully stops the background refresh loop
func (mc *MetadataCache) Stop() {
	mc.logger.Info("Stopping metadata cache")
	close(mc.stopCh)
	mc.wg.Wait()
	mc.logger.Info("Metadata cache stopped")
}

// Get returns the cached metadata
// If the cache is empty or has an error, it returns the error
func (mc *MetadataCache) Get() (*models.MetadataResponse, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.lastErr != nil {
		return nil, mc.lastErr
	}

	if mc.metadata == nil {
		return nil, ErrCacheNotReady
	}

	// Return a copy to prevent mutation
	result := &models.MetadataResponse{
		Namespaces: append([]string{}, mc.metadata.Namespaces...),
		Kinds:      append([]string{}, mc.metadata.Kinds...),
		TimeRange:  mc.metadata.TimeRange,
	}

	return result, nil
}

// refresh queries the executor and updates the cache
func (mc *MetadataCache) refresh(ctx context.Context) error {
	start := time.Now()

	// Query all metadata (no time filtering - get everything)
	// Use 0 for start and a far future time for end to get all data
	startTimeNs := int64(0)
	endTimeNs := time.Now().Add(24 * time.Hour).UnixNano()

	// Check if executor supports efficient metadata query
	metadataExecutor, ok := mc.executor.(interface {
		QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error)
	})

	if !ok {
		mc.mu.Lock()
		mc.lastErr = ErrMetadataQueryNotSupported
		mc.mu.Unlock()
		return ErrMetadataQueryNotSupported
	}

	namespaces, kinds, minTime, maxTime, err := metadataExecutor.QueryDistinctMetadata(ctx, startTimeNs, endTimeNs)
	if err != nil {
		mc.logger.Error("Failed to refresh metadata cache: %v", err)
		mc.mu.Lock()
		mc.lastErr = err
		mc.mu.Unlock()
		return err
	}

	elapsed := time.Since(start)

	// Update cache
	mc.mu.Lock()
	mc.metadata = &models.MetadataResponse{
		Namespaces: namespaces,
		Kinds:      kinds,
		TimeRange: models.TimeRangeInfo{
			Earliest: minTime / 1e9,
			Latest:   maxTime / 1e9,
		},
	}
	mc.lastErr = nil
	mc.mu.Unlock()

	mc.logger.DebugWithFields("Metadata cache refreshed",
		logging.Field("namespaces", len(namespaces)),
		logging.Field("kinds", len(kinds)),
		logging.Field("duration_ms", elapsed.Milliseconds()))

	return nil
}

// refreshLoop runs in the background and periodically refreshes the cache
func (mc *MetadataCache) refreshLoop() {
	defer mc.wg.Done()

	ticker := time.NewTicker(mc.refreshPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := mc.refresh(ctx); err != nil {
				mc.logger.Error("Background metadata cache refresh failed: %v", err)
			}
			cancel()

		case <-mc.stopCh:
			return
		}
	}
}

// Errors
var (
	ErrCacheNotReady             = &APIError{Code: "CACHE_NOT_READY", Message: "Metadata cache is not ready"}
	ErrMetadataQueryNotSupported = &APIError{Code: "NOT_SUPPORTED", Message: "Executor does not support metadata queries"}
)
