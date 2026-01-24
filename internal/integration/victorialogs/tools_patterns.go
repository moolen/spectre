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
	Namespace string `json:"namespace"`          // Required: namespace to query
	Severity  string `json:"severity,omitempty"` // Optional: filter by severity (error, warn)
	Limit     int    `json:"limit,omitempty"`    // Optional: max templates to return (default 50)
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
	Pattern    string   `json:"pattern"`              // Masked pattern with <VAR> placeholders
	Count      int      `json:"count"`                // Occurrences in current time window
	IsNovel    bool     `json:"is_novel"`             // True if not in previous time window
	SampleLog  string   `json:"sample_log"`           // One raw log matching this template
	Pods       []string `json:"pods,omitempty"`       // Unique pod names that produced this pattern
	Containers []string `json:"containers,omitempty"` // Unique container names that produced this pattern
}

// templateMetadata tracks sample logs and labels for each template ID
type templateMetadata struct {
	sampleLog  string
	pods       map[string]struct{}
	containers map[string]struct{}
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
	currentLogs, err := t.fetchLogsWithSampling(ctx, params.Namespace, params.Severity, timeRange, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current logs: %w", err)
	}

	// Mine templates from current logs and collect metadata (sample, pods, containers)
	currentTemplates, metadata := t.mineTemplatesWithMetadata(params.Namespace, currentLogs)

	// NOVL-01: Compare to previous time window for novelty detection
	// Previous window = same duration immediately before current window
	duration := timeRange.End.Sub(timeRange.Start)
	previousTimeRange := TimeRange{
		Start: timeRange.Start.Add(-duration),
		End:   timeRange.Start,
	}

	// Fetch logs for previous time window (same sampling)
	previousLogs, err := t.fetchLogsWithSampling(ctx, params.Namespace, params.Severity, previousTimeRange, params.Limit)
	if err != nil {
		// Log warning but continue (novelty detection fails gracefully)
		t.ctx.Logger.Warn("Failed to fetch previous window for novelty detection: %v", err)
		previousLogs = []LogEntry{} // Empty previous = all current templates novel
	}

	// Mine templates from previous logs (no metadata needed)
	previousTemplates := t.mineTemplates(params.Namespace, previousLogs)

	// NOVL-02: Detect novel templates
	novelty := t.templateStore.CompareTimeWindows(params.Namespace, currentTemplates, previousTemplates)

	// Build response with novelty flags and metadata
	templates := make([]PatternTemplate, 0, len(currentTemplates))
	novelCount := 0

	for _, tmpl := range currentTemplates {
		isNovel := novelty[tmpl.ID]
		if isNovel {
			novelCount++
		}

		pt := PatternTemplate{
			Pattern: tmpl.Pattern,
			Count:   tmpl.Count,
			IsNovel: isNovel,
		}

		// Add metadata if available (may be nil if template was from previous processing)
		if meta, exists := metadata[tmpl.ID]; exists && meta != nil {
			pt.SampleLog = meta.sampleLog

			// Convert sets to slices
			if len(meta.pods) > 0 {
				pt.Pods = setToSlice(meta.pods)
			}
			if len(meta.containers) > 0 {
				pt.Containers = setToSlice(meta.containers)
			}
		}

		templates = append(templates, pt)
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
func (t *PatternsTool) fetchLogsWithSampling(ctx context.Context, namespace, severity string, timeRange TimeRange, targetSamples int) ([]LogEntry, error) {
	// For pattern mining, we want a good sample size to capture diverse patterns
	// Use targetSamples * 20 as our fetch limit (e.g., 50 * 20 = 1000 logs)
	// This gives us enough logs for meaningful pattern extraction without overwhelming the system
	maxLogs := targetSamples * 20
	if maxLogs < 500 {
		maxLogs = 500 // Minimum 500 logs for pattern mining
	}
	if maxLogs > 5000 {
		maxLogs = 5000 // Cap at 5000 to avoid memory issues
	}

	t.ctx.Logger.Debug("Fetching up to %d logs for pattern mining from namespace %s (severity=%s)", maxLogs, namespace, severity)

	// Fetch logs with limit
	query := QueryParams{
		TimeRange: timeRange,
		Namespace: namespace,
		Limit:     maxLogs,
	}

	// Apply severity filter using regex pattern
	switch severity {
	case "error", "errors":
		query.RegexMatch = GetErrorPattern()
	case "warn", "warning", "warnings":
		query.RegexMatch = GetWarningPattern()
	case "":
		// No filter - fetch all logs
	default:
		return nil, fmt.Errorf("invalid severity filter: %s (valid: error, warn)", severity)
	}

	result, err := t.ctx.Client.QueryLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	t.ctx.Logger.Debug("Fetched %d logs for pattern mining from namespace %s", len(result.Logs), namespace)
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

// mineTemplatesWithMetadata processes logs and collects metadata (sample, pods, containers)
func (t *PatternsTool) mineTemplatesWithMetadata(namespace string, logs []LogEntry) ([]logprocessing.Template, map[string]*templateMetadata) {
	metadata := make(map[string]*templateMetadata)

	// Process each log through template store and collect metadata
	for _, log := range logs {
		message := extractMessage(log)
		templateID, _ := t.templateStore.Process(namespace, message)

		// Initialize metadata for this template if needed
		if _, exists := metadata[templateID]; !exists {
			metadata[templateID] = &templateMetadata{
				sampleLog:  message, // First log becomes the sample
				pods:       make(map[string]struct{}),
				containers: make(map[string]struct{}),
			}
		}

		// Collect labels
		meta := metadata[templateID]
		if log.Pod != "" {
			meta.pods[log.Pod] = struct{}{}
		}
		if log.Container != "" {
			meta.containers[log.Container] = struct{}{}
		}
	}

	// Get templates sorted by count
	templates, err := t.templateStore.ListTemplates(namespace)
	if err != nil {
		t.ctx.Logger.Warn("Failed to list templates for %s: %v", namespace, err)
		return []logprocessing.Template{}, metadata
	}

	return templates, metadata
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

// setToSlice converts a set (map[string]struct{}) to a sorted slice
func setToSlice(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for k := range set {
		result = append(result, k)
	}
	// Sort for consistent output
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
