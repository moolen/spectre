package analysis

import (
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
	PatternConfigMapNotFound             = "ConfigMapNotFound"
	PatternSecretNotFound                = "SecretNotFound"
	PatternVolumeMountFailed             = "VolumeMountFailed"
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
