package embeddedstore

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	DataDir                string
	HotMaxEvents           int
	HotMaxResourceVersions int
	FlushInterval          time.Duration
	CheckpointInterval     time.Duration
	SegmentTargetBytes     int64
	CompactionMinSegments  int
	MetricsRegisterer      prometheus.Registerer
}

type Backend struct {
	engine *Engine
}

const (
	defaultHotMaxEvents           int           = 50000
	defaultHotMaxResourceVersions int           = 32
	defaultFlushInterval          time.Duration = 30 * time.Second
	defaultCheckpointInterval     time.Duration = 0
	defaultSegmentTargetBytes     int64         = 16 << 20
	defaultCompactionMinSegments  int           = 4
)

var (
	applyProjectionEventFnMu sync.RWMutex
	applyProjectionEventFn   = applyProjectionEventDirect
)

func Open(cfg Config) (*Backend, error) {
	engineCfg, err := cfg.EffectiveEngineConfig()
	if err != nil {
		return nil, fmt.Errorf("open embedded backend: %w", err)
	}
	engine, err := OpenEngine(engineCfg)
	if err != nil {
		return nil, fmt.Errorf("open embedded backend: %w", err)
	}
	return &Backend{engine: engine}, nil
}

func (cfg Config) EffectiveEngineConfig() (EngineConfig, error) {
	if cfg.DataDir == "" {
		return EngineConfig{}, fmt.Errorf("data dir is empty")
	}
	if cfg.HotMaxEvents < 0 {
		return EngineConfig{}, fmt.Errorf("hot max events must be positive")
	}
	if cfg.HotMaxResourceVersions < 0 {
		return EngineConfig{}, fmt.Errorf("hot max resource versions must be positive")
	}
	if cfg.FlushInterval < 0 {
		return EngineConfig{}, fmt.Errorf("flush interval must be positive")
	}
	if cfg.CheckpointInterval < 0 {
		return EngineConfig{}, fmt.Errorf("checkpoint interval must be positive")
	}
	if cfg.SegmentTargetBytes < 0 {
		return EngineConfig{}, fmt.Errorf("segment target bytes must be positive")
	}
	if cfg.CompactionMinSegments != 0 && cfg.CompactionMinSegments < 2 {
		return EngineConfig{}, fmt.Errorf("compaction min segments must be at least 2")
	}

	engineCfg := EngineConfig{
		DataDir:                cfg.DataDir,
		HotMaxEvents:           cfg.HotMaxEvents,
		HotMaxResourceVersions: cfg.HotMaxResourceVersions,
		FlushInterval:          cfg.FlushInterval,
		CheckpointInterval:     cfg.CheckpointInterval,
		SegmentTargetBytes:     cfg.SegmentTargetBytes,
		CompactionMinSegments:  cfg.CompactionMinSegments,
		MetricsRegisterer:      cfg.MetricsRegisterer,
	}
	if engineCfg.HotMaxEvents == 0 {
		engineCfg.HotMaxEvents = defaultHotMaxEvents
	}
	if engineCfg.HotMaxResourceVersions == 0 {
		engineCfg.HotMaxResourceVersions = defaultHotMaxResourceVersions
	}
	if engineCfg.FlushInterval == 0 {
		engineCfg.FlushInterval = defaultFlushInterval
	}
	if engineCfg.CheckpointInterval == 0 {
		engineCfg.CheckpointInterval = defaultCheckpointInterval
	}
	if engineCfg.SegmentTargetBytes == 0 {
		engineCfg.SegmentTargetBytes = defaultSegmentTargetBytes
	}
	if engineCfg.CompactionMinSegments == 0 {
		engineCfg.CompactionMinSegments = defaultCompactionMinSegments
	}

	return engineCfg, nil
}

func (b *Backend) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return fmt.Errorf("start embedded backend: backend is nil")
	}
	return b.engine.Start(ctx)
}

func (b *Backend) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return fmt.Errorf("stop embedded backend: backend is nil")
	}
	return b.engine.Stop(ctx)
}

func (b *Backend) Close() error {
	if b == nil || b.engine == nil {
		return nil
	}
	return b.engine.Close()
}

func (b *Backend) ProcessEvent(ctx context.Context, event models.Event) error {
	return b.ProcessBatch(ctx, []models.Event{event})
}

func (b *Backend) ProcessBatch(ctx context.Context, events []models.Event) error {
	if len(events) == 0 {
		return nil
	}
	if b == nil {
		return fmt.Errorf("process embedded batch: backend is nil")
	}
	return b.engine.ProcessBatch(ctx, events)
}

func (b *Backend) QueryExecutor() *QueryExecutor {
	if b == nil {
		return nil
	}
	return b.engine.QueryExecutor()
}

func (b *Backend) AnalysisStore() *Store {
	if b == nil {
		return nil
	}
	return b.engine.AnalysisStore()
}

func (b *Backend) IsReady() bool {
	return b != nil && b.engine != nil && b.engine.IsReady()
}

func (b *Backend) Name() string {
	return "embedded-backend"
}

func applyProjectionEvent(projection *Projection, event models.Event) error {
	applyProjectionEventFnMu.RLock()
	fn := applyProjectionEventFn
	applyProjectionEventFnMu.RUnlock()

	return fn(projection, event)
}

func applyProjectionEventDirect(projection *Projection, event models.Event) error {
	return projection.Apply(event)
}

func setApplyProjectionEventFnForTest(fn func(*Projection, models.Event) error) func() {
	applyProjectionEventFnMu.Lock()
	previous := applyProjectionEventFn
	applyProjectionEventFn = fn
	applyProjectionEventFnMu.Unlock()

	return func() {
		applyProjectionEventFnMu.Lock()
		applyProjectionEventFn = previous
		applyProjectionEventFnMu.Unlock()
	}
}

func applyProjectionEventUsesDefaultImplementation() bool {
	applyProjectionEventFnMu.RLock()
	fn := applyProjectionEventFn
	applyProjectionEventFnMu.RUnlock()

	return reflect.ValueOf(fn).Pointer() == reflect.ValueOf(applyProjectionEventDirect).Pointer()
}
