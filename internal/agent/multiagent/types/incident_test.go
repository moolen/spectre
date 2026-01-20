package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestIncidentFacts_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	facts := IncidentFacts{
		Symptoms: []Symptom{
			{
				Description: "Pod is crashing repeatedly",
				Resource:    "my-pod",
				Namespace:   "default",
				Kind:        "Pod",
				Severity:    "high",
				FirstSeen:   "10 minutes ago",
			},
			{
				Description: "Service is returning 503 errors",
				Resource:    "my-service",
				Namespace:   "default",
				Kind:        "Service",
				Severity:    "critical",
			},
		},
		Timeline: Timeline{
			IncidentStart:  "about 15 minutes ago",
			UserReportedAt: now,
			DurationStr:    "ongoing for 15 minutes",
		},
		MitigationsAttempted: []Mitigation{
			{
				Description: "Restarted the pod",
				Result:      "no effect",
			},
		},
		IsOngoing: true,
		UserConstraints: []string{
			"focus on the database connection",
		},
		AffectedResource: &ResourceRef{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "my-pod",
			UID:       "abc-123",
		},
		ExtractedAt: now,
	}

	// Serialize
	data, err := json.Marshal(facts)
	if err != nil {
		t.Fatalf("failed to marshal IncidentFacts: %v", err)
	}

	// Deserialize
	var decoded IncidentFacts
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal IncidentFacts: %v", err)
	}

	// Verify fields
	if len(decoded.Symptoms) != 2 {
		t.Errorf("expected 2 symptoms, got %d", len(decoded.Symptoms))
	}
	if decoded.Symptoms[0].Description != "Pod is crashing repeatedly" {
		t.Errorf("unexpected symptom description: %s", decoded.Symptoms[0].Description)
	}
	if decoded.Symptoms[0].Severity != "high" {
		t.Errorf("expected severity 'high', got '%s'", decoded.Symptoms[0].Severity)
	}
	if decoded.Timeline.IncidentStart != "about 15 minutes ago" {
		t.Errorf("unexpected incident start: %s", decoded.Timeline.IncidentStart)
	}
	if !decoded.Timeline.UserReportedAt.Equal(now) {
		t.Errorf("timestamp mismatch: expected %v, got %v", now, decoded.Timeline.UserReportedAt)
	}
	if len(decoded.MitigationsAttempted) != 1 {
		t.Errorf("expected 1 mitigation, got %d", len(decoded.MitigationsAttempted))
	}
	if decoded.MitigationsAttempted[0].Result != "no effect" {
		t.Errorf("unexpected mitigation result: %s", decoded.MitigationsAttempted[0].Result)
	}
	if !decoded.IsOngoing {
		t.Error("expected IsOngoing to be true")
	}
	if len(decoded.UserConstraints) != 1 {
		t.Errorf("expected 1 user constraint, got %d", len(decoded.UserConstraints))
	}
	if decoded.AffectedResource == nil {
		t.Fatal("expected AffectedResource to be set")
	}
	if decoded.AffectedResource.Name != "my-pod" {
		t.Errorf("unexpected affected resource name: %s", decoded.AffectedResource.Name)
	}
}

func TestIncidentFacts_MinimalSerialization(t *testing.T) {
	// Test with minimal required fields
	now := time.Now().UTC().Truncate(time.Second)
	facts := IncidentFacts{
		Symptoms: []Symptom{
			{
				Description: "Something is broken",
				Severity:    "medium",
			},
		},
		Timeline: Timeline{
			UserReportedAt: now,
		},
		IsOngoing:   false,
		ExtractedAt: now,
	}

	data, err := json.Marshal(facts)
	if err != nil {
		t.Fatalf("failed to marshal minimal IncidentFacts: %v", err)
	}

	var decoded IncidentFacts
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal minimal IncidentFacts: %v", err)
	}

	if len(decoded.Symptoms) != 1 {
		t.Errorf("expected 1 symptom, got %d", len(decoded.Symptoms))
	}
	if decoded.AffectedResource != nil {
		t.Error("expected AffectedResource to be nil")
	}
	if len(decoded.MitigationsAttempted) != 0 {
		t.Errorf("expected 0 mitigations, got %d", len(decoded.MitigationsAttempted))
	}
}

