package analysis

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const fieldNameReason = "reason"

// ============================================================================
// RESOURCE-AWARE PATTERN DETECTION
// ============================================================================
// This module detects significant state transitions and patterns in Kubernetes
// resources by analyzing EventDiff arrays. It identifies critical changes like
// container crashes, OOMKills, probe failures, and other failure indicators.

// DetectedPattern represents a significant pattern found in a resource change.
type DetectedPattern struct {
	Type        string  `json:"type"`        // Pattern type (e.g., "ContainerTerminated", "OOMKilled")
	Severity    float64 `json:"severity"`    // Impact score (0.0-1.0)
	Description string  `json:"description"` // Human-readable description
	Path        string  `json:"path"`        // JSON path where pattern was detected
}

// Pattern types and their base severities
const (
	// Container state patterns
	PatternContainerOOMKilled        = "ContainerOOMKilled"
	PatternContainerCrashed          = "ContainerCrashed"
	PatternContainerTerminated       = "ContainerTerminated"
	PatternContainerImagePullFailed  = "ContainerImagePullFailed"
	PatternContainerCrashLoopBackOff = "ContainerCrashLoopBackOff"
	PatternContainerStartFailed      = "ContainerStartFailed"

	// Probe patterns
	PatternLivenessProbeFailure  = "LivenessProbeFailure"
	PatternReadinessProbeFailure = "ReadinessProbeFailure"
	PatternStartupProbeFailure   = "StartupProbeFailure"

	// Resource patterns
	PatternPodEvicted          = "PodEvicted"
	PatternPodUnschedulable    = "PodUnschedulable"
	PatternPodFailedScheduling = "PodFailedScheduling"

	// Configuration patterns
	PatternConfigMapNotFound  = "ConfigMapNotFound"
	PatternSecretNotFound     = "SecretNotFound"
	PatternVolumeMountFailed  = "VolumeMountFailed"
	PatternPersistentVolumeClaimNotBound = "PersistentVolumeClaimNotBound"

	// Deployment/ReplicaSet patterns
	PatternReplicasUnavailable = "ReplicasUnavailable"
	PatternRolloutStalled      = "RolloutStalled"
)

// Exit code meanings (POSIX and Kubernetes conventions)
var exitCodeMeanings = map[int32]string{
	0:   "Success",
	1:   "General Error",
	2:   "Misuse of shell builtin",
	126: "Command cannot execute",
	127: "Command not found",
	128: "Invalid exit code",
	130: "Terminated by Ctrl+C (SIGINT)",
	137: "Killed (SIGKILL) - often OOMKilled",
	139: "Segmentation fault (SIGSEGV)",
	143: "Terminated (SIGTERM)",
	255: "Exit code out of range",
}

// DetectResourcePatterns analyzes an event's diffs and full snapshot to detect
// significant resource-specific patterns.
func DetectResourcePatterns(event *ChangeEventInfo, resourceKind string) []DetectedPattern {
	if event == nil {
		return nil
	}

	var patterns []DetectedPattern

	// Detect patterns based on resource kind
	switch strings.ToLower(resourceKind) {
	case "pod":
		patterns = append(patterns, detectPodPatterns(event)...)
	case "deployment":
		patterns = append(patterns, detectDeploymentPatterns(event)...)
	case "replicaset":
		patterns = append(patterns, detectReplicaSetPatterns(event)...)
	case "statefulset":
		patterns = append(patterns, detectStatefulSetPatterns(event)...)
	}

	return patterns
}

// ============================================================================
// POD PATTERN DETECTION
// ============================================================================

// detectPodPatterns detects patterns specific to Pod resources.
func detectPodPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	// Analyze diffs for container state changes
	for _, diff := range event.Diff {
		patterns = append(patterns, analyzeContainerStateDiff(diff)...)
		patterns = append(patterns, analyzeProbeFailures(diff)...)
		patterns = append(patterns, analyzeVolumeIssues(diff)...)
	}

	// Analyze full snapshot if available (for CREATE events)
	if event.FullSnapshot != nil {
		patterns = append(patterns, analyzePodStatus(event.FullSnapshot)...)
	}

	return patterns
}

