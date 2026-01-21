//go:build disabled

package gathering

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
	spectretools "github.com/moolen/spectre/internal/agent/tools"
)

// =============================================================================
// ADK Tool Wrappers for Existing Spectre Tools
// =============================================================================

// SpectreToolWrapper wraps an existing Spectre tool as an ADK tool.
type SpectreToolWrapper struct {
	spectreTool spectretools.Tool
}

// WrapSpectreTool creates an ADK tool from an existing Spectre tool.
func WrapSpectreTool(t spectretools.Tool) (tool.Tool, error) {
	wrapper := &SpectreToolWrapper{spectreTool: t}
	return functiontool.New(functiontool.Config{
		Name:        t.Name(),
		Description: t.Description(),
	}, wrapper.execute)
}

// execute is the handler that bridges Spectre tools to ADK.
func (w *SpectreToolWrapper) execute(ctx tool.Context, args map[string]any) (map[string]any, error) {
	// Convert args to json.RawMessage for Spectre tools
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("failed to marshal args: %v", err)}, nil
	}

	// Execute the Spectre tool
	result, err := w.spectreTool.Execute(context.Background(), argsJSON)
	if err != nil {
		return map[string]any{"error": fmt.Sprintf("tool execution failed: %v", err)}, nil
	}

	// Convert result to map for ADK
	if !result.Success {
		return map[string]any{
			"success": false,
			"error":   result.Error,
		}, nil
	}

	// Serialize and deserialize to convert to map[string]any
	dataJSON, err := json.Marshal(result.Data)
	if err != nil {
		return map[string]any{
			"success": true,
			"summary": result.Summary,
			"data":    fmt.Sprintf("%v", result.Data),
		}, nil
	}

	var dataMap map[string]any
	if err := json.Unmarshal(dataJSON, &dataMap); err != nil {
		return map[string]any{
			"success": true,
			"summary": result.Summary,
			"data":    string(dataJSON),
		}, nil
	}

	return map[string]any{
		"success": true,
		"summary": result.Summary,
		"data":    dataMap,
	}, nil
}

// =============================================================================
// Submit System Snapshot Tool
// =============================================================================

// SubmitSystemSnapshotArgs is the input schema for the submit_system_snapshot tool.
type SubmitSystemSnapshotArgs struct {
	// ClusterHealth contains overall cluster health status.
	ClusterHealth *ClusterHealthArg `json:"cluster_health,omitempty"`

	// AffectedResource contains details about the primary affected resource.
	AffectedResource *ResourceDetailsArg `json:"affected_resource,omitempty"`

	// CausalPaths contains potential root cause paths from Spectre's analysis.
	CausalPaths []CausalPathArg `json:"causal_paths,omitempty"`

	// Anomalies contains detected anomalies in the time window.
	Anomalies []AnomalyArg `json:"anomalies,omitempty"`

	// RecentChanges contains resource changes in the time window.
	RecentChanges []ChangeArg `json:"recent_changes,omitempty"`

	// RelatedResources contains resources related to the affected resource.
	RelatedResources []ResourceSummaryArg `json:"related_resources,omitempty"`

	// K8sEvents contains relevant Kubernetes events.
	K8sEvents []K8sEventArg `json:"k8s_events,omitempty"`

	// ToolCallCount is the number of tool calls made to gather this data.
	ToolCallCount int `json:"tool_call_count"`

	// Errors contains non-fatal errors encountered during gathering.
	Errors []string `json:"errors,omitempty"`
}

// ClusterHealthArg contains overall cluster health status.
type ClusterHealthArg struct {
	OverallStatus  string   `json:"overall_status"`
	TotalResources int      `json:"total_resources"`
	ErrorCount     int      `json:"error_count"`
	WarningCount   int      `json:"warning_count"`
	TopIssues      []string `json:"top_issues,omitempty"`
}

