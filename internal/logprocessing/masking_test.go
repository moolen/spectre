package logprocessing

import (
	"testing"
)

func TestAggressiveMask_IPAddresses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 address",
			input:    "connected to 10.0.0.1",
			expected: "connected to <IP>",
		},
		{
			name:     "IPv6 address",
			input:    "connected to fe80::1",
			expected: "connected to <IP>",
		},
		{
			name:     "Multiple IPs",
			input:    "from 192.168.1.1 to 192.168.1.2",
			expected: "from <IP> to <IP>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggressiveMask(tt.input)
			if result != tt.expected {
				t.Errorf("AggressiveMask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAggressiveMask_UUIDs(t *testing.T) {
	input := "request id 123e4567-e89b-12d3-a456-426614174000"
	expected := "request id <UUID>"
	result := AggressiveMask(input)
	if result != expected {
		t.Errorf("AggressiveMask(%q) = %q, want %q", input, result, expected)
	}
}

func TestAggressiveMask_Timestamps(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ISO8601 timestamp",
			input:    "at 2026-01-21T14:30:00Z",
			expected: "at <TIMESTAMP>",
		},
		{
			name:     "Unix timestamp",
			input:    "timestamp 1737470400",
			expected: "timestamp <TIMESTAMP>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggressiveMask(tt.input)
			if result != tt.expected {
				t.Errorf("AggressiveMask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAggressiveMask_StatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTP status code preserved with returned",
			input:    "returned 404 error",
			expected: "returned 404 error",
		},
		{
			name:     "HTTP status code preserved with status",
			input:    "status code 500",
			expected: "status code 500",
		},
		{
			name:     "Generic number masked",
			input:    "processing 12345 items",
			expected: "processing <NUM> items",
		},
		{
			name:     "Response code preserved",
			input:    "http response 200",
			expected: "http response 200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggressiveMask(tt.input)
			if result != tt.expected {
				t.Errorf("AggressiveMask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAggressiveMask_HexStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Hex with 0x prefix",
			input:    "address 0xDEADBEEF",
			expected: "address <HEX>",
		},
		{
			name:     "Long hex string",
			input:    "hash 1234567890abcdef1234567890abcdef",
			expected: "hash <HEX>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggressiveMask(tt.input)
			if result != tt.expected {
				t.Errorf("AggressiveMask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAggressiveMask_Paths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unix path",
			input:    "file /var/log/app.log",
			expected: "file <PATH>",
		},
		{
			name:     "Windows path",
			input:    "file C:\\Users\\test\\app.log",
			expected: "file <PATH>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggressiveMask(tt.input)
			if result != tt.expected {
				t.Errorf("AggressiveMask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAggressiveMask_URLs(t *testing.T) {
	input := "fetching http://example.com/api/v1/users"
	expected := "fetching <URL>"
	result := AggressiveMask(input)
	if result != expected {
		t.Errorf("AggressiveMask(%q) = %q, want %q", input, result, expected)
	}
}

func TestAggressiveMask_Emails(t *testing.T) {
	input := "sent to user@example.com"
	expected := "sent to <EMAIL>"
	result := AggressiveMask(input)
	if result != expected {
		t.Errorf("AggressiveMask(%q) = %q, want %q", input, result, expected)
	}
}

func TestAggressiveMask_Combined(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple patterns",
			input:    "user@example.com connected from 10.0.0.1 at 2026-01-21T14:30:00Z",
			expected: "<EMAIL> connected from <IP> at <TIMESTAMP>",
		},
		{
			name:     "K8s pod and status code",
			input:    "pod nginx-deployment-66b6c48dd5-8w7xz returned 200",
			expected: "pod <K8S_NAME> returned 200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggressiveMask(tt.input)
			if result != tt.expected {
				t.Errorf("AggressiveMask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
