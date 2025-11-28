package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// mockQueryExecutor is a mock implementation of QueryExecutor for testing
type mockQueryExecutor struct {
	executeFunc func(*models.QueryRequest) (*models.QueryResult, error)
}

func (m *mockQueryExecutor) Execute(q *models.QueryRequest) (*models.QueryResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(q)
	}
	return &models.QueryResult{
		Events:          []models.Event{},
		Count:           0,
		ExecutionTimeMs: 0,
	}, nil
}

// mockReadinessChecker is a mock implementation of ReadinessChecker
type mockReadinessChecker struct {
	ready bool
}

func (m *mockReadinessChecker) IsReady() bool {
	return m.ready
}

// Helper functions for creating test events
func createTestEvent(id, kind, namespace, name string, timestamp int64) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
			UID:       "uid-" + id,
		},
		Data:     json.RawMessage(`{"kind":"` + kind + `"}`),
		DataSize: 100,
	}
}

func createTestEventWithGroup(id, group, kind, namespace, name string, timestamp int64) models.Event {
	evt := createTestEvent(id, kind, namespace, name, timestamp)
	evt.Resource.Group = group
	return evt
}

// Test SearchHandler

func TestSearchHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		queryParams url.Values
		mockExecute func(*models.QueryRequest) (*models.QueryResult, error)
		wantStatus  int
		wantErrCode string
		validate    func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "valid search with no filters",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
				"end":   {"2000"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if q.StartTimestamp != 1000 || q.EndTimestamp != 2000 {
					t.Errorf("unexpected query: start=%d, end=%d", q.StartTimestamp, q.EndTimestamp)
				}
				return &models.QueryResult{
					Events: []models.Event{
						createTestEvent("1", "Pod", "default", "test-pod", 1500000000000),
					},
					Count:           1,
					ExecutionTimeMs: 10,
				}, nil
			},
			wantStatus: http.StatusOK,
			validate: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp models.SearchResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp.Count != 1 {
					t.Errorf("expected count=1, got %d", resp.Count)
				}
				if len(resp.Resources) != 1 {
					t.Errorf("expected 1 resource, got %d", len(resp.Resources))
				}
			},
		},
		{
			name:   "valid search with filters",
			method: http.MethodGet,
			queryParams: url.Values{
				"start":     {"1000"},
				"end":       {"2000"},
				"kind":      {"Pod"},
				"namespace": {"default"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if q.Filters.Kind != "Pod" || q.Filters.Namespace != "default" {
					t.Errorf("filters not set correctly: kind=%s, namespace=%s", q.Filters.Kind, q.Filters.Namespace)
				}
				return &models.QueryResult{
					Events:          []models.Event{},
					Count:           0,
					ExecutionTimeMs: 5,
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "missing start timestamp",
			method: http.MethodGet,
			queryParams: url.Values{
				"end": {"2000"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "missing end timestamp",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "invalid timestamp format",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"not-a-number"},
				"end":   {"2000"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "start greater than end",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"2000"},
				"end":   {"1000"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "negative timestamp",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"-1"},
				"end":   {"2000"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "executor error",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
				"end":   {"2000"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				return nil, &ValidationError{message: "executor failed"}
			},
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
		{
			name:   "invalid namespace format",
			method: http.MethodGet,
			queryParams: url.Values{
				"start":     {"1000"},
				"end":       {"2000"},
				"namespace": {"invalid_namespace_!"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "namespace too long",
			method: http.MethodGet,
			queryParams: url.Values{
				"start":     {"1000"},
				"end":       {"2000"},
				"namespace": {strings.Repeat("a", 64)},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mockQueryExecutor{executeFunc: tt.mockExecute}
			logger := logging.GetLogger("test")
			handler := NewSearchHandler(mockExecutor, logger)

			req := httptest.NewRequest(tt.method, "/v1/search?"+tt.queryParams.Encode(), nil)
			rr := httptest.NewRecorder()

			handler.Handle(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if tt.wantErrCode != "" {
				var errResp ErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to unmarshal error response: %v", err)
				}
				if errResp.Error != tt.wantErrCode {
					t.Errorf("expected error code %s, got %s", tt.wantErrCode, errResp.Error)
				}
			}

			if tt.validate != nil {
				tt.validate(t, rr)
			}
		})
	}
}

// Test TimelineHandler

func TestTimelineHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		queryParams url.Values
		mockExecute func(*models.QueryRequest) (*models.QueryResult, error)
		wantStatus  int
		wantErrCode string
		validate    func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "valid timeline request",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
				"end":   {"2000"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				// First call is for resources
				if q.Filters.Kind == "" {
					return &models.QueryResult{
						Events: []models.Event{
							createTestEvent("1", "Pod", "default", "test-pod", 1500000000000),
						},
						Count:           1,
						ExecutionTimeMs: 10,
					}, nil
				}
				// Second call is for K8s Events
				return &models.QueryResult{
					Events:          []models.Event{},
					Count:           0,
					ExecutionTimeMs: 5,
				}, nil
			},
			wantStatus: http.StatusOK,
			validate: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp models.SearchResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp.Count != 1 {
					t.Errorf("expected count=1, got %d", resp.Count)
				}
			},
		},
		{
			name:   "K8s events attachment",
			method: http.MethodGet,
			queryParams: url.Values{
				"start":     {"1000"},
				"end":       {"2000"},
				"namespace": {"default"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if q.Filters.Kind == "Event" {
					// K8s Events query
					return &models.QueryResult{
						Events: []models.Event{
							createTestEvent("event-1", "Event", "default", "test-event", 1500000000000),
						},
						Count:           1,
						ExecutionTimeMs: 5,
					}, nil
				}
				// Resources query
				return &models.QueryResult{
					Events: []models.Event{
						createTestEvent("1", "Pod", "default", "test-pod", 1500000000000),
					},
					Count:           1,
					ExecutionTimeMs: 10,
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "K8s events query error is handled gracefully",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
				"end":   {"2000"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if q.Filters.Kind == "Event" {
					// Return error for K8s events query
					return nil, &ValidationError{message: "events query failed"}
				}
				// Resources query succeeds
				return &models.QueryResult{
					Events: []models.Event{
						createTestEvent("1", "Pod", "default", "test-pod", 1500000000000),
					},
					Count:           1,
					ExecutionTimeMs: 10,
				}, nil
			},
			wantStatus: http.StatusOK, // Should still succeed, just without K8s events
		},
		{
			name:   "invalid request parameters",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"invalid"},
				"end":   {"2000"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "executor error for resources",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
				"end":   {"2000"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if q.Filters.Kind == "" {
					return nil, &ValidationError{message: "executor failed"}
				}
				return &models.QueryResult{}, nil
			},
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mockQueryExecutor{executeFunc: tt.mockExecute}
			logger := logging.GetLogger("test")
			handler := NewTimelineHandler(mockExecutor, logger)

			req := httptest.NewRequest(tt.method, "/v1/timeline?"+tt.queryParams.Encode(), nil)
			rr := httptest.NewRecorder()

			handler.Handle(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if tt.wantErrCode != "" {
				var errResp ErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to unmarshal error response: %v", err)
				}
				if errResp.Error != tt.wantErrCode {
					t.Errorf("expected error code %s, got %s", tt.wantErrCode, errResp.Error)
				}
			}

			if tt.validate != nil {
				tt.validate(t, rr)
			}
		})
	}
}

// Test MetadataHandler

func TestMetadataHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		queryParams url.Values
		mockExecute func(*models.QueryRequest) (*models.QueryResult, error)
		wantStatus  int
		wantErrCode string
		validate    func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "valid metadata request",
			method:      http.MethodGet,
			queryParams: url.Values{},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				return &models.QueryResult{
					Events: []models.Event{
						createTestEvent("1", "Pod", "default", "pod1", 1500000000000),
						createTestEventWithGroup("2", "apps", "Deployment", "default", "deploy1", 1600000000000),
					},
					Count:           2,
					ExecutionTimeMs: 10,
				}, nil
			},
			wantStatus: http.StatusOK,
			validate: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp models.MetadataResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if len(resp.Namespaces) != 1 || resp.Namespaces[0] != "default" {
					t.Errorf("expected namespaces=['default'], got %v", resp.Namespaces)
				}
				if len(resp.Kinds) != 2 {
					t.Errorf("expected 2 kinds, got %d", len(resp.Kinds))
				}
				if len(resp.Groups) != 2 {
					t.Errorf("expected 2 groups, got %d", len(resp.Groups))
				}
				if resp.TotalEvents != 2 {
					t.Errorf("expected TotalEvents=2, got %d", resp.TotalEvents)
				}
			},
		},
		{
			name:   "metadata with time range",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"1000"},
				"end":   {"2000"},
			},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if q.StartTimestamp != 1000 || q.EndTimestamp != 2000 {
					t.Errorf("time range not set correctly: start=%d, end=%d", q.StartTimestamp, q.EndTimestamp)
				}
				return &models.QueryResult{
					Events:          []models.Event{},
					Count:           0,
					ExecutionTimeMs: 5,
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "empty metadata",
			method:      http.MethodGet,
			queryParams: url.Values{},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				return &models.QueryResult{
					Events:          []models.Event{},
					Count:           0,
					ExecutionTimeMs: 0,
				}, nil
			},
			wantStatus: http.StatusOK,
			validate: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp models.MetadataResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if len(resp.Namespaces) != 0 {
					t.Errorf("expected empty namespaces, got %v", resp.Namespaces)
				}
				if resp.TotalEvents != 0 {
					t.Errorf("expected TotalEvents=0, got %d", resp.TotalEvents)
				}
				if resp.TimeRange.Earliest != 0 || resp.TimeRange.Latest != 0 {
					t.Errorf("expected zero time range, got earliest=%d, latest=%d", resp.TimeRange.Earliest, resp.TimeRange.Latest)
				}
			},
		},
		{
			name:   "invalid start timestamp",
			method: http.MethodGet,
			queryParams: url.Values{
				"start": {"invalid"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:   "invalid end timestamp",
			method: http.MethodGet,
			queryParams: url.Values{
				"end": {"invalid"},
			},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:        "executor error",
			method:      http.MethodGet,
			queryParams: url.Values{},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				return nil, &ValidationError{message: "executor failed"}
			},
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
		{
			name:        "sorted metadata fields",
			method:      http.MethodGet,
			queryParams: url.Values{},
			mockExecute: func(q *models.QueryRequest) (*models.QueryResult, error) {
				return &models.QueryResult{
					Events: []models.Event{
						createTestEvent("1", "Pod", "z-namespace", "pod1", 1500000000000),
						createTestEvent("2", "Service", "a-namespace", "svc1", 1600000000000),
						createTestEvent("3", "Deployment", "m-namespace", "deploy1", 1700000000000),
					},
					Count:           3,
					ExecutionTimeMs: 10,
				}, nil
			},
			wantStatus: http.StatusOK,
			validate: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp models.MetadataResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				// Check that namespaces are sorted
				if len(resp.Namespaces) >= 2 {
					for i := 1; i < len(resp.Namespaces); i++ {
						if resp.Namespaces[i-1] > resp.Namespaces[i] {
							t.Errorf("namespaces not sorted: %v", resp.Namespaces)
						}
					}
				}
				// Check that kinds are sorted
				if len(resp.Kinds) >= 2 {
					for i := 1; i < len(resp.Kinds); i++ {
						if resp.Kinds[i-1] > resp.Kinds[i] {
							t.Errorf("kinds not sorted: %v", resp.Kinds)
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mockQueryExecutor{executeFunc: tt.mockExecute}
			logger := logging.GetLogger("test")
			handler := NewMetadataHandler(mockExecutor, logger)

			req := httptest.NewRequest(tt.method, "/v1/metadata?"+tt.queryParams.Encode(), nil)
			rr := httptest.NewRecorder()

			handler.Handle(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if tt.wantErrCode != "" {
				var errResp ErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to unmarshal error response: %v", err)
				}
				if errResp.Error != tt.wantErrCode {
					t.Errorf("expected error code %s, got %s", tt.wantErrCode, errResp.Error)
				}
			}

			if tt.validate != nil {
				tt.validate(t, rr)
			}
		})
	}
}

// Test Validator

func TestValidator_ValidateTimestamps(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		start     int64
		end       int64
		wantError bool
	}{
		{"valid range", 1000, 2000, false},
		{"same timestamps", 1000, 1000, false},
		{"negative start", -1, 2000, true},
		{"negative end", 1000, -1, true},
		{"start greater than end", 2000, 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTimestamps(tt.start, tt.end)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidator_ValidateFilters(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		filters   models.QueryFilters
		wantError bool
	}{
		{"empty filters", models.QueryFilters{}, false},
		{"valid filters", models.QueryFilters{Kind: "Pod", Namespace: "default"}, false},
		{"group too long", models.QueryFilters{Group: strings.Repeat("a", 256)}, true},
		{"version too long", models.QueryFilters{Version: strings.Repeat("a", 256)}, true},
		{"kind too long", models.QueryFilters{Kind: strings.Repeat("a", 256)}, true},
		{"namespace too long", models.QueryFilters{Namespace: strings.Repeat("a", 64)}, true},
		{"invalid namespace format - uppercase", models.QueryFilters{Namespace: "Invalid"}, true},
		{"invalid namespace format - starts with hyphen", models.QueryFilters{Namespace: "-invalid"}, true},
		{"invalid namespace format - ends with hyphen", models.QueryFilters{Namespace: "invalid-"}, true},
		{"valid namespace with hyphen", models.QueryFilters{Namespace: "valid-namespace"}, false},
		{"valid single char namespace", models.QueryFilters{Namespace: "a"}, false},
		{"valid 63 char namespace", models.QueryFilters{Namespace: strings.Repeat("a", 63)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFilters(tt.filters)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidator_ValidateQuery(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		query     *models.QueryRequest
		wantError bool
	}{
		{
			"valid query",
			&models.QueryRequest{
				StartTimestamp: 1000,
				EndTimestamp:   2000,
				Filters:        models.QueryFilters{},
			},
			false,
		},
		{
			"invalid timestamps",
			&models.QueryRequest{
				StartTimestamp: 2000,
				EndTimestamp:   1000,
				Filters:        models.QueryFilters{},
			},
			true,
		},
		{
			"invalid filters",
			&models.QueryRequest{
				StartTimestamp: 1000,
				EndTimestamp:   2000,
				Filters:        models.QueryFilters{Namespace: strings.Repeat("a", 64)},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateQuery(tt.query)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test DateParser

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		wantError bool
		validate  func(*testing.T, int64)
	}{
		{"empty string", "", "start", true, nil},
		{"valid Unix timestamp", "1609459200", "start", false, func(t *testing.T, ts int64) {
			if ts != 1609459200 {
				t.Errorf("expected 1609459200, got %d", ts)
			}
		}},
		{"Unix timestamp as string", "1000", "start", false, func(t *testing.T, ts int64) {
			if ts != 1000 {
				t.Errorf("expected 1000, got %d", ts)
			}
		}},
		{"negative Unix timestamp", "-1", "start", true, nil},
		{"zero Unix timestamp", "0", "start", false, func(t *testing.T, ts int64) {
			if ts != 0 {
				t.Errorf("expected 0, got %d", ts)
			}
		}},
		{"human readable date - today", "today", "start", false, func(t *testing.T, ts int64) {
			// Should parse successfully
			if ts < 0 {
				t.Errorf("expected non-negative timestamp, got %d", ts)
			}
		}},
		{"human readable date - yesterday", "yesterday", "start", false, func(t *testing.T, ts int64) {
			// Should parse successfully
			if ts < 0 {
				t.Errorf("expected non-negative timestamp, got %d", ts)
			}
		}},
		{"invalid format", "not-a-date-or-number", "start", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimestamp(tt.input, tt.fieldName)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestParseOptionalTimestamp(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal int64
		want       int64
		wantError  bool
	}{
		{"empty string returns default", "", 100, 100, false},
		{"valid timestamp", "2000", 100, 2000, false},
		{"invalid timestamp", "invalid", 100, 100, true},
		{"negative timestamp", "-1", 100, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseOptionalTimestamp(tt.input, tt.defaultVal)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantError && result != tt.want {
				t.Errorf("expected %d, got %d", tt.want, result)
			}
		})
	}
}

// Test Server Routes

func TestServer_Routes(t *testing.T) {
	mockExecutor := &mockQueryExecutor{}
	mockChecker := &mockReadinessChecker{ready: true}
	server := New(8080, mockExecutor, mockChecker)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"health check", http.MethodGet, "/health", http.StatusOK},
		{"readiness check", http.MethodGet, "/ready", http.StatusOK},
		{"not found", http.MethodGet, "/nonexistent", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			server.server.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestServer_MethodEnforcement(t *testing.T) {
	mockExecutor := &mockQueryExecutor{
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			return &models.QueryResult{Events: []models.Event{}, Count: 0, ExecutionTimeMs: 0}, nil
		},
	}
	mockChecker := &mockReadinessChecker{ready: true}
	server := New(8080, mockExecutor, mockChecker)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"GET allowed", http.MethodGet, "/v1/search?start=1000&end=2000", http.StatusOK}, // GET is allowed, will return 200 if query is valid
		{"POST not allowed", http.MethodPost, "/v1/search?start=1000&end=2000", http.StatusMethodNotAllowed},
		{"PUT not allowed", http.MethodPut, "/v1/search?start=1000&end=2000", http.StatusMethodNotAllowed},
		{"DELETE not allowed", http.MethodDelete, "/v1/search?start=1000&end=2000", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			server.server.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestServer_ReadinessCheck(t *testing.T) {
	mockExecutor := &mockQueryExecutor{}

	tests := []struct {
		name         string
		ready        bool
		wantStatus   int
		wantReady    bool
		wantBodyJSON bool
	}{
		{"ready", true, http.StatusOK, true, true},
		{"not ready", false, http.StatusServiceUnavailable, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChecker := &mockReadinessChecker{ready: tt.ready}
			server := New(8080, mockExecutor, mockChecker)

			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rr := httptest.NewRecorder()

			server.server.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if tt.wantBodyJSON {
				var resp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if ready, ok := resp["ready"].(bool); !ok || ready != tt.wantReady {
					t.Errorf("expected ready=%v, got %v", tt.wantReady, ready)
				}
			}
		})
	}
}

// Test CORS Middleware

func TestCORS_Middleware(t *testing.T) {
	mockExecutor := &mockQueryExecutor{}
	mockChecker := &mockReadinessChecker{ready: true}
	server := New(8080, mockExecutor, mockChecker)

	tests := []struct {
		name         string
		method       string
		checkHeaders func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			"CORS headers present",
			http.MethodGet,
			func(t *testing.T, rr *httptest.ResponseRecorder) {
				if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
					t.Errorf("expected Access-Control-Allow-Origin=*, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
				}
				if rr.Header().Get("Access-Control-Allow-Methods") == "" {
					t.Errorf("Access-Control-Allow-Methods header missing")
				}
				if rr.Header().Get("Access-Control-Allow-Headers") == "" {
					t.Errorf("Access-Control-Allow-Headers header missing")
				}
			},
		},
		{
			"OPTIONS preflight",
			http.MethodOptions,
			func(t *testing.T, rr *httptest.ResponseRecorder) {
				if rr.Code != http.StatusNoContent {
					t.Errorf("expected status 204, got %d", rr.Code)
				}
				if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
					t.Errorf("expected Access-Control-Allow-Origin=*, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rr := httptest.NewRecorder()

			server.server.Handler.ServeHTTP(rr, req)

			tt.checkHeaders(t, rr)
		})
	}
}

// Test Error Responses

func TestErrorResponse_Format(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
		message    string
		validate   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			"validation error format",
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"test message",
			func(t *testing.T, rr *httptest.ResponseRecorder) {
				if rr.Header().Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type=application/json, got %s", rr.Header().Get("Content-Type"))
				}
				var errResp ErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to unmarshal error response: %v", err)
				}
				if errResp.Error != "INVALID_REQUEST" {
					t.Errorf("expected error=INVALID_REQUEST, got %s", errResp.Error)
				}
				if errResp.Message != "test message" {
					t.Errorf("expected message='test message', got %s", errResp.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tt.statusCode)

			response := map[string]string{
				"error":   tt.errorCode,
				"message": tt.message,
			}
			if err := writeJSON(w, response); err != nil {
				t.Fatalf("writeJSON failed: %v", err)
			}

			tt.validate(t, w)
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		args    []interface{}
		want    string
	}{
		{"simple message", "test error", nil, "test error"},
		{"formatted message", "error: %v", []interface{}{"test"}, "error: test"},
		{"multiple args", "start=%d end=%d", []interface{}{1000, 2000}, "start=1000 end=2000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.message, tt.args...)
			if err.Error() != tt.want {
				t.Errorf("expected %q, got %q", tt.want, err.Error())
			}
			if err.GetMessage() != tt.want {
				t.Errorf("expected GetMessage()=%q, got %q", tt.want, err.GetMessage())
			}
		})
	}
}

// Test writeJSON helper

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		validate func(*testing.T, []byte)
	}{
		{
			"simple map",
			map[string]string{"key": "value"},
			func(t *testing.T, body []byte) {
				var result map[string]string
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["key"] != "value" {
					t.Errorf("expected value, got %s", result["key"])
				}
			},
		},
		{
			"struct",
			struct {
				Field string `json:"field"`
			}{Field: "test"},
			func(t *testing.T, body []byte) {
				var result struct {
					Field string `json:"field"`
				}
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result.Field != "test" {
					t.Errorf("expected 'test', got %s", result.Field)
				}
			},
		},
		{
			"HTML escaping disabled",
			map[string]string{"html": "<script>alert('xss')</script>"},
			func(t *testing.T, body []byte) {
				bodyStr := string(body)
				if !strings.Contains(bodyStr, "<script>") {
					t.Errorf("expected HTML to not be escaped, but <script> not found in %s", bodyStr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeJSON(&buf, tt.data); err != nil {
				t.Fatalf("writeJSON failed: %v", err)
			}
			tt.validate(t, buf.Bytes())
		})
	}
}
