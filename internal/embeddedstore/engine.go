package embeddedstore

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

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

	readers, checkpointHighWaterMark, projection, err := loadEngineState(rootDir, manifest)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: load state: %w", err)
	}

	if projection == nil {
		projection = NewProjection()
	}

	metrics := NewMetrics(cfg.MetricsRegisterer)
	hot := newHotStore(HotStoreConfig{MaxEvents: cfg.HotMaxEvents, MaxResourceVersions: cfg.HotMaxResourceVersions}, metrics)
	manifest, tail, err := openOrCreateActiveTail(rootDir, manifest)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: open tail journal: %w", err)
	}

	nextHighWaterMark := maxUint64(manifest.FlushHighWaterMark, maxSegmentHighWaterMark(manifest.ActiveSegments), checkpointHighWaterMark)
	if err := recoverTailState(projection, hot, tail, nextHighWaterMark); err != nil {
		_ = tail.Close()
		return nil, fmt.Errorf("open embedded engine: recover tail journal: %w", err)
	}
	nextHighWaterMark = maxUint64(nextHighWaterMark, tail.meta.LastHighWaterMark)
	manifest.ActiveTail = tail.meta

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
	engine.refreshQueryPlanner()
	engine.metrics.SetActiveSegments(len(engine.segmentReaders))
	engine.ready.Store(true)

	return engine, nil
}

func openOrCreateActiveTail(rootDir string, manifest Manifest) (Manifest, *tailJournal, error) {
	originalMeta := manifest.ActiveTail
	meta := originalMeta
	if meta.ID == "" {
		meta = TailJournalMeta{
			ID:                newTailJournalID(manifest.ActiveCheckpoint.HighWaterMark),
			BaseHighWaterMark: manifest.ActiveCheckpoint.HighWaterMark,
			LastHighWaterMark: manifest.ActiveCheckpoint.HighWaterMark,
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

func recoverTailState(projection *Projection, hot *hotStore, tail *tailJournal, afterHighWaterMark uint64) error {
	if projection == nil || hot == nil || tail == nil {
		return nil
	}

	return tail.ReplaySince(context.Background(), afterHighWaterMark, func(event models.Event, _ uint64) error {
		if err := projection.Apply(event); err != nil {
			return err
		}
		hot.Append([]models.Event{event})
		return nil
	})
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
