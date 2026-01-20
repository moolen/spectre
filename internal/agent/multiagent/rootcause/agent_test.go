package rootcause

import (
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/agent/multiagent/types"
)

// TestPipelineStateFlow tests that the data structures flow correctly
// through the pipeline stages via session state.
func TestPipelineStateFlow(t *testing.T) {
	// Stage 1: Intake Agent produces IncidentFacts
	incidentFacts := types.IncidentFacts{
		Symptoms: []types.Symptom{
			{
				Description: "Pod my-app is crashing with CrashLoopBackOff",
				Resource:    "my-app",
				Namespace:   "production",
				Kind:        "Pod",
				Severity:    "critical",
				FirstSeen:   "5 minutes ago",
			},
		},
		Timeline: types.Timeline{
			IncidentStart: "about 5 minutes ago",
			DurationStr:   "ongoing for 5 minutes",
		},
		IsOngoing: true,
		AffectedResource: &types.ResourceRef{
			Kind:      "Pod",
			Namespace: "production",
			Name:      "my-app",
		},
	}

	factsJSON, err := json.Marshal(incidentFacts)
	if err != nil {
		t.Fatalf("failed to marshal incident facts: %v", err)
	}

	// Simulate state storage
	state := make(map[string]string)
	state[types.StateKeyIncidentFacts] = string(factsJSON)
	state[types.StateKeyPipelineStage] = types.PipelineStageIntake

	// Stage 2: Gathering Agent produces SystemSnapshot
	systemSnapshot := types.SystemSnapshot{
		ClusterHealth: &types.ClusterHealthSummary{
			OverallStatus:  "degraded",
			TotalResources: 100,
			ErrorCount:     1,
			WarningCount:   3,
			TopIssues: []string{
				"Pod my-app is in CrashLoopBackOff",
			},
		},
		AffectedResource: &types.ResourceDetails{
			Kind:         "Pod",
			Namespace:    "production",
			Name:         "my-app",
			UID:          "pod-uid-123",
			Status:       "CrashLoopBackOff",
			ErrorMessage: "Container app exited with code 1: Error connecting to database",
			Conditions: []types.ConditionSummary{
				{
					Type:   "Ready",
					Status: "False",
					Reason: "ContainersNotReady",
				},
			},
		},
		CausalPaths: []types.CausalPathSummary{
			{
				PathID:             "path-1",
				RootCauseKind:      "Secret",
				RootCauseName:      "db-credentials",
				RootCauseNamespace: "production",
				Confidence:         0.82,
				Explanation:        "Secret db-credentials was updated, causing pod restart with connection failure",
				StepCount:          2,
				ChangeType:         "UPDATE",
			},
		},
		RecentChanges: []types.ChangeSummary{
			{
				ResourceKind:      "Secret",
				ResourceName:      "db-credentials",
				ResourceNamespace: "production",
				ChangeType:        "UPDATE",
				ImpactScore:       0.9,
				Description:       "Updated database password",
				Timestamp:         "2024-01-15T10:00:00Z",
				ChangedFields:     []string{"data.password"},
			},
		},
		ToolCallCount: 4,
	}

	snapshotJSON, err := json.Marshal(systemSnapshot)
	if err != nil {
		t.Fatalf("failed to marshal system snapshot: %v", err)
	}

	state[types.StateKeySystemSnapshot] = string(snapshotJSON)
	state[types.StateKeyPipelineStage] = types.PipelineStageGathering

	// Stage 3: Builder Agent produces hypotheses
	rawHypotheses := []types.Hypothesis{
		{
			ID:    "hyp-1",
			Claim: "The Secret db-credentials update introduced an invalid database password, causing authentication failures",
			SupportingEvidence: []types.EvidenceRef{
				{
					Type:        types.EvidenceTypeChange,
					SourceID:    "change-secret-1",
					Description: "Secret db-credentials was updated 5 minutes before incident",
					Strength:    types.EvidenceStrengthStrong,
				},
				{
					Type:        types.EvidenceTypeCausalPath,
					SourceID:    "path-1",
					Description: "Spectre detected causal path from Secret to Pod failure",
					Strength:    types.EvidenceStrengthStrong,
				},
			},
			Assumptions: []types.Assumption{
				{
					Description:         "The application validates database credentials on startup",
					IsVerified:          false,
					Falsifiable:         true,
					FalsificationMethod: "Check if app has startup health check for DB connection",
				},
			},
			ValidationPlan: types.ValidationPlan{
				ConfirmationChecks: []types.ValidationTask{
					{
						Description: "Check pod logs for database authentication errors",
						Command:     "kubectl logs my-app -n production | grep -i 'auth\\|password\\|credential'",
						Expected:    "Should see authentication failure messages",
					},
				},
				FalsificationChecks: []types.ValidationTask{
					{
						Description: "Verify database is reachable with correct credentials",
						Command:     "kubectl exec -it my-app -n production -- nc -zv db-host 5432",
						Expected:    "If DB is unreachable, issue is network not credentials",
					},
				},
			},
			Confidence: 0.78,
			Status:     types.HypothesisStatusPending,
		},
	}

	hypothesesJSON, err := json.Marshal(rawHypotheses)
	if err != nil {
		t.Fatalf("failed to marshal hypotheses: %v", err)
	}

	state[types.StateKeyRawHypotheses] = string(hypothesesJSON)
	state[types.StateKeyPipelineStage] = types.PipelineStageBuilding

	// Stage 4: Reviewer Agent produces reviewed hypotheses
	reviewedHypotheses := types.ReviewedHypotheses{
		Hypotheses: []types.Hypothesis{
			{
				ID:    "hyp-1",
				Claim: "The Secret db-credentials update introduced an invalid database password, causing authentication failures",
				SupportingEvidence: []types.EvidenceRef{
					{
						Type:        types.EvidenceTypeChange,
						SourceID:    "change-secret-1",
						Description: "Secret db-credentials was updated 5 minutes before incident",
						Strength:    types.EvidenceStrengthStrong,
					},
					{
						Type:        types.EvidenceTypeCausalPath,
						SourceID:    "path-1",
						Description: "Spectre detected causal path from Secret to Pod failure",
						Strength:    types.EvidenceStrengthStrong,
					},
				},
				Assumptions: []types.Assumption{
					{
						Description:         "The application validates database credentials on startup",
						IsVerified:          false,
						Falsifiable:         true,
						FalsificationMethod: "Check if app has startup health check for DB connection",
					},
				},
				ValidationPlan: types.ValidationPlan{
					ConfirmationChecks: []types.ValidationTask{
						{
							Description: "Check pod logs for database authentication errors",
							Command:     "kubectl logs my-app -n production | grep -i 'auth\\|password\\|credential'",
							Expected:    "Should see authentication failure messages",
						},
					},
					FalsificationChecks: []types.ValidationTask{
						{
							Description: "Verify database is reachable with correct credentials",
							Command:     "kubectl exec -it my-app -n production -- nc -zv db-host 5432",
							Expected:    "If DB is unreachable, issue is network not credentials",
						},
					},
				},
				Confidence: 0.78,
				Status:     types.HypothesisStatusApproved,
			},
		},
		ReviewNotes: "Hypothesis is well-supported by strong evidence from both the recent change and Spectre's causal analysis. The temporal correlation and error messages align with the claimed root cause.",
	}

	reviewedJSON, err := json.Marshal(reviewedHypotheses)
	if err != nil {
		t.Fatalf("failed to marshal reviewed hypotheses: %v", err)
	}

	state[types.StateKeyReviewedHypotheses] = string(reviewedJSON)
	state[types.StateKeyFinalHypotheses] = string(reviewedJSON)
	state[types.StateKeyPipelineStage] = types.PipelineStageReviewing

	// Verify the complete pipeline state
	t.Run("verify incident facts can be read from state", func(t *testing.T) {
		var facts types.IncidentFacts
		if err := json.Unmarshal([]byte(state[types.StateKeyIncidentFacts]), &facts); err != nil {
			t.Fatalf("failed to unmarshal incident facts: %v", err)
		}
		if len(facts.Symptoms) != 1 {
			t.Errorf("expected 1 symptom, got %d", len(facts.Symptoms))
		}
		if facts.Symptoms[0].Severity != "critical" {
			t.Errorf("expected severity 'critical', got '%s'", facts.Symptoms[0].Severity)
		}
	})

	t.Run("verify system snapshot can be read from state", func(t *testing.T) {
		var snapshot types.SystemSnapshot
		if err := json.Unmarshal([]byte(state[types.StateKeySystemSnapshot]), &snapshot); err != nil {
			t.Fatalf("failed to unmarshal system snapshot: %v", err)
		}
		if snapshot.ClusterHealth == nil {
			t.Fatal("expected cluster health to be set")
		}
		if len(snapshot.CausalPaths) != 1 {
			t.Errorf("expected 1 causal path, got %d", len(snapshot.CausalPaths))
		}
	})

	t.Run("verify raw hypotheses can be read from state", func(t *testing.T) {
		var hypotheses []types.Hypothesis
		if err := json.Unmarshal([]byte(state[types.StateKeyRawHypotheses]), &hypotheses); err != nil {
			t.Fatalf("failed to unmarshal raw hypotheses: %v", err)
		}
		if len(hypotheses) != 1 {
			t.Errorf("expected 1 hypothesis, got %d", len(hypotheses))
		}
		if hypotheses[0].Status != types.HypothesisStatusPending {
			t.Errorf("expected status 'pending', got '%s'", hypotheses[0].Status)
		}
	})

	t.Run("verify reviewed hypotheses can be read from state", func(t *testing.T) {
		var reviewed types.ReviewedHypotheses
		if err := json.Unmarshal([]byte(state[types.StateKeyReviewedHypotheses]), &reviewed); err != nil {
			t.Fatalf("failed to unmarshal reviewed hypotheses: %v", err)
		}
		if len(reviewed.Hypotheses) != 1 {
			t.Errorf("expected 1 hypothesis, got %d", len(reviewed.Hypotheses))
		}
		if reviewed.Hypotheses[0].Status != types.HypothesisStatusApproved {
			t.Errorf("expected status 'approved', got '%s'", reviewed.Hypotheses[0].Status)
		}
		if reviewed.ReviewNotes == "" {
			t.Error("expected review notes to be set")
		}
	})

	t.Run("verify final hypotheses matches reviewed", func(t *testing.T) {
		if state[types.StateKeyFinalHypotheses] != state[types.StateKeyReviewedHypotheses] {
			t.Error("expected final hypotheses to match reviewed hypotheses")
		}
	})
}

