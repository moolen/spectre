package embeddedstore

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type Engine struct {
	mu sync.Mutex

	logger                 *logging.Logger
	config                 EngineConfig
	metrics                *Metrics
	rootDir                string
	manifest               Manifest
	hot                    *hotStore
	tail                   *tailJournal
	projection             *Projection
	queryExec              *QueryExecutor
	analysis               *Store
	segmentReaders         []*segmentReader
	nextHighWaterMark      uint64
	ready                  atomic.Bool
	periodicFlushStop      func()
	periodicCheckpointStop func()
	backgroundTasks        sync.WaitGroup
	closed                 bool
}

var (
	defaultTimelineWarmupWindows                  = []time.Duration{time.Hour, 4 * time.Hour}
	defaultTimelineWarmupTimeout                  = 15 * time.Minute
	defaultAssociatedIndexWarmupHorizon           = 6 * time.Hour
	defaultAssociatedIndexWarmupTimeout           = 5 * time.Minute
	defaultAssociatedIndexWarmupMaxSegments       = 64
	defaultAssociatedIndexWarmupMaxBytes    int64 = 64 << 20
	defaultRecentEventTimelineCacheHorizon        = 4 * time.Hour
	defaultRecentEventTimelineCacheTimeout        = 5 * time.Minute
)

func OpenEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("open embedded engine: data dir is empty")
	}

	rootDir := embeddedRootDir(cfg.DataDir)
	manifest, err := loadOrCreateManifest(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: load manifest: %w", err)
	}
	needsSegmentIndexMigration := manifest.SegmentIndexGeneration < associatedIndexGeneration && len(manifest.ActiveSegments) > 0

	readers, tail, recoveredHighWaterMark, projection, recoveredHot, mode, replayedTailEvents, err := loadEngineState(rootDir, manifest)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: load state: %w", err)
	}

	if projection == nil {
		projection = NewProjection()
	}
	if cfg.ProjectionHistoryFallback {
		projection.EnableHistoricalEventRetention()
	}

	metrics := NewMetrics(cfg.MetricsRegisterer)
	hot := newHotStore(HotStoreConfig{MaxEvents: cfg.HotMaxEvents, MaxResourceVersions: cfg.HotMaxResourceVersions}, metrics)
	if recoveredHot != nil {
		hot.Append(recoveredHot.ExtractFlushBatch(0).Events)
	}

	nextHighWaterMark := maxUint64(manifest.FlushHighWaterMark, maxSegmentHighWaterMark(manifest.ActiveSegments), recoveredHighWaterMark)
	if tail == nil {
		manifest, tail, err = openOrCreateActiveTail(rootDir, manifest, nextHighWaterMark)
		if err != nil {
			return nil, fmt.Errorf("open embedded engine: open tail journal: %w", err)
		}
	} else {
		manifest.ActiveTail = tail.meta
	}

	if mode == startupModeRepair {
		replayedOnRepair, err := recoverTailState(projection, hot, tail, nextHighWaterMark)
		if err != nil {
			logging.GetLogger("embedded.engine").WarnWithFields(
				"embedded repair startup discarding inconsistent tail journal",
				logging.Field("tail_id", manifest.ActiveTail.ID),
				logging.Field("recovered_high_water_mark", nextHighWaterMark),
				logging.Field("error", err.Error()),
			)
			if tail != nil {
				updatedMeta, resetErr := tail.Reset(nextHighWaterMark)
				if resetErr != nil {
					_ = tail.Close()
					return nil, fmt.Errorf("open embedded engine: reset inconsistent tail journal: %w", resetErr)
				}
				manifest.ActiveTail = updatedMeta
				if err := storeManifest(rootDir, manifest); err != nil {
					_ = tail.Close()
					return nil, fmt.Errorf("open embedded engine: persist reset tail journal: %w", err)
				}
			} else {
				manifest.ActiveTail = TailJournalMeta{}
				manifest, tail, err = openOrCreateActiveTail(rootDir, manifest, nextHighWaterMark)
				if err != nil {
					return nil, fmt.Errorf("open embedded engine: recreate tail journal after repair fallback: %w", err)
				}
			}
		} else {
			replayedTailEvents += replayedOnRepair
		}
	}
	if tail != nil {
		nextHighWaterMark = maxUint64(nextHighWaterMark, tail.meta.LastHighWaterMark)
		manifest.ActiveTail = tail.meta
	}

	engine := &Engine{
		logger:            logging.GetLogger("embedded.engine"),
		config:            cfg,
		metrics:           metrics,
		rootDir:           rootDir,
		manifest:          manifest,
		hot:               hot,
		tail:              tail,
		projection:        projection,
		queryExec:         NewQueryExecutor(projection),
		analysis:          NewAnalysisStore(projection),
		segmentReaders:    readers,
		nextHighWaterMark: nextHighWaterMark,
	}
	if !cfg.ProjectionHistoryFallback {
		engine.queryExec.DisableProjectionHistoryFallback()
		engine.queryExec.ConfigureRecentEventTimelineCache(defaultRecentEventTimelineCacheHorizon)
	}
	engine.queryExec.SetMetrics(metrics)
	plannerStart := time.Now()
	engine.refreshQueryPlanner()
	engine.logger.DebugWithFields(
		"embedded startup planner refreshed",
		logging.Field("duration_ms", time.Since(plannerStart).Milliseconds()),
	)
	engine.metrics.RecordStartupMode(mode.String())
	engine.metrics.RecordTailReplay(replayedTailEvents)
	engine.metrics.SetActiveSegments(len(engine.segmentReaders))
	engine.setActiveTailMetrics()
	engine.logStartup(mode, replayedTailEvents)
	engine.ready.Store(true)
	engine.startBackgroundRecentEventTimelineCacheSeed()
	if needsSegmentIndexMigration {
		engine.startBackgroundSegmentIndexMigration()
	}
	engine.startBackgroundAssociatedIndexWarmup()

	return engine, nil
}

