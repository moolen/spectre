package causalpaths

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	"github.com/stretchr/testify/assert"
)

func TestClassifyEdge(t *testing.T) {
	tests := []struct {
		name             string
		relationshipType string
		expected         string
	}{
		// Cause-introducing edges
		{"MANAGES is cause-introducing", "MANAGES", EdgeCategoryCauseIntroducing},
		{"TRIGGERED_BY is cause-introducing", "TRIGGERED_BY", EdgeCategoryCauseIntroducing},
		{"REFERENCES_SPEC is cause-introducing", "REFERENCES_SPEC", EdgeCategoryCauseIntroducing},
		{"USES_SERVICE_ACCOUNT is cause-introducing", "USES_SERVICE_ACCOUNT", EdgeCategoryCauseIntroducing},
		{"GRANTS_TO is cause-introducing", "GRANTS_TO", EdgeCategoryCauseIntroducing},
		{"BINDS_ROLE is cause-introducing", "BINDS_ROLE", EdgeCategoryCauseIntroducing},

		// Materialization edges
		{"OWNS is materialization", "OWNS", EdgeCategoryMaterialization},
		{"SCHEDULED_ON is materialization", "SCHEDULED_ON", EdgeCategoryMaterialization},
		{"SELECTS is materialization", "SELECTS", EdgeCategoryMaterialization},
		{"MOUNTS is materialization", "MOUNTS", EdgeCategoryMaterialization},
		{"CREATES_OBSERVED is materialization", "CREATES_OBSERVED", EdgeCategoryMaterialization},

		// Unknown edges default to materialization
		{"UNKNOWN defaults to materialization", "UNKNOWN_EDGE_TYPE", EdgeCategoryMaterialization},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyEdge(tt.relationshipType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCausalWeight(t *testing.T) {
	tests := []struct {
		name         string
		edgeCategory string
		expected     float64
	}{
		{"Cause-introducing has weight 1.0", EdgeCategoryCauseIntroducing, 1.0},
		{"Materialization has weight 0.0", EdgeCategoryMaterialization, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCausalWeight(tt.edgeCategory)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsCauseIntroducingAnomaly(t *testing.T) {
	tests := []struct {
		name        string
		anomalyType string
		category    anomaly.AnomalyCategory
		expected    bool
	}{
		// Change category anomalies are cause-introducing
		{"ConfigMapModified is cause-introducing", "ConfigMapModified", anomaly.CategoryChange, true},
		{"SecretModified is cause-introducing", "SecretModified", anomaly.CategoryChange, true},
		{"HelmReleaseUpdated is cause-introducing", "HelmReleaseUpdated", anomaly.CategoryChange, true},
		{"ImageChanged is cause-introducing", "ImageChanged", anomaly.CategoryChange, true},

		// State category - specific types
		{"NodeNotReady is cause-introducing", "NodeNotReady", anomaly.CategoryState, true},
		{"NodeMemoryPressure is cause-introducing", "NodeMemoryPressure", anomaly.CategoryState, true},
		{"NodeDiskPressure is cause-introducing", "NodeDiskPressure", anomaly.CategoryState, true},

		// Non-cause-introducing
		{"CrashLoopBackOff is derived", "CrashLoopBackOff", anomaly.CategoryState, false},
		{"OOMKilled is derived", "OOMKilled", anomaly.CategoryState, false},
		{"PodFailed is derived", "PodFailed", anomaly.CategoryState, false},

		// Event category
		{"Event anomalies are not cause-introducing", "WarningEvent", anomaly.CategoryEvent, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCauseIntroducingAnomaly(tt.anomalyType, tt.category)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDerivedFailureAnomaly(t *testing.T) {
	tests := []struct {
		name        string
		anomalyType string
		expected    bool
	}{
		{"CrashLoopBackOff is derived", "CrashLoopBackOff", true},
		// ImagePullBackOff and ErrImagePull are now context-dependent (not in static list)
		// Use IsContextuallyDerivedAnomaly() or ClassifyImagePullAnomaly() instead
		{"ImagePullBackOff is context-dependent (not in static list)", "ImagePullBackOff", false},
		{"OOMKilled is derived", "OOMKilled", true},
		{"PodFailed is derived", "PodFailed", true},
		{"PodPending is derived", "PodPending", true},
		{"ErrorStatus is derived", "ErrorStatus", true},

		{"ConfigMapModified is not derived", "ConfigMapModified", false},
		{"NodeNotReady is not derived", "NodeNotReady", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDerivedFailureAnomaly(tt.anomalyType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasCauseIntroducingAnomaly(t *testing.T) {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)
	oneMinuteFromNow := now.Add(1 * time.Minute)

	tests := []struct {
		name                string
		anomalies           []anomaly.Anomaly
		symptomFirstFailure time.Time
		expected            bool
	}{
		{
			name:                "No anomalies",
			anomalies:           []anomaly.Anomaly{},
			symptomFirstFailure: now,
			expected:            false,
		},
		{
			name: "Has cause-introducing anomaly before failure",
			anomalies: []anomaly.Anomaly{
				{Type: "ConfigMapModified", Category: anomaly.CategoryChange, Timestamp: oneMinuteAgo},
			},
			symptomFirstFailure: now,
			expected:            true,
		},
		{
			name: "Has cause-introducing anomaly after failure (should not count)",
			anomalies: []anomaly.Anomaly{
				{Type: "ConfigMapModified", Category: anomaly.CategoryChange, Timestamp: oneMinuteFromNow},
			},
			symptomFirstFailure: now,
			expected:            false,
		},
		{
			name: "Has only derived failure anomaly",
			anomalies: []anomaly.Anomaly{
				{Type: "CrashLoopBackOff", Category: anomaly.CategoryState, Timestamp: oneMinuteAgo},
			},
			symptomFirstFailure: now,
			expected:            false,
		},
		{
			name: "Mixed anomalies - one cause-introducing",
			anomalies: []anomaly.Anomaly{
				{Type: "CrashLoopBackOff", Category: anomaly.CategoryState, Timestamp: oneMinuteAgo},
				{Type: "SecretModified", Category: anomaly.CategoryChange, Timestamp: oneMinuteAgo},
			},
			symptomFirstFailure: now,
			expected:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasCauseIntroducingAnomaly(tt.anomalies, tt.symptomFirstFailure)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasOnlyDerivedAnomalies(t *testing.T) {
	tests := []struct {
		name      string
		anomalies []anomaly.Anomaly
		expected  bool
	}{
		{
			name:      "Empty anomalies",
			anomalies: []anomaly.Anomaly{},
			expected:  true,
		},
		{
			name: "Only derived anomalies",
			anomalies: []anomaly.Anomaly{
				{Type: "CrashLoopBackOff", Category: anomaly.CategoryState},
				{Type: "OOMKilled", Category: anomaly.CategoryState},
			},
			expected: true,
		},
		{
			name: "Has cause-introducing anomaly",
			anomalies: []anomaly.Anomaly{
				{Type: "ConfigMapModified", Category: anomaly.CategoryChange},
			},
			expected: false,
		},
		{
			name: "Mixed - includes cause-introducing",
			anomalies: []anomaly.Anomaly{
				{Type: "CrashLoopBackOff", Category: anomaly.CategoryState},
				{Type: "ImageChanged", Category: anomaly.CategoryChange},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasOnlyDerivedAnomalies(tt.anomalies)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFirstCauseIntroducingAnomaly(t *testing.T) {
	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)
	oneMinuteAgo := now.Add(-1 * time.Minute)
	thirtySecondsAgo := now.Add(-30 * time.Second)

	tests := []struct {
		name         string
		anomalies    []anomaly.Anomaly
		beforeTime   time.Time
		expectedNil  bool
		expectedType string
	}{
		{
			name:        "No anomalies",
			anomalies:   []anomaly.Anomaly{},
			beforeTime:  now,
			expectedNil: true,
		},
		{
			name: "Single cause-introducing anomaly",
			anomalies: []anomaly.Anomaly{
				{Type: "ConfigMapModified", Category: anomaly.CategoryChange, Timestamp: oneMinuteAgo},
			},
			beforeTime:   now,
			expectedNil:  false,
			expectedType: "ConfigMapModified",
		},
		{
			name: "Multiple cause-introducing anomalies - returns earliest",
			anomalies: []anomaly.Anomaly{
				{Type: "SecretModified", Category: anomaly.CategoryChange, Timestamp: oneMinuteAgo},
				{Type: "ConfigMapModified", Category: anomaly.CategoryChange, Timestamp: twoMinutesAgo},
				{Type: "ImageChanged", Category: anomaly.CategoryChange, Timestamp: thirtySecondsAgo},
			},
			beforeTime:   now,
			expectedNil:  false,
			expectedType: "ConfigMapModified", // earliest
		},
		{
			name: "All anomalies after beforeTime",
			anomalies: []anomaly.Anomaly{
				{Type: "ConfigMapModified", Category: anomaly.CategoryChange, Timestamp: now.Add(1 * time.Minute)},
			},
			beforeTime:  now,
			expectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFirstCauseIntroducingAnomaly(tt.anomalies, tt.beforeTime)
			if tt.expectedNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedType, result.Type)
			}
		})
	}
}
