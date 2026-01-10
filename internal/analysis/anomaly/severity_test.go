package anomaly

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSeverity_KindSpecific(t *testing.T) {
	tests := []struct {
		name             string
		category         AnomalyCategory
		anomalyType      string
		kind             string
		expectedSeverity Severity
	}{
		{
			name:             "ReplicaSet SpecModified should be Low",
			category:         CategoryChange,
			anomalyType:      "SpecModified",
			kind:             "ReplicaSet",
			expectedSeverity: SeverityLow,
		},
		{
			name:             "Deployment SpecModified should be Medium (default)",
			category:         CategoryChange,
			anomalyType:      "SpecModified",
			kind:             "Deployment",
			expectedSeverity: SeverityMedium,
		},
		{
			name:             "ConfigMap SpecModified should use WorkloadSpecModified rule",
			category:         CategoryChange,
			anomalyType:      "ConfigMapModified",
			kind:             "ConfigMap",
			expectedSeverity: SeverityHigh,
		},
		{
			name:             "Generic SpecModified without kind should be Medium",
			category:         CategoryChange,
			anomalyType:      "SpecModified",
			kind:             "",
			expectedSeverity: SeverityMedium,
		},
		{
			name:             "Unknown type should default to Medium",
			category:         CategoryChange,
			anomalyType:      "UnknownType",
			kind:             "Pod",
			expectedSeverity: SeverityMedium,
		},
		{
			name:             "ImageChanged should be High regardless of kind",
			category:         CategoryChange,
			anomalyType:      "ImageChanged",
			kind:             "Deployment",
			expectedSeverity: SeverityHigh,
		},
		{
			name:             "CrashLoopBackOff should be Critical",
			category:         CategoryState,
			anomalyType:      "CrashLoopBackOff",
			kind:             "Pod",
			expectedSeverity: SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSeverity(tt.category, tt.anomalyType, tt.kind)
			assert.Equal(t, tt.expectedSeverity, result,
				"GetSeverity(%v, %q, %q) = %v, want %v",
				tt.category, tt.anomalyType, tt.kind, result, tt.expectedSeverity)
		})
	}
}

func TestClassifyK8sEventSeverity(t *testing.T) {
	tests := []struct {
		reason           string
		expectedSeverity Severity
	}{
		{"FailedScheduling", SeverityCritical},
		{"BackOff", SeverityHigh},
		{"Evicted", SeverityHigh},
		{"Pulled", SeverityLow},
		{"UnknownReason", SeverityMedium}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			result := ClassifyK8sEventSeverity(tt.reason)
			assert.Equal(t, tt.expectedSeverity, result,
				"ClassifyK8sEventSeverity(%q) = %v, want %v",
				tt.reason, result, tt.expectedSeverity)
		})
	}
}