func openOrCreateActiveTail(rootDir string, manifest Manifest, baseHighWaterMark uint64) (Manifest, *tailJournal, error) {
	originalMeta := manifest.ActiveTail
	meta := originalMeta
	if meta.ID == "" {
		meta = TailJournalMeta{
			ID:                newTailJournalID(baseHighWaterMark),
			BaseHighWaterMark: baseHighWaterMark,
			LastHighWaterMark: baseHighWaterMark,
		}
	}

	tail, err := openTailJournal(rootDir, meta)
	if err != nil {
		return manifest, nil, err
	}

	manifest.ActiveTail = tail.meta
	if manifest.ActiveTail != originalMeta {
		if err := storeManifest(rootDir, manifest); err != nil {
			_ = tail.Close()
			return manifest, nil, err
		}
	}

	return manifest, tail, nil
}

func recoverTailState(projection *Projection, hot *hotStore, tail *tailJournal, afterHighWaterMark uint64) (int, error) {
	if projection == nil || hot == nil || tail == nil {
		return 0, nil
	}

	replayedEvents := 0
	recoveredHotEvents := make([]models.Event, 0)
	err := tail.ReplaySince(context.Background(), afterHighWaterMark, func(event models.Event, _ uint64) error {
		if err := recoverTailProjectionEvent(projection, event); err != nil {
			return err
		}
		recoveredHotEvents = append(recoveredHotEvents, event)
		replayedEvents++
		return nil
	})
	if err != nil {
		return 0, err
	}
	hot.Append(recoveredHotEvents)
	return replayedEvents, nil
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

func (e *Engine) setActiveTailMetrics() {
	if e == nil || e.metrics == nil {
		return
	}
	if e.tail != nil {
		e.metrics.SetActiveTail(e.tail.meta.EventCount, e.tail.meta.SizeBytes)
		return
	}
	e.metrics.SetActiveTail(e.manifest.ActiveTail.EventCount, e.manifest.ActiveTail.SizeBytes)
}

func (e *Engine) setActiveTailMetricsLocked() {
	if e == nil || e.metrics == nil {
		return
	}
	if e.tail != nil {
		e.metrics.SetActiveTail(e.tail.meta.EventCount, e.tail.meta.SizeBytes)
		return
	}
	e.metrics.SetActiveTail(e.manifest.ActiveTail.EventCount, e.manifest.ActiveTail.SizeBytes)
}

func (e *Engine) logStartup(mode startupMode, replayedTailEvents int) {
	if e == nil {
		return
	}
	if e.logger == nil {
		e.logger = logging.GetLogger("embedded.engine")
	}

	e.logger.InfoWithFields(
		"embedded engine opened",
		logging.Field("startup_mode", mode.String()),
		logging.Field("replayed_tail_events", replayedTailEvents),
		logging.Field("active_tail_events", e.manifest.ActiveTail.EventCount),
		logging.Field("active_tail_bytes", e.manifest.ActiveTail.SizeBytes),
		logging.Field("active_segments", len(e.segmentReaders)),
	)
}

func (e *Engine) refreshQueryPlanner() {
	if e == nil || e.queryExec == nil {
		return
	}

	if e.config.ProjectionHistoryFallback {
		e.queryExec.SetSharedCache((*QueryPlanner)(nil))
		return
	}

	previous := e.queryExec.sharedPlanner()
	e.queryExec.SetSharedCache(newQueryPlanner(e.projection, e.hot, e.segmentReaders, previous))
}

func (e *Engine) beginBackgroundTask() bool {
	if e == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return false
	}
	e.backgroundTasks.Add(1)
	return true
}

