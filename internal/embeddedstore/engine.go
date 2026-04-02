package embeddedstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type Engine struct {
	mu sync.Mutex

	logger            *logging.Logger
	config            EngineConfig
	rootDir           string
	manifest          Manifest
	hot               *hotStore
	projection        *Projection
	queryExec         *QueryExecutor
	analysis          *Store
	segmentReaders    []*segmentReader
	nextHighWaterMark uint64
	ready             atomic.Bool
	periodicFlushStop context.CancelFunc
	closed            bool
}

func OpenEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("open embedded engine: data dir is empty")
	}

	rootDir := embeddedRootDir(cfg.DataDir)
	manifest, err := loadOrCreateManifest(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: load manifest: %w", err)
	}

	readers, replayEvents, checkpointHighWaterMark, projection, err := loadEngineState(rootDir, manifest)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: load state: %w", err)
	}

	if projection == nil {
		projection = NewProjection()
	}
	if len(replayEvents) > 0 {
		sort.SliceStable(replayEvents, func(i, j int) bool {
			return compareEventOrder(replayEvents[i], replayEvents[j]) < 0
		})
		for i := range replayEvents {
			if err := applyProjectionEvent(projection, replayEvents[i]); err != nil {
				return nil, fmt.Errorf("open embedded engine: replay event %d: %w", i, err)
			}
		}
	}

	engine := &Engine{
		logger:            logging.GetLogger("embedded.engine"),
		config:            cfg,
		rootDir:           rootDir,
		manifest:          manifest,
		hot:               newHotStore(HotStoreConfig{MaxEvents: cfg.HotMaxEvents, MaxResourceVersions: cfg.HotMaxResourceVersions}),
		projection:        projection,
		queryExec:         NewQueryExecutor(projection),
		analysis:          NewAnalysisStore(projection),
		segmentReaders:    readers,
		nextHighWaterMark: maxUint64(manifest.FlushHighWaterMark, maxSegmentHighWaterMark(manifest.ActiveSegments), checkpointHighWaterMark),
	}
	engine.queryExec.SetSharedCache(newQueryPlanner(engine.projection, engine.hot, engine.segmentReaders))
	engine.ready.Store(true)

	return engine, nil
}

func (e *Engine) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if e == nil {
		return fmt.Errorf("start embedded engine: engine is nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("start embedded engine: engine is closed")
	}
	if e.config.FlushInterval <= 0 || e.periodicFlushStop != nil {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	e.periodicFlushStop = cancel
	interval := e.config.FlushInterval
	go e.runPeriodicFlush(runCtx, interval)

	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e == nil {
		return fmt.Errorf("stop embedded engine: engine is nil")
	}
	return e.Close()
}

func (e *Engine) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	if e.periodicFlushStop != nil {
		e.periodicFlushStop()
		e.periodicFlushStop = nil
	}
	e.mu.Unlock()

	if err := e.Flush(context.Background()); err != nil {
		return err
	}
	if err := e.Checkpoint(context.Background()); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	e.ready.Store(false)
	return nil
}

func (e *Engine) ProcessEvent(ctx context.Context, event models.Event) error {
	return e.ProcessBatch(ctx, []models.Event{event})
}

func (e *Engine) ProcessBatch(ctx context.Context, events []models.Event) error {
	if len(events) == 0 {
		return nil
	}
	if e == nil {
		return fmt.Errorf("process embedded batch: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("process embedded batch: engine is closed")
	}

	wasReady := e.ready.Load()
	for i := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := applyProjectionEvent(e.projection, events[i]); err != nil {
			e.ready.Store(false)
			return fmt.Errorf("process embedded batch: apply event %d: %w", i, err)
		}
		e.hot.Append([]models.Event{events[i]})
		e.nextHighWaterMark++
	}
	if wasReady {
		e.ready.Store(true)
	}
	if e.shouldFlushHotBySizeLocked() {
		if err := e.flushLocked(ctx); err != nil {
			e.logAutoFlushError("size-triggered flush failed", err)
		}
	}

	return nil
}

func (e *Engine) Flush(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("flush embedded engine: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}

	return e.flushLocked(ctx)
}

func (e *Engine) runPeriodicFlush(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Flush(context.Background()); err != nil {
				e.logAutoFlushError("periodic flush failed", err)
			}
		}
	}
}

func (e *Engine) flushLocked(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	batch := e.hot.ExtractFlushBatch(0)
	if len(batch.Events) == 0 {
		return nil
	}

	segmentID := newSegmentID(e.nextHighWaterMark)
	meta, err := writeSegment(e.rootDir, segmentID, batch.Events)
	if err != nil {
		return fmt.Errorf("flush embedded engine: write segment: %w", err)
	}

	reader, err := openSegmentReader(e.rootDir, meta)
	if err != nil {
		return fmt.Errorf("flush embedded engine: open segment reader: %w", err)
	}

	updatedManifest := e.manifest
	updatedManifest.ActiveSegments = append(updatedManifest.ActiveSegments, SegmentMeta{
		ID:            meta.ID,
		HighWaterMark: e.nextHighWaterMark,
	})
	updatedManifest.FlushHighWaterMark = e.nextHighWaterMark
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("flush embedded engine: store manifest: %w", err)
	}

	removed := e.hot.CommitFlushedBatch(batch)
	if removed != len(batch.Events) {
		return fmt.Errorf("flush embedded engine: removed %d of %d flushed events", removed, len(batch.Events))
	}

	e.manifest = updatedManifest
	e.segmentReaders = append(e.segmentReaders, reader)
	e.queryExec.SetSharedCache(newQueryPlanner(e.projection, e.hot, e.segmentReaders))
	return nil
}

