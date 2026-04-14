package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/storage"
)

func TestImportHandler_JSONImportIncludesWarnings(t *testing.T) {
	st, err := storage.New(t.TempDir(), 10*1024*1024)
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}
	defer st.Close()

	handler := NewImportHandler(st, logging.GetLogger("import-handler-test"))

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

	req := httptest.NewRequest(http.MethodPost, "/v1/import", body)
	req.Header.Set("Content-Type", ContentTypeEventsJSON)
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

	if len(response.Warnings) == 0 {
		t.Fatalf("response warnings = %v, want at least one warning", response.Warnings)
	}

	if !strings.Contains(response.Warnings[0], "missing-payload") {
		t.Fatalf("response warnings = %v, want warning mentioning %q", response.Warnings, "missing-payload")
	}
}
