package api

import (
	"context"
	"fmt"
	"testing"

	analysisembedded "github.com/moolen/spectre/internal/analysis/store/embedded"
	spectreapi "github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/logging"
)

type goldenHarness interface {
	AnomalyHandler() *handlers.AnomalyHandler
	CausalPathsHandler() *handlers.CausalPathsHandler
	Cleanup(context.Context) error
}

type goldenHarnessFactory func(t *testing.T, fixtureFile string) (goldenHarness, error)

type graphGoldenHarness struct {
	*TestHarness
}

func newGraphGoldenHarness(t *testing.T, fixtureFile string) (goldenHarness, error) {
	t.Helper()

	harness, err := NewTestHarness(t)
	if err != nil {
		return nil, err
	}
	if err := harness.SeedEventsFromAuditLog(context.Background(), fixtureFile); err != nil {
		_ = harness.Cleanup(context.Background())
		return nil, fmt.Errorf("failed to seed events from fixture: %w", err)
	}
	return &graphGoldenHarness{TestHarness: harness}, nil
}

func (h *graphGoldenHarness) AnomalyHandler() *handlers.AnomalyHandler {
	return handlers.NewAnomalyHandler(h.GetGraphService(), logging.GetLogger("test"), nil)
}

func (h *graphGoldenHarness) CausalPathsHandler() *handlers.CausalPathsHandler {
	return handlers.NewCausalPathsHandler(h.GetGraphService(), logging.GetLogger("test"), nil)
}

type embeddedGoldenHarness struct {
	graphService *spectreapi.GraphService
}

func newEmbeddedGoldenHarness(t *testing.T, fixtureFile string) (goldenHarness, error) {
	t.Helper()

	events, err := LoadAuditLog(fixtureFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load fixture events: %w", err)
	}

	analysisStore, err := analysisembedded.New(events)
	if err != nil {
		return nil, fmt.Errorf("failed to build embedded analysis store: %w", err)
	}

	return &embeddedGoldenHarness{
		graphService: spectreapi.NewGraphService(analysisStore, logging.GetLogger("test"), nil),
	}, nil
}

func (h *embeddedGoldenHarness) AnomalyHandler() *handlers.AnomalyHandler {
	return handlers.NewAnomalyHandler(h.graphService, logging.GetLogger("test"), nil)
}

func (h *embeddedGoldenHarness) CausalPathsHandler() *handlers.CausalPathsHandler {
	return handlers.NewCausalPathsHandler(h.graphService, logging.GetLogger("test"), nil)
}

func (h *embeddedGoldenHarness) Cleanup(context.Context) error {
	return nil
}