// ResourceDetailsArg provides detailed information about a specific resource.
type ResourceDetailsArg struct {
	Kind          string         `json:"kind"`
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	UID           string         `json:"uid"`
	Status        string         `json:"status"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	CreatedAt     string         `json:"created_at,omitempty"`
	LastUpdatedAt string         `json:"last_updated_at,omitempty"`
	Conditions    []ConditionArg `json:"conditions,omitempty"`
}

// ConditionArg summarizes a Kubernetes condition.
type ConditionArg struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastTransitionTime string `json:"last_transition_time,omitempty"`
}

// CausalPathArg summarizes a causal path.
type CausalPathArg struct {
	PathID             string  `json:"path_id"`
	RootCauseKind      string  `json:"root_cause_kind"`
	RootCauseName      string  `json:"root_cause_name"`
	RootCauseNamespace string  `json:"root_cause_namespace,omitempty"`
	RootCauseUID       string  `json:"root_cause_uid,omitempty"`
	Confidence         float64 `json:"confidence"`
	Explanation        string  `json:"explanation"`
	StepCount          int     `json:"step_count"`
	FirstAnomalyAt     string  `json:"first_anomaly_at,omitempty"`
	ChangeType         string  `json:"change_type,omitempty"`
}

// AnomalyArg summarizes a detected anomaly.
type AnomalyArg struct {
	ResourceKind      string `json:"resource_kind"`
	ResourceName      string `json:"resource_name"`
	ResourceNamespace string `json:"resource_namespace,omitempty"`
	AnomalyType       string `json:"anomaly_type"`
	Severity          string `json:"severity"`
	Summary           string `json:"summary"`
	Timestamp         string `json:"timestamp"`
}

// ChangeArg summarizes a resource change.
type ChangeArg struct {
	ResourceKind      string   `json:"resource_kind"`
	ResourceName      string   `json:"resource_name"`
	ResourceNamespace string   `json:"resource_namespace,omitempty"`
	ResourceUID       string   `json:"resource_uid,omitempty"`
	ChangeType        string   `json:"change_type"`
	ImpactScore       float64  `json:"impact_score"`
	Description       string   `json:"description"`
	Timestamp         string   `json:"timestamp"`
	ChangedFields     []string `json:"changed_fields,omitempty"`
}

// ResourceSummaryArg provides basic information about a related resource.
type ResourceSummaryArg struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid,omitempty"`
	Status    string `json:"status"`
	Relation  string `json:"relation"`
}

// K8sEventArg summarizes a Kubernetes event.
type K8sEventArg struct {
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	Type               string `json:"type"`
	Count              int    `json:"count"`
	Timestamp          string `json:"timestamp"`
	InvolvedObjectKind string `json:"involved_object_kind,omitempty"`
	InvolvedObjectName string `json:"involved_object_name,omitempty"`
}

// SubmitSystemSnapshotResult is the output of the submit_system_snapshot tool.
type SubmitSystemSnapshotResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// NewSubmitSystemSnapshotTool creates the submit_system_snapshot tool.
func NewSubmitSystemSnapshotTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "submit_system_snapshot",
		Description: `Submit the gathered system data to complete the gathering phase.
Call this tool exactly once after you have gathered sufficient data from the other tools.
Include ALL relevant data you collected from tool calls.`,
	}, submitSystemSnapshot)
}

