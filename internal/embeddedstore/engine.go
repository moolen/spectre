package embeddedstore

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/moolen/spectre/internal/logging"
)

type Engine struct {
	mu sync.Mutex

	logger            *logging.Logger
	config            EngineConfig
	metrics           *Metrics
	rootDir           string
	manifest          Manifest
	hot               *hotStore
	projection        *Projection
	queryExec         *QueryExecutor
	analysis          *Store
	segmentReaders    []*segmentReader
	nextHighWaterMark uint64
	ready             atomic.Bool
	periodicFlushStop func()
	periodicCheckpointStop func()
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

	readers, checkpointHighWaterMark, projection, err := loadEngineState(rootDir, manifest)
	if err != nil {
		return nil, fmt.Errorf("open embedded engine: load state: %w", err)
	}

	if projection == nil {
		projection = NewProjection()
	}

	metrics := NewMetrics(cfg.MetricsRegisterer)
	hot := newHotStore(HotStoreConfig{MaxEvents: cfg.HotMaxEvents, MaxResourceVersions: cfg.HotMaxResourceVersions}, metrics)

	engine := &Engine{
		logger:            logging.GetLogger("embedded.engine"),
		config:            cfg,
		metrics:           metrics,
		rootDir:           rootDir,
		manifest:          manifest,
		hot:               hot,
		projection:        projection,
		queryExec:         NewQueryExecutor(projection),
		analysis:          NewAnalysisStore(projection),
		segmentReaders:    readers,
		nextHighWaterMark: maxUint64(manifest.FlushHighWaterMark, maxSegmentHighWaterMark(manifest.ActiveSegments), checkpointHighWaterMark),
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