func TestSystemSnapshot_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := SystemSnapshot{
		ClusterHealth: &ClusterHealthSummary{
			OverallStatus:  "degraded",
			TotalResources: 150,
			ErrorCount:     3,
			WarningCount:   7,
			TopIssues: []string{
				"Pod my-pod is CrashLoopBackOff",
				"Service my-service has no healthy endpoints",
			},
		},
		AffectedResource: &ResourceDetails{
			Kind:          "Pod",
			Namespace:     "default",
			Name:          "my-pod",
			UID:           "abc-123",
			Status:        "CrashLoopBackOff",
			ErrorMessage:  "Container exited with code 1",
			CreatedAt:     "2024-01-15T10:00:00Z",
			LastUpdatedAt: "2024-01-15T10:30:00Z",
			Conditions: []ConditionSummary{
				{
					Type:               "Ready",
					Status:             "False",
					Reason:             "ContainersNotReady",
					Message:            "containers with unready status: [app]",
					LastTransitionTime: "2024-01-15T10:25:00Z",
				},
			},
		},
		CausalPaths: []CausalPathSummary{
			{
				PathID:             "path-1",
				RootCauseKind:      "ConfigMap",
				RootCauseName:      "my-config",
				RootCauseNamespace: "default",
				Confidence:         0.78,
				Explanation:        "ConfigMap change triggered pod restart",
				StepCount:          2,
				ChangeType:         "UPDATE",
			},
		},
		Anomalies: []AnomalySummary{
			{
				ResourceKind:      "Pod",
				ResourceName:      "my-pod",
				ResourceNamespace: "default",
				AnomalyType:       "restart_rate",
				Severity:          "high",
				Summary:           "Pod restart rate exceeded threshold",
				Timestamp:         "2024-01-15T10:25:00Z",
			},
		},
		RecentChanges: []ChangeSummary{
			{
				ResourceKind:      "ConfigMap",
				ResourceName:      "my-config",
				ResourceNamespace: "default",
				ResourceUID:       "config-uid-123",
				ChangeType:        "UPDATE",
				ImpactScore:       0.85,
				Description:       "Changed DATABASE_URL value",
				Timestamp:         "2024-01-15T10:20:00Z",
				ChangedFields:     []string{"data.DATABASE_URL"},
			},
		},
		RelatedResources: []ResourceSummary{
			{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "my-deployment",
				UID:       "deploy-uid-123",
				Status:    "Available",
				Relation:  "owner",
			},
		},
		K8sEvents: []K8sEventSummary{
			{
				Reason:             "BackOff",
				Message:            "Back-off restarting failed container",
				Type:               "Warning",
				Count:              5,
				Timestamp:          "2024-01-15T10:28:00Z",
				InvolvedObjectKind: "Pod",
				InvolvedObjectName: "my-pod",
			},
		},
		GatheredAt:    now,
		ToolCallCount: 6,
		Errors:        []string{"timeout fetching metrics"},
	}

	// Serialize
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to marshal SystemSnapshot: %v", err)
	}

	// Deserialize
	var decoded SystemSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SystemSnapshot: %v", err)
	}

	// Verify cluster health
	if decoded.ClusterHealth == nil {
		t.Fatal("expected ClusterHealth to be set")
	}
	if decoded.ClusterHealth.OverallStatus != "degraded" {
		t.Errorf("unexpected overall status: %s", decoded.ClusterHealth.OverallStatus)
	}
	if decoded.ClusterHealth.ErrorCount != 3 {
		t.Errorf("expected error count 3, got %d", decoded.ClusterHealth.ErrorCount)
	}

	// Verify affected resource
	if decoded.AffectedResource == nil {
		t.Fatal("expected AffectedResource to be set")
	}
	if decoded.AffectedResource.Status != "CrashLoopBackOff" {
		t.Errorf("unexpected status: %s", decoded.AffectedResource.Status)
	}
	if len(decoded.AffectedResource.Conditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(decoded.AffectedResource.Conditions))
	}

	// Verify causal paths
	if len(decoded.CausalPaths) != 1 {
		t.Errorf("expected 1 causal path, got %d", len(decoded.CausalPaths))
	}
	if decoded.CausalPaths[0].Confidence != 0.78 {
		t.Errorf("expected confidence 0.78, got %f", decoded.CausalPaths[0].Confidence)
	}

	// Verify anomalies
	if len(decoded.Anomalies) != 1 {
		t.Errorf("expected 1 anomaly, got %d", len(decoded.Anomalies))
	}

	// Verify changes
	if len(decoded.RecentChanges) != 1 {
		t.Errorf("expected 1 change, got %d", len(decoded.RecentChanges))
	}
	if decoded.RecentChanges[0].ImpactScore != 0.85 {
		t.Errorf("expected impact score 0.85, got %f", decoded.RecentChanges[0].ImpactScore)
	}

	// Verify related resources
	if len(decoded.RelatedResources) != 1 {
		t.Errorf("expected 1 related resource, got %d", len(decoded.RelatedResources))
	}

	// Verify events
	if len(decoded.K8sEvents) != 1 {
		t.Errorf("expected 1 event, got %d", len(decoded.K8sEvents))
	}
	if decoded.K8sEvents[0].Count != 5 {
		t.Errorf("expected event count 5, got %d", decoded.K8sEvents[0].Count)
	}

	// Verify metadata
	if decoded.ToolCallCount != 6 {
		t.Errorf("expected tool call count 6, got %d", decoded.ToolCallCount)
	}
	if len(decoded.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(decoded.Errors))
	}
}

