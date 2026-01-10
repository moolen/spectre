package analysis

import (
	"testing"
	"time"
)

func TestComputeJSONDiff(t *testing.T) {
	tests := []struct {
		name     string
		oldJSON  string
		newJSON  string
		expected []EventDiff
		wantErr  bool
	}{
		{
			name:    "simple field change",
			oldJSON: `{"spec":{"replicas":1}}`,
			newJSON: `{"spec":{"replicas":3}}`,
			expected: []EventDiff{
				{Path: "spec.replicas", OldValue: float64(1), NewValue: float64(3), Op: "replace"},
			},
		},
		{
			name:    "field added",
			oldJSON: `{"metadata":{"name":"test"}}`,
			newJSON: `{"metadata":{"name":"test","namespace":"default"}}`,
			expected: []EventDiff{
				{Path: "metadata.namespace", NewValue: "default", Op: "add"},
			},
		},
		{
			name:    "field removed",
			oldJSON: `{"metadata":{"name":"test","labels":{"app":"web"}}}`,
			newJSON: `{"metadata":{"name":"test"}}`,
			expected: []EventDiff{
				{Path: "metadata.labels", OldValue: map[string]any{"app": "web"}, Op: "remove"},
			},
		},
		{
			name:    "nested object change",
			oldJSON: `{"spec":{"template":{"spec":{"containers":[{"name":"app","image":"v1"}]}}}}`,
			newJSON: `{"spec":{"template":{"spec":{"containers":[{"name":"app","image":"v2"}]}}}}`,
			expected: []EventDiff{
				{
					Path:     "spec.template.spec.containers",
					OldValue: []any{map[string]any{"name": "app", "image": "v1"}},
					NewValue: []any{map[string]any{"name": "app", "image": "v2"}},
					Op:       "replace",
				},
			},
		},
		{
			name:    "multiple changes",
			oldJSON: `{"spec":{"replicas":1,"image":"v1"},"status":{"ready":true}}`,
			newJSON: `{"spec":{"replicas":3,"image":"v2"},"status":{"ready":false}}`,
			expected: []EventDiff{
				{Path: "spec.image", OldValue: "v1", NewValue: "v2", Op: "replace"},
				{Path: "spec.replicas", OldValue: float64(1), NewValue: float64(3), Op: "replace"},
				{Path: "status.ready", OldValue: true, NewValue: false, Op: "replace"},
			},
		},
		{
			name:    "spec changes",
			oldJSON: `{"spec":{"values": {"a": "f", "x": "y"}}}`,
			newJSON: `{"spec":{"values": {"x": "y"}}}`,
			expected: []EventDiff{
				{Path: "spec.values.a", Op: "remove"},
			},
		},
		{
			name:    "empty old (new resource)",
			oldJSON: ``,
			newJSON: `{"metadata":{"name":"test"}}`,
			expected: []EventDiff{
				{Path: "metadata", NewValue: map[string]any{"name": "test"}, Op: "add"},
			},
		},
		{
			name:    "empty new (deleted resource)",
			oldJSON: `{"metadata":{"name":"test"}}`,
			newJSON: ``,
			expected: []EventDiff{
				{Path: "metadata", OldValue: map[string]any{"name": "test"}, Op: "remove"},
			},
		},
		{
			name:     "both empty",
			oldJSON:  ``,
			newJSON:  ``,
			expected: nil,
		},
		{
			name:     "identical objects",
			oldJSON:  `{"spec":{"replicas":3}}`,
			newJSON:  `{"spec":{"replicas":3}}`,
			expected: nil,
		},
		{
			name:    "type change",
			oldJSON: `{"value":"string"}`,
			newJSON: `{"value":123}`,
			expected: []EventDiff{
				{Path: "value", OldValue: "string", NewValue: float64(123), Op: "replace"},
			},
		},
		{
			name:    "invalid old JSON",
			oldJSON: `{invalid}`,
			newJSON: `{"valid":true}`,
			wantErr: true,
		},
		{
			name:    "invalid new JSON",
			oldJSON: `{"valid":true}`,
			newJSON: `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs, err := ComputeJSONDiff([]byte(tt.oldJSON), []byte(tt.newJSON))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(diffs) != len(tt.expected) {
				t.Errorf("expected %d diffs, got %d: %+v", len(tt.expected), len(diffs), diffs)
				return
			}

			for i, expected := range tt.expected {
				if diffs[i].Path != expected.Path {
					t.Errorf("diff[%d].Path = %q, want %q", i, diffs[i].Path, expected.Path)
				}
				if diffs[i].Op != expected.Op {
					t.Errorf("diff[%d].Op = %q, want %q", i, diffs[i].Op, expected.Op)
				}
			}
		})
	}
}

func TestParseJSONToMap(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantNil bool
		wantErr bool
	}{
		{
			name:    "valid JSON",
			data:    []byte(`{"key":"value"}`),
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "nil data",
			data:    nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{invalid}`),
			wantNil: false,
			wantErr: true,
		},
		{
			name:    "nested object",
			data:    []byte(`{"spec":{"replicas":3,"template":{"containers":[]}}}`),
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseJSONToMap(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantNil && result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("expected non-nil result, got nil")
			}
		})
	}
}

