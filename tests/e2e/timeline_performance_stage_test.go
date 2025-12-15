package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TimelinePerformanceStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	k8sClient *helpers.K8sClient
	apiClient *helpers.APIClient

	// Test data
	baseTime      time.Time
	hoursToSpan   int
	events        []*models.Event
	testNamespace string

	// Query performance tracking
	queryDurationMs    int
	baselineDurationMs int
}

// Performance thresholds
const (
	// Maximum acceptable query time for timeline API (in milliseconds)
	maxAcceptableQueryTime = 5000 // 5 seconds

	// Maximum degradation factor - query time should not increase by more than this factor
	// compared to baseline (10 files). This allows for some degradation but prevents
	// linear/exponential growth.
	maxDegradationFactor = 3.0

	// Number of resources to create per hour to ensure meaningful data
	resourcesPerHour = 10
)

func NewTimelinePerformanceStage(t *testing.T) (*TimelinePerformanceStage, *TimelinePerformanceStage, *TimelinePerformanceStage) {
	s := &TimelinePerformanceStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *TimelinePerformanceStage) and() *TimelinePerformanceStage {
	return s
}

// Given methods

func (s *TimelinePerformanceStage) a_test_environment() *TimelinePerformanceStage {
	s.testCtx = helpers.SetupE2ETest(s.t)
	s.k8sClient = s.testCtx.K8sClient
	s.apiClient = s.testCtx.APIClient
	return s
}

func (s *TimelinePerformanceStage) events_spanning_hours(hours int) *TimelinePerformanceStage {
	s.hoursToSpan = hours
	s.testNamespace = fmt.Sprintf("e2e-timeline-perf-%dh", hours)

	// Use a base time in the past to ensure all events are in closed hour files
	// This is important because the regression affects queries across many files
	s.baseTime = time.Now().Add(-time.Duration(hours+1) * time.Hour).Truncate(time.Hour)

	s.t.Logf("Generating events spanning %d hours (from %s to %s)",
		hours, s.baseTime.Format(time.RFC3339), s.baseTime.Add(time.Duration(hours)*time.Hour).Format(time.RFC3339))

	s.events = s.generateEventsSpanningHours(s.baseTime, hours, s.testNamespace)
	s.t.Logf("Generated %d events across %d hours (%d events per hour)",
		len(s.events), hours, len(s.events)/hours)

	return s
}

// When methods

