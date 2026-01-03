package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClassifySymptomType validates symptom classification from observed facts
func TestClassifySymptomType(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		errorMessage    string
		containerIssues []string
		expectedType    string
	}{
		// Container issue classification (highest priority)
		{
			name:            "ImagePullBackOff from container issue",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"ImagePullBackOff"},
			expectedType:    "ImagePullError",
		},
		{
			name:            "ErrImagePull from container issue",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"ErrImagePull"},
			expectedType:    "ImagePullError",
		},
		{
			name:            "CrashLoopBackOff from container issue",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"CrashLoopBackOff"},
			expectedType:    "CrashLoop",
		},
		{
			name:            "OOMKilled from container issue",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"OOMKilled"},
			expectedType:    "OOMKilled",
		},
		{
			name:            "ContainerCreating status",
			status:          "Pending",
			errorMessage:    "",
			containerIssues: []string{"ContainerCreating"},
			expectedType:    "ContainerStartup",
		},
		{
			name:            "Multiple container issues - first takes priority",
			status:          "Error",
			errorMessage:    "",
			containerIssues: []string{"ImagePullBackOff", "CrashLoopBackOff"},
			expectedType:    "ImagePullError",
		},

		// Error message pattern matching
		{
			name:            "Image pull error from message",
			status:          "Error",
			errorMessage:    "Failed to pull image from registry",
			containerIssues: []string{},
			expectedType:    "ImagePullError",
		},
		{
			name:            "Image failed error from message",
			status:          "Error",
			errorMessage:    "image pull failed: unauthorized",
			containerIssues: []string{},
			expectedType:    "ImagePullError",
		},
		{
			name:            "Crash from message",
			status:          "Error",
			errorMessage:    "Container crashed with exit code 1",
			containerIssues: []string{},
			expectedType:    "CrashLoop",
		},
		{
			name:            "Crash and backoff from message",
			status:          "Error",
			errorMessage:    "Container crashed and is in back-off",
			containerIssues: []string{},
			expectedType:    "CrashLoop",
		},
		{
			name:            "OOM from message",
			status:          "Error",
			errorMessage:    "Container killed due to OOM",
			containerIssues: []string{},
			expectedType:    "OOMKilled",
		},
		{
			name:            "Out of memory from message",
			status:          "Error",
			errorMessage:    "Out of memory error occurred",
			containerIssues: []string{},
			expectedType:    "OOMKilled",
		},
		{
			name:            "Evicted from message",
			status:          "Failed",
			errorMessage:    "Pod was evicted due to resource pressure",
			containerIssues: []string{},
			expectedType:    "Evicted",
		},
		{
			name:            "Unschedulable from message",
			status:          "Pending",
			errorMessage:    "Pod is unschedulable due to node selector",
			containerIssues: []string{},
			expectedType:    "SchedulingFailure",
		},
		{
			name:            "Insufficient resources from message",
			status:          "Pending",
			errorMessage:    "0/3 nodes available: insufficient cpu",
			containerIssues: []string{},
			expectedType:    "SchedulingFailure",
		},

		// Case insensitivity
		{
			name:            "Case insensitive - IMAGE",
			status:          "Error",
			errorMessage:    "Failed to PULL IMAGE",
			containerIssues: []string{},
			expectedType:    "ImagePullError",
		},
		{
			name:            "Case insensitive - CRASH",
			status:          "Error",
			errorMessage:    "Container CRASH detected",
			containerIssues: []string{},
			expectedType:    "CrashLoop",
		},

		// Status-based fallback
		{
			name:            "Generic Error status",
			status:          "Error",
			errorMessage:    "Something went wrong",
			containerIssues: []string{},
			expectedType:    "Error",
		},
		{
			name:            "Warning status",
			status:          "Warning",
			errorMessage:    "Resource limit exceeded",
			containerIssues: []string{},
			expectedType:    "Warning",
		},
		{
			name:            "Terminating status",
			status:          "Terminating",
			errorMessage:    "",
			containerIssues: []string{},
			expectedType:    "Terminating",
		},
		{
			name:            "Pending with node reference",
			status:          "Pending",
			errorMessage:    "Waiting for node assignment",
			containerIssues: []string{},
			expectedType:    "SchedulingFailure",
		},
		{
			name:            "Pending without node reference",
			status:          "Pending",
			errorMessage:    "Waiting for startup",
			containerIssues: []string{},
			expectedType:    "Pending",
		},
		{
			name:            "Unknown status",
			status:          "Unknown",
			errorMessage:    "",
			containerIssues: []string{},
			expectedType:    "Unknown",
		},

		// Container issues take priority over error message
		{
			name:            "Container issue overrides message",
			status:          "Error",
			errorMessage:    "Out of memory error",
			containerIssues: []string{"CrashLoopBackOff"},
			expectedType:    "CrashLoop", // CrashLoop, not OOMKilled
		},

		// Edge cases
		{
			name:            "Empty everything",
			status:          "",
			errorMessage:    "",
			containerIssues: []string{},
			expectedType:    "Unknown",
		},
		{
			name:            "Empty container issues slice",
			status:          "Error",
			errorMessage:    "Unknown error",
			containerIssues: []string{},
			expectedType:    "Error",
		},
		{
			name:            "Nil container issues treated as empty",
			status:          "Error",
			errorMessage:    "Unknown error",
			containerIssues: nil,
			expectedType:    "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifySymptomType(tt.status, tt.errorMessage, tt.containerIssues)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

// TestClassifySymptomTypePriority validates the priority order
func TestClassifySymptomTypePriority(t *testing.T) {
	t.Run("Container issues have priority over error message", func(t *testing.T) {
		// Even though error message says OOM, container issue says ImagePull
		result := classifySymptomType(
			"Error",
			"Out of memory",
			[]string{"ImagePullBackOff"},
		)
		assert.Equal(t, "ImagePullError", result, "Container issue should take priority")
	})

	t.Run("Error message has priority over status", func(t *testing.T) {
		// Even though status is Error, error message indicates crash
		result := classifySymptomType(
			"Error",
			"Container crashed with exit code 1",
			[]string{},
		)
		assert.Equal(t, "CrashLoop", result, "Error message should take priority over status")
	})

	t.Run("Status is fallback when no other indicators", func(t *testing.T) {
		result := classifySymptomType(
			"Warning",
			"",
			[]string{},
		)
		assert.Equal(t, "Warning", result, "Status should be used as fallback")
	})
}

// TestClassifySymptomTypeEdgeCases validates edge case handling
func TestClassifySymptomTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		errorMessage    string
		containerIssues []string
		expectedType    string
	}{
		{
			name:            "Very long error message",
			status:          "Error",
			errorMessage:    "This is a very long error message that contains the word image and pull and failed but should still be detected correctly",
			containerIssues: []string{},
			expectedType:    "ImagePullError",
		},
		{
			name:            "Error message with special characters",
			status:          "Error",
			errorMessage:    "Failed to pull image: @#$%^&*()",
			containerIssues: []string{},
			expectedType:    "ImagePullError",
		},
		{
			name:            "Multiple keywords in message",
			status:          "Error",
			errorMessage:    "image pull failed and container crashed due to OOM",
			containerIssues: []string{},
			expectedType:    "ImagePullError", // First match wins
		},
		{
			name:            "Partial keyword match with 'pull' and 'failed'",
			status:          "Error",
			errorMessage:    "image pull failed for container",
			containerIssues: []string{},
			expectedType:    "ImagePullError", // Contains "image", "pull", and "failed"
		},
		{
			name:            "Status with mixed case",
			status:          "eRrOr",
			errorMessage:    "",
			containerIssues: []string{},
			expectedType:    "Unknown", // Status comparison is case-sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifySymptomType(tt.status, tt.errorMessage, tt.containerIssues)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

// TestClassifySymptomTypeRealWorldExamples validates with realistic Kubernetes errors
func TestClassifySymptomTypeRealWorldExamples(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		errorMessage    string
		containerIssues []string
		expectedType    string
	}{
		{
			name:   "Real ImagePullBackOff error",
			status: "Error",
			errorMessage: `Back-off pulling image "nginx:nonexistent"`,
			containerIssues: []string{"ImagePullBackOff"},
			expectedType:    "ImagePullError",
		},
		{
			name:   "Real CrashLoopBackOff error",
			status: "Error",
			errorMessage: "Back-off restarting failed container",
			containerIssues: []string{"CrashLoopBackOff"},
			expectedType:    "CrashLoop",
		},
		{
			name:   "Real OOMKilled error",
			status: "Error",
			errorMessage: "Container was OOMKilled",
			containerIssues: []string{"OOMKilled"},
			expectedType:    "OOMKilled",
		},
		{
			name:   "Real scheduling failure",
			status: "Pending",
			errorMessage: "0/3 nodes are available: 3 Insufficient cpu.",
			containerIssues: []string{},
			expectedType:    "SchedulingFailure",
		},
		{
			name:   "Real eviction",
			status: "Failed",
			errorMessage: "The node was low on resource: ephemeral-storage. Pod evicted. Container app was using 10Gi, which exceeds its request of 0.",
			containerIssues: []string{},
			expectedType:    "Evicted", // Message must contain "evicted" keyword
		},
		{
			name:   "Real image pull authentication failure",
			status: "Error",
			errorMessage: `Failed to pull image "private.registry.io/app:v1": rpc error: code = Unknown desc = failed to pull and unpack image "private.registry.io/app:v1": failed to resolve reference "private.registry.io/app:v1": failed to authorize: failed to fetch oauth token: unexpected status: 401 Unauthorized`,
			containerIssues: []string{"ErrImagePull"},
			expectedType:    "ImagePullError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifySymptomType(tt.status, tt.errorMessage, tt.containerIssues)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}
