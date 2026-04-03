package analysis

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	containerStateRegex = regexp.MustCompile(`^status\.(container|initContainer)Statuses\.(\d+)\.state\.(\w+)\.?(.*)$`)
	conditionRegex      = regexp.MustCompile(`^status\.conditions\.(\d+)\.(type|status|reason)$`)
)

// detectPodPatterns detects patterns specific to Pod resources.
func detectPodPatterns(event *ChangeEventInfo) []DetectedPattern {
	var patterns []DetectedPattern

	for _, diff := range event.Diff {
		patterns = append(patterns, analyzeContainerStateDiff(diff)...)
		patterns = append(patterns, analyzeProbeFailures(diff)...)
		patterns = append(patterns, analyzeVolumeIssues(diff)...)
	}

	if event.FullSnapshot != nil {
		patterns = append(patterns, analyzePodStatus(event.FullSnapshot)...)
	}

	return patterns
}

// analyzeContainerStateDiff detects container state transitions in diffs.
func analyzeContainerStateDiff(diff EventDiff) []DetectedPattern {
	var patterns []DetectedPattern
	matches := containerStateRegex.FindStringSubmatch(diff.Path)
	if len(matches) == 0 {
		return nil
	}

	containerType := matches[1]
	containerIdx := matches[2]
	stateType := matches[3]
	stateField := matches[4]

	if stateType == "terminated" && stateField == "exitCode" {
		if exitCode, ok := diff.NewValue.(float64); ok {
			pattern := analyzeExitCode(int32(exitCode), containerType, containerIdx, diff.Path)
			if pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	if stateType == "terminated" && stateField == fieldNameReason {
		if reason, ok := diff.NewValue.(string); ok {
			pattern := analyzeTerminationReason(reason, containerType, containerIdx, diff.Path)
			if pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	if stateType == "waiting" && stateField == fieldNameReason {
		if reason, ok := diff.NewValue.(string); ok {
			pattern := analyzeWaitingReason(reason, containerType, containerIdx, diff.Path)
			if pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	if stateType == "running" && diff.Op == "remove" {
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
		return nil
	}
	if exitCode == 137 {
		return &DetectedPattern{
			Type:        PatternContainerOOMKilled,
			Severity:    0.50,
			Description: fmt.Sprintf("%s container [%s] was OOMKilled (exit code 137)", containerType, containerIdx),
			Path:        path,
		}
	}
	if exitCode == 139 {
		return &DetectedPattern{
			Type:        PatternContainerCrashed,
			Severity:    0.45,
			Description: fmt.Sprintf("%s container [%s] crashed with segmentation fault (exit code 139)", containerType, containerIdx),
			Path:        path,
		}
	}

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
	matches := conditionRegex.FindStringSubmatch(diff.Path)
	if len(matches) == 0 {
		return nil
	}

	conditionField := matches[2]
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

	if conditionField == "type" && diff.NewValue == "PodScheduled" {
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

	if phase, ok := status["phase"].(string); ok && strings.ToLower(phase) == "failed" {
		patterns = append(patterns, DetectedPattern{
			Type:        PatternContainerCrashed,
			Severity:    0.40,
			Description: "Pod is in Failed phase",
			Path:        "status.phase",
		})
	}

	if reason, ok := status["reason"].(string); ok && strings.Contains(strings.ToLower(reason), "evicted") {
		patterns = append(patterns, DetectedPattern{
			Type:        PatternPodEvicted,
			Severity:    0.45,
			Description: fmt.Sprintf("Pod was evicted: %s", reason),
			Path:        "status.reason",
		})
	}

	return patterns
}