func (e *Engine) startBackgroundSegmentIndexMigration() {
	if !e.beginBackgroundTask() {
		return
	}

	go func() {
		defer e.backgroundTasks.Done()

		e.mu.Lock()
		if e.closed || e.manifest.SegmentIndexGeneration >= associatedIndexGeneration {
			e.mu.Unlock()
			return
		}
		snapshotManifest := e.manifest
		e.mu.Unlock()

		updatedManifest, updatedReaders, rewrittenCount, err := migrateSegmentIndexGeneration(e.rootDir, snapshotManifest, e.logger)
		if err != nil {
			e.logger.WarnWithFields(
				"embedded startup segment index migration failed; will retry on next startup",
				logging.Field("error", err.Error()),
				logging.Field("active_segments", len(snapshotManifest.ActiveSegments)),
			)
			return
		}

		e.mu.Lock()

		if e.closed || e.manifest.SegmentIndexGeneration >= associatedIndexGeneration {
			e.mu.Unlock()
			return
		}
		if !reflect.DeepEqual(e.manifest.ActiveSegments, snapshotManifest.ActiveSegments) {
			e.logger.WarnWithFields(
				"embedded startup segment index migration discarded stale rewrite after active segment change",
				logging.Field("snapshot_active_segments", len(snapshotManifest.ActiveSegments)),
				logging.Field("current_active_segments", len(e.manifest.ActiveSegments)),
			)
			e.mu.Unlock()
			return
		}

		mergedManifest := e.manifest
		mergedManifest.ActiveSegments = updatedManifest.ActiveSegments
		mergedManifest.SegmentIndexGeneration = updatedManifest.SegmentIndexGeneration
		if err := storeManifest(e.rootDir, mergedManifest); err != nil {
			e.logger.WarnWithFields(
				"embedded startup segment index migration failed to publish manifest; will retry on next startup",
				logging.Field("error", err.Error()),
				logging.Field("active_segments", len(snapshotManifest.ActiveSegments)),
			)
			e.mu.Unlock()
			return
		}

		e.manifest = mergedManifest
		if updatedReaders != nil {
			e.segmentReaders = updatedReaders
		}
		e.refreshQueryPlanner()
		e.metrics.SetActiveSegments(len(e.segmentReaders))
		e.logger.InfoWithFields(
			"embedded startup segment index migration published new planner state",
			logging.Field("rewritten_segments", rewrittenCount),
			logging.Field("active_segments", len(e.segmentReaders)),
		)
		e.mu.Unlock()
		e.startBackgroundAssociatedIndexWarmup()
	}()
}