func TestFilterNoisyPaths(t *testing.T) {
	tests := []struct {
		name     string
		diffs    []EventDiff
		expected int // number of diffs after filtering
	}{
		{
			name: "filter managedFields",
			diffs: []EventDiff{
				{Path: "metadata.managedFields", Op: "replace"},
				{Path: "spec.replicas", Op: "replace"},
			},
			expected: 1,
		},
		{
			name: "filter resourceVersion",
			diffs: []EventDiff{
				{Path: "metadata.resourceVersion", Op: "replace"},
				{Path: "spec.image", Op: "replace"},
			},
			expected: 1,
		},
		{
			name: "filter multiple noisy paths",
			diffs: []EventDiff{
				{Path: "metadata.managedFields.0", Op: "replace"},
				{Path: "metadata.generation", Op: "replace"},
				{Path: "metadata.uid", Op: "add"},
				{Path: "metadata.creationTimestamp", Op: "add"},
				{Path: "status.observedGeneration", Op: "replace"},
				{Path: "spec.replicas", Op: "replace"},
				{Path: "spec.template", Op: "replace"},
			},
			expected: 2,
		},
		{
			name: "no noisy paths",
			diffs: []EventDiff{
				{Path: "spec.replicas", Op: "replace"},
				{Path: "spec.image", Op: "replace"},
				{Path: "metadata.labels", Op: "add"},
			},
			expected: 3,
		},
		{
			name:     "empty diffs",
			diffs:    []EventDiff{},
			expected: 0,
		},
		{
			name:     "nil diffs",
			diffs:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterNoisyPaths(tt.diffs)
			if len(result) != tt.expected {
				t.Errorf("expected %d diffs after filtering, got %d: %+v", tt.expected, len(result), result)
			}
		})
	}
}

func TestConvertEventsToDiffFormat(t *testing.T) {
	tests := []struct {
		name        string
		events      []ChangeEventInfo
		filterNoisy bool
		checkFirst  func(t *testing.T, event ChangeEventInfo)
		checkSecond func(t *testing.T, event ChangeEventInfo)
	}{
		{
			name: "converts first event to fullSnapshot",
			events: []ChangeEventInfo{
				{
					EventID:   "1",
					Timestamp: time.Now(),
					EventType: "CREATE",
					Data:      []byte(`{"metadata":{"name":"test"},"spec":{"replicas":1}}`),
				},
			},
			filterNoisy: false,
			checkFirst: func(t *testing.T, event ChangeEventInfo) {
				if event.FullSnapshot == nil {
					t.Error("expected FullSnapshot to be set for first event")
				}
				// Data is intentionally kept for anomaly detection (container status checks)
				if event.Diff != nil {
					t.Error("expected Diff to be nil for first event")
				}
			},
		},
		{
			name: "converts subsequent events to diff",
			// Events in REVERSE chronological order (newest first)
			events: []ChangeEventInfo{
				{
					EventID:   "2",
					Timestamp: time.Now().Add(time.Minute),
					EventType: "UPDATE",
					Data:      []byte(`{"spec":{"replicas":3}}`),
				},
				{
					EventID:   "1",
					Timestamp: time.Now(),
					EventType: "CREATE",
					Data:      []byte(`{"spec":{"replicas":1}}`),
				},
			},
			filterNoisy: false,
			checkFirst: func(t *testing.T, event ChangeEventInfo) {
				// First in output (newest) should have Diff
				if event.Diff == nil {
					t.Error("expected Diff to be set for first event (newest)")
				}
				if len(event.Diff) != 1 {
					t.Errorf("expected 1 diff, got %d", len(event.Diff))
				}
				if event.FullSnapshot != nil {
					t.Error("expected FullSnapshot to be nil for newest event")
				}
				// Data is intentionally kept for anomaly detection (container status checks)
			},
			checkSecond: func(t *testing.T, event ChangeEventInfo) {
				// Second in output (oldest) should have FullSnapshot
				if event.FullSnapshot == nil {
					t.Error("expected FullSnapshot to be set for oldest event")
				}
				if event.Diff != nil {
					t.Error("expected Diff to be nil for oldest event")
				}
				// Data is intentionally kept for anomaly detection (container status checks)
			},
		},
		{
			name: "filters noisy paths when enabled",
			// Events in REVERSE chronological order (newest first)
			events: []ChangeEventInfo{
				{
					EventID:   "2",
					Timestamp: time.Now().Add(time.Minute),
					EventType: "UPDATE",
					Data:      []byte(`{"metadata":{"resourceVersion":"2"},"spec":{"replicas":3}}`),
				},
				{
					EventID:   "1",
					Timestamp: time.Now(),
					EventType: "CREATE",
					Data:      []byte(`{"metadata":{"resourceVersion":"1"},"spec":{"replicas":1}}`),
				},
			},
			filterNoisy: true,
			checkFirst: func(t *testing.T, event ChangeEventInfo) {
				// First event (newest) should have diff with noisy paths filtered
				for _, d := range event.Diff {
					if d.Path == "metadata.resourceVersion" {
						t.Error("expected metadata.resourceVersion to be filtered out")
					}
				}
			},
		},
		{
			name:        "handles empty events",
			events:      []ChangeEventInfo{},
			filterNoisy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertEventsToDiffFormat(tt.events, tt.filterNoisy)

			if len(result) != len(tt.events) {
				t.Errorf("expected %d events, got %d", len(tt.events), len(result))
				return
			}

			if len(result) > 0 && tt.checkFirst != nil {
				tt.checkFirst(t, result[0])
			}
			if len(result) > 1 && tt.checkSecond != nil {
				tt.checkSecond(t, result[1])
			}
		})
	}
}

