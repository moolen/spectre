package embeddedstore

import (
	"context"
	"fmt"
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
	closed                 bool
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

	e.queryExec.SetSharedCache(newQueryPlanner(e.projection, e.hot, e.segmentReaders))
}
