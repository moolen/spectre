package victorialogs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logprocessing"
)

// PatternsTool provides aggregated log patterns with novelty detection
type PatternsTool struct {
	ctx           ToolContext
	templateStore *logprocessing.TemplateStore
}

// PatternsParams defines input parameters for patterns tool
type PatternsParams struct {
	TimeRangeParams
	Namespace string `json:"namespace"`        // Required: namespace to query
	Limit     int    `json:"limit,omitempty"`  // Optional: max templates to return (default 50)
}

// PatternsResponse returns templates with counts and novelty flags
type PatternsResponse struct {
	TimeRange  string            `json:"time_range"`
	Namespace  string            `json:"namespace"`
	Templates  []PatternTemplate `json:"templates"`    // Sorted by count descending
	TotalLogs  int               `json:"total_logs"`
	NovelCount int               `json:"novel_count"`  // Count of novel templates
}

// PatternTemplate represents a log template with metadata
type PatternTemplate struct {
	TemplateID string `json:"template_id"`
	Pattern    string `json:"pattern"`     // Masked pattern with <VAR> placeholders
	Count      int    `json:"count"`       // Occurrences in current time window
	IsNovel    bool   `json:"is_novel"`    // True if not in previous time window
	SampleLog  string `json:"sample_log"`  // One raw log matching this template
}

// Execute runs the patterns tool
func (t *PatternsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	// Parse parameters
	var params PatternsParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate required namespace
	if params.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	// Default limit
	if params.Limit == 0 {
		params.Limit = 50
	}

	// Parse time range
	timeRange := parseTimeRange(params.TimeRangeParams)

	// MINE-06: Time-window batching for efficiency
	// Fetch logs for current time window with sampling for high-volume
	currentLogs, err := t.fetchLogsWithSampling(ctx, params.Namespace, timeRange, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current logs: %w", err)
	}

	// Mine templates from current logs
	currentTemplates := t.mineTemplates(params.Namespace, currentLogs)

	// NOVL-01: Compare to previous time window for novelty detection
	// Previous window = same duration immediately before current window
	duration := timeRange.End.Sub(timeRange.Start)
	previousTimeRange := TimeRange{
		Start: timeRange.Start.Add(-duration),
		End:   timeRange.Start,
	}

	// Fetch logs for previous time window (same sampling)
	previousLogs, err := t.fetchLogsWithSampling(ctx, params.Namespace, previousTimeRange, params.Limit)
	if err != nil {
		// Log warning but continue (novelty detection fails gracefully)
		t.ctx.Logger.Warn("Failed to fetch previous window for novelty detection: %v", err)
		previousLogs = []LogEntry{} // Empty previous = all current templates novel
	}

	// Mine templates from previous logs
	previousTemplates := t.mineTemplates(params.Namespace, previousLogs)

	// NOVL-02: Detect novel templates
	novelty := t.templateStore.CompareTimeWindows(params.Namespace, currentTemplates, previousTemplates)

	// Build response with novelty flags
	templates := make([]PatternTemplate, 0, len(currentTemplates))
	novelCount := 0
	sampleMap := buildSampleMap(currentLogs)

	for _, tmpl := range currentTemplates {
		isNovel := novelty[tmpl.ID]
		if isNovel {
			novelCount++
		}

		templates = append(templates, PatternTemplate{
			TemplateID: tmpl.ID,
			Pattern:    tmpl.Pattern,
			Count:      tmpl.Count,
			IsNovel:    isNovel,
			SampleLog:  sampleMap[tmpl.Pattern], // One raw log for this pattern
		})
	}

	// Limit response size (already sorted by count from ListTemplates)
	if len(templates) > params.Limit {
		templates = templates[:params.Limit]
	}

	return &PatternsResponse{
		TimeRange:  fmt.Sprintf("%s to %s", timeRange.Start.Format(time.RFC3339), timeRange.End.Format(time.RFC3339)),
		Namespace:  params.Namespace,
		Templates:  templates,
		TotalLogs:  len(currentLogs),
		NovelCount: novelCount,
	}, nil
}

// fetchLogsWithSampling fetches logs with sampling for high-volume namespaces (MINE-05)
func (t *PatternsTool) fetchLogsWithSampling(ctx context.Context, namespace string, timeRange TimeRange, targetSamples int) ([]LogEntry, error) {
	// Query for log count first
	countQuery := QueryParams{
		TimeRange: timeRange,
		Namespace: namespace,
		Limit:     1,
	}
	result, err := t.ctx.Client.QueryLogs(ctx, countQuery)
	if err != nil {
		return nil, err
	}

	totalLogs := result.Count

	// MINE-05: Sample high-volume namespaces
	// If namespace has more than targetSamples * 10 logs, apply sampling
	samplingThreshold := targetSamples * 10
	limit := totalLogs
	if totalLogs > samplingThreshold {
		// Fetch sample size (targetSamples * 2 for better template coverage)
		limit = targetSamples * 2
		t.ctx.Logger.Info("High-volume namespace %s (%d logs), sampling %d", namespace, totalLogs, limit)
	}

	// Fetch logs with limit
	query := QueryParams{
		TimeRange: timeRange,
		Namespace: namespace,
		Limit:     limit,
	}

	result, err = t.ctx.Client.QueryLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Logs, nil
}

// mineTemplates processes logs through TemplateStore and returns sorted templates
func (t *PatternsTool) mineTemplates(namespace string, logs []LogEntry) []logprocessing.Template {
	// Process each log through template store
	for _, log := range logs {
		// Extract message field (JSON or plain text)
		message := extractMessage(log)
		_, _ = t.templateStore.Process(namespace, message)
	}

	// Get templates sorted by count
	templates, err := t.templateStore.ListTemplates(namespace)
	if err != nil {
		t.ctx.Logger.Warn("Failed to list templates for %s: %v", namespace, err)
		return []logprocessing.Template{}
	}

	return templates
}

// extractMessage extracts message from LogEntry (handles JSON and plain text)
func extractMessage(log LogEntry) string {
	// If log has Message field (_msg), use it
	if log.Message != "" {
		return log.Message
	}

	// Fallback: return JSON representation
	data, _ := json.Marshal(log)
	return string(data)
}

// buildSampleMap creates map of pattern -> first matching raw log
func buildSampleMap(logs []LogEntry) map[string]string {
	// Simple approach: store first occurrence of each unique message
	// More sophisticated: store during mining, but requires TemplateStore modification
	// For v1: accept that sample might not be perfect match
	sampleMap := make(map[string]string)
	for _, log := range logs {
		msg := extractMessage(log)
		if len(sampleMap) < 100 { // Limit map size
			// Use raw message as key for now - pattern matching would be more accurate
			if _, exists := sampleMap[msg]; !exists {
				sampleMap[msg] = msg
			}
		}
	}
	return sampleMap
}