func (s *TimelinePerformanceStage) events_are_imported() *TimelinePerformanceStage {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	importPayload := map[string]interface{}{
		"events": s.events,
	}

	payloadJSON, err := json.Marshal(importPayload)
	s.require.NoError(err, "failed to marshal import payload")

	s.t.Logf("Importing %d events (%.2f MB)", len(s.events), float64(len(payloadJSON))/(1024*1024))

	importURL := fmt.Sprintf("%s/v1/storage/import?validate=true&overwrite=true", s.apiClient.BaseURL)
	importReq, err := http.NewRequestWithContext(ctx, "POST", importURL, bytes.NewReader(payloadJSON))
	s.require.NoError(err, "failed to create import request")
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+json")

	importResp, err := http.DefaultClient.Do(importReq)
	s.require.NoError(err, "failed to execute import request")
	defer importResp.Body.Close()

	s.require.Equal(http.StatusOK, importResp.StatusCode, "import request failed with status %d", importResp.StatusCode)

	var importReport map[string]interface{}
	err = json.NewDecoder(importResp.Body).Decode(&importReport)
	s.require.NoError(err, "failed to decode import response")

	if totalEvents, ok := importReport["total_events"].(float64); ok {
		s.assert.Equal(float64(len(s.events)), totalEvents, "All events should be imported")
		s.t.Logf("✓ Imported %.0f events", totalEvents)
	}

	if filesCreated, ok := importReport["files_created"].(float64); ok {
		s.t.Logf("✓ Created %.0f storage files", filesCreated)
		// We expect roughly one file per hour (may vary slightly due to hour boundaries)
		s.assert.InDelta(float64(s.hoursToSpan), filesCreated, float64(s.hoursToSpan)*0.2,
			"Number of files created should be close to hours spanned")
	}

	// Wait for namespace to appear in metadata
	s.t.Logf("Waiting for namespace %s to appear in metadata...", s.testNamespace)
	helpers.EventuallyCondition(s.t, func() bool {
		metadataCtx, metadataCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer metadataCancel()

		// Query metadata for the time range of imported events
		startTime := s.baseTime.Unix()
		endTime := s.baseTime.Add(time.Duration(s.hoursToSpan) * time.Hour).Unix()

		metadata, err := s.apiClient.GetMetadata(metadataCtx, &startTime, &endTime)
		if err != nil {
			s.t.Logf("GetMetadata failed: %v", err)
			return false
		}

		for _, ns := range metadata.Namespaces {
			if ns != s.testNamespace {
				continue
			}
			s.t.Logf("✓ Namespace %s found in metadata", s.testNamespace)
			// Also verify we have Deployment kind
			hasDeploymentKind := false
			for _, kind := range metadata.Kinds {
				if kind == "Deployment" {
					hasDeploymentKind = true
					break
				}
			}
			if !hasDeploymentKind {
				s.t.Logf("Deployment kind not yet in metadata, found kinds: %v", metadata.Kinds)
				return false
			}
			return true
		}

		s.t.Logf("Namespace %s not yet in metadata, found: %v", s.testNamespace, metadata.Namespaces)
		return false
	}, helpers.SlowEventuallyOption)

	// Verify we can query the data via search API first
	s.t.Logf("Verifying data is searchable...")
	helpers.EventuallyCondition(s.t, func() bool {
		searchCtx, searchCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer searchCancel()

		startTime := s.baseTime.Unix()
		endTime := s.baseTime.Add(time.Duration(s.hoursToSpan) * time.Hour).Unix()

		resp, err := s.apiClient.Search(searchCtx, startTime, endTime, s.testNamespace, "Deployment")
		if err != nil {
			s.t.Logf("Search failed: %v", err)
			return false
		}

		if resp.Count > 0 {
			s.t.Logf("✓ Found %d Deployment resources via search API", resp.Count)
			return true
		}

		s.t.Logf("Search returned 0 results, waiting for indexing...")
		return false
	}, helpers.SlowEventuallyOption)

	return s
}

func (s *TimelinePerformanceStage) timeline_is_queried_for_last_hour() *TimelinePerformanceStage {
	// Query the last complete hour of data (not the current open hour)
	queryEndTime := s.baseTime.Add(time.Duration(s.hoursToSpan) * time.Hour)
	queryStartTime := queryEndTime.Add(-1 * time.Hour)

	s.t.Logf("Querying timeline for 1-hour window: %s to %s",
		queryStartTime.Format(time.RFC3339), queryEndTime.Format(time.RFC3339))

	startTs := queryStartTime.Unix()
	endTs := queryEndTime.Unix()

	// Wait for data to be indexed and available via timeline API
	var lastResp *helpers.SearchResponse
	helpers.EventuallyCondition(s.t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := s.apiClient.Timeline(ctx, startTs, endTs, s.testNamespace, "Deployment")
		if err != nil {
			s.t.Logf("Timeline query failed: %v", err)
			return false
		}

		lastResp = resp
		if resp.Count == 0 {
			s.t.Logf("Timeline query returned 0 resources, waiting for indexing...")
			return false
		}

		s.t.Logf("Timeline query completed in %d ms with %d resources", resp.ExecutionTimeMs, resp.Count)
		return true
	}, helpers.SlowEventuallyOption)

	s.require.NotNil(lastResp, "Timeline query should have succeeded")
	s.queryDurationMs = lastResp.ExecutionTimeMs

	// Verify we got some results
	s.assert.Greater(lastResp.Count, 0, "Should find resources in the queried time window")

	return s
}

// Then methods

func (s *TimelinePerformanceStage) query_performance_is_acceptable() *TimelinePerformanceStage {
	s.assert.LessOrEqual(s.queryDurationMs, maxAcceptableQueryTime,
		"Query duration (%d ms) should be less than maximum acceptable time (%d ms)",
		s.queryDurationMs, maxAcceptableQueryTime)

	if s.queryDurationMs <= maxAcceptableQueryTime {
		s.t.Logf("✓ Query performance is acceptable: %d ms (max: %d ms)",
			s.queryDurationMs, maxAcceptableQueryTime)
	}

	return s
}

