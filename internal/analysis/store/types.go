package store

import (
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// ResourceWithDistance represents one node in an ownership chain.
type ResourceWithDistance struct {
	Resource graph.ResourceIdentity
	Distance int
}

// ManagerData contains manager relationship data for a resource.
type ManagerData struct {
	Manager     graph.ResourceIdentity
	ManagesEdge graph.ManagesEdge
}

// RelatedResourceData contains related resource metadata and timeline context.
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
	Namespace          string
	Timestamp          int64
	IncludeAnomalies   bool
	IncludeCausalPaths bool
	Lookback           time.Duration
	MaxDepth           int
	Limit              int
	Cursor             string
}

// NamespaceGraphData contains graph topology and metadata at a point in time.
type NamespaceGraphData struct {
	Graph       NamespaceGraph
	Anomalies   []map[string]any
	CausalPaths []map[string]any
	Metadata    NamespaceGraphMetadata
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

// NamespaceGraphChangeEvent represents the latest event for a graph node.
type NamespaceGraphChangeEvent struct {
	Timestamp       int64
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
	Timestamp        int64
	NodeCount        int
	EdgeCount        int
	QueryExecutionMs int64
	HasMore          bool
	NextCursor       string
	Cached           bool
	CacheAgeMs       int64
}
