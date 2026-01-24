package graph

import (
	"encoding/json"
	"time"
)

// NodeType represents the type of graph node
type NodeType string

const (
	NodeTypeResourceIdentity NodeType = "ResourceIdentity"
	NodeTypeChangeEvent      NodeType = "ChangeEvent"
	NodeTypeK8sEvent         NodeType = "K8sEvent"
	NodeTypeDashboard        NodeType = "Dashboard"
	NodeTypePanel            NodeType = "Panel"
	NodeTypeQuery            NodeType = "Query"
	NodeTypeMetric           NodeType = "Metric"
	NodeTypeService          NodeType = "Service"
	NodeTypeVariable         NodeType = "Variable"
	NodeTypeAlert            NodeType = "Alert"
)

// EdgeType represents the type of graph edge
type EdgeType string

const (
	EdgeTypeOwns               EdgeType = "OWNS"
	EdgeTypeChanged            EdgeType = "CHANGED"
	EdgeTypeSelects            EdgeType = "SELECTS"
	EdgeTypeScheduledOn        EdgeType = "SCHEDULED_ON"
	EdgeTypeMounts             EdgeType = "MOUNTS"
	EdgeTypeUsesServiceAccount EdgeType = "USES_SERVICE_ACCOUNT"
	EdgeTypeEmittedEvent       EdgeType = "EMITTED_EVENT"

	// RBAC relationship types
	EdgeTypeBindsRole EdgeType = "BINDS_ROLE" // RoleBinding/ClusterRoleBinding → Role/ClusterRole
	EdgeTypeGrantsTo  EdgeType = "GRANTS_TO"  // RoleBinding/ClusterRoleBinding → ServiceAccount/User/Group

	// Custom Resource relationship types (inferred with confidence)
	EdgeTypeReferencesSpec  EdgeType = "REFERENCES_SPEC"  // Explicit spec references
	EdgeTypeManages         EdgeType = "MANAGES"          // Lifecycle management (inferred)
	EdgeTypeCreatesObserved EdgeType = "CREATES_OBSERVED" // Observed creation correlation

	// Dashboard relationship types
	EdgeTypeContains    EdgeType = "CONTAINS"     // Dashboard -> Panel
	EdgeTypeHas         EdgeType = "HAS"          // Panel -> Query
	EdgeTypeUses        EdgeType = "USES"         // Query -> Metric
	EdgeTypeTracks      EdgeType = "TRACKS"       // Metric -> Service
	EdgeTypeHasVariable EdgeType = "HAS_VARIABLE" // Dashboard -> Variable
	EdgeTypeMonitors    EdgeType = "MONITORS"     // Alert -> Metric/Service
)

// ResourceIdentity represents a persistent Kubernetes resource node
type ResourceIdentity struct {
	UID       string            `json:"uid"`       // K8s UID (primary key)
	Kind      string            `json:"kind"`      // e.g., "Pod", "Deployment"
	APIGroup  string            `json:"apiGroup"`  // e.g., "apps", "" for core
	Version   string            `json:"version"`   // e.g., "v1"
	Namespace string            `json:"namespace"` // empty for cluster-scoped
	Name      string            `json:"name"`      // resource name
	Labels    map[string]string `json:"labels"`    // resource labels
	FirstSeen int64             `json:"firstSeen"` // Unix nanoseconds
	LastSeen  int64             `json:"lastSeen"`  // Unix nanoseconds
	Deleted   bool              `json:"deleted"`   // true if resource was deleted
	DeletedAt int64             `json:"deletedAt"` // Unix nanoseconds (if deleted)
}

// ChangeEvent represents a state change event node
type ChangeEvent struct {
	ID              string   `json:"id"`              // Spectre Event.ID
	Timestamp       int64    `json:"timestamp"`       // Unix nanoseconds
	EventType       string   `json:"eventType"`       // CREATE, UPDATE, DELETE
	Status          string   `json:"status"`          // Ready, Warning, Error, Terminating, Unknown
	ErrorMessage    string   `json:"errorMessage"`    // extracted error (optional)
	ContainerIssues []string `json:"containerIssues"` // CrashLoopBackOff, ImagePullBackOff, OOMKilled
	ConfigChanged   bool     `json:"configChanged"`   // spec changed
	StatusChanged   bool     `json:"statusChanged"`   // status changed
	ReplicasChanged bool     `json:"replicasChanged"` // for controllers
	ImpactScore     float64  `json:"impactScore"`     // 0.0-1.0
	Data            string   `json:"data,omitempty"`  // Full resource JSON (for timeline reconstruction)
}