func (e *Engine) shouldFlushHotBySizeLocked() bool {
	if e.config.SegmentTargetBytes <= 0 {
		return false
	}
	batch := e.hot.ExtractFlushBatch(0)
	if len(batch.Events) == 0 {
		return false
	}
	return estimatedEventsSizeBytes(batch.Events) >= e.config.SegmentTargetBytes
}

func estimatedEventsSizeBytes(events []models.Event) int64 {
	var total int64
	for i := range events {
		// Estimate serialized footprint; exact framing size is unnecessary for threshold triggering.
		total += int64(len(events[i].ID) + len(events[i].Type))
		total += int64(len(events[i].Resource.UID) + len(events[i].Resource.Namespace) + len(events[i].Resource.Kind))
		total += int64(len(events[i].Resource.Name) + len(events[i].Resource.Group) + len(events[i].Resource.Version))
		total += int64(len(events[i].Data))
		total += 32
	}
	return total
}

func (e *Engine) logAutoFlushError(message string, err error) {
	if e == nil || err == nil {
		return
	}
	if e.logger == nil {
		e.logger = logging.GetLogger("embedded.engine")
	}
	e.logger.ErrorWithErr(message, err)
}

func (e *Engine) Checkpoint(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("checkpoint embedded engine: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}

	meta, err := writeCheckpoint(e.rootDir, e.projection, e.nextHighWaterMark)
	if err != nil {
		return fmt.Errorf("checkpoint embedded engine: write checkpoint: %w", err)
	}

	updatedManifest := e.manifest
	updatedManifest.Checkpoints = append(updatedManifest.Checkpoints, meta)
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("checkpoint embedded engine: store manifest: %w", err)
	}

	e.manifest = updatedManifest
	return nil
}

func (e *Engine) QueryExecutor() *QueryExecutor {
	if e == nil {
		return nil
	}
	return e.queryExec
}

func (e *Engine) AnalysisStore() *Store {
	if e == nil {
		return nil
	}
	return e.analysis
}

func (e *Engine) IsReady() bool {
	return e != nil && e.ready.Load()
}

func embeddedRootDir(dataDir string) string {
	return filepath.Join(dataDir, "embedded")
}

func loadEngineState(rootDir string, manifest Manifest) ([]*segmentReader, []models.Event, uint64, *Projection, error) {
	var checkpointProjection *Projection
	var checkpointHighWaterMark uint64
	if len(manifest.Checkpoints) > 0 {
		latestCheckpoint := latestCheckpointMeta(manifest.Checkpoints)
		var err error
		checkpointProjection, checkpointHighWaterMark, err = loadCheckpoint(rootDir, latestCheckpoint)
		if err != nil {
			return nil, nil, 0, nil, err
		}
	}

	readers := make([]*segmentReader, 0, len(manifest.ActiveSegments))
	replayEvents := make([]models.Event, 0)
	for i := range manifest.ActiveSegments {
		segmentMeta := manifest.ActiveSegments[i]
		reader, err := openSegmentReader(rootDir, segmentBundleMeta{ID: segmentMeta.ID})
		if err != nil {
			return nil, nil, 0, nil, fmt.Errorf("open active segment %q: %w", segmentMeta.ID, err)
		}
		readers = append(readers, reader)

		needsReplay := checkpointProjection == nil || segmentMeta.HighWaterMark > checkpointHighWaterMark
		if !needsReplay || reader.meta.EventCount == 0 {
			continue
		}

		events, err := reader.ScanTimeRange(context.Background(), reader.meta.MinTimestamp, reader.meta.MaxTimestamp)
		if err != nil {
			return nil, nil, 0, nil, fmt.Errorf("scan active segment %q: %w", segmentMeta.ID, err)
		}
		replayEvents = append(replayEvents, events...)
	}

	return readers, replayEvents, checkpointHighWaterMark, checkpointProjection, nil
}

func latestCheckpointMeta(checkpoints []CheckpointMeta) CheckpointMeta {
	latest := checkpoints[0]
	for i := 1; i < len(checkpoints); i++ {
		if checkpoints[i].HighWaterMark >= latest.HighWaterMark {
			latest = checkpoints[i]
		}
	}
	return latest
}

func maxSegmentHighWaterMark(segments []SegmentMeta) uint64 {
	var maxValue uint64
	for i := range segments {
		if segments[i].HighWaterMark > maxValue {
			maxValue = segments[i].HighWaterMark
		}
	}
	return maxValue
}

func maxUint64(values ...uint64) uint64 {
	var maxValue uint64
	for i := range values {
		if values[i] > maxValue {
			maxValue = values[i]
		}
	}
	return maxValue
}

func newSegmentID(highWaterMark uint64) string {
	return fmt.Sprintf("seg-%020d-%d", highWaterMark, time.Now().UTC().UnixNano())
}
