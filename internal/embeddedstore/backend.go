package embeddedstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

type Config struct {
	DataDir string
}

type Backend struct {
	engine *Engine
}

var (
	applyProjectionEventFnMu sync.RWMutex
	applyProjectionEventFn   = applyProjectionEventDirect
)

func Open(cfg Config) (*Backend, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("open embedded backend: data dir is empty")
	}

	engine, err := OpenEngine(EngineConfig{DataDir: cfg.DataDir})
	if err != nil {
		return nil, fmt.Errorf("open embedded backend: %w", err)
	}
	return &Backend{engine: engine}, nil
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