// K8sEvent represents a Kubernetes Event object node
type K8sEvent struct {
	ID        string `json:"id"`        // Event ID
	Timestamp int64  `json:"timestamp"` // Unix nanoseconds
	Reason    string `json:"reason"`    // e.g., "FailedScheduling"
	Message   string `json:"message"`   // event message
	Type      string `json:"type"`      // Warning, Normal, Error
	Count     int    `json:"count"`     // event count (if repeated)
	Source    string `json:"source"`    // component that generated event
}

// AlertNode represents a Grafana Alert Rule node in the graph
type AlertNode struct {
	UID         string            `json:"uid"`         // Alert rule UID (primary key)
	Title       string            `json:"title"`       // Alert rule title
	FolderTitle string            `json:"folderTitle"` // Folder containing the rule
	RuleGroup   string            `json:"ruleGroup"`   // Alert rule group name
	Condition   string            `json:"condition"`   // PromQL expression (stored for display, parsed separately)
	Labels      map[string]string `json:"labels"`      // Alert labels
	Annotations map[string]string `json:"annotations"` // Alert annotations including severity
	Updated     string            `json:"updated"`     // ISO8601 timestamp for incremental sync
	Integration string            `json:"integration"` // Integration name (e.g., "grafana_prod")
}

// DashboardNode represents a Grafana Dashboard node in the graph
type DashboardNode struct {
	UID       string   `json:"uid"`       // Dashboard UID (primary key)
	Title     string   `json:"title"`     // Dashboard title
	Version   int      `json:"version"`   // Dashboard version number
	Tags      []string `json:"tags"`      // Dashboard tags
	Folder    string   `json:"folder"`    // Folder path
	URL       string   `json:"url"`       // Dashboard URL
	FirstSeen int64    `json:"firstSeen"` // Unix nano timestamp when first seen
	LastSeen  int64    `json:"lastSeen"`  // Unix nano timestamp when last seen
}

// PanelNode represents a Grafana Panel node in the graph
type PanelNode struct {
	ID           string `json:"id"`           // Unique: dashboardUID + panelID
	DashboardUID string `json:"dashboardUID"` // Parent dashboard
	Title        string `json:"title"`        // Panel title
	Type         string `json:"type"`         // Panel type (graph, table, etc.)
	GridPosX     int    `json:"gridPosX"`     // Layout position X
	GridPosY     int    `json:"gridPosY"`     // Layout position Y
}

// QueryNode represents a PromQL query node in the graph
type QueryNode struct {
	ID             string            `json:"id"`             // Unique: dashboardUID + panelID + refID
	RefID          string            `json:"refId"`          // Query reference (A, B, C, etc.)
	RawPromQL      string            `json:"rawPromQL"`      // Original PromQL
	DatasourceUID  string            `json:"datasourceUID"`  // Datasource UID
	Aggregations   []string          `json:"aggregations"`   // Extracted functions
	LabelSelectors map[string]string `json:"labelSelectors"` // Extracted matchers
	HasVariables   bool              `json:"hasVariables"`   // Contains Grafana variables
}

// MetricNode represents a Prometheus metric node in the graph
type MetricNode struct {
	Name      string `json:"name"`      // Metric name (e.g., http_requests_total)
	FirstSeen int64  `json:"firstSeen"` // Unix nano timestamp
	LastSeen  int64  `json:"lastSeen"`  // Unix nano timestamp
}

// ServiceNode represents an inferred service node in the graph
type ServiceNode struct {
	Name         string `json:"name"`         // Service name (from app/service/job labels)
	Cluster      string `json:"cluster"`      // Cluster name (scoping)
	Namespace    string `json:"namespace"`    // Namespace (scoping)
	InferredFrom string `json:"inferredFrom"` // Label used for inference (app/service/job)
	FirstSeen    int64  `json:"firstSeen"`    // Unix nano timestamp
	LastSeen     int64  `json:"lastSeen"`     // Unix nano timestamp
}