func TestConvertSingleEventToDiff(t *testing.T) {
	tests := []struct {
		name        string
		event       *ChangeEventInfo
		prevData    []byte
		filterNoisy bool
		check       func(t *testing.T, event *ChangeEventInfo)
	}{
		{
			name: "no previous data creates fullSnapshot",
			event: &ChangeEventInfo{
				EventID: "1",
				Data:    []byte(`{"spec":{"replicas":1}}`),
			},
			prevData:    nil,
			filterNoisy: false,
			check: func(t *testing.T, event *ChangeEventInfo) {
				if event.FullSnapshot == nil {
					t.Error("expected FullSnapshot to be set")
				}
				if event.Diff != nil {
					t.Error("expected Diff to be nil")
				}
				if event.Data != nil {
					t.Error("expected Data to be cleared")
				}
			},
		},
		{
			name: "with previous data creates diff",
			event: &ChangeEventInfo{
				EventID: "2",
				Data:    []byte(`{"spec":{"replicas":3}}`),
			},
			prevData:    []byte(`{"spec":{"replicas":1}}`),
			filterNoisy: false,
			check: func(t *testing.T, event *ChangeEventInfo) {
				if event.Diff == nil {
					t.Error("expected Diff to be set")
				}
				if event.FullSnapshot != nil {
					t.Error("expected FullSnapshot to be nil")
				}
				if event.Data != nil {
					t.Error("expected Data to be cleared")
				}
			},
		},
		{
			name:        "nil event does not panic",
			event:       nil,
			prevData:    nil,
			filterNoisy: false,
			check:       func(t *testing.T, event *ChangeEventInfo) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConvertSingleEventToDiff(tt.event, tt.prevData, tt.filterNoisy)
			if tt.event != nil {
				tt.check(t, tt.event)
			}
		})
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		prefix   string
		key      string
		expected string
	}{
		{"", "key", "key"},
		{"prefix", "key", "prefix.key"},
		{"a.b", "c", "a.b.c"},
		{"spec", "replicas", "spec.replicas"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+"_"+tt.key, func(t *testing.T) {
			result := joinPath(tt.prefix, tt.key)
			if result != tt.expected {
				t.Errorf("joinPath(%q, %q) = %q, want %q", tt.prefix, tt.key, result, tt.expected)
			}
		})
	}
}

func TestSimplifyValue(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		isLarge bool // if true, expect summarized output
	}{
		{
			name:    "nil value",
			input:   nil,
			isLarge: false,
		},
		{
			name:    "string value",
			input:   "test",
			isLarge: false,
		},
		{
			name:    "number value",
			input:   float64(42),
			isLarge: false,
		},
		{
			name:    "small map",
			input:   map[string]any{"a": 1, "b": 2},
			isLarge: false,
		},
		{
			name: "large map (>10 keys)",
			input: map[string]any{
				"a": 1, "b": 2, "c": 3, "d": 4, "e": 5,
				"f": 6, "g": 7, "h": 8, "i": 9, "j": 10, "k": 11,
			},
			isLarge: true,
		},
		{
			name:    "small array",
			input:   []any{1, 2, 3},
			isLarge: false,
		},
		{
			name:    "large array (>10 elements)",
			input:   []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			isLarge: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := simplifyValue(tt.input)

			if tt.isLarge {
				resultMap, ok := result.(map[string]any)
				if !ok {
					t.Errorf("expected large value to be summarized as map, got %T", result)
					return
				}
				if _, hasType := resultMap["_type"]; !hasType {
					t.Error("expected summarized value to have _type field")
				}
			}
		})
	}
}
