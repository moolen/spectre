package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
)

func TestParseTimestampForNamespaceGraph(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNanos int64
		wantErr   bool
	}{
		{
			name:      "unix seconds",
			input:     "1704067200",
			wantNanos: 1704067200000000000,
			wantErr:   false,
		},
		{
			name:      "unix milliseconds",
			input:     "1704067200000",
			wantNanos: 1704067200000000000,
			wantErr:   false,
		},
		{
			name:      "unix nanoseconds",
			input:     "1704067200000000000",
			wantNanos: 1704067200000000000,
			wantErr:   false,
		},
		{
			name:      "RFC3339",
			input:     "2024-01-01T00:00:00Z",
			wantNanos: 1704067200000000000,
			wantErr:   false,
		},
		{
			name:      "RFC3339Nano",
			input:     "2024-01-01T00:00:00.123456789Z",
			wantNanos: 1704067200123456789,
			wantErr:   false,
		},
		{
			name:    "invalid format",
			input:   "not-a-timestamp",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimestampForNamespaceGraph(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimestampForNamespaceGraph() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantNanos {
				t.Errorf("parseTimestampForNamespaceGraph() = %v, want %v", got, tt.wantNanos)
			}
		})
	}
}

func TestNamespaceGraphHandlerValidation(t *testing.T) {
	handler := &NamespaceGraphHandler{
		logger: nil,
	}

	tests := []struct {
		name           string
		query          string
		wantStatusCode int
	}{
		{
			name:           "missing namespace",
			query:          "?timestamp=1704067200",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "missing timestamp",
			query:          "?namespace=default",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "invalid timestamp format",
			query:          "?namespace=default&timestamp=invalid",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "namespace too long",
			query:          "?namespace=" + strings.Repeat("a", 100) + "&timestamp=1704067200",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/namespace-graph"+tt.query, http.NoBody)
			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("expected status %d, got %d", tt.wantStatusCode, w.Code)
			}

			var body map[string]map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if body["error"]["code"] != "INVALID_REQUEST" {
				t.Fatalf("expected INVALID_REQUEST, got %q", body["error"]["code"])
			}
		})
	}
}

func TestNamespaceGraphHandlerParseInput(t *testing.T) {
	handler := &NamespaceGraphHandler{}

	tests := []struct {
		name                   string
		query                  string
		wantNamespace          string
		wantIncludeAnomalies   bool
		wantIncludeCausalPaths bool
		wantLimit              int
		wantMaxDepth           int
		wantErr                bool
	}{
		{
			name:                   "all parameters",
			query:                  "?namespace=production&timestamp=1704067200&includeAnomalies=true&includeCausalPaths=true&limit=50&maxDepth=5&lookback=20m",
			wantNamespace:          "production",
			wantIncludeAnomalies:   true,
			wantIncludeCausalPaths: true,
			wantLimit:              50,
			wantMaxDepth:           5,
			wantErr:                false,
		},
		{
			name:                   "only required parameters",
			query:                  "?namespace=default&timestamp=1704067200",
			wantNamespace:          "default",
			wantIncludeAnomalies:   false,
			wantIncludeCausalPaths: false,
			wantLimit:              0,
			wantMaxDepth:           0,
			wantErr:                false,
		},
		{
			name:          "RFC3339 timestamp",
			query:         "?namespace=default&timestamp=2024-01-01T00:00:00Z",
			wantNamespace: "default",
			wantLimit:     0,
			wantMaxDepth:  0,
			wantErr:       false,
		},
		{
			name:    "missing namespace",
			query:   "?timestamp=1704067200",
			wantErr: true,
		},
		{
			name:    "missing timestamp",
			query:   "?namespace=default",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/namespace-graph"+tt.query, http.NoBody)

			input, err := handler.parseInput(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if input.Namespace != tt.wantNamespace {
				t.Errorf("Namespace = %q, want %q", input.Namespace, tt.wantNamespace)
			}
			if input.IncludeAnomalies != tt.wantIncludeAnomalies {
				t.Errorf("IncludeAnomalies = %v, want %v", input.IncludeAnomalies, tt.wantIncludeAnomalies)
			}
			if input.IncludeCausalPaths != tt.wantIncludeCausalPaths {
				t.Errorf("IncludeCausalPaths = %v, want %v", input.IncludeCausalPaths, tt.wantIncludeCausalPaths)
			}
			if input.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", input.Limit, tt.wantLimit)
			}
			if input.MaxDepth != tt.wantMaxDepth {
				t.Errorf("MaxDepth = %d, want %d", input.MaxDepth, tt.wantMaxDepth)
			}
		})
	}
}

func TestNamespaceGraphHandlerParseInput_PrepareAnalyzeInputAppliesDefaults(t *testing.T) {
	handler := &NamespaceGraphHandler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/namespace-graph?namespace=default&timestamp=1704067200", http.NoBody)

	input, err := handler.parseInput(req)
	if err != nil {
		t.Fatalf("parseInput returned error: %v", err)
	}

	prepared, err := namespacegraph.PrepareAnalyzeInput(input, time.Unix(1704067200, 0).UTC())
	if err != nil {
		t.Fatalf("PrepareAnalyzeInput returned error: %v", err)
	}

	if prepared.Limit != namespacegraph.DefaultLimit {
		t.Fatalf("expected default limit %d, got %d", namespacegraph.DefaultLimit, prepared.Limit)
	}
	if prepared.MaxDepth != namespacegraph.DefaultMaxDepth {
		t.Fatalf("expected default max depth %d, got %d", namespacegraph.DefaultMaxDepth, prepared.MaxDepth)
	}
	if prepared.Lookback != namespacegraph.DefaultLookback {
		t.Fatalf("expected default lookback %v, got %v", namespacegraph.DefaultLookback, prepared.Lookback)
	}
}
