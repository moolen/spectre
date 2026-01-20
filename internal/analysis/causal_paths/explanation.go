package causalpaths

import (
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
)

// ExplanationBuilder generates human-readable explanations for causal paths
type ExplanationBuilder struct{}

// NewExplanationBuilder creates a new ExplanationBuilder instance
func NewExplanationBuilder() *ExplanationBuilder {
	return &ExplanationBuilder{}
}

// GenerateExplanation creates a human-readable explanation for a causal path
func (e *ExplanationBuilder) GenerateExplanation(path CausalPath) string {
	if len(path.Steps) == 0 {
		return "No causal path identified."
	}

	root := path.CandidateRoot
	rootAnomaly := e.getPrimaryAnomaly(root.Anomalies)

	var sb strings.Builder

	// 1. Root cause statement
	sb.WriteString(fmt.Sprintf("%s '%s/%s'",
		root.Resource.Kind, root.Resource.Namespace, root.Resource.Name))

	if rootAnomaly != nil {
		sb.WriteString(fmt.Sprintf(" %s at %s",
			e.describeAnomaly(rootAnomaly),
			rootAnomaly.Timestamp.Format(time.RFC3339)))
	}

	// 2. Propagation path (if more than root and symptom)
	if len(path.Steps) > 2 {
		sb.WriteString(". This propagated through: ")

		intermediateNodes := make([]string, 0)
		for i := 1; i < len(path.Steps)-1; i++ {
			step := path.Steps[i]
			nodeDesc := step.Node.Resource.Kind
			if step.Edge != nil {
				edgeDesc := e.describeEdge(step.Edge.RelationshipType)
				nodeDesc = fmt.Sprintf("%s (%s)", nodeDesc, edgeDesc)
			}
			intermediateNodes = append(intermediateNodes, nodeDesc)
		}
		sb.WriteString(strings.Join(intermediateNodes, " -> "))
	}

	// 3. Impact statement
	symptom := path.Steps[len(path.Steps)-1].Node
	symptomAnomaly := e.getPrimaryAnomaly(symptom.Anomalies)

	sb.WriteString(fmt.Sprintf(", ultimately affecting %s '%s/%s'",
		symptom.Resource.Kind, symptom.Resource.Namespace, symptom.Resource.Name))

	if symptomAnomaly != nil {
		sb.WriteString(fmt.Sprintf(" which %s", e.describeSymptomAnomaly(symptomAnomaly)))
	}
	sb.WriteString(".")

	// 4. Confidence statement
	sb.WriteString(fmt.Sprintf(" Confidence: %.0f%%.", path.ConfidenceScore*100))

	return sb.String()
}

// getPrimaryAnomaly returns the most relevant anomaly for explanation
// Prefers cause-introducing anomalies, then highest severity
func (e *ExplanationBuilder) getPrimaryAnomaly(anomalies []anomaly.Anomaly) *anomaly.Anomaly {
	if len(anomalies) == 0 {
		return nil
	}

	// First, look for cause-introducing anomalies
	var best *anomaly.Anomaly
	for i := range anomalies {
		a := &anomalies[i]
		if IsCauseIntroducingAnomaly(a.Type, a.Category) {
			if best == nil || e.severityRank(a.Severity) > e.severityRank(best.Severity) {
				best = a
			}
		}
	}
	if best != nil {
		return best
	}

	// Fall back to highest severity
	best = &anomalies[0]
	for i := 1; i < len(anomalies); i++ {
		if e.severityRank(anomalies[i].Severity) > e.severityRank(best.Severity) {
			best = &anomalies[i]
		}
	}
	return best
}

// severityRank converts severity to numeric rank for comparison
func (e *ExplanationBuilder) severityRank(severity anomaly.Severity) int {
	switch severity {
	case anomaly.SeverityCritical:
		return 4
	case anomaly.SeverityHigh:
		return 3
	case anomaly.SeverityMedium:
		return 2
	case anomaly.SeverityLow:
		return 1
	default:
		return 0
	}
}

// describeAnomaly returns a human-readable description of an anomaly
func (e *ExplanationBuilder) describeAnomaly(a *anomaly.Anomaly) string {
	switch a.Type {
	case "ConfigMapModified":
		return "had configuration modified"
	case "SecretModified":
		return "had secret modified"
	case "HelmReleaseUpdated":
		return "was updated"
	case "HelmUpgrade":
		return "was upgraded to a new version"
	case "HelmRollback":
		return "was rolled back to a previous version"
	case "ValuesChanged":
		return "had Helm values configuration changed"
	case "HelmReleaseFailed":
		return "failed to reconcile"
	case "KustomizationUpdated":
		return "was updated"
	case "KustomizationFailed":
		return "failed to reconcile"
	case "ImageChanged":
		return "had container image changed"
	case "EnvironmentChanged":
		return "had environment variables changed"
	case "WorkloadSpecModified":
		return "had workload spec modified"
	case "SpecModified":
		return "had spec modified"
	case "ResourceDeleted":
		return "was deleted"
	case "NodeNotReady":
		return "became NotReady"
	case "NodeMemoryPressure":
		return "experienced memory pressure"
	case "NodeDiskPressure":
		return "experienced disk pressure"
	case "NodePIDPressure":
		return "experienced PID pressure"
	case "RoleModified":
		return "had RBAC role modified"
	case "RoleBindingModified":
		return "had RBAC permissions modified"
	case "PVCBindingFailed":
		return "failed to bind to a PersistentVolume"
	case "VolumeMountFailed":
		return "failed to mount volume"
	case "VolumeOutOfSpace":
		return "ran out of disk space"
	case "ReadOnlyFilesystem":
		return "has read-only filesystem"
	default:
		if a.Summary != "" {
			return strings.ToLower(a.Summary)
		}
		return fmt.Sprintf("had %s anomaly", strings.ToLower(a.Type))
	}
}

// describeSymptomAnomaly returns a description for symptom anomalies
func (e *ExplanationBuilder) describeSymptomAnomaly(a *anomaly.Anomaly) string {
	switch a.Type {
	case "CrashLoopBackOff":
		return "entered CrashLoopBackOff"
	case reasonImagePullBackOff:
		return "failed to pull container image"
	case "OOMKilled":
		return "was OOM killed"
	case "PodFailed":
		return "failed"
	case "PodPending":
		return "is stuck pending"
	case "ErrorStatus":
		return "entered error state"
	case "FailedScheduling":
		return "failed to be scheduled"
	default:
		if a.Summary != "" {
			return strings.ToLower(a.Summary)
		}
		return fmt.Sprintf("experienced %s", strings.ToLower(a.Type))
	}
}

// describeEdge returns a human-readable edge description
func (e *ExplanationBuilder) describeEdge(relationshipType string) string {
	switch relationshipType {
	case "OWNS":
		return "owns"
	case "MANAGES":
		return "manages"
	case "SCHEDULED_ON":
		return "scheduled on"
	case "REFERENCES_SPEC":
		return "references"
	case "MOUNTS":
		return "mounts"
	case "USES_SERVICE_ACCOUNT":
		return "uses"
	case "SELECTS":
		return "selects"
	case "GRANTS_TO":
		return "grants to"
	case "BINDS_ROLE":
		return "binds"
	default:
		return strings.ToLower(strings.ReplaceAll(relationshipType, "_", " "))
	}
}