func (e *Engine) startBackgroundAssociatedIndexWarmup() {
	if e == nil || e.queryExec == nil || !e.beginBackgroundTask() {
		return
	}

	go func() {
		defer e.backgroundTasks.Done()

		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), defaultAssociatedIndexWarmupTimeout)
		defer cancel()

		loadedSegments, err := e.queryExec.warmRecentAssociatedEventIndexes(
			ctx,
			start.UnixNano(),
			defaultAssociatedIndexWarmupHorizon,
			defaultAssociatedIndexWarmupMaxSegments,
			defaultAssociatedIndexWarmupMaxBytes,
		)
		if err != nil {
			if ctx.Err() != nil {
				e.logger.WarnWithFields(
					"embedded startup associated-index warmup timed out",
					logging.Field("error", err.Error()),
					logging.Field("duration_ms", time.Since(start).Milliseconds()),
				)
				return
			}
			e.logger.WarnWithFields(
				"embedded startup associated-index warmup failed",
				logging.Field("error", err.Error()),
				logging.Field("duration_ms", time.Since(start).Milliseconds()),
			)
			return
		}

		e.logger.InfoWithFields(
			"embedded startup associated-index warmup completed",
			logging.Field("loaded_segments", loadedSegments),
			logging.Field("duration_ms", time.Since(start).Milliseconds()),
		)
	}()
}

func (e *Engine) startBackgroundRecentEventTimelineCacheSeed() {
	if e == nil || e.queryExec == nil || e.config.ProjectionHistoryFallback ||
		defaultRecentEventTimelineCacheHorizon <= 0 || !e.beginBackgroundTask() {
		return
	}

	go func() {
		defer e.backgroundTasks.Done()

		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), defaultRecentEventTimelineCacheTimeout)
		defer cancel()

		if err := seedRecentEventTimelineCache(e.queryExec, ctx, start.UnixNano(), defaultRecentEventTimelineCacheHorizon); err != nil {
			if ctx.Err() != nil {
				e.logger.WarnWithFields(
					"embedded startup recent event timeline cache seed timed out",
					logging.Field("error", err.Error()),
					logging.Field("duration_ms", time.Since(start).Milliseconds()),
				)
				return
			}
			e.logger.WarnWithFields(
				"embedded startup recent event timeline cache seed failed",
				logging.Field("error", err.Error()),
				logging.Field("duration_ms", time.Since(start).Milliseconds()),
			)
			return
		}

		e.queryExec.recentEventCacheMu.RLock()
		cachedEvents := len(e.queryExec.recentEventCache)
		e.queryExec.recentEventCacheMu.RUnlock()
		e.logger.InfoWithFields(
			"embedded startup recent event timeline cache seeded",
			logging.Field("horizon", defaultRecentEventTimelineCacheHorizon.String()),
			logging.Field("events", cachedEvents),
			logging.Field("duration_ms", time.Since(start).Milliseconds()),
		)
	}()
}

func (e *Engine) startBackgroundTimelineWarmup() {
	if e == nil || e.queryExec == nil || e.config.ProjectionHistoryFallback || !e.beginBackgroundTask() {
		return
	}

	go func() {
		defer e.backgroundTasks.Done()

		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()

		start := time.Now()
		endTimeNs := start.UnixNano()
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimelineWarmupTimeout)
		defer cancel()

		if err := e.queryExec.warmTimelineCaches(ctx, endTimeNs, defaultTimelineWarmupWindows, models.DefaultPageSize); err != nil {
			if ctx.Err() != nil {
				e.logger.WarnWithFields(
					"embedded startup timeline warmup timed out",
					logging.Field("error", err.Error()),
					logging.Field("duration_ms", time.Since(start).Milliseconds()),
				)
				return
			}
			e.logger.WarnWithFields(
				"embedded startup timeline warmup failed",
				logging.Field("error", err.Error()),
				logging.Field("duration_ms", time.Since(start).Milliseconds()),
			)
			return
		}

		e.logger.InfoWithFields(
			"embedded startup timeline warmup completed",
			logging.Field("windows", len(defaultTimelineWarmupWindows)),
			logging.Field("duration_ms", time.Since(start).Milliseconds()),
		)
	}()
}
