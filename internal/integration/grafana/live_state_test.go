package grafana

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// mockGrafanaClientForLiveState implements GrafanaClientInterface for testing
type mockGrafanaClientForLiveState struct {
	queryResponse *QueryResponse
	queryError    error
}

func (m *mockGrafanaClientForLiveState) QueryDataSource(ctx context.Context, datasourceUID string, expr string, from string, to string, scopedVars map[string]ScopedVar) (*QueryResponse, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResponse, nil
}

func (m *mockGrafanaClientForLiveState) GetAlertRule(ctx context.Context, uid string) (*AlertRule, error) {
	return nil, nil
}

func (m *mockGrafanaClientForLiveState) GetAlertStates(ctx context.Context) ([]AlertState, error) {
	return nil, nil
}

func (m *mockGrafanaClientForLiveState) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	return nil, nil
}

func (m *mockGrafanaClientForLiveState) ListDashboards(ctx context.Context) ([]DashboardMeta, error) {
	return nil, nil
}

func (m *mockGrafanaClientForLiveState) GetDashboard(ctx context.Context, uid string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockGrafanaClientForLiveState) ListDatasources(ctx context.Context) ([]map[string]interface{}, error) {
	return nil, nil
}

func TestLiveStateProvider_FetchLiveStateTransitions_Empty(t *testing.T) {
	logger := logging.GetLogger("test")
	mock := &mockGrafanaClientForLiveState{
		queryResponse: &QueryResponse{
			Results: map[string]QueryResult{
				"A": {Frames: []DataFrame{}},
			},
		},
	}

	provider := NewLiveStateProvider(mock, "prometheus-uid", "test-integration", logger)

	now := time.Now()
	transitions, err := provider.FetchLiveStateTransitions(
		context.Background(),
		"TestAlert",
		now.Add(-1*time.Hour),
		now,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(transitions) != 0 {
		t.Errorf("expected 0 transitions, got %d", len(transitions))
	}
}

func TestLiveStateProvider_FetchLiveStateTransitions_FiringAlert(t *testing.T) {
	logger := logging.GetLogger("test")

	// Simulate an alert that was firing from T+10min to T+30min
	baseTime := time.Now().Truncate(time.Minute)
	firingStart := baseTime.Add(10 * time.Minute)

	// Create timestamps for firing period (every minute)
	var timestamps []interface{}
	var values []interface{}
	for i := 0; i < 20; i++ {
		ts := firingStart.Add(time.Duration(i) * time.Minute)
		timestamps = append(timestamps, float64(ts.UnixMilli()))
		values = append(values, float64(1))
	}

	mock := &mockGrafanaClientForLiveState{
		queryResponse: &QueryResponse{
			Results: map[string]QueryResult{
				"A": {
					Frames: []DataFrame{
						{
							Schema: DataFrameSchema{
								Fields: []DataFrameField{
									{Name: "Time", Type: "time"},
									{
										Name:   "Value",
										Type:   "number",
										Labels: map[string]string{"alertstate": "firing"},
									},
								},
							},
							Data: DataFrameData{
								Values: [][]interface{}{timestamps, values},
							},
						},
					},
				},
			},
		},
	}

	provider := NewLiveStateProvider(mock, "prometheus-uid", "test-integration", logger)

	transitions, err := provider.FetchLiveStateTransitions(
		context.Background(),
		"TestAlert",
		baseTime,
		baseTime.Add(1*time.Hour),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least one transition (normal -> firing)
	if len(transitions) < 1 {
		t.Fatalf("expected at least 1 transition, got %d", len(transitions))
	}

	// First transition should be normal -> firing
	if transitions[0].FromState != "normal" || transitions[0].ToState != "firing" {
		t.Errorf("expected normal->firing, got %s->%s", transitions[0].FromState, transitions[0].ToState)
	}
}

func TestLiveStateProvider_FetchLiveStateTransitions_MultipleFiringPeriods(t *testing.T) {
	logger := logging.GetLogger("test")

	baseTime := time.Now().Truncate(time.Minute)

	// First firing period: T+5min to T+10min
	// Second firing period: T+20min to T+25min
	var timestamps []interface{}
	var values []interface{}

	// First period
	for i := 5; i < 10; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		timestamps = append(timestamps, float64(ts.UnixMilli()))
		values = append(values, float64(1))
	}

	// Second period (gap of 10 minutes -> should trigger normal state)
	for i := 20; i < 25; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		timestamps = append(timestamps, float64(ts.UnixMilli()))
		values = append(values, float64(1))
	}

	mock := &mockGrafanaClientForLiveState{
		queryResponse: &QueryResponse{
			Results: map[string]QueryResult{
				"A": {
					Frames: []DataFrame{
						{
							Schema: DataFrameSchema{
								Fields: []DataFrameField{
									{Name: "Time", Type: "time"},
									{
										Name:   "Value",
										Type:   "number",
										Labels: map[string]string{"alertstate": "firing"},
									},
								},
							},
							Data: DataFrameData{
								Values: [][]interface{}{timestamps, values},
							},
						},
					},
				},
			},
		},
	}

	provider := NewLiveStateProvider(mock, "prometheus-uid", "test-integration", logger)

	transitions, err := provider.FetchLiveStateTransitions(
		context.Background(),
		"TestAlert",
		baseTime,
		baseTime.Add(30*time.Minute),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have transitions:
	// 1. normal -> firing (at ~T+5min)
	// 2. firing -> normal (gap detected after T+10min)
	// 3. normal -> firing (at ~T+20min)
	if len(transitions) < 3 {
		t.Fatalf("expected at least 3 transitions, got %d: %+v", len(transitions), transitions)
	}

	// Verify first transition
	if transitions[0].FromState != "normal" || transitions[0].ToState != "firing" {
		t.Errorf("transition 0: expected normal->firing, got %s->%s",
			transitions[0].FromState, transitions[0].ToState)
	}

	// Find the transition back to normal
	foundNormal := false
	for _, tr := range transitions {
		if tr.ToState == "normal" {
			foundNormal = true
			break
		}
	}
	if !foundNormal {
		t.Errorf("expected at least one transition to normal state")
	}
}

func TestLiveStateProvider_FetchLiveStateTransitions_PendingToFiring(t *testing.T) {
	logger := logging.GetLogger("test")

	baseTime := time.Now().Truncate(time.Minute)

	// Pending from T+5 to T+10, then firing from T+10 to T+20
	var pendingTimestamps []interface{}
	var pendingValues []interface{}
	var firingTimestamps []interface{}
	var firingValues []interface{}

	// Pending period
	for i := 5; i < 10; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		pendingTimestamps = append(pendingTimestamps, float64(ts.UnixMilli()))
		pendingValues = append(pendingValues, float64(1))
	}

	// Firing period (overlapping slightly at T+10)
	for i := 10; i < 20; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		firingTimestamps = append(firingTimestamps, float64(ts.UnixMilli()))
		firingValues = append(firingValues, float64(1))
	}

	mock := &mockGrafanaClientForLiveState{
		queryResponse: &QueryResponse{
			Results: map[string]QueryResult{
				"A": {
					Frames: []DataFrame{
						{
							Schema: DataFrameSchema{
								Fields: []DataFrameField{
									{Name: "Time", Type: "time"},
									{
										Name:   "Value",
										Type:   "number",
										Labels: map[string]string{"alertstate": "pending"},
									},
								},
							},
							Data: DataFrameData{
								Values: [][]interface{}{pendingTimestamps, pendingValues},
							},
						},
						{
							Schema: DataFrameSchema{
								Fields: []DataFrameField{
									{Name: "Time", Type: "time"},
									{
										Name:   "Value",
										Type:   "number",
										Labels: map[string]string{"alertstate": "firing"},
									},
								},
							},
							Data: DataFrameData{
								Values: [][]interface{}{firingTimestamps, firingValues},
							},
						},
					},
				},
			},
		},
	}

	provider := NewLiveStateProvider(mock, "prometheus-uid", "test-integration", logger)

	transitions, err := provider.FetchLiveStateTransitions(
		context.Background(),
		"TestAlert",
		baseTime,
		baseTime.Add(30*time.Minute),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have transitions: normal -> pending -> firing
	if len(transitions) < 2 {
		t.Fatalf("expected at least 2 transitions, got %d", len(transitions))
	}

	// First should be normal -> pending
	if transitions[0].FromState != "normal" || transitions[0].ToState != "pending" {
		t.Errorf("transition 0: expected normal->pending, got %s->%s",
			transitions[0].FromState, transitions[0].ToState)
	}

	// Should have pending -> firing transition
	foundPendingToFiring := false
	for _, tr := range transitions {
		if tr.FromState == "pending" && tr.ToState == "firing" {
			foundPendingToFiring = true
			break
		}
	}
	if !foundPendingToFiring {
		t.Errorf("expected pending->firing transition, transitions: %+v", transitions)
	}
}

