package logprocessing

import (
	"testing"
)

func TestExtractMessage_JSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "JSON with msg field",
			input:    `{"msg":"test"}`,
			expected: "test",
		},
		{
			name:     "JSON with message field",
			input:    `{"message":"hello world"}`,
			expected: "hello world",
		},
		{
			name:     "JSON with log field",
			input:    `{"log":"kubernetes log"}`,
			expected: "kubernetes log",
		},
		{
			name:     "plain text",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "JSON without message field",
			input:    `{"level":"info","data":"value"}`,
			expected: `{"level":"info","data":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMessage(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractMessage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPreProcess(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "uppercase with whitespace",
			input:    "  UPPERCASE  ",
			expected: "uppercase",
		},
		{
			name:     "mixed case",
			input:    "MiXeD CaSe",
			expected: "mixed case",
		},
		{
			name:     "JSON extraction and normalization",
			input:    `{"msg":"ERROR Message"}`,
			expected: "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreProcess(tt.input)
			if result != tt.expected {
				t.Errorf("PreProcess(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