// analyzeContainerStateDiff detects container state transitions in diffs.
func analyzeContainerStateDiff(diff EventDiff) []DetectedPattern {
	var patterns []DetectedPattern

	// Match paths like: status.containerStatuses.0.state.terminated.exitCode
	// or: status.initContainerStatuses.1.state.waiting.reason
	containerStateRegex := regexp.MustCompile(`^status\.(container|initContainer)Statuses\.(\d+)\.state\.(\w+)\.?(.*)$`)
	matches := containerStateRegex.FindStringSubmatch(diff.Path)

	if len(matches) == 0 {
		return nil
	}

	containerType := matches[1]  // "container" or "initContainer"
	containerIdx := matches[2]   // index
	stateType := matches[3]      // "terminated", "waiting", "running"
	stateField := matches[4]     // "exitCode", "reason", etc.

	// Detect terminated state with exit codes
	if stateType == "terminated" && stateField == "exitCode" {
		if exitCode, ok := diff.NewValue.(float64); ok {
			exitCodeInt := int32(exitCode)
			pattern := analyzeExitCode(exitCodeInt, containerType, containerIdx, diff.Path)
			if pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	// Detect terminated state with reason
	if stateType == "terminated" && stateField == fieldNameReason {
		if reason, ok := diff.NewValue.(string); ok {
			pattern := analyzeTerminationReason(reason, containerType, containerIdx, diff.Path)
			if pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	// Detect waiting state with failure reasons
	if stateType == "waiting" && stateField == fieldNameReason {
		if reason, ok := diff.NewValue.(string); ok {
			pattern := analyzeWaitingReason(reason, containerType, containerIdx, diff.Path)
			if pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	// Detect state transition from running to terminated
	if stateType == "running" && diff.Op == "remove" {
		// Container was running and is no longer running
		patterns = append(patterns, DetectedPattern{
			Type:        PatternContainerTerminated,
			Severity:    0.30,
			Description: fmt.Sprintf("%s container [%s] stopped running", containerType, containerIdx),
			Path:        diff.Path,
		})
	}

	return patterns
}

// analyzeExitCode interprets exit codes and creates appropriate patterns.
func analyzeExitCode(exitCode int32, containerType, containerIdx, path string) *DetectedPattern {
	if exitCode == 0 {
		return nil // Successful exit
	}

	// Exit code 137 is typically OOMKilled
	if exitCode == 137 {
		return &DetectedPattern{
			Type:        PatternContainerOOMKilled,
			Severity:    0.50,
			Description: fmt.Sprintf("%s container [%s] was OOMKilled (exit code 137)", containerType, containerIdx),
			Path:        path,
		}
	}

	// Exit code 139 is segmentation fault
	if exitCode == 139 {
		return &DetectedPattern{
			Type:        PatternContainerCrashed,
			Severity:    0.45,
			Description: fmt.Sprintf("%s container [%s] crashed with segmentation fault (exit code 139)", containerType, containerIdx),
			Path:        path,
		}
	}

	// Other non-zero exit codes
	meaning := exitCodeMeanings[exitCode]
	if meaning == "" {
		meaning = "Unknown error"
	}

	return &DetectedPattern{
		Type:        PatternContainerCrashed,
		Severity:    0.40,
		Description: fmt.Sprintf("%s container [%s] exited with code %d (%s)", containerType, containerIdx, exitCode, meaning),
		Path:        path,
	}
}

// analyzeTerminationReason interprets termination reasons.
func analyzeTerminationReason(reason, containerType, containerIdx, path string) *DetectedPattern {
	reasonLower := strings.ToLower(reason)

	if strings.Contains(reasonLower, "oomkilled") {
		return &DetectedPattern{
			Type:        PatternContainerOOMKilled,
			Severity:    0.50,
			Description: fmt.Sprintf("%s container [%s] was OOMKilled", containerType, containerIdx),
			Path:        path,
		}
	}

	if strings.Contains(reasonLower, "error") || strings.Contains(reasonLower, "failed") {
		return &DetectedPattern{
			Type:        PatternContainerCrashed,
			Severity:    0.40,
			Description: fmt.Sprintf("%s container [%s] terminated: %s", containerType, containerIdx, reason),
			Path:        path,
		}
	}

	// Non-error termination (e.g., "Completed")
	if strings.Contains(reasonLower, "completed") {
		return nil
	}

	return &DetectedPattern{
		Type:        PatternContainerTerminated,
		Severity:    0.25,
		Description: fmt.Sprintf("%s container [%s] terminated: %s", containerType, containerIdx, reason),
		Path:        path,
	}
}

// analyzeWaitingReason interprets waiting state reasons.
func analyzeWaitingReason(reason, containerType, containerIdx, path string) *DetectedPattern {
	reasonLower := strings.ToLower(reason)

	if strings.Contains(reasonLower, "crashloopbackoff") {
		return &DetectedPattern{
			Type:        PatternContainerCrashLoopBackOff,
			Severity:    0.50,
			Description: fmt.Sprintf("%s container [%s] in CrashLoopBackOff", containerType, containerIdx),
			Path:        path,
		}
	}

	if strings.Contains(reasonLower, "imagepullbackoff") || strings.Contains(reasonLower, "errimagepull") {
		return &DetectedPattern{
			Type:        PatternContainerImagePullFailed,
			Severity:    0.45,
			Description: fmt.Sprintf("%s container [%s] cannot pull image: %s", containerType, containerIdx, reason),
			Path:        path,
		}
	}

	if strings.Contains(reasonLower, "createcontainererror") || strings.Contains(reasonLower, "startcontainererror") {
		return &DetectedPattern{
			Type:        PatternContainerStartFailed,
			Severity:    0.40,
			Description: fmt.Sprintf("%s container [%s] failed to start: %s", containerType, containerIdx, reason),
			Path:        path,
		}
	}

	return nil
}

// analyzeProbeFailures detects probe failure patterns.
func analyzeProbeFailures(diff EventDiff) []DetectedPattern {
	var patterns []DetectedPattern

	// Match paths like: status.conditions.X.type or status.conditions.X.status
	conditionRegex := regexp.MustCompile(`^status\.conditions\.(\d+)\.(type|status|reason)$`)
	matches := conditionRegex.FindStringSubmatch(diff.Path)

	if len(matches) == 0 {
		return nil
	}

	conditionField := matches[2]

	// Look for probe failures in condition reasons
	if conditionField == fieldNameReason {
		if reason, ok := diff.NewValue.(string); ok {
			reasonLower := strings.ToLower(reason)

			if strings.Contains(reasonLower, "livenessprobe") {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternLivenessProbeFailure,
					Severity:    0.40,
					Description: fmt.Sprintf("Liveness probe failed: %s", reason),
					Path:        diff.Path,
				})
			}

			if strings.Contains(reasonLower, "readinessprobe") {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReadinessProbeFailure,
					Severity:    0.30,
					Description: fmt.Sprintf("Readiness probe failed: %s", reason),
					Path:        diff.Path,
				})
			}

			if strings.Contains(reasonLower, "startupprobe") {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternStartupProbeFailure,
					Severity:    0.35,
					Description: fmt.Sprintf("Startup probe failed: %s", reason),
					Path:        diff.Path,
				})
			}
		}
	}

	// Look for PodScheduled condition failures
	if conditionField == "type" && diff.NewValue == "PodScheduled" {
		// Check if status is False (would be in a sibling diff)
		patterns = append(patterns, DetectedPattern{
			Type:        PatternPodFailedScheduling,
			Severity:    0.35,
			Description: "Pod scheduling condition changed",
			Path:        diff.Path,
		})
	}

	return patterns
}