// VariableNode represents a Grafana dashboard variable node in the graph
type VariableNode struct {
	DashboardUID   string `json:"dashboardUID"`   // Parent dashboard UID
	Name           string `json:"name"`           // Variable name
	Type           string `json:"type"`           // Variable type (query/textbox/custom/interval)
	Classification string `json:"classification"` // Classification (scoping/entity/detail/unknown)
	FirstSeen      int64  `json:"firstSeen"`      // Unix nano timestamp
	LastSeen       int64  `json:"lastSeen"`       // Unix nano timestamp
}

// OwnsEdge represents ownership relationship properties
type OwnsEdge struct {
	Controller         bool `json:"controller"`         // true if ownerRef has controller: true
	BlockOwnerDeletion bool `json:"blockOwnerDeletion"` // prevents deletion
}

// ChangedEdge represents resource-to-event relationship properties
type ChangedEdge struct {
	SequenceNumber int `json:"sequenceNumber"` // order within resource timeline
}

// TriggeredByEdge represents causality inference properties
type TriggeredByEdge struct {
	Confidence float64 `json:"confidence"` // 0.0-1.0 heuristic confidence
	LagMs      int64   `json:"lagMs"`      // milliseconds between cause and effect
	Reason     string  `json:"reason"`     // human-readable explanation
}

// PrecededByEdge represents temporal ordering properties
type PrecededByEdge struct {
	DurationMs int64 `json:"durationMs"` // time between events
}

// SelectsEdge represents label selector relationship properties
type SelectsEdge struct {
	SelectorLabels map[string]string `json:"selectorLabels"` // selector used
}

// ScheduledOnEdge represents Pod-to-Node scheduling properties
type ScheduledOnEdge struct {
	ScheduledAt  int64 `json:"scheduledAt"`  // timestamp when scheduled
	TerminatedAt int64 `json:"terminatedAt"` // if Pod terminated
}

// MountsEdge represents volume mount relationship properties
type MountsEdge struct {
	VolumeName string `json:"volumeName"` // name in Pod spec
	MountPath  string `json:"mountPath"`  // mount path
}

// UsesServiceAccountEdge represents Pod-to-ServiceAccount relationship
// Currently has no additional properties beyond the relationship itself
type UsesServiceAccountEdge struct {
}

// BindsRoleEdge represents RoleBinding/ClusterRoleBinding → Role/ClusterRole relationship
type BindsRoleEdge struct {
	RoleKind string `json:"roleKind"` // "Role" or "ClusterRole"
	RoleName string `json:"roleName"` // Name of the role
}

// GrantsToEdge represents RoleBinding/ClusterRoleBinding → Subject (ServiceAccount/User/Group)
type GrantsToEdge struct {
	SubjectKind      string `json:"subjectKind"`      // "ServiceAccount", "User", or "Group"
	SubjectName      string `json:"subjectName"`      // Name of the subject
	SubjectNamespace string `json:"subjectNamespace"` // Namespace (for ServiceAccount)
}

// EvidenceType categorizes evidence for inferred relationships
type EvidenceType string

const (
	EvidenceTypeLabel      EvidenceType = "label"      // Label match
	EvidenceTypeAnnotation EvidenceType = "annotation" // Annotation match
	EvidenceTypeTemporal   EvidenceType = "temporal"   // Temporal proximity
	EvidenceTypeNamespace  EvidenceType = "namespace"  // Same namespace
	EvidenceTypeOwnership  EvidenceType = "ownership"  // OwnerReference present
	EvidenceTypeReconcile  EvidenceType = "reconcile"  // Reconcile event correlation
)

// EvidenceItem represents a piece of evidence for an inferred relationship
type EvidenceItem struct {
	Type      EvidenceType `json:"type"`      // Label, Temporal, Annotation, etc.
	Value     string       `json:"value"`     // Human-readable evidence description
	Weight    float64      `json:"weight"`    // How much this evidence contributes to confidence
	Timestamp int64        `json:"timestamp"` // When evidence was observed

	// Structured fields for evidence validation (optional, used for revalidation)
	// These provide machine-readable data in addition to the human-readable Value
	Key        string `json:"key,omitempty"`        // Label/annotation key (for label/annotation evidence)
	MatchValue string `json:"matchValue,omitempty"` // Expected value to match (for label/annotation evidence)
	SourceUID  string `json:"sourceUID,omitempty"`  // Source resource UID (for ownership evidence)
	TargetUID  string `json:"targetUID,omitempty"`  // Target resource UID (for ownership evidence)
	LagMs      int64  `json:"lagMs,omitempty"`      // Time lag in milliseconds (for temporal evidence)
	WindowMs   int64  `json:"windowMs,omitempty"`   // Time window in milliseconds (for temporal evidence)
	Namespace  string `json:"namespace,omitempty"`  // Namespace (for namespace evidence)
}

