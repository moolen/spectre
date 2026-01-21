//go:build disabled

package types

import "time"

// IncidentFacts is the output of IncidentIntakeAgent.
// It contains only facts extracted from the user's description - no speculation.
type IncidentFacts struct {
	// Symptoms describes what is failing or broken.
	Symptoms []Symptom `json:"symptoms"`

	// Timeline captures when the incident started and its duration.
	Timeline Timeline `json:"timeline"`

	// MitigationsAttempted lists what the user has already tried.
	MitigationsAttempted []Mitigation `json:"mitigations_attempted,omitempty"`

	// IsOngoing indicates whether the incident is still active.
	IsOngoing bool `json:"is_ongoing"`

	// UserConstraints captures any focus areas or exclusions the user specified.
	// Examples: "ignore network issues", "focus on the database"
	UserConstraints []string `json:"user_constraints,omitempty"`

	// AffectedResource is set if the user explicitly named a resource.
	AffectedResource *ResourceRef `json:"affected_resource,omitempty"`

	// ExtractedAt is when these facts were extracted.
	ExtractedAt time.Time `json:"extracted_at"`
}

// Symptom describes an observed problem.
type Symptom struct {
	// Description is the symptom in the user's own words.
	Description string `json:"description"`

	// Resource is the affected resource name if mentioned.
	Resource string `json:"resource,omitempty"`

	// Namespace is the Kubernetes namespace if mentioned.
	Namespace string `json:"namespace,omitempty"`

	// Kind is the Kubernetes resource kind if mentioned (Pod, Deployment, etc.).
	Kind string `json:"kind,omitempty"`

	// Severity is the assessed severity based on user language.
	// Values: critical, high, medium, low
	Severity string `json:"severity"`

	// FirstSeen is when the symptom was first observed (e.g., "10 minutes ago").
	FirstSeen string `json:"first_seen,omitempty"`
}

// Timeline captures temporal information about the incident.
type Timeline struct {
	// IncidentStart is when symptoms first appeared (in user's words).
	IncidentStart string `json:"incident_start,omitempty"`

	// UserReportedAt is when the user reported the incident to the agent.
	UserReportedAt time.Time `json:"user_reported_at"`

	// DurationStr is a human-readable duration (e.g., "ongoing for 10 minutes").
	DurationStr string `json:"duration_str,omitempty"`

	// StartTimestamp is the Unix timestamp (seconds) for the start of the investigation window.
	// This is calculated by the intake agent based on user input or defaults to now - 15 minutes.
	StartTimestamp int64 `json:"start_timestamp"`

	// EndTimestamp is the Unix timestamp (seconds) for the end of the investigation window.
	// This is typically the current time when the incident is ongoing.
	EndTimestamp int64 `json:"end_timestamp"`
}

// Mitigation describes an attempted remediation.
type Mitigation struct {
	// Description is what was tried.
	Description string `json:"description"`

	// Result is the outcome if known.
	// Values: "no effect", "partial", "unknown", "made worse"
	Result string `json:"result,omitempty"`
}

// ResourceRef identifies a specific Kubernetes resource.
type ResourceRef struct {
	// UID is the Kubernetes UID if known.
	UID string `json:"uid,omitempty"`

	// Kind is the resource kind (Pod, Deployment, Service, etc.).
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace"`

	// Name is the resource name.
	Name string `json:"name"`
}

// SystemSnapshot is the output of InformationGatheringAgent.
// It contains raw data collected from Spectre tools - no interpretation.
type SystemSnapshot struct {
	// ClusterHealth contains overall cluster health status.
	ClusterHealth *ClusterHealthSummary `json:"cluster_health,omitempty"`

	// AffectedResource contains details about the primary affected resource.
	AffectedResource *ResourceDetails `json:"affected_resource,omitempty"`

	// CausalPaths contains potential root cause paths from Spectre's analysis.
	CausalPaths []CausalPathSummary `json:"causal_paths,omitempty"`

	// Anomalies contains detected anomalies in the time window.
	Anomalies []AnomalySummary `json:"anomalies,omitempty"`

	// RecentChanges contains resource changes in the time window.
	RecentChanges []ChangeSummary `json:"recent_changes,omitempty"`

	// RelatedResources contains resources related to the affected resource.
	RelatedResources []ResourceSummary `json:"related_resources,omitempty"`

	// K8sEvents contains relevant Kubernetes events.
	K8sEvents []K8sEventSummary `json:"k8s_events,omitempty"`

	// GatheredAt is when this snapshot was collected.
	GatheredAt time.Time `json:"gathered_at"`

	// ToolCallCount is the number of tool calls made to gather this data.
	ToolCallCount int `json:"tool_call_count"`

	// Errors contains non-fatal errors encountered during gathering.
	Errors []string `json:"errors,omitempty"`
}

// ClusterHealthSummary contains overall cluster health status.
type ClusterHealthSummary struct {
	// OverallStatus is the cluster-wide health status.
	OverallStatus string `json:"overall_status"`

	// TotalResources is the total number of tracked resources.
	TotalResources int `json:"total_resources"`

	// ErrorCount is the number of resources in error state.
	ErrorCount int `json:"error_count"`

	// WarningCount is the number of resources in warning state.
	WarningCount int `json:"warning_count"`

	// TopIssues lists the most significant issues.
	TopIssues []string `json:"top_issues,omitempty"`
}

