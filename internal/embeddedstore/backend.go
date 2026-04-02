package embeddedstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/models"
)

type Config struct {
	DataDir string
}

type Backend struct {
	journal    *Journal
	projection *Projection
	queryExec  api.QueryExecutor
	analysis   analysisstore.AnalysisStore
	ready      atomic.Bool
}

var (
	applyProjectionEventFnMu sync.RWMutex
	applyProjectionEventFn   = applyProjectionEventDirect
)

func Open(cfg Config) (*Backend, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("open embedded backend: data dir is empty")
	}

	journalRoot := filepath.Join(cfg.DataDir, "embedded")
	journal, err := OpenJournal(journalRoot)
	if err != nil {
		return nil, fmt.Errorf("open embedded backend: %w", err)
	}

	events, err := journal.Replay(context.Background())
	if err != nil {
		_ = journal.Close()
		return nil, fmt.Errorf("open embedded backend: replay journal: %w", err)
	}

	projection, err := BuildProjection(events)
	if err != nil {
		_ = journal.Close()
		return nil, fmt.Errorf("open embedded backend: rebuild projection: %w", err)
	}

	backend := &Backend{
		journal:    journal,
		projection: projection,
	}
	backend.queryExec = NewQueryExecutor(projection)
	backend.analysis = NewAnalysisStore(projection)
	backend.ready.Store(true)

	return backend, nil
}

func (b *Backend) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return fmt.Errorf("start embedded backend: backend is nil")
	}
	return nil
}

func (b *Backend) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return fmt.Errorf("stop embedded backend: backend is nil")
	}
	return b.Close()
}

func (b *Backend) Close() error {
	if b == nil || b.journal == nil {
		return nil
	}
	b.ready.Store(false)
	return b.journal.Close()
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
	if err := b.journal.AppendBatch(ctx, events); err != nil {
		return err
	}
	wasReady := b.ready.Load()
	for i := range events {
		if err := applyProjectionEvent(b.projection, events[i]); err != nil {
			b.ready.Store(false)
			return fmt.Errorf("process embedded batch: apply event %d: %w", i, err)
		}
	}
	if wasReady {
		b.ready.Store(true)
	}
	return nil
}

func (b *Backend) QueryExecutor() api.QueryExecutor {
	if b == nil {
		return nil
	}
	return b.queryExec
}

func (b *Backend) AnalysisStore() analysisstore.AnalysisStore {
	if b == nil {
		return nil
	}
	return b.analysis
}

func (b *Backend) IsReady() bool {
	return b != nil && b.ready.Load()
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