// ValidationState tracks the validation state of inferred edges
type ValidationState string

const (
	ValidationStateValid   ValidationState = "valid"   // Passes validation checks
	ValidationStateStale   ValidationState = "stale"   // Needs revalidation
	ValidationStateInvalid ValidationState = "invalid" // Failed validation
	ValidationStatePending ValidationState = "pending" // Not yet validated
)

// ReferencesSpecEdge represents explicit references in resource spec
// Example: HelmRelease → Secret (valuesFrom.secretKeyRef)
type ReferencesSpecEdge struct {
	FieldPath    string `json:"fieldPath"`              // JSONPath to the reference
	RefKind      string `json:"refKind"`                // Referenced resource kind
	RefName      string `json:"refName"`                // Referenced resource name
	RefNamespace string `json:"refNamespace,omitempty"` // Namespace (if different)
}

// ManagesEdge represents lifecycle management relationship (INFERRED)
// Example: HelmRelease → Deployment (HelmRelease manages Deployment lifecycle)
type ManagesEdge struct {
	Confidence      float64         `json:"confidence"`      // 0.0-1.0 confidence score
	Evidence        []EvidenceItem  `json:"evidence"`        // Evidence supporting this relationship
	FirstObserved   int64           `json:"firstObserved"`   // When first detected (Unix nanoseconds)
	LastValidated   int64           `json:"lastValidated"`   // Last validation timestamp
	ValidationState ValidationState `json:"validationState"` // Current validation state
}

// AnnotatesEdge represents label/annotation-based linkage
// Example: Deployment has label "helm.toolkit.fluxcd.io/name: myrelease"
type AnnotatesEdge struct {
	AnnotationKey   string  `json:"annotationKey"`   // Full annotation key
	AnnotationValue string  `json:"annotationValue"` // Annotation value
	Confidence      float64 `json:"confidence"`      // Confidence based on annotation reliability
}

// CreatesObservedEdge represents observed creation following reconcile
// Example: HelmRelease reconciled → new Pod appeared within 30s
type CreatesObservedEdge struct {
	Confidence       float64 `json:"confidence"`       // Temporal correlation confidence
	ObservedLagMs    int64   `json:"observedLagMs"`    // Time between reconcile and creation
	ReconcileEventID string  `json:"reconcileEventId"` // Event ID of triggering reconcile
	Evidence         string  `json:"evidence"`         // Why we believe this
}

// Node represents a generic graph node
type Node struct {
	Type       NodeType        `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

// Edge represents a generic graph edge
type Edge struct {
	Type       EdgeType        `json:"type"`
	FromUID    string          `json:"fromUID"`
	ToUID      string          `json:"toUID"`
	Properties json.RawMessage `json:"properties"`
}

// GraphQuery represents a Cypher query with parameters
type GraphQuery struct {
	Query      string                 `json:"query"`
	Parameters map[string]interface{} `json:"parameters"`
	Timeout    int                    `json:"timeout,omitempty"` // Timeout in milliseconds (0 = default)
}

// QueryResult represents the result of a graph query
type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Stats   QueryStats      `json:"stats"`
}

// QueryStats represents query execution statistics
type QueryStats struct {
	NodesCreated         int           `json:"nodesCreated"`
	NodesDeleted         int           `json:"nodesDeleted"`
	RelationshipsCreated int           `json:"relationshipsCreated"`
	RelationshipsDeleted int           `json:"relationshipsDeleted"`
	PropertiesSet        int           `json:"propertiesSet"`
	LabelsAdded          int           `json:"labelsAdded"`
	ExecutionTime        time.Duration `json:"executionTime"`
}

// GraphStats represents overall graph statistics
type GraphStats struct {
	NodeCount        int              `json:"nodeCount"`
	EdgeCount        int              `json:"edgeCount"`
	NodesByType      map[NodeType]int `json:"nodesByType"`
	EdgesByType      map[EdgeType]int `json:"edgesByType"`
	OldestTimestamp  int64            `json:"oldestTimestamp"` // Unix nanoseconds
	NewestTimestamp  int64            `json:"newestTimestamp"` // Unix nanoseconds
	MemoryUsageBytes int64            `json:"memoryUsageBytes"`
}
