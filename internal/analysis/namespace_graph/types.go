package namespacegraph

import (
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
)

// AnalyzeInput contains the parameters for namespace graph analysis
type AnalyzeInput struct {
	Namespace          string        // Required: Kubernetes namespace
	Timestamp          int64         // Required: Point in time (Unix nanoseconds)
	IncludeAnomalies   bool          // Optional: Include anomaly detection
	IncludeCausalPaths bool          // Optional: Include causal path analysis
	Lookback           time.Duration // Optional: Lookback for analysis (default 10m)
	MaxDepth           int           // Optional: Max relationship traversal depth (default 3)
	Limit              int           // Optional: Pagination limit (default 100)
	Cursor             string        // Optional: Pagination cursor (base64 encoded)
}

// NamespaceGraphResponse is the API response structure
type NamespaceGraphResponse struct {
	Graph       Graph                    `json:"graph"`
	Anomalies   []anomaly.Anomaly        `json:"anomalies,omitempty"`
	CausalPaths []causalpaths.CausalPath `json:"causalPaths,omitempty"`
	Metadata    Metadata                 `json:"metadata"`
}

// Graph contains nodes and edges
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a Kubernetes resource in the graph
type Node struct {
	UID         string            `json:"uid"`
	Kind        string            `json:"kind"`
	APIGroup    string            `json:"apiGroup,omitempty"`
	Namespace   string            `json:"namespace"` // Empty for cluster-scoped
	Name        string            `json:"name"`
	Status      string            `json:"status"` // "unknown" for now
	LatestEvent *ChangeEventInfo  `json:"latestEvent,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ChangeEventInfo contains information about a resource change
type ChangeEventInfo struct {
	Timestamp       int64    `json:"timestamp"`
	EventType       string   `json:"eventType"`                 // CREATE, UPDATE, DELETE
	Status          string   `json:"status"`                    // Ready, Warning, Error, Terminating, Unknown
	ErrorMessage    string   `json:"errorMessage,omitempty"`    // Human-readable error description
	ContainerIssues []string `json:"containerIssues,omitempty"` // CrashLoopBackOff, ImagePullBackOff, OOMKilled
	ImpactScore     float64  `json:"impactScore,omitempty"`     // 0.0-1.0 severity score
	SpecChanges     string   `json:"specChanges,omitempty"`     // Git-style unified diff of spec changes within lookback window
}

// Edge represents a relationship between resources
type Edge struct {
	ID               string `json:"id"`
	Source           string `json:"source"`           // Source node UID
	Target           string `json:"target"`           // Target node UID
	RelationshipType string `json:"relationshipType"` // OWNS, SELECTS, etc.
}

// Metadata provides response metadata and pagination info
type Metadata struct {
	Namespace        string `json:"namespace"`
	Timestamp        int64  `json:"timestamp"`
	NodeCount        int    `json:"nodeCount"`
	EdgeCount        int    `json:"edgeCount"`
	QueryExecutionMs int64  `json:"queryExecutionMs"`
	HasMore          bool   `json:"hasMore"`
	NextCursor       string `json:"nextCursor,omitempty"`
	// Cache metadata
	Cached   bool  `json:"cached,omitempty"`   // True if response was served from cache
	CacheAge int64 `json:"cacheAgeMs,omitempty"` // Age of cached response in milliseconds
}

// PaginationCursor represents the decoded pagination cursor
type PaginationCursor struct {
	LastKind string `json:"lastKind"`
	LastName string `json:"lastName"`
}