// CausalPathSummary summarizes a causal path from Spectre's root cause analysis.
type CausalPathSummary struct {
	// PathID is a unique identifier for this causal path.
	PathID string `json:"path_id"`

	// RootCauseKind is the Kubernetes kind of the root cause resource.
	RootCauseKind string `json:"root_cause_kind"`

	// RootCauseName is the name of the root cause resource.
	RootCauseName string `json:"root_cause_name"`

	// RootCauseNamespace is the namespace of the root cause resource.
	RootCauseNamespace string `json:"root_cause_namespace,omitempty"`

	// RootCauseUID is the UID of the root cause resource.
	RootCauseUID string `json:"root_cause_uid,omitempty"`

	// Confidence is Spectre's confidence in this causal path.
	Confidence float64 `json:"confidence"`

	// Explanation is a human-readable explanation of the causal chain.
	Explanation string `json:"explanation"`

	// StepCount is the number of hops in the causal path.
	StepCount int `json:"step_count"`

	// FirstAnomalyAt is when the first anomaly in this path was detected.
	FirstAnomalyAt string `json:"first_anomaly_at,omitempty"`

	// ChangeType is the type of change that triggered this path (if applicable).
	ChangeType string `json:"change_type,omitempty"`
}

// AnomalySummary summarizes a detected anomaly.
type AnomalySummary struct {
	// ResourceKind is the Kubernetes kind of the affected resource.
	ResourceKind string `json:"resource_kind"`

	// ResourceName is the name of the affected resource.
	ResourceName string `json:"resource_name"`

	// ResourceNamespace is the namespace of the affected resource.
	ResourceNamespace string `json:"resource_namespace,omitempty"`

	// AnomalyType categorizes the anomaly.
	AnomalyType string `json:"anomaly_type"`

	// Severity indicates the anomaly severity.
	Severity string `json:"severity"`

	// Summary is a brief description of the anomaly.
	Summary string `json:"summary"`

	// Timestamp is when the anomaly was detected.
	Timestamp string `json:"timestamp"`
}

// ChangeSummary summarizes a resource change.
type ChangeSummary struct {
	// ResourceKind is the Kubernetes kind of the changed resource.
	ResourceKind string `json:"resource_kind"`

	// ResourceName is the name of the changed resource.
	ResourceName string `json:"resource_name"`

	// ResourceNamespace is the namespace of the changed resource.
	ResourceNamespace string `json:"resource_namespace,omitempty"`

	// ResourceUID is the UID of the changed resource.
	ResourceUID string `json:"resource_uid,omitempty"`

	// ChangeType is the type of change (CREATE, UPDATE, DELETE).
	ChangeType string `json:"change_type"`

	// ImpactScore is Spectre's assessment of change impact (0.0-1.0).
	ImpactScore float64 `json:"impact_score"`

	// Description is a summary of what changed.
	Description string `json:"description"`

	// Timestamp is when the change occurred.
	Timestamp string `json:"timestamp"`

	// ChangedFields lists the specific fields that changed (for updates).
	ChangedFields []string `json:"changed_fields,omitempty"`
}

// ResourceSummary provides basic information about a related resource.
type ResourceSummary struct {
	// Kind is the Kubernetes resource kind.
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace"`

	// Name is the resource name.
	Name string `json:"name"`

	// UID is the resource UID.
	UID string `json:"uid,omitempty"`

	// Status is the current resource status.
	Status string `json:"status"`

	// Relation describes how this resource relates to the affected resource.
	// Values: owner, owned_by, scheduled_on, uses, used_by, etc.
	Relation string `json:"relation"`
}

// ResourceDetails provides detailed information about a specific resource.
type ResourceDetails struct {
	// Kind is the Kubernetes resource kind.
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace"`

	// Name is the resource name.
	Name string `json:"name"`

	// UID is the resource UID.
	UID string `json:"uid"`

	// Status is the current resource status.
	Status string `json:"status"`

	// ErrorMessage contains error details if the resource is failing.
	ErrorMessage string `json:"error_message,omitempty"`

	// CreatedAt is when the resource was created.
	CreatedAt string `json:"created_at,omitempty"`

	// LastUpdatedAt is when the resource was last updated.
	LastUpdatedAt string `json:"last_updated_at,omitempty"`

	// Conditions contains Kubernetes conditions for the resource.
	Conditions []ConditionSummary `json:"conditions,omitempty"`
}

// ConditionSummary summarizes a Kubernetes condition.
type ConditionSummary struct {
	// Type is the condition type.
	Type string `json:"type"`

	// Status is the condition status (True, False, Unknown).
	Status string `json:"status"`

	// Reason is a brief reason for the condition.
	Reason string `json:"reason,omitempty"`

	// Message provides additional details.
	Message string `json:"message,omitempty"`

	// LastTransitionTime is when the condition last changed.
	LastTransitionTime string `json:"last_transition_time,omitempty"`
}

// K8sEventSummary summarizes a Kubernetes event.
type K8sEventSummary struct {
	// Reason is the event reason.
	Reason string `json:"reason"`

	// Message is the event message.
	Message string `json:"message"`

	// Type is the event type (Warning, Normal).
	Type string `json:"type"`

	// Count is how many times this event occurred.
	Count int `json:"count"`

	// Timestamp is when the event occurred.
	Timestamp string `json:"timestamp"`

	// InvolvedObjectKind is the kind of the involved resource.
	InvolvedObjectKind string `json:"involved_object_kind,omitempty"`

	// InvolvedObjectName is the name of the involved resource.
	InvolvedObjectName string `json:"involved_object_name,omitempty"`
}
