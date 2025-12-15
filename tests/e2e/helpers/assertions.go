// Package helpers provides assertion utilities for e2e testing.
package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EventuallyOption configures Eventually assertion behavior.
type EventuallyOption struct {
	Timeout  time.Duration
	Interval time.Duration
}

// DefaultEventuallyOption provides sensible defaults for async operations.
var DefaultEventuallyOption = EventuallyOption{
	Timeout:  30 * time.Second,
	Interval: 3 * time.Second,
}

// SlowEventuallyOption for operations that take longer (config reload, etc).
var SlowEventuallyOption = EventuallyOption{
	Timeout:  90 * time.Second,
	Interval: 5 * time.Second,
}

// EventuallyResourceCreated waits for a resource to be created in the API.
func EventuallyResourceCreated(t *testing.T, client *APIClient, namespace, kind, name string, opts EventuallyOption) *Resource {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	var result *Resource

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 90 // Last 90 seconds
		endTime := now + 10   // Slightly into future for clock skew

		resp, err := client.Search(ctx, startTime, endTime, namespace, kind)
		if err != nil {
			t.Logf("Search failed: %v", err)
			return false
		}

		// Find resource by name
		for _, r := range resp.Resources {
			if r.Name == name {
				result = &r
				return true
			}
		}
		return false
	}, opts.Timeout, opts.Interval)

	require.NotNil(t, result, "Resource %s/%s/%s not found in API after %v", namespace, kind, name, opts.Timeout)
	return result
}

// EventuallyEventCreated waits for an event to appear in the API.
func EventuallyEventCreated(t *testing.T, client *APIClient, resourceID, reason string, opts EventuallyOption) *K8sEvent {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	var result *K8sEvent

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 60 // Last 60 seconds
		endTime := now + 10

		resp, err := client.GetEvents(ctx, resourceID, &startTime, &endTime, nil)
		if err != nil {
			t.Logf("GetEvents failed: %v", err)
			return false
		}

		// Find event by reason
		for _, e := range resp.Events {
			if e.Reason == reason {
				result = &e
				return true
			}
		}
		return false
	}, opts.Timeout, opts.Interval)

	require.NotNil(t, result, "Event with reason %s not found after %v", reason, opts.Timeout)
	return result
}

// EventuallyEventCount waits for a specific number of events.
func EventuallyEventCount(t *testing.T, client *APIClient, resourceID string, expectedCount int, opts EventuallyOption) {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 120
		endTime := now + 10

		resp, err := client.GetEvents(ctx, resourceID, &startTime, &endTime, nil)
		if err != nil {
			t.Logf("GetEvents failed: %v", err)
			return false
		}

		t.Logf("Event count: %d (expected: %d)", len(resp.Events), expectedCount)
		return len(resp.Events) >= expectedCount
	}, opts.Timeout, opts.Interval)
}

// EventuallySegmentsCount waits for a specific number of segments.
func EventuallySegmentsCount(t *testing.T, client *APIClient, resourceID string, expectedCount int, opts EventuallyOption) {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 120
		endTime := now + 10

		resp, err := client.GetSegments(ctx, resourceID, &startTime, &endTime)
		if err != nil {
			t.Logf("GetSegments failed: %v", err)
			return false
		}

		t.Logf("Segment count: %d (expected: %d)", len(resp.Segments), expectedCount)
		return len(resp.Segments) >= expectedCount
	}, opts.Timeout, opts.Interval)
}

// EventuallyCondition waits for a custom condition to be true.
func EventuallyCondition(t *testing.T, condition func() bool, opts EventuallyOption) {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	assert.Eventually(t, condition, opts.Timeout, opts.Interval)
}

// AssertEventExists verifies an event exists with expected properties.
func AssertEventExists(t *testing.T, event *K8sEvent, expectedReason string) {
	require.NotNil(t, event)
	assert.Equal(t, expectedReason, event.Reason, "Event reason mismatch")
	assert.NotZero(t, event.Timestamp, "Event timestamp should not be zero")
}

// AssertNamespaceInMetadata verifies a namespace appears in metadata.
func AssertNamespaceInMetadata(t *testing.T, metadata *MetadataResponse, namespace string) {
	require.NotNil(t, metadata)
	assert.Contains(t, metadata.Namespaces, namespace, "Namespace %s not found in metadata", namespace)
}

// AssertKindInMetadata verifies a resource kind appears in metadata.
func AssertKindInMetadata(t *testing.T, metadata *MetadataResponse, kind string) {
	require.NotNil(t, metadata)
	assert.Contains(t, metadata.Kinds, kind, "Kind %s not found in metadata", kind)
}