func (s *TimelinePerformanceStage) baseline_performance_is_recorded() *TimelinePerformanceStage {
	// For the 10-file test, record the baseline performance
	if s.hoursToSpan == 10 {
		s.baselineDurationMs = s.queryDurationMs
		s.t.Logf("✓ Baseline performance recorded: %d ms with %d storage files",
			s.baselineDurationMs, s.hoursToSpan)
	}
	return s
}

func (s *TimelinePerformanceStage) performance_does_not_degrade_significantly() *TimelinePerformanceStage {
	// For tests with more than 10 files, we need a baseline to compare against
	// Since tests run independently, we'll use a heuristic: the query time should not
	// grow linearly with the number of files.
	//
	// With proper indexing:
	// - 10 files should take ~T ms
	// - 500 files should take ~T to 3*T ms (not 50*T ms)
	//
	// We'll use a simple heuristic: query time should not be proportional to file count

	// Calculate what linear growth would be
	linearExpectedMs := s.queryDurationMs * 10 / s.hoursToSpan
	if linearExpectedMs > 0 {
		actualGrowthFactor := float64(s.queryDurationMs) / float64(linearExpectedMs)

		s.t.Logf("Performance analysis for %d files:", s.hoursToSpan)
		s.t.Logf("  Actual query time: %d ms", s.queryDurationMs)
		s.t.Logf("  Linear growth would expect: %d ms (10 files) × %d = %d ms",
			linearExpectedMs, s.hoursToSpan/10, linearExpectedMs*s.hoursToSpan/10)
		s.t.Logf("  Actual growth factor vs linear: %.2fx", actualGrowthFactor)

		// The actual time should be much less than linear growth would predict
		// If we're seeing 50x files (500 vs 10), linear growth would mean 50x time
		// But with proper indexing, we should see only 1-3x degradation
		if s.hoursToSpan > 10 {
			fileGrowthFactor := float64(s.hoursToSpan) / 10.0
			maxExpectedMs := int(float64(linearExpectedMs) * maxDegradationFactor)

			s.assert.LessOrEqual(s.queryDurationMs, maxExpectedMs,
				"Query time should not degrade linearly with file count. "+
					"With %dx more files, query time should be at most %dx slower, but got %d ms (expected max %d ms)",
				int(fileGrowthFactor), int(maxDegradationFactor), s.queryDurationMs, maxExpectedMs)

			if s.queryDurationMs <= maxExpectedMs {
				s.t.Logf("✓ Performance scales well: %d ms with %d files (%.2fx degradation, max allowed: %.1fx)",
					s.queryDurationMs, s.hoursToSpan, actualGrowthFactor, maxDegradationFactor)
			}
		}
	}

	return s
}

// Helper methods

func (s *TimelinePerformanceStage) generateEventsSpanningHours(baseTime time.Time, hours int, namespace string) []*models.Event {
	var events []*models.Event

	// Create events for multiple Deployment resources across the time span
	// This simulates a realistic scenario with ongoing activity
	for hour := 0; hour < hours; hour++ {
		hourTime := baseTime.Add(time.Duration(hour) * time.Hour)

		// Create multiple resources per hour to ensure we have meaningful data
		for resourceIdx := 0; resourceIdx < resourcesPerHour; resourceIdx++ {
			resourceName := fmt.Sprintf("perf-test-deploy-%d", resourceIdx)
			resourceUID := uuid.New().String()

			// Create a few events per resource (create + updates) within the hour
			// Spread events across the hour
			for eventIdx := 0; eventIdx < 3; eventIdx++ {
				eventTime := hourTime.Add(time.Duration(eventIdx*15+resourceIdx*2) * time.Minute)

				var eventType models.EventType
				switch eventIdx {
				case 0:
					eventType = models.EventTypeCreate
				case 1, 2:
					eventType = models.EventTypeUpdate
				}

				event := &models.Event{
					ID:        uuid.New().String(),
					Timestamp: eventTime.UnixNano(),
					Type:      eventType,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: namespace,
						Name:      resourceName,
						UID:       resourceUID,
					},
					Data: []byte(fmt.Sprintf(
						`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":%q,"namespace":%q,"uid":%q,"resourceVersion":"%d"},"spec":{"replicas":%d}}`,
						resourceName, namespace, resourceUID, eventIdx+1, eventIdx+1,
					)),
				}
				event.DataSize = int32(len(event.Data))
				events = append(events, event)
			}
		}
	}

	return events
}