func TestSystemSnapshot_EmptySerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := SystemSnapshot{
		GatheredAt:    now,
		ToolCallCount: 0,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to marshal empty SystemSnapshot: %v", err)
	}

	var decoded SystemSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal empty SystemSnapshot: %v", err)
	}

	if decoded.ClusterHealth != nil {
		t.Error("expected ClusterHealth to be nil")
	}
	if decoded.AffectedResource != nil {
		t.Error("expected AffectedResource to be nil")
	}
	if len(decoded.CausalPaths) != 0 {
		t.Errorf("expected 0 causal paths, got %d", len(decoded.CausalPaths))
	}
}

func TestSymptom_Severity(t *testing.T) {
	validSeverities := []string{"critical", "high", "medium", "low"}
	for _, sev := range validSeverities {
		s := Symptom{
			Description: "test",
			Severity:    sev,
		}
		data, err := json.Marshal(s)
		if err != nil {
			t.Errorf("failed to marshal symptom with severity %s: %v", sev, err)
		}
		var decoded Symptom
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("failed to unmarshal symptom with severity %s: %v", sev, err)
		}
		if decoded.Severity != sev {
			t.Errorf("severity mismatch: expected %s, got %s", sev, decoded.Severity)
		}
	}
}

func TestMitigation_Result(t *testing.T) {
	validResults := []string{"no effect", "partial", "unknown", "made worse"}
	for _, result := range validResults {
		m := Mitigation{
			Description: "tried something",
			Result:      result,
		}
		data, err := json.Marshal(m)
		if err != nil {
			t.Errorf("failed to marshal mitigation with result %s: %v", result, err)
		}
		var decoded Mitigation
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("failed to unmarshal mitigation with result %s: %v", result, err)
		}
		if decoded.Result != result {
			t.Errorf("result mismatch: expected %s, got %s", result, decoded.Result)
		}
	}
}

func TestResourceRef_Complete(t *testing.T) {
	ref := ResourceRef{
		UID:       "uid-12345",
		Kind:      "Deployment",
		Namespace: "production",
		Name:      "web-app",
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("failed to marshal ResourceRef: %v", err)
	}

	var decoded ResourceRef
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ResourceRef: %v", err)
	}

	if decoded.UID != "uid-12345" {
		t.Errorf("unexpected UID: %s", decoded.UID)
	}
	if decoded.Kind != "Deployment" {
		t.Errorf("unexpected Kind: %s", decoded.Kind)
	}
	if decoded.Namespace != "production" {
		t.Errorf("unexpected Namespace: %s", decoded.Namespace)
	}
	if decoded.Name != "web-app" {
		t.Errorf("unexpected Name: %s", decoded.Name)
	}
}

func TestCausalPathSummary_Confidence(t *testing.T) {
	// Test confidence values
	testCases := []struct {
		confidence float64
	}{
		{0.0},
		{0.5},
		{0.85},
		{1.0},
	}

	for _, tc := range testCases {
		path := CausalPathSummary{
			PathID:        "test-path",
			RootCauseKind: "Pod",
			RootCauseName: "test-pod",
			Confidence:    tc.confidence,
			Explanation:   "test explanation",
			StepCount:     1,
		}

		data, err := json.Marshal(path)
		if err != nil {
			t.Errorf("failed to marshal path with confidence %f: %v", tc.confidence, err)
		}

		var decoded CausalPathSummary
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("failed to unmarshal path with confidence %f: %v", tc.confidence, err)
		}

		if decoded.Confidence != tc.confidence {
			t.Errorf("confidence mismatch: expected %f, got %f", tc.confidence, decoded.Confidence)
		}
	}
}

func TestChangeSummary_ChangedFields(t *testing.T) {
	change := ChangeSummary{
		ResourceKind:  "ConfigMap",
		ResourceName:  "app-config",
		ChangeType:    "UPDATE",
		ImpactScore:   0.7,
		Description:   "Updated configuration",
		Timestamp:     "2024-01-15T10:00:00Z",
		ChangedFields: []string{"data.DB_HOST", "data.DB_PORT", "data.LOG_LEVEL"},
	}

	data, err := json.Marshal(change)
	if err != nil {
		t.Fatalf("failed to marshal ChangeSummary: %v", err)
	}

	var decoded ChangeSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ChangeSummary: %v", err)
	}

	if len(decoded.ChangedFields) != 3 {
		t.Errorf("expected 3 changed fields, got %d", len(decoded.ChangedFields))
	}
	if decoded.ChangedFields[0] != "data.DB_HOST" {
		t.Errorf("unexpected first changed field: %s", decoded.ChangedFields[0])
	}
}
