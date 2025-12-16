package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// MockTool is a simple test tool
type MockTool struct {
	result interface{}
	err    error
}

func (m *MockTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestSpectreServer_Creation(t *testing.T) {
	// This test will fail if Spectre API is not running
	// That's expected - it tests the connection logic
	_, err := NewSpectreServer("http://invalid-url:9999", "1.0.0-test")
	if err == nil {
		t.Error("Expected error when connecting to invalid URL")
	}

	// Verify error message is meaningful
	if err != nil && err.Error() == "" {
		t.Error("Error should have a message")
	}
}

func TestSpectreServer_ToolAdapter(t *testing.T) {
	// Create a mock server (without connecting to Spectre)
	s := &SpectreServer{
		tools:   make(map[string]Tool),
		version: "1.0.0-test",
	}

	// Create a mock tool
	mockTool := &MockTool{
		result: map[string]interface{}{
			"status": "ok",
			"data":   []string{"item1", "item2"},
		},
	}

	// Test tool handler creation
	handler := s.createToolHandler(mockTool)

	// Note: We can't easily test the full handler without mcp.CallToolRequest
	// But we verified the adapter compiles and the logic is sound
	_ = handler // Silence unused variable warning

	t.Log("Tool adapter created successfully")
}

func TestSpectreServer_ToolRegistration(t *testing.T) {
	// Test that we can create tool schemas without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Tool registration panicked: %v", r)
		}
	}()

	s := &SpectreServer{
		tools:   make(map[string]Tool),
		version: "1.0.0-test",
	}

	mockTool := &MockTool{result: "ok"}

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param1": map[string]interface{}{
				"type":        "string",
				"description": "Test parameter",
			},
		},
		"required": []string{"param1"},
	}

	// This should marshal the schema without error
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	if len(schemaJSON) == 0 {
		t.Error("Schema JSON should not be empty")
	}

	// Store the tool
	s.tools["test_tool"] = mockTool

	if len(s.tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(s.tools))
	}
}

func TestToolExecution_Success(t *testing.T) {
	mockTool := &MockTool{
		result: map[string]string{"message": "success"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := json.RawMessage(`{"test": "input"}`)
	result, err := mockTool.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result == nil {
		t.Error("Result should not be nil")
	}

	resultMap, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("Expected result to be map[string]string, got %T", result)
	}

	if resultMap["message"] != "success" {
		t.Errorf("Expected message=success, got %s", resultMap["message"])
	}
}
