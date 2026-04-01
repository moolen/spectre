package store

import (
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// ResourceWithDistance represents one node in an ownership chain.
// Resource intentionally uses graph.ResourceIdentity as the canonical resource
// identity type shared at the store boundary.
type ResourceWithDistance struct {
	Resource graph.ResourceIdentity
	// Distance is ownership-chain distance from the symptom resource:
	// symptom is 0, direct owner is 1, and distances increase upstream.
	Distance int
}

// ManagerData contains manager relationship data for a resource.
// Manager intentionally uses graph.ResourceIdentity as the canonical resource
// identity type shared at the store boundary.
type ManagerData struct {
	Manager     graph.ResourceIdentity
	ManagesEdge graph.ManagesEdge
}

// RelatedResourceData contains related resource metadata and timeline context.
// Resource intentionally uses graph.ResourceIdentity as the canonical resource
// identity type shared at the store boundary.
type RelatedResourceData struct {
	Resource           graph.ResourceIdentity
	RelationshipType   string
	Events             []ChangeEventInfo
	ReferenceTargetUID string
}

// EventSignificance scores and explains event relevance.
type EventSignificance struct {
	Score   float64
	Reasons []string
}

// EventDiff represents one JSON-level change in an event diff.
type EventDiff struct {
	Path     string
	OldValue any
	NewValue any
	Op       string
}

// ChangeEventInfo represents a resource lifecycle change event.
type ChangeEventInfo struct {
	EventID       string
	// Timestamp is normalized to time.Time by store implementations.
	Timestamp     time.Time
	EventType     string
	Status        string
	ConfigChanged bool
	StatusChanged bool
	Description   string
	Significance  *EventSignificance
	Diff          []EventDiff
	FullSnapshot  map[string]any
	Data          []byte
}

// K8sEventInfo represents a Kubernetes Event associated with a resource.
type K8sEventInfo struct {
	EventID      string
	// Timestamp is normalized to time.Time by store implementations.
	Timestamp    time.Time
	Reason       string
	Message      string
	Type         string
	Count        int
	Source       string
	Significance *EventSignificance
}

// NamespaceGraphQuery captures inputs for namespace graph retrieval.
type NamespaceGraphQuery struct {
	Namespace string
	// TimestampNs is the point-in-time graph view in Unix nanoseconds.
	TimestampNs int64
	// LookbackNs is the temporal query window in nanoseconds.
	LookbackNs int64
	MaxDepth   int
	Limit      int
	// Cursor is an opaque pagination token returned by a previous result page.
	Cursor string
}

// NamespaceGraphData contains store-level read-model data used to assemble the
// namespace graph view and related metadata.
type NamespaceGraphData struct {
	Graph    NamespaceGraph
	Metadata NamespaceGraphMetadata
}

// NamespaceGraph groups resources and relationships.
type NamespaceGraph struct {
	Nodes []NamespaceGraphNode
	Edges []NamespaceGraphEdge
}

// NamespaceGraphNode represents one resource in the namespace graph.
type NamespaceGraphNode struct {
	UID         string
	Kind        string
	APIGroup    string
	Namespace   string
	Name        string
	Status      string
	LatestEvent *NamespaceGraphChangeEvent
	Labels      map[string]string
}

// NamespaceGraphChangeEvent represents latest persisted/derived event fields
// needed by namespace-graph consumers. Analyzer/service-level enrichments
// (for example anomalies and causal paths) are out of scope for store results.
type NamespaceGraphChangeEvent struct {
	// TimestampNs is the event timestamp in Unix nanoseconds.
	TimestampNs     int64
	EventType       string
	Status          string
	ErrorMessage    string
	ContainerIssues []string
	ImpactScore     float64
	SpecChanges     string
	SpecReplicas    *int
}

// NamespaceGraphEdge represents a relationship between resources.
type NamespaceGraphEdge struct {
	ID               string
	Source           string
	Target           string
	RelationshipType string
}

// NamespaceGraphMetadata carries response and pagination metadata.
type NamespaceGraphMetadata struct {
	Namespace        string
	// TimestampNs is the graph evaluation point in Unix nanoseconds.
	TimestampNs      int64
	NodeCount        int
	EdgeCount        int
	QueryExecutionMs int64
	HasMore          bool
	NextCursor       string
	Cached           bool
	CacheAgeMs       int64
}
