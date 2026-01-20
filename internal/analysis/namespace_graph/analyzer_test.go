package namespacegraph

import (
	"testing"
	"time"
)

func TestAnalyzeInputDefaults(t *testing.T) {
	tests := []struct {
		name         string
		input        AnalyzeInput
		wantLimit    int
		wantMaxDepth int
		wantLookback time.Duration
	}{
		{
			name:         "empty input gets defaults",
			input:        AnalyzeInput{},
			wantLimit:    DefaultLimit,
			wantMaxDepth: DefaultMaxDepth,
			wantLookback: DefaultLookback,
		},
		{
			name: "explicit values preserved",
			input: AnalyzeInput{
				Limit:    50,
				MaxDepth: 5,
				Lookback: 20 * time.Minute,
			},
			wantLimit:    50,
			wantMaxDepth: 5,
			wantLookback: 20 * time.Minute,
		},
		{
			name: "limit capped at max",
			input: AnalyzeInput{
				Limit: 1000,
			},
			wantLimit:    MaxLimit,
			wantMaxDepth: DefaultMaxDepth,
			wantLookback: DefaultLookback,
		},
		{
			name: "maxDepth capped at max",
			input: AnalyzeInput{
				MaxDepth: 20,
			},
			wantLimit:    DefaultLimit,
			wantMaxDepth: MaxMaxDepth,
			wantLookback: DefaultLookback,
		},
		{
			name: "lookback capped at max",
			input: AnalyzeInput{
				Lookback: 48 * time.Hour,
			},
			wantLimit:    DefaultLimit,
			wantMaxDepth: DefaultMaxDepth,
			wantLookback: MaxLookback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults like the analyzer does
			input := tt.input
			if input.Limit <= 0 {
				input.Limit = DefaultLimit
			}
			if input.Limit > MaxLimit {
				input.Limit = MaxLimit
			}
			if input.MaxDepth <= 0 {
				input.MaxDepth = DefaultMaxDepth
			}
			if input.MaxDepth > MaxMaxDepth {
				input.MaxDepth = MaxMaxDepth
			}
			if input.Lookback <= 0 {
				input.Lookback = DefaultLookback
			}
			if input.Lookback > MaxLookback {
				input.Lookback = MaxLookback
			}

			if input.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", input.Limit, tt.wantLimit)
			}
			if input.MaxDepth != tt.wantMaxDepth {
				t.Errorf("MaxDepth = %d, want %d", input.MaxDepth, tt.wantMaxDepth)
			}
			if input.Lookback != tt.wantLookback {
				t.Errorf("Lookback = %v, want %v", input.Lookback, tt.wantLookback)
			}
		})
	}
}

