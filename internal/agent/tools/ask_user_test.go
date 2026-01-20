package tools

import (
	"testing"
)

func TestParseUserResponse_ExplicitYes(t *testing.T) {
	testCases := []string{"yes", "Yes", "YES", "y", "Y", "yeah", "yep", "correct", "confirmed", "ok", "okay"}

	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			result := ParseUserResponse(input, false)
			if !result.Confirmed {
				t.Errorf("expected Confirmed=true for input %q", input)
			}
			if result.HasClarification {
				t.Errorf("expected HasClarification=false for input %q", input)
			}
		})
	}
}

func TestParseUserResponse_ExplicitNo(t *testing.T) {
	testCases := []string{"no", "No", "NO", "n", "N", "nope", "wrong", "incorrect"}

	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			result := ParseUserResponse(input, true) // Even with defaultConfirm=true
			if result.Confirmed {
				t.Errorf("expected Confirmed=false for input %q", input)
			}
			if result.HasClarification {
				t.Errorf("expected HasClarification=false for input %q", input)
			}
		})
	}
}

func TestParseUserResponse_EmptyWithDefaultConfirm(t *testing.T) {
	result := ParseUserResponse("", true)
	if !result.Confirmed {
		t.Error("expected Confirmed=true for empty input with defaultConfirm=true")
	}
	if result.HasClarification {
		t.Error("expected HasClarification=false for empty input")
	}
}

func TestParseUserResponse_EmptyWithoutDefaultConfirm(t *testing.T) {
	result := ParseUserResponse("", false)
	if result.Confirmed {
		t.Error("expected Confirmed=false for empty input with defaultConfirm=false")
	}
	if result.HasClarification {
		t.Error("expected HasClarification=false for empty input")
	}
}

func TestParseUserResponse_WhitespaceOnly(t *testing.T) {
	result := ParseUserResponse("   \t\n  ", true)
	if !result.Confirmed {
		t.Error("expected whitespace-only to be treated as empty (defaultConfirm=true)")
	}
}

func TestParseUserResponse_Clarification(t *testing.T) {
	testCases := []string{
		"Actually the namespace is production",
		"The time was about 30 minutes ago",
		"wait, I also saw errors in the api-gateway",
		"It started at 10am",
	}

	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			result := ParseUserResponse(input, true)
			if result.Confirmed {
				t.Errorf("expected Confirmed=false for clarification input %q", input)
			}
			if !result.HasClarification {
				t.Errorf("expected HasClarification=true for input %q", input)
			}
			if result.Response != input {
				t.Errorf("expected Response=%q, got %q", input, result.Response)
			}
		})
	}
}

func TestParseUserResponse_TrimsWhitespace(t *testing.T) {
	result := ParseUserResponse("  yes  ", false)
	if !result.Confirmed {
		t.Error("expected Confirmed=true after trimming whitespace")
	}
	if result.Response != "yes" {
		t.Errorf("expected Response to be trimmed, got %q", result.Response)
	}
}

func TestPendingUserQuestion_Fields(t *testing.T) {
	pending := PendingUserQuestion{
		Question:       "Is this correct?",
		Summary:        "Found 3 symptoms",
		DefaultConfirm: true,
		AgentName:      "incident_intake_agent",
	}

	if pending.Question != "Is this correct?" {
		t.Errorf("unexpected Question: %s", pending.Question)
	}
	if pending.Summary != "Found 3 symptoms" {
		t.Errorf("unexpected Summary: %s", pending.Summary)
	}
	if !pending.DefaultConfirm {
		t.Error("expected DefaultConfirm=true")
	}
	if pending.AgentName != "incident_intake_agent" {
		t.Errorf("unexpected AgentName: %s", pending.AgentName)
	}
}

func TestUserQuestionResponse_Fields(t *testing.T) {
	resp := UserQuestionResponse{
		Confirmed:        false,
		Response:         "Actually it's in the staging namespace",
		HasClarification: true,
	}

	if resp.Confirmed {
		t.Error("expected Confirmed=false")
	}
	if resp.Response != "Actually it's in the staging namespace" {
		t.Errorf("unexpected Response: %s", resp.Response)
	}
	if !resp.HasClarification {
		t.Error("expected HasClarification=true")
	}
}

func TestAskUserQuestionArgs_Fields(t *testing.T) {
	args := AskUserQuestionArgs{
		Question:       "Please confirm the extracted information.",
		Summary:        "Symptoms: pod crash loop",
		DefaultConfirm: true,
	}

	if args.Question != "Please confirm the extracted information." {
		t.Errorf("unexpected Question: %s", args.Question)
	}
	if args.Summary != "Symptoms: pod crash loop" {
		t.Errorf("unexpected Summary: %s", args.Summary)
	}
	if !args.DefaultConfirm {
		t.Error("expected DefaultConfirm=true")
	}
}

func TestNewAskUserQuestionTool_ReturnsValidTool(t *testing.T) {
	tool, err := NewAskUserQuestionTool()
	if err != nil {
		t.Fatalf("unexpected error creating tool: %v", err)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}