// TestStateKeyConstants verifies the state key constants are correctly prefixed.
func TestStateKeyConstants(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		expected string
	}{
		{"IncidentFacts", types.StateKeyIncidentFacts, "temp:incident_facts"},
		{"SystemSnapshot", types.StateKeySystemSnapshot, "temp:system_snapshot"},
		{"RawHypotheses", types.StateKeyRawHypotheses, "temp:raw_hypotheses"},
		{"ReviewedHypotheses", types.StateKeyReviewedHypotheses, "temp:reviewed_hypotheses"},
		{"FinalHypotheses", types.StateKeyFinalHypotheses, "final_hypotheses"},
		{"PipelineStage", types.StateKeyPipelineStage, "temp:pipeline_stage"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.key != tc.expected {
				t.Errorf("expected key '%s', got '%s'", tc.expected, tc.key)
			}
		})
	}
}

// TestPipelineStageConstants verifies pipeline stage values.
func TestPipelineStageConstants(t *testing.T) {
	stages := []string{
		types.PipelineStageIntake,
		types.PipelineStageGathering,
		types.PipelineStageBuilding,
		types.PipelineStageReviewing,
	}

	// Verify stages are distinct
	seen := make(map[string]bool)
	for _, stage := range stages {
		if seen[stage] {
			t.Errorf("duplicate pipeline stage: %s", stage)
		}
		seen[stage] = true
	}

	// Verify expected values
	if types.PipelineStageIntake != "intake" {
		t.Errorf("unexpected intake stage: %s", types.PipelineStageIntake)
	}
	if types.PipelineStageGathering != "gathering" {
		t.Errorf("unexpected gathering stage: %s", types.PipelineStageGathering)
	}
	if types.PipelineStageBuilding != "building" {
		t.Errorf("unexpected building stage: %s", types.PipelineStageBuilding)
	}
	if types.PipelineStageReviewing != "reviewing" {
		t.Errorf("unexpected reviewing stage: %s", types.PipelineStageReviewing)
	}
}