func TestLiveStateProvider_DeriveTransitions_FiringPrecedence(t *testing.T) {
	logger := logging.GetLogger("test")
	provider := NewLiveStateProvider(nil, "", "", logger)

	baseTime := time.Now().Truncate(time.Second)

	// We can test via parseAlertsResponse by constructing a response with overlapping data
	// at the same timestamp. Firing should take precedence over pending.
	response := &QueryResponse{
		Results: map[string]QueryResult{
			"A": {
				Frames: []DataFrame{
					{
						Schema: DataFrameSchema{
							Fields: []DataFrameField{
								{Name: "Time", Type: "time"},
								{Name: "Value", Type: "number", Labels: map[string]string{"alertstate": "pending"}},
							},
						},
						Data: DataFrameData{
							Values: [][]interface{}{
								{float64(baseTime.UnixMilli())},
								{float64(1)},
							},
						},
					},
					{
						Schema: DataFrameSchema{
							Fields: []DataFrameField{
								{Name: "Time", Type: "time"},
								{Name: "Value", Type: "number", Labels: map[string]string{"alertstate": "firing"}},
							},
						},
						Data: DataFrameData{
							Values: [][]interface{}{
								{float64(baseTime.UnixMilli())},
								{float64(1)},
							},
						},
					},
				},
			},
		},
	}

	// Use a tight time window to avoid triggering the "transition back to normal" logic
	transitions, err := provider.parseAlertsResponse(response, baseTime.Add(-time.Minute), baseTime.Add(time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have exactly one transition: normal -> firing (firing takes precedence over pending)
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d: %+v", len(transitions), transitions)
	}

	if transitions[0].ToState != "firing" {
		t.Errorf("expected firing state (precedence), got %s", transitions[0].ToState)
	}
}

func TestLiveStateProvider_QueryError(t *testing.T) {
	logger := logging.GetLogger("test")
	mock := &mockGrafanaClientForLiveState{
		queryError: fmt.Errorf("connection refused"),
	}

	provider := NewLiveStateProvider(mock, "prometheus-uid", "test-integration", logger)

	now := time.Now()
	_, err := provider.FetchLiveStateTransitions(
		context.Background(),
		"TestAlert",
		now.Add(-1*time.Hour),
		now,
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !liveStateContains(err.Error(), "connection refused") {
		t.Errorf("expected error to contain 'connection refused', got: %v", err)
	}
}

func liveStateContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && liveStateContainsHelper(s, substr))
}

func liveStateContainsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