func TestPaginationCursor(t *testing.T) {
	tests := []struct {
		name       string
		cursor     PaginationCursor
		wantEncode bool
	}{
		{
			name: "simple cursor",
			cursor: PaginationCursor{
				LastKind: "Pod",
				LastName: "my-pod-xyz",
			},
			wantEncode: true,
		},
		{
			name: "empty cursor",
			cursor: PaginationCursor{
				LastKind: "",
				LastName: "",
			},
			wantEncode: true,
		},
		{
			name: "special characters in name",
			cursor: PaginationCursor{
				LastKind: "ConfigMap",
				LastName: "my-config-map-v1.2.3",
			},
			wantEncode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := encodeCursor(tt.cursor)
			if encoded == "" && tt.wantEncode {
				t.Error("Expected non-empty encoded cursor")
			}

			// Decode
			decoded, err := decodeCursor(encoded)
			if err != nil {
				t.Fatalf("Failed to decode cursor: %v", err)
			}

			if decoded.LastKind != tt.cursor.LastKind {
				t.Errorf("LastKind = %q, want %q", decoded.LastKind, tt.cursor.LastKind)
			}
			if decoded.LastName != tt.cursor.LastName {
				t.Errorf("LastName = %q, want %q", decoded.LastName, tt.cursor.LastName)
			}
		})
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	tests := []struct {
		name    string
		cursor  string
		wantErr bool
	}{
		{
			name:    "invalid base64",
			cursor:  "not-valid-base64!@#$",
			wantErr: true,
		},
		{
			name:    "valid base64 but invalid JSON",
			cursor:  "aW52YWxpZCBqc29u", // "invalid json" in base64
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeCursor(tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeCursor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildNodes(t *testing.T) {
	analyzer := &Analyzer{}

	resources := []resourceResult{
		{
			UID:       "uid-1",
			Kind:      "Pod",
			APIGroup:  "",
			Namespace: "default",
			Name:      "my-pod",
			Labels:    map[string]string{"app": "test"},
		},
		{
			UID:       "uid-2",
			Kind:      "Node",
			APIGroup:  "",
			Namespace: "", // cluster-scoped
			Name:      "node-1",
		},
	}

	latestEvents := map[string]*ChangeEventInfo{
		"uid-1": {
			Timestamp: 1704067200000000000,
			EventType: "MODIFIED",
			Status:    StatusReady,
		},
	}

	nodes := analyzer.buildNodes(resources, latestEvents)

	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	// Check first node (with event)
	if nodes[0].UID != "uid-1" {
		t.Errorf("Node 0 UID = %q, want %q", nodes[0].UID, "uid-1")
	}
	if nodes[0].Status != StatusReady {
		t.Errorf("Node 0 Status = %q, want %q", nodes[0].Status, StatusReady)
	}
	if nodes[0].LatestEvent == nil {
		t.Error("Expected node 0 to have a latest event")
	} else if nodes[0].LatestEvent.EventType != "MODIFIED" {
		t.Errorf("Node 0 LatestEvent.EventType = %q, want %q", nodes[0].LatestEvent.EventType, "MODIFIED")
	}

	// Check second node (without event)
	if nodes[1].UID != "uid-2" {
		t.Errorf("Node 1 UID = %q, want %q", nodes[1].UID, "uid-2")
	}
	if nodes[1].LatestEvent != nil {
		t.Error("Expected node 1 to have no latest event")
	}
	if nodes[1].Namespace != "" {
		t.Errorf("Node 1 Namespace = %q, want empty (cluster-scoped)", nodes[1].Namespace)
	}
}

func TestBuildEdges(t *testing.T) {
	analyzer := &Analyzer{}

	edgeResults := []edgeResult{
		{
			SourceUID:        "uid-1",
			TargetUID:        "uid-2",
			RelationshipType: "OWNS",
			EdgeID:           "edge-1",
		},
		{
			SourceUID:        "uid-2",
			TargetUID:        "uid-3",
			RelationshipType: "SELECTS",
			EdgeID:           "edge-2",
		},
	}

	edges := analyzer.buildEdges(edgeResults)

	if len(edges) != 2 {
		t.Fatalf("Expected 2 edges, got %d", len(edges))
	}

	if edges[0].Source != "uid-1" || edges[0].Target != "uid-2" {
		t.Errorf("Edge 0 Source/Target = %q/%q, want uid-1/uid-2", edges[0].Source, edges[0].Target)
	}
	if edges[0].RelationshipType != "OWNS" {
		t.Errorf("Edge 0 RelationshipType = %q, want OWNS", edges[0].RelationshipType)
	}

	if edges[1].RelationshipType != "SELECTS" {
		t.Errorf("Edge 1 RelationshipType = %q, want SELECTS", edges[1].RelationshipType)
	}
}

func TestConstants(t *testing.T) {
	// Verify constants have sensible values
	if DefaultLimit <= 0 {
		t.Errorf("DefaultLimit should be positive, got %d", DefaultLimit)
	}
	if MaxLimit <= DefaultLimit {
		t.Errorf("MaxLimit (%d) should be greater than DefaultLimit (%d)", MaxLimit, DefaultLimit)
	}
	if DefaultMaxDepth <= 0 {
		t.Errorf("DefaultMaxDepth should be positive, got %d", DefaultMaxDepth)
	}
	if MaxMaxDepth <= DefaultMaxDepth {
		t.Errorf("MaxMaxDepth (%d) should be greater than DefaultMaxDepth (%d)", MaxMaxDepth, DefaultMaxDepth)
	}
	if DefaultLookback <= 0 {
		t.Errorf("DefaultLookback should be positive, got %v", DefaultLookback)
	}
	if MaxLookback <= DefaultLookback {
		t.Errorf("MaxLookback (%v) should be greater than DefaultLookback (%v)", MaxLookback, DefaultLookback)
	}
	if QueryTimeoutMs <= 0 {
		t.Errorf("QueryTimeoutMs should be positive, got %d", QueryTimeoutMs)
	}
}