// submitSystemSnapshot is the handler for the submit_system_snapshot tool.
func submitSystemSnapshot(ctx tool.Context, args SubmitSystemSnapshotArgs) (SubmitSystemSnapshotResult, error) {
	// Convert tool args to SystemSnapshot
	snapshot := types.SystemSnapshot{
		GatheredAt:    time.Now(),
		ToolCallCount: args.ToolCallCount,
		Errors:        args.Errors,
	}

	// Convert cluster health
	if args.ClusterHealth != nil {
		snapshot.ClusterHealth = &types.ClusterHealthSummary{
			OverallStatus:  args.ClusterHealth.OverallStatus,
			TotalResources: args.ClusterHealth.TotalResources,
			ErrorCount:     args.ClusterHealth.ErrorCount,
			WarningCount:   args.ClusterHealth.WarningCount,
			TopIssues:      args.ClusterHealth.TopIssues,
		}
	}

	// Convert affected resource
	if args.AffectedResource != nil {
		snapshot.AffectedResource = &types.ResourceDetails{
			Kind:          args.AffectedResource.Kind,
			Namespace:     args.AffectedResource.Namespace,
			Name:          args.AffectedResource.Name,
			UID:           args.AffectedResource.UID,
			Status:        args.AffectedResource.Status,
			ErrorMessage:  args.AffectedResource.ErrorMessage,
			CreatedAt:     args.AffectedResource.CreatedAt,
			LastUpdatedAt: args.AffectedResource.LastUpdatedAt,
		}
		for _, c := range args.AffectedResource.Conditions {
			snapshot.AffectedResource.Conditions = append(snapshot.AffectedResource.Conditions, types.ConditionSummary{
				Type:               c.Type,
				Status:             c.Status,
				Reason:             c.Reason,
				Message:            c.Message,
				LastTransitionTime: c.LastTransitionTime,
			})
		}
	}

	// Convert causal paths
	for _, cp := range args.CausalPaths {
		snapshot.CausalPaths = append(snapshot.CausalPaths, types.CausalPathSummary{
			PathID:             cp.PathID,
			RootCauseKind:      cp.RootCauseKind,
			RootCauseName:      cp.RootCauseName,
			RootCauseNamespace: cp.RootCauseNamespace,
			RootCauseUID:       cp.RootCauseUID,
			Confidence:         cp.Confidence,
			Explanation:        cp.Explanation,
			StepCount:          cp.StepCount,
			FirstAnomalyAt:     cp.FirstAnomalyAt,
			ChangeType:         cp.ChangeType,
		})
	}

	// Convert anomalies
	for _, a := range args.Anomalies {
		snapshot.Anomalies = append(snapshot.Anomalies, types.AnomalySummary{
			ResourceKind:      a.ResourceKind,
			ResourceName:      a.ResourceName,
			ResourceNamespace: a.ResourceNamespace,
			AnomalyType:       a.AnomalyType,
			Severity:          a.Severity,
			Summary:           a.Summary,
			Timestamp:         a.Timestamp,
		})
	}

	// Convert recent changes
	for _, c := range args.RecentChanges {
		snapshot.RecentChanges = append(snapshot.RecentChanges, types.ChangeSummary{
			ResourceKind:      c.ResourceKind,
			ResourceName:      c.ResourceName,
			ResourceNamespace: c.ResourceNamespace,
			ResourceUID:       c.ResourceUID,
			ChangeType:        c.ChangeType,
			ImpactScore:       c.ImpactScore,
			Description:       c.Description,
			Timestamp:         c.Timestamp,
			ChangedFields:     c.ChangedFields,
		})
	}

	// Convert related resources
	for _, r := range args.RelatedResources {
		snapshot.RelatedResources = append(snapshot.RelatedResources, types.ResourceSummary{
			Kind:      r.Kind,
			Namespace: r.Namespace,
			Name:      r.Name,
			UID:       r.UID,
			Status:    r.Status,
			Relation:  r.Relation,
		})
	}

	// Convert K8s events
	for _, e := range args.K8sEvents {
		snapshot.K8sEvents = append(snapshot.K8sEvents, types.K8sEventSummary{
			Reason:             e.Reason,
			Message:            e.Message,
			Type:               e.Type,
			Count:              e.Count,
			Timestamp:          e.Timestamp,
			InvolvedObjectKind: e.InvolvedObjectKind,
			InvolvedObjectName: e.InvolvedObjectName,
		})
	}

	// Serialize to JSON
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return SubmitSystemSnapshotResult{
			Status:  "error",
			Message: fmt.Sprintf("failed to serialize system snapshot: %v", err),
		}, err
	}

	// Write to session state for the next agent
	actions := ctx.Actions()
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[types.StateKeySystemSnapshot] = string(snapshotJSON)
	actions.StateDelta[types.StateKeyPipelineStage] = types.PipelineStageGathering

	// Don't escalate - let the SequentialAgent continue to the next stage
	actions.SkipSummarization = true

	return SubmitSystemSnapshotResult{
		Status:  "success",
		Message: fmt.Sprintf("Gathered data with %d tool calls, %d causal paths, %d changes, %d anomalies", args.ToolCallCount, len(args.CausalPaths), len(args.RecentChanges), len(args.Anomalies)),
	}, nil
}
