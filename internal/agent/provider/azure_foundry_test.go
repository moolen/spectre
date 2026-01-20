package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAzureFoundryProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AzureFoundryConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: AzureFoundryConfig{
				Endpoint: "https://test.services.ai.azure.com",
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			cfg: AzureFoundryConfig{
				APIKey: "test-key",
			},
			wantErr: true,
			errMsg:  "endpoint is required",
		},
		{
			name: "missing api key",
			cfg: AzureFoundryConfig{
				Endpoint: "https://test.services.ai.azure.com",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "endpoint without anthropic suffix",
			cfg: AzureFoundryConfig{
				Endpoint: "https://test.services.ai.azure.com",
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "endpoint with anthropic suffix",
			cfg: AzureFoundryConfig{
				Endpoint: "https://test.services.ai.azure.com/anthropic",
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "endpoint with trailing slash",
			cfg: AzureFoundryConfig{
				Endpoint: "https://test.services.ai.azure.com/",
				APIKey:   "test-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAzureFoundryProvider(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if provider == nil {
				t.Error("expected provider, got nil")
			}
		})
	}
}

func TestAzureFoundryProvider_Name(t *testing.T) {
	provider, _ := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: "https://test.services.ai.azure.com",
		APIKey:   "test-key",
	})

	if got := provider.Name(); got != "azure-foundry" {
		t.Errorf("Name() = %q, want %q", got, "azure-foundry")
	}
}

func TestAzureFoundryProvider_Model(t *testing.T) {
	provider, _ := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: "https://test.services.ai.azure.com",
		APIKey:   "test-key",
		Model:    "claude-3-5-sonnet",
	})

	if got := provider.Model(); got != "claude-3-5-sonnet" {
		t.Errorf("Model() = %q, want %q", got, "claude-3-5-sonnet")
	}
}

func TestAzureFoundryProvider_DefaultModel(t *testing.T) {
	provider, _ := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: "https://test.services.ai.azure.com",
		APIKey:   "test-key",
	})

	expected := DefaultAzureFoundryConfig().Model
	if got := provider.Model(); got != expected {
		t.Errorf("Model() = %q, want default %q", got, expected)
	}
}

func TestAzureFoundryProvider_Chat(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/anthropic/v1/messages" {
			t.Errorf("expected /anthropic/v1/messages, got %s", r.URL.Path)
		}

		// Verify headers
		if apiKey := r.Header.Get("x-api-key"); apiKey != "test-key" {
			t.Errorf("expected x-api-key header 'test-key', got %q", apiKey)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", contentType)
		}
		if version := r.Header.Get("anthropic-version"); version != "2023-06-01" {
			t.Errorf("expected anthropic-version '2023-06-01', got %q", version)
		}

		// Return a mock response
		resp := azureResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []azureResponseBlock{
				{Type: "text", Text: "Hello! How can I help you?"},
			},
			Model:      "claude-3-5-sonnet",
			StopReason: "end_turn",
			Usage: azureUsage{
				InputTokens:  10,
				OutputTokens: 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with test server URL
	provider, err := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Model:    "claude-3-5-sonnet",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Make a chat request
	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}
	resp, err := provider.Chat(context.Background(), "You are a helpful assistant.", messages, nil)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}

	// Verify response
	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello! How can I help you?")
	}
	if resp.StopReason != StopReasonEndTurn {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, StopReasonEndTurn)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want %d", resp.Usage.InputTokens, 10)
	}
	if resp.Usage.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want %d", resp.Usage.OutputTokens, 8)
	}
}