// analyzeVolumeIssues detects volume mount and claim issues.
func analyzeVolumeIssues(diff EventDiff) []DetectedPattern {
	var patterns []DetectedPattern

	path := strings.ToLower(diff.Path)

	// Detect volume mount failures in container status
	if strings.Contains(path, "status.containerstatus") && strings.Contains(path, "state.waiting") {
		if reason, ok := diff.NewValue.(string); ok {
			reasonLower := strings.ToLower(reason)
			if strings.Contains(reasonLower, "volumemount") || strings.Contains(reasonLower, "volume") {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternVolumeMountFailed,
					Severity:    0.35,
					Description: fmt.Sprintf("Volume mount issue: %s", reason),
					Path:        diff.Path,
				})
			}
		}
	}

	// Detect PVC binding issues in conditions
	if strings.Contains(path, "status.conditions") && strings.Contains(path, "reason") {
		if reason, ok := diff.NewValue.(string); ok {
			reasonLower := strings.ToLower(reason)
			if strings.Contains(reasonLower, "persistentvolumeclaim") || strings.Contains(reasonLower, "pvc") {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternPersistentVolumeClaimNotBound,
					Severity:    0.30,
					Description: fmt.Sprintf("PVC binding issue: %s", reason),
					Path:        diff.Path,
				})
			}
		}
	}

	return patterns
}

