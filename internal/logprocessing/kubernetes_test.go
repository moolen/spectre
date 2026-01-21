package logprocessing

import (
	"testing"
)

func TestMaskKubernetesNames_Pods(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Pod name",
			input:    "pod nginx-deployment-66b6c48dd5-8w7xz started",
			expected: "pod <K8S_NAME> started",
		},
		{
			name:     "Multiple pod names",
			input:    "pod app-abc12345-xyz78 and pod service-def67890-abc12",
			expected: "pod <K8S_NAME> and pod <K8S_NAME>",
		},
		{
			name:     "Pod name in context",
			input:    "container in pod api-server-7d9b8c6f5d-4k2m1 crashed",
			expected: "container in pod <K8S_NAME> crashed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskKubernetesNames(tt.input)
			if result != tt.expected {
				t.Errorf("MaskKubernetesNames(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskKubernetesNames_ReplicaSets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ReplicaSet name",
			input:    "replicaset nginx-deployment-66b6c48dd5 created",
			expected: "replicaset <K8S_NAME> created",
		},
		{
			name:     "ReplicaSet scaling",
			input:    "scaled replicaset api-server-7d9b8c6f5d to 3 replicas",
			expected: "scaled replicaset <K8S_NAME> to 3 replicas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskKubernetesNames(tt.input)
			if result != tt.expected {
				t.Errorf("MaskKubernetesNames(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskKubernetesNames_NoMatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Plain deployment name",
			input: "deployment nginx created",
		},
		{
			name:  "Short hash",
			input: "app-abc created",
		},
		{
			name:  "No Kubernetes names",
			input: "regular log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskKubernetesNames(tt.input)
			if result != tt.input {
				t.Errorf("MaskKubernetesNames(%q) = %q, want %q (unchanged)", tt.input, result, tt.input)
			}
		})
	}
}
