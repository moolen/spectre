package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type fakeBatchIngestor struct {
	batches [][]models.Event
}

func (f *fakeBatchIngestor) ProcessBatch(_ context.Context, events []models.Event) error {
	chunk := make([]models.Event, len(events))
	copy(chunk, events)
	f.batches = append(f.batches, chunk)
	return nil
}

func TestImportHandler_JSONImportIncludesWarnings(t *testing.T) {
	ingestor := &fakeBatchIngestor{}
	handler := NewImportHandler(ingestor, logging.GetLogger("import-handler-test"))

	body := strings.NewReader(`{
		"kind":"Event",
		"apiVersion":"audit.k8s.io/v1",
		"auditID":"audit-warning",
		"stage":"ResponseComplete",
		"stageTimestamp":"2024-01-02T03:04:05Z",
		"verb":"update",
		"objectRef":{
			"resource":"deployments",
			"namespace":"default",
			"name":"missing-payload",
			"apiGroup":"apps",
			"apiVersion":"v1"
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/storage/import", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Handle() status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var response struct {
		Status   string   `json:"status"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "success" {
		t.Fatalf("response status = %q, want %q", response.Status, "success")
	}
	if len(ingestor.batches) != 1 {
		t.Fatalf("expected one processed batch, got %d", len(ingestor.batches))
	}
	if len(ingestor.batches[0]) != 0 {
		t.Fatalf("expected warning-only import to process an empty batch, got %d events", len(ingestor.batches[0]))
	}
	if len(response.Warnings) == 0 {
		t.Fatalf("response warnings = %v, want at least one warning", response.Warnings)
	}
	if !strings.Contains(response.Warnings[0], "missing-payload") {
		t.Fatalf("response warnings = %v, want warning mentioning %q", response.Warnings, "missing-payload")
	}
}
