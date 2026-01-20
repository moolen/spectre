package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogger_WriteEvents(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")

	// Create logger
	logger, err := NewLogger(logPath, "test-session-123")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Log various events
	if err := logger.LogSessionStart("claude-3", "http://localhost:8080"); err != nil {
		t.Errorf("LogSessionStart failed: %v", err)
	}

	if err := logger.LogUserMessage("test message"); err != nil {
		t.Errorf("LogUserMessage failed: %v", err)
	}

	if err := logger.LogAgentActivated("incident_intake_agent"); err != nil {
		t.Errorf("LogAgentActivated failed: %v", err)
	}

	if err := logger.LogToolStart("incident_intake_agent", "cluster_health", map[string]interface{}{"namespace": "default"}); err != nil {
		t.Errorf("LogToolStart failed: %v", err)
	}

	if err := logger.LogToolComplete("incident_intake_agent", "cluster_health", true, 100*time.Millisecond, map[string]interface{}{"status": "ok"}); err != nil {
		t.Errorf("LogToolComplete failed: %v", err)
	}

	if err := logger.LogAgentText("incident_intake_agent", "test response", false); err != nil {
		t.Errorf("LogAgentText failed: %v", err)
	}

	if err := logger.LogError("incident_intake_agent", errors.New("test error")); err != nil {
		t.Errorf("LogError failed: %v", err)
	}

	if err := logger.LogPipelineComplete(5 * time.Second); err != nil {
		t.Errorf("LogPipelineComplete failed: %v", err)
	}

	if err := logger.LogSessionEnd(); err != nil {
		t.Errorf("LogSessionEnd failed: %v", err)
	}

	// Close logger
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	// Read and verify log file
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var events []Event
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Errorf("failed to unmarshal event: %v", err)
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error scanning log file: %v", err)
	}

	// Verify event count
	expectedCount := 9
	if len(events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(events))
	}

	// Verify event types in order
	expectedTypes := []EventType{
		EventTypeSessionStart,
		EventTypeUserMessage,
		EventTypeAgentActivated,
		EventTypeToolStart,
		EventTypeToolComplete,
		EventTypeAgentText,
		EventTypeError,
		EventTypePipelineComplete,
		EventTypeSessionEnd,
	}

	for i, expected := range expectedTypes {
		if i >= len(events) {
			break
		}
		if events[i].Type != expected {
			t.Errorf("event %d: expected type %s, got %s", i, expected, events[i].Type)
		}
		if events[i].SessionID != "test-session-123" {
			t.Errorf("event %d: expected session ID test-session-123, got %s", i, events[i].SessionID)
		}
	}

	// Verify specific event data
	if events[0].Data["model"] != "claude-3" {
		t.Errorf("session start: expected model claude-3, got %v", events[0].Data["model"])
	}

	if events[1].Data["message"] != "test message" {
		t.Errorf("user message: expected 'test message', got %v", events[1].Data["message"])
	}

	if events[2].Agent != "incident_intake_agent" {
		t.Errorf("agent activated: expected agent incident_intake_agent, got %s", events[2].Agent)
	}

	if events[3].Data["tool_name"] != "cluster_health" {
		t.Errorf("tool start: expected tool_name cluster_health, got %v", events[3].Data["tool_name"])
	}

	if events[4].Data["success"] != true {
		t.Errorf("tool complete: expected success true, got %v", events[4].Data["success"])
	}

	if events[6].Data["error"] != "test error" {
		t.Errorf("error: expected error 'test error', got %v", events[6].Data["error"])
	}
}

func TestLogger_Append(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")

	// Create first logger and write an event
	logger1, err := NewLogger(logPath, "session-1")
	if err != nil {
		t.Fatalf("failed to create logger 1: %v", err)
	}
	if err := logger1.LogSessionStart("claude-3", "http://localhost:8080"); err != nil {
		t.Errorf("LogSessionStart failed: %v", err)
	}
	if err := logger1.Close(); err != nil {
		t.Fatalf("failed to close logger 1: %v", err)
	}

	// Create second logger (should append)
	logger2, err := NewLogger(logPath, "session-2")
	if err != nil {
		t.Fatalf("failed to create logger 2: %v", err)
	}
	if err := logger2.LogSessionStart("claude-3", "http://localhost:8080"); err != nil {
		t.Errorf("LogSessionStart failed: %v", err)
	}
	if err := logger2.Close(); err != nil {
		t.Fatalf("failed to close logger 2: %v", err)
	}

	// Read and verify both events exist
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var events []Event
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Errorf("failed to unmarshal event: %v", err)
			continue
		}
		events = append(events, event)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	if events[0].SessionID != "session-1" {
		t.Errorf("first event: expected session-1, got %s", events[0].SessionID)
	}

	if events[1].SessionID != "session-2" {
		t.Errorf("second event: expected session-2, got %s", events[1].SessionID)
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")

	// Create logger
	logger, err := NewLogger(logPath, "test-session")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write events concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				_ = logger.LogAgentActivated("test-agent")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Close and verify file is readable
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	// Read and count events
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Errorf("failed to unmarshal event: %v", err)
			continue
		}
		count++
	}

	expected := 100
	if count != expected {
		t.Errorf("expected %d events, got %d", expected, count)
	}
}