func TestAzureFoundryProvider_ChatWithTools(t *testing.T) {
	// Create a test server that returns a tool use response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode request to verify tools are sent
		var req azureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify tools were sent
		if len(req.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(req.Tools))
		}
		if req.Tools[0].Name != "get_weather" {
			t.Errorf("expected tool name 'get_weather', got %q", req.Tools[0].Name)
		}

		// Return a tool use response
		resp := azureResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []azureResponseBlock{
				{
					Type:  "tool_use",
					ID:    "toolu_123",
					Name:  "get_weather",
					Input: json.RawMessage(`{"location": "San Francisco"}`),
				},
			},
			Model:      "claude-3-5-sonnet",
			StopReason: "tool_use",
			Usage: azureUsage{
				InputTokens:  20,
				OutputTokens: 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	tools := []ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the weather for a location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city to get weather for",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	messages := []Message{
		{Role: RoleUser, Content: "What's the weather in San Francisco?"},
	}

	resp, err := provider.Chat(context.Background(), "", messages, tools)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}

	// Verify tool call response
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "get_weather" {
		t.Errorf("tool name = %q, want %q", resp.ToolCalls[0].Name, "get_weather")
	}
	if resp.StopReason != StopReasonToolUse {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, StopReasonToolUse)
	}
}

func TestAzureFoundryProvider_ErrorHandling(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := azureErrorResponse{
			Type: "error",
		}
		resp.Error.Type = "authentication_error"
		resp.Error.Message = "Invalid API key"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: server.URL,
		APIKey:   "invalid-key",
	})

	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}

	_, err := provider.Chat(context.Background(), "", messages, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error contains useful information
	errStr := err.Error()
	if !contains(errStr, "401") && !contains(errStr, "authentication_error") {
		t.Errorf("error should contain status code or error type: %v", err)
	}
}

func TestAzureFoundryProvider_ConvertMessage(t *testing.T) {
	provider, _ := NewAzureFoundryProvider(AzureFoundryConfig{
		Endpoint: "https://test.services.ai.azure.com",
		APIKey:   "test-key",
	})

	tests := []struct {
		name    string
		message Message
		want    azureMessage
	}{
		{
			name: "user text message",
			message: Message{
				Role:    RoleUser,
				Content: "Hello",
			},
			want: azureMessage{
				Role: "user",
				Content: []azureContentPart{
					{Type: "text", Text: "Hello"},
				},
			},
		},
		{
			name: "assistant text message",
			message: Message{
				Role:    RoleAssistant,
				Content: "Hi there!",
			},
			want: azureMessage{
				Role: "assistant",
				Content: []azureContentPart{
					{Type: "text", Text: "Hi there!"},
				},
			},
		},
		{
			name: "tool result message",
			message: Message{
				Role: RoleUser,
				ToolResult: []ToolResultBlock{
					{
						ToolUseID: "toolu_123",
						Content:   `{"temperature": 72}`,
						IsError:   false,
					},
				},
			},
			want: azureMessage{
				Role: "user",
				Content: []azureContentPart{
					{
						Type:      "tool_result",
						ToolUseID: "toolu_123",
						Content:   `{"temperature": 72}`,
						IsError:   false,
					},
				},
			},
		},
		{
			name: "assistant with tool use",
			message: Message{
				Role: RoleAssistant,
				ToolUse: []ToolUseBlock{
					{
						ID:    "toolu_123",
						Name:  "get_weather",
						Input: json.RawMessage(`{"location": "NYC"}`),
					},
				},
			},
			want: azureMessage{
				Role: "assistant",
				Content: []azureContentPart{
					{
						Type:  "tool_use",
						ID:    "toolu_123",
						Name:  "get_weather",
						Input: json.RawMessage(`{"location": "NYC"}`),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.convertMessage(tt.message)
			if got.Role != tt.want.Role {
				t.Errorf("Role = %q, want %q", got.Role, tt.want.Role)
			}
			if len(got.Content) != len(tt.want.Content) {
				t.Errorf("Content length = %d, want %d", len(got.Content), len(tt.want.Content))
				return
			}
			for i := range got.Content {
				if got.Content[i].Type != tt.want.Content[i].Type {
					t.Errorf("Content[%d].Type = %q, want %q", i, got.Content[i].Type, tt.want.Content[i].Type)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