// analyzePodStatus analyzes the full pod status from a snapshot.
func analyzePodStatus(snapshot map[string]any) []DetectedPattern {
	var patterns []DetectedPattern

	status, ok := snapshot["status"].(map[string]any)
	if !ok {
		return nil
	}

	// Check pod phase
	if phase, ok := status["phase"].(string); ok {
		phaseLower := strings.ToLower(phase)
		if phaseLower == "failed" {
			patterns = append(patterns, DetectedPattern{
				Type:        PatternContainerCrashed,
				Severity:    0.40,
				Description: "Pod is in Failed phase",
				Path:        "status.phase",
			})
		}
	}

	// Check for eviction in reason
	if reason, ok := status["reason"].(string); ok {
		reasonLower := strings.ToLower(reason)
		if strings.Contains(reasonLower, "evicted") {
			patterns = append(patterns, DetectedPattern{
				Type:        PatternPodEvicted,
				Severity:    0.45,
				Description: fmt.Sprintf("Pod was evicted: %s", reason),
				Path:        "status.reason",
			})
		}
	}

	return patterns
}

// ============================================================================
// DEPLOYMENT PATTERN DETECTION
// ============================================================================

// detectDeploymentPatterns detects patterns specific to Deployment resources.
func detectDeploymentPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		// Detect replicas unavailable
		if diff.Path == "status.unavailableReplicas" {
			if unavailable, ok := diff.NewValue.(float64); ok && unavailable > 0 {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReplicasUnavailable,
					Severity:    0.35,
					Description: fmt.Sprintf("Deployment has %d unavailable replicas", int(unavailable)),
					Path:        diff.Path,
				})
			}
		}

		// Detect rollout issues in conditions
		if strings.Contains(diff.Path, "status.conditions") && strings.Contains(diff.Path, "reason") {
			if reason, ok := diff.NewValue.(string); ok {
				reasonLower := strings.ToLower(reason)
				if strings.Contains(reasonLower, "progressdeadlineexceeded") {
					patterns = append(patterns, DetectedPattern{
						Type:        PatternRolloutStalled,
						Severity:    0.40,
						Description: "Deployment rollout stalled: progress deadline exceeded",
						Path:        diff.Path,
					})
				}
			}
		}
	}

	return patterns
}

// ============================================================================
// REPLICASET PATTERN DETECTION
// ============================================================================

// detectReplicaSetPatterns detects patterns specific to ReplicaSet resources.
func detectReplicaSetPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		// Detect replicas not ready
		if diff.Path == "status.readyReplicas" {
			if ready, ok := diff.NewValue.(float64); ok && ready == 0 {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReplicasUnavailable,
					Severity:    0.30,
					Description: "ReplicaSet has 0 ready replicas",
					Path:        diff.Path,
				})
			}
		}
	}

	return patterns
}

// ============================================================================
// STATEFULSET PATTERN DETECTION
// ============================================================================

// detectStatefulSetPatterns detects patterns specific to StatefulSet resources.
func detectStatefulSetPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		// Detect replicas not ready
		if diff.Path == "status.readyReplicas" {
			if ready, ok := diff.NewValue.(float64); ok && ready == 0 {
				patterns = append(patterns, DetectedPattern{
					Type:        PatternReplicasUnavailable,
					Severity:    0.35,
					Description: "StatefulSet has 0 ready replicas",
					Path:        diff.Path,
				})
			}
		}

		// Detect update issues
		if strings.Contains(diff.Path, "status.conditions") && strings.Contains(diff.Path, "reason") {
			if reason, ok := diff.NewValue.(string); ok {
				reasonLower := strings.ToLower(reason)
				if strings.Contains(reasonLower, "blocked") || strings.Contains(reasonLower, "failed") {
					patterns = append(patterns, DetectedPattern{
						Type:        PatternRolloutStalled,
						Severity:    0.40,
						Description: fmt.Sprintf("StatefulSet update issue: %s", reason),
						Path:        diff.Path,
					})
				}
			}
		}
	}

	return patterns
}

// ============================================================================
// PATTERN UTILITIES
// ============================================================================

// GetHighestSeverityPattern returns the pattern with the highest severity.
func GetHighestSeverityPattern(patterns []DetectedPattern) *DetectedPattern {
	if len(patterns) == 0 {
		return nil
	}

	highest := &patterns[0]
	for i := range patterns {
		if patterns[i].Severity > highest.Severity {
			highest = &patterns[i]
		}
	}
	return highest
}

// ExtractContainerIndexFromPath extracts the container index from a path like
// "status.containerStatuses.2.state.terminated.exitCode" and returns "2".
func ExtractContainerIndexFromPath(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		if (part == "containerStatuses" || part == "initContainerStatuses") && i+1 < len(parts) {
			if _, err := strconv.Atoi(parts[i+1]); err == nil {
				return parts[i+1]
			}
		}
	}
	return ""
}
