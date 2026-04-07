package api

import (
	"context"
	"testing"

	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
)

type testStreamingQueryExecutor struct{}

func (testStreamingQueryExecutor) Execute(context.Context, *models.QueryRequest) (*models.QueryResult, error) {
	return &models.QueryResult{}, nil
}

func (testStreamingQueryExecutor) SetSharedCache(interface{}) {}

func TestApplyEntryPagination_SortsAndSlicesByCursor(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewTimelineConnectService(testStreamingQueryExecutor{}, logger, tracer)

	entries := []*apptimeline.TimelineResourceEntry{
		{Kind: "Pod", Namespace: "zeta", Name: "pod-c"},
		{Kind: "Deployment", Namespace: "alpha", Name: "deploy-a"},
		{Kind: "Pod", Namespace: "alpha", Name: "pod-a"},
	}

	page1, page1Resp, err := service.applyEntryPagination(entries, &models.PaginationRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("expected no error paginating first page, got %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 entries on first page, got %d", len(page1))
	}
	if page1[0].Kind != "Deployment" || page1[0].Namespace != "alpha" || page1[0].Name != "deploy-a" {
		t.Fatalf("unexpected first entry on page 1: %#v", page1[0])
	}
	if page1[1].Kind != "Pod" || page1[1].Namespace != "alpha" || page1[1].Name != "pod-a" {
		t.Fatalf("unexpected second entry on page 1: %#v", page1[1])
	}
	if !page1Resp.HasMore {
		t.Fatal("expected first page to report more results")
	}

	page2, page2Resp, err := service.applyEntryPagination(entries, &models.PaginationRequest{
		PageSize: 2,
		Cursor:   page1Resp.NextCursor,
	})
	if err != nil {
		t.Fatalf("expected no error paginating second page, got %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 entry on second page, got %d", len(page2))
	}
	if page2[0].Kind != "Pod" || page2[0].Namespace != "zeta" || page2[0].Name != "pod-c" {
		t.Fatalf("unexpected entry on page 2: %#v", page2[0])
	}
	if page2Resp.HasMore {
		t.Fatal("expected second page to be terminal")
	}
}
