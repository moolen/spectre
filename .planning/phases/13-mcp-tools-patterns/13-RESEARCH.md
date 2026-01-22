# Phase 13: MCP Tools - Patterns - Research

**Researched:** 2026-01-22
**Domain:** Log pattern mining with Drain algorithm and novelty detection for MCP tools
**Confidence:** HIGH

## Summary

Phase 13 implements a pattern mining MCP tool for Logz.io integration that matches VictoriaLogs' existing patterns tool API. The implementation reuses the existing Drain algorithm infrastructure in `internal/logprocessing/` which has already been extracted as common code. The tool follows established MCP tool design patterns, provides namespace-scoped pattern storage, and includes novelty detection via time-window comparison.

The codebase already contains a complete, production-ready implementation of pattern mining for VictoriaLogs (`internal/integration/victorialogs/tools_patterns.go`). This phase requires creating an identical tool for Logz.io that reuses all the same infrastructure: Drain algorithm wrapper, TemplateStore, masking pipeline, and novelty detection logic.

**Primary recommendation:** Clone VictoriaLogs' PatternsTool structure for Logz.io, adapting only the log fetching mechanism to use Logz.io's Elasticsearch API while preserving identical parameters, response format, and behavior.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/faceair/drain | v0.0.0-20220227014011-bcc52881b814 | Drain algorithm for log template mining | Already integrated, production-proven in VictoriaLogs tool |
| internal/logprocessing | N/A (in-tree) | Wrapper around Drain with masking and template management | Already extracted as common code, namespace-scoped storage |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/texttheater/golang-levenshtein | v0.0.0-20200805054039-cae8b0eaed6c | String similarity for template comparison | Already used in logprocessing package |
| encoding/json | stdlib | JSON marshaling for MCP tool interface | All MCP tools use this for parameters and responses |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| github.com/faceair/drain | github.com/jaeyo/go-drain3 | go-drain3 is a more recent port of Drain3 with persistence support, but switching would break VictoriaLogs parity and require re-extraction |

**Installation:**
```bash
# Already in go.mod - no new dependencies needed
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── logprocessing/                    # Already exists - common pattern mining code
│   ├── drain.go                     # Drain algorithm wrapper
│   ├── store.go                     # TemplateStore with namespace-scoping
│   ├── template.go                  # Template struct and ID generation
│   ├── masking.go                   # Variable masking (IP, UUID, timestamps, etc.)
│   ├── normalize.go                 # Log normalization (lowercase, trim)
│   └── kubernetes.go                # K8s name masking
├── integration/
│   └── logzio/
│       ├── tools_patterns.go        # NEW: Patterns tool (clone of VictoriaLogs version)
│       ├── tools_logs.go            # Already exists
│       ├── tools_overview.go        # Already exists
│       ├── client.go                # Already exists - has QueryLogs method
│       └── logzio.go                # Integration lifecycle - need to add templateStore field
```

### Pattern 1: VictoriaLogs Patterns Tool Structure (REFERENCE IMPLEMENTATION)
**What:** Complete patterns tool with novelty detection and metadata collection
**When to use:** This is the blueprint for Logz.io patterns tool
**Example:**
```go
// From internal/integration/victorialogs/tools_patterns.go
type PatternsTool struct {
	ctx           ToolContext
	templateStore *logprocessing.TemplateStore
}

type PatternsParams struct {
	TimeRangeParams
	Namespace string `json:"namespace"`          // Required
	Severity  string `json:"severity,omitempty"` // Optional: error, warn
	Limit     int    `json:"limit,omitempty"`    // Default 50, max 50
}

type PatternsResponse struct {
	TimeRange  string            `json:"time_range"`
	Namespace  string            `json:"namespace"`
	Templates  []PatternTemplate `json:"templates"`
	TotalLogs  int               `json:"total_logs"`
	NovelCount int               `json:"novel_count"`
}

type PatternTemplate struct {
	Pattern    string   `json:"pattern"`              // Masked with <VAR>
	Count      int      `json:"count"`                // Occurrences
	IsNovel    bool     `json:"is_novel"`             // True if not in previous window
	SampleLog  string   `json:"sample_log"`           // One raw log
	Pods       []string `json:"pods,omitempty"`       // Unique pods
	Containers []string `json:"containers,omitempty"` // Unique containers
}

func (t *PatternsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	// 1. Parse parameters
	// 2. Fetch current time window logs with sampling (targetSamples * 20, max 5000)
	// 3. Mine templates and collect metadata (sample, pods, containers)
	// 4. Fetch previous time window logs (same duration before current)
	// 5. Mine templates from previous window (no metadata needed)
	// 6. Compare windows to detect novel patterns
	// 7. Build response with novelty flags, limit to params.Limit
	// 8. Return response
}
```
**Source:** `/home/moritz/dev/spectre-via-ssh/internal/integration/victorialogs/tools_patterns.go`

### Pattern 2: TemplateStore Usage (Already Implemented)
**What:** Namespace-scoped template storage with thread-safe operations
**When to use:** All pattern mining tools use this for consistency
**Example:**
```go
// From internal/logprocessing/store.go
type TemplateStore struct {
	namespaces map[string]*NamespaceTemplates
	config     DrainConfig
	mu         sync.RWMutex
}

// Process a log through the full pipeline:
// 1. PreProcess (normalize)
// 2. Drain.Train (cluster)
// 3. AggressiveMask (mask variables)
// 4. GenerateTemplateID (stable hash)
// 5. Store/update with count
templateID, err := store.Process(namespace, logMessage)

// List templates sorted by count (most common first)
templates, err := store.ListTemplates(namespace)

// Novelty detection - compare two time windows
novelty := store.CompareTimeWindows(namespace, currentTemplates, previousTemplates)
// Returns map[templateID]bool - true if pattern is novel
```
**Source:** `/home/moritz/dev/spectre-via-ssh/internal/logprocessing/store.go`

### Pattern 3: MCP Tool Registration (Integration Pattern)
**What:** Dynamic tool registration during integration startup
**When to use:** All integrations register tools in RegisterTools method
**Example:**
```go
// From internal/integration/victorialogs/victorialogs.go
func (v *VictoriaLogsIntegration) RegisterTools(registry integration.ToolRegistry) error {
	// Create tool context
	toolCtx := ToolContext{
		Client:   v.client,
		Logger:   v.logger,
		Instance: v.name,
	}

	// Register patterns tool: victorialogs_{name}_patterns
	patternsTool := &PatternsTool{
		ctx:           toolCtx,
		templateStore: v.templateStore,
	}
	patternsName := fmt.Sprintf("victorialogs_%s_patterns", v.name)
	patternsSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to query (required)",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by severity level (error, warn)",
				"enum":        []string{"error", "warn"},
			},
			// ... other parameters
		},
		"required": []string{"namespace"},
	}
	err := registry.RegisterTool(patternsName, "Get aggregated log patterns with novelty detection", patternsTool.Execute, patternsSchema)
}
```
**Source:** `/home/moritz/dev/spectre-via-ssh/internal/integration/victorialogs/victorialogs.go`

### Pattern 4: Time Range Parsing with Defaults
**What:** Consistent time range handling across tools
**When to use:** All log tools use this pattern
**Example:**
```go
// From internal/integration/victorialogs/tools_patterns.go
type TimeRangeParams struct {
	StartTime int `json:"start_time,omitempty"` // Unix seconds or millis
	EndTime   int `json:"end_time,omitempty"`   // Unix seconds or millis
}

func parseTimeRange(params TimeRangeParams) TimeRange {
	now := time.Now()
	start := parseTimestamp(params.StartTime, now.Add(-1*time.Hour))
	end := parseTimestamp(params.EndTime, now)
	return TimeRange{Start: start, End: end}
}

func parseTimestamp(ts int, defaultTime time.Time) time.Time {
	if ts == 0 {
		return defaultTime
	}
	// Handle both seconds and milliseconds
	if ts > 1e12 {
		return time.Unix(0, int64(ts)*int64(time.Millisecond))
	}
	return time.Unix(int64(ts), 0)
}
```

### Pattern 5: Log Sampling for Pattern Mining
**What:** Fetch sufficient logs for pattern diversity without overwhelming memory
**When to use:** Pattern mining tools need representative samples
**Example:**
```go
// From internal/integration/victorialogs/tools_patterns.go
func (t *PatternsTool) fetchLogsWithSampling(ctx context.Context, namespace, severity string, timeRange TimeRange, targetSamples int) ([]LogEntry, error) {
	// For pattern mining, fetch targetSamples * 20 (e.g., 50 * 20 = 1000 logs)
	// This gives enough logs for meaningful pattern extraction
	maxLogs := targetSamples * 20
	if maxLogs < 500 {
		maxLogs = 500 // Minimum 500 logs
	}
	if maxLogs > 5000 {
		maxLogs = 5000 // Cap at 5000 to avoid memory issues
	}

	// Build query with limit
	query := QueryParams{
		TimeRange: timeRange,
		Namespace: namespace,
		Limit:     maxLogs,
	}

	// Apply severity filter
	switch severity {
	case "error":
		query.RegexMatch = GetErrorPattern()
	case "warn":
		query.RegexMatch = GetWarningPattern()
	}

	return t.ctx.Client.QueryLogs(ctx, query)
}
```

### Pattern 6: Novelty Detection via Time Window Comparison
**What:** Detect new patterns by comparing current to previous time window
**When to use:** All patterns tools implement this for anomaly detection
**Example:**
```go
// From internal/integration/victorialogs/tools_patterns.go
// Current window
currentLogs, _ := fetchLogsWithSampling(ctx, namespace, severity, timeRange, limit)
currentTemplates, metadata := mineTemplatesWithMetadata(namespace, currentLogs)

// Previous window = same duration immediately before current
duration := timeRange.End.Sub(timeRange.Start)
previousTimeRange := TimeRange{
	Start: timeRange.Start.Add(-duration),
	End:   timeRange.Start,
}

// Previous window (no metadata needed)
previousLogs, _ := fetchLogsWithSampling(ctx, namespace, severity, previousTimeRange, limit)
previousTemplates := mineTemplates(namespace, previousLogs)

// Detect novel patterns
novelty := t.templateStore.CompareTimeWindows(namespace, currentTemplates, previousTemplates)
// novelty[templateID] = true if pattern exists in current but not previous

// Mark templates
for _, tmpl := range currentTemplates {
	pt := PatternTemplate{
		Pattern: tmpl.Pattern,
		Count:   tmpl.Count,
		IsNovel: novelty[tmpl.ID], // Flag from comparison
	}
	templates = append(templates, pt)
}
```

### Anti-Patterns to Avoid
- **Sharing TemplateStore across backends:** Each integration needs its own instance - VictoriaLogs and Logz.io patterns must not interfere with each other
- **Processing all logs without limits:** Pattern mining must cap at 5000 logs to prevent memory exhaustion
- **Merging current and previous logs before mining:** Must mine separately then compare - otherwise can't detect novelty
- **Forgetting .keyword suffix for Elasticsearch:** Logz.io aggregations need `.keyword` suffix for exact matching (e.g., `kubernetes.namespace.keyword`)

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Log template mining | Custom regex or heuristic clustering | `internal/logprocessing.TemplateStore` | Drain algorithm is research-proven, handles variable patterns, already extracted and production-tested |
| Variable masking | Simple regex replace | `logprocessing.AggressiveMask` | Masks 10+ variable types (IP, UUID, hex, paths, emails, timestamps) in correct order, preserves HTTP status codes |
| Template ID generation | Sequential integers or random UUIDs | `logprocessing.GenerateTemplateID` | SHA-256 hash of namespace+pattern gives stable IDs across restarts and clients |
| Namespace-scoped storage | Global map with namespace prefix keys | `TemplateStore` with `NamespaceTemplates` | Thread-safe with proper locking, lazy namespace creation, isolated Drain instances per namespace |
| Time window comparison | Manual set operations | `TemplateStore.CompareTimeWindows` | Compares by pattern (not ID) for cross-window matching, handles edge cases |
| Log normalization | ad-hoc preprocessing | `logprocessing.PreProcess` | Consistent lowercase/trim, JSON message extraction |

**Key insight:** Pattern mining has subtle edge cases (wildcard normalization, namespace isolation, thread safety) that are already solved. The VictoriaLogs implementation took multiple iterations to get right - don't repeat that learning curve.

## Common Pitfalls

### Pitfall 1: Forgetting to Initialize TemplateStore in Integration
**What goes wrong:** PatternsTool receives nil templateStore, panics on Process() call
**Why it happens:** Integration struct needs templateStore field, must be initialized in Start() method
**How to avoid:**
```go
// In logzio.go
type LogzioIntegration struct {
	// ...existing fields...
	templateStore *logprocessing.TemplateStore  // ADD THIS
}

// In Start() method
func (l *LogzioIntegration) Start(ctx context.Context) error {
	// ...existing initialization...

	// Initialize template store for pattern mining
	l.templateStore = logprocessing.NewTemplateStore(logprocessing.DefaultDrainConfig())

	return nil
}

// In RegisterTools() method - pass to patterns tool
patternsTool := &PatternsTool{
	ctx:           toolCtx,
	templateStore: l.templateStore,  // Pass the store
}
```
**Warning signs:** Test failures with nil pointer dereference in PatternsTool.Execute

### Pitfall 2: Not Using .keyword Suffix for Logz.io Elasticsearch Filters
**What goes wrong:** Logz.io queries fail to filter correctly, return no results or wrong results
**Why it happens:** Elasticsearch text fields need `.keyword` suffix for exact matching
**How to avoid:** Use `.keyword` suffix for all term queries in BuildLogsQuery
```go
// WRONG - will use analyzed text field
"term": map[string]interface{}{
	"kubernetes.namespace": params.Namespace,  // NO!
}

// CORRECT - exact match on keyword field
"term": map[string]interface{}{
	"kubernetes.namespace.keyword": params.Namespace,  // YES!
}
```
**Warning signs:** Patterns tool returns empty results even when logs exist, severity filters don't work

### Pitfall 3: Severity Pattern Mismatch Between Overview and Patterns Tools
**What goes wrong:** overview tool shows errors but patterns tool finds none for same namespace
**Why it happens:** Different regex patterns for error detection
**How to avoid:** Reuse exact same severity patterns from overview tool
```go
// In tools_patterns.go - use existing patterns from severity.go
switch severity {
case "error", "errors":
	query.RegexMatch = GetErrorPattern()   // Reuse from severity.go
case "warn", "warning", "warnings":
	query.RegexMatch = GetWarningPattern() // Reuse from severity.go
}
```
**Warning signs:** Inconsistent error counts between overview and patterns tool

### Pitfall 4: Breaking VictoriaLogs Parity with Different Parameters or Response Format
**What goes wrong:** AI using VictoriaLogs patterns learns parameters/format, then Logz.io patterns tool fails or confuses AI
**Why it happens:** Changing parameter names, adding/removing fields, different defaults
**How to avoid:** Exact copy of VictoriaLogs types
```go
// MUST match VictoriaLogs exactly:
type PatternsParams struct {
	TimeRangeParams
	Namespace string `json:"namespace"`          // Same field name
	Severity  string `json:"severity,omitempty"` // Same field name
	Limit     int    `json:"limit,omitempty"`    // Same field name, same default (50)
}

type PatternsResponse struct {
	TimeRange  string            `json:"time_range"`  // Same field name
	Namespace  string            `json:"namespace"`   // Same field name
	Templates  []PatternTemplate `json:"templates"`   // Same field name
	TotalLogs  int               `json:"total_logs"`  // Same field name
	NovelCount int               `json:"novel_count"` // Same field name
}

// Schema must match too
patternsSchema := map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"namespace": map[string]interface{}{
			"type":        "string",
			"description": "Kubernetes namespace to query (required)",  // Same description
		},
		"severity": map[string]interface{}{
			"type":        "string",
			"description": "Optional: filter by severity level (error, warn)",
			"enum":        []string{"error", "warn"},  // Same enum values
		},
		// ...
	},
	"required": []string{"namespace"},  // Same required fields
}
```
**Warning signs:** User feedback that tool behaves differently from VictoriaLogs, AI needs to learn separate patterns

### Pitfall 5: Fetching Insufficient Logs for Pattern Mining
**What goes wrong:** Only finds a few generic patterns, misses diverse patterns
**Why it happens:** Using logs tool's default limit (100) instead of pattern mining sampling
**How to avoid:** Use targetSamples * 20 multiplier (500-5000 logs)
```go
// WRONG - too few logs
maxLogs := params.Limit  // Only 50 logs for 50 templates

// CORRECT - sufficient sampling
maxLogs := params.Limit * 20  // 1000 logs for 50 templates
if maxLogs < 500 {
	maxLogs = 500   // Minimum for diversity
}
if maxLogs > 5000 {
	maxLogs = 5000  // Maximum for memory safety
}
```
**Warning signs:** Patterns tool returns very few templates (<5) for busy namespaces, all patterns are very generic

### Pitfall 6: Not Handling Previous Window Fetch Failures Gracefully
**What goes wrong:** Tool fails completely if previous window query fails (API timeout, rate limit)
**Why it happens:** Treating previous window fetch as hard requirement
**How to avoid:** Log warning but continue - all patterns marked as novel
```go
previousLogs, err := fetchLogsWithSampling(ctx, namespace, severity, previousTimeRange, limit)
if err != nil {
	// Don't fail - log warning and continue
	t.ctx.Logger.Warn("Failed to fetch previous window for novelty detection: %v", err)
	previousLogs = []LogEntry{} // Empty previous = all current templates novel
}
```
**Warning signs:** Tool fails with "failed to fetch previous logs" when API is slow/rate-limited

## Code Examples

Verified patterns from existing implementations:

### Tool Structure (Clone for Logz.io)
```go
// Source: internal/integration/victorialogs/tools_patterns.go
package logzio

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

	// Fetch current window logs
	currentLogs, err := t.fetchLogsWithSampling(ctx, params.Namespace, params.Severity, timeRange, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current logs: %w", err)
	}

	// Mine templates from current logs with metadata
	currentTemplates, metadata := t.mineTemplatesWithMetadata(params.Namespace, currentLogs)

	// Fetch previous window for novelty detection
	duration := timeRange.End.Sub(timeRange.Start)
	previousTimeRange := TimeRange{
		Start: timeRange.Start.Add(-duration),
		End:   timeRange.Start,
	}

	previousLogs, err := t.fetchLogsWithSampling(ctx, params.Namespace, params.Severity, previousTimeRange, params.Limit)
	if err != nil {
		// Warn but continue - novelty detection fails gracefully
		t.ctx.Logger.Warn("Failed to fetch previous window for novelty detection: %v", err)
		previousLogs = []LogEntry{}
	}

	// Mine templates from previous logs
	previousTemplates := t.mineTemplates(params.Namespace, previousLogs)

	// Detect novel templates
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

		// Add metadata if available
		if meta, exists := metadata[tmpl.ID]; exists && meta != nil {
			pt.SampleLog = meta.sampleLog

			if len(meta.pods) > 0 {
				pt.Pods = setToSlice(meta.pods)
			}
			if len(meta.containers) > 0 {
				pt.Containers = setToSlice(meta.containers)
			}
		}

		templates = append(templates, pt)
	}

	// Limit response size
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
```

### Logz.io-Specific: Fetch Logs with Sampling
```go
// Logz.io version - uses Client.QueryLogs with Elasticsearch API
func (t *PatternsTool) fetchLogsWithSampling(ctx context.Context, namespace, severity string, timeRange TimeRange, targetSamples int) ([]LogEntry, error) {
	// Calculate sampling limit
	maxLogs := targetSamples * 20
	if maxLogs < 500 {
		maxLogs = 500
	}
	if maxLogs > 5000 {
		maxLogs = 5000
	}

	t.ctx.Logger.Debug("Fetching up to %d logs for pattern mining from namespace %s (severity=%s)", maxLogs, namespace, severity)

	// Build query params
	query := QueryParams{
		TimeRange: timeRange,
		Namespace: namespace,
		Limit:     maxLogs,
	}

	// Apply severity filter using regex patterns
	switch severity {
	case "error", "errors":
		query.RegexMatch = GetErrorPattern()
	case "warn", "warning", "warnings":
		query.RegexMatch = GetWarningPattern()
	case "":
		// No filter
	default:
		return nil, fmt.Errorf("invalid severity filter: %s (valid: error, warn)", severity)
	}

	// Fetch logs via Logz.io client
	result, err := t.ctx.Client.QueryLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	t.ctx.Logger.Debug("Fetched %d logs for pattern mining from namespace %s", len(result.Logs), namespace)
	return result.Logs, nil
}
```

### Template Mining with Metadata Collection
```go
// Source: internal/integration/victorialogs/tools_patterns.go
type templateMetadata struct {
	sampleLog  string
	pods       map[string]struct{}
	containers map[string]struct{}
}

func (t *PatternsTool) mineTemplatesWithMetadata(namespace string, logs []LogEntry) ([]logprocessing.Template, map[string]*templateMetadata) {
	metadata := make(map[string]*templateMetadata)

	// Process each log through template store
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

func (t *PatternsTool) mineTemplates(namespace string, logs []LogEntry) []logprocessing.Template {
	// Process each log (no metadata needed for previous window)
	for _, log := range logs {
		message := extractMessage(log)
		_, _ = t.templateStore.Process(namespace, message)
	}

	templates, err := t.templateStore.ListTemplates(namespace)
	if err != nil {
		t.ctx.Logger.Warn("Failed to list templates for %s: %v", namespace, err)
		return []logprocessing.Template{}
	}

	return templates
}

func extractMessage(log LogEntry) string {
	// If log has Message field, use it
	if log.Message != "" {
		return log.Message
	}

	// Fallback: return JSON representation
	data, _ := json.Marshal(log)
	return string(data)
}
```

### Tool Registration in Integration
```go
// In internal/integration/logzio/logzio.go RegisterTools method
func (l *LogzioIntegration) RegisterTools(registry integration.ToolRegistry) error {
	l.logger.Info("Registering MCP tools for Logz.io integration: %s", l.name)

	// Store registry reference
	l.registry = registry

	// Create tool context
	toolCtx := ToolContext{
		Client:   l.client,
		Logger:   l.logger,
		Instance: l.name,
	}

	// Instantiate tools
	overviewTool := &OverviewTool{ctx: toolCtx}
	logsTool := &LogsTool{ctx: toolCtx}
	patternsTool := &PatternsTool{       // NEW
		ctx:           toolCtx,           // NEW
		templateStore: l.templateStore,   // NEW - pass the store
	}                                     // NEW

	// Register overview tool (existing)
	overviewName := fmt.Sprintf("logzio_%s_overview", l.name)
	// ... existing overview registration ...

	// Register logs tool (existing)
	logsName := fmt.Sprintf("logzio_%s_logs", l.name)
	// ... existing logs registration ...

	// Register patterns tool (NEW)
	patternsName := fmt.Sprintf("logzio_%s_patterns", l.name)
	patternsDesc := fmt.Sprintf("Get aggregated log patterns with novelty detection for Logz.io %s. Returns log templates with occurrence counts. Use after overview to understand error patterns.", l.name)
	patternsSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to query (required)",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by severity level (error, warn). Only logs matching the severity pattern will be processed.",
				"enum":        []string{"error", "warn"},
			},
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "End timestamp (Unix seconds or milliseconds). Default: now",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max templates to return (default 50)",
			},
		},
		"required": []string{"namespace"},
	}

	if err := registry.RegisterTool(patternsName, patternsDesc, patternsTool.Execute, patternsSchema); err != nil {
		return fmt.Errorf("failed to register patterns tool: %w", err)
	}
	l.logger.Info("Registered tool: %s", patternsName)

	return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Python-based Drain3 | Go port (github.com/faceair/drain) | 2022 | Enables in-process pattern mining, no subprocess overhead |
| Pattern storage per backend | Shared `internal/logprocessing/` package | Phase 12 (recently) | Logz.io can reuse all VictoriaLogs infrastructure |
| Manual regex for log parsing | Drain algorithm with learned clusters | Research paper 2017, adopted 2022 | Handles variable logs without manual patterns |
| Global pattern storage | Namespace-scoped TemplateStore | Phase 11 CONTEXT | Prevents pattern pollution across tenants |
| Match only (classification) | Train + Match with counts | Phase 11 implementation | Enables pattern ranking by frequency |

**Deprecated/outdated:**
- Manual regex patterns for log template extraction - replaced by Drain algorithm
- Cross-namespace pattern sharing - replaced by namespace-scoped storage

## Open Questions

Things that couldn't be fully resolved:

1. **Logz.io API rate limits during pattern mining**
   - What we know: Fetching 1000-5000 logs for pattern mining could hit rate limits
   - What's unclear: Logz.io's exact rate limit thresholds, whether /v1/search counts differently than aggregations
   - Recommendation: Monitor for 429 errors, implement exponential backoff if needed

2. **Elasticsearch regex performance for severity filtering**
   - What we know: Overview tool uses regex for error/warn detection, patterns tool reuses same patterns
   - What's unclear: Whether regex filtering on 5000 logs is fast enough in Logz.io Elasticsearch
   - Recommendation: Test with production namespaces, consider caching severity patterns if slow

3. **Optimal sampling multiplier for diverse pattern capture**
   - What we know: VictoriaLogs uses targetSamples * 20 (e.g., 50 * 20 = 1000 logs)
   - What's unclear: Whether Logz.io log patterns have different diversity characteristics
   - Recommendation: Start with same multiplier, validate coverage with real namespaces

## Sources

### Primary (HIGH confidence)
- `/home/moritz/dev/spectre-via-ssh/internal/integration/victorialogs/tools_patterns.go` - Reference implementation of patterns tool
- `/home/moritz/dev/spectre-via-ssh/internal/logprocessing/store.go` - TemplateStore with namespace-scoping and novelty detection
- `/home/moritz/dev/spectre-via-ssh/internal/logprocessing/drain.go` - Drain algorithm wrapper
- `/home/moritz/dev/spectre-via-ssh/internal/logprocessing/template.go` - Template struct and ID generation
- `/home/moritz/dev/spectre-via-ssh/internal/logprocessing/masking.go` - Variable masking patterns
- `/home/moritz/dev/spectre-via-ssh/internal/integration/logzio/client.go` - Logz.io QueryLogs API
- `/home/moritz/dev/spectre-via-ssh/internal/integration/logzio/query.go` - Elasticsearch DSL query builder
- `/home/moritz/dev/spectre-via-ssh/internal/integration/victorialogs/victorialogs.go` - Integration RegisterTools pattern
- `/home/moritz/dev/spectre-via-sh/internal/mcp/server.go` - MCP tool registry implementation
- [Go Packages: github.com/faceair/drain](https://pkg.go.dev/github.com/faceair/drain) - Official Drain library documentation

### Secondary (MEDIUM confidence)
- [GitHub: faceair/drain](https://github.com/faceair/drain) - Drain implementation source code with examples
- [Model Context Protocol Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25) - Tool design patterns and best practices
- [Drain3: The Unsung Hero of Templatizing Logs](https://medium.com/@srikrishnan.tech/drain3-the-unsung-hero-of-templatizing-logs-for-machine-learning-8b83ba1ef480) - Drain algorithm best practices
- [How Drain3 Works: Parsing Unstructured Logs](https://medium.com/@lets.see.1016/how-drain3-works-parsing-unstructured-logs-into-structured-format-3458ce05b69a) - Drain algorithm internals

### Tertiary (LOW confidence)
- [Log Anomaly Detection via Evidential Deep Learning](https://www.mdpi.com/2076-3417/14/16/7055) - Time window comparison approaches for novelty detection
- [Temporal Logical Attention Network for Log-Based Anomaly Detection](https://pmc.ncbi.nlm.nih.gov/articles/PMC11679089/) - Multi-scale temporal patterns in logs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All dependencies already in codebase, production-proven in VictoriaLogs
- Architecture: HIGH - Reference implementation exists, exact structure to clone
- Pitfalls: HIGH - Based on actual VictoriaLogs implementation experience

**Research date:** 2026-01-22
**Valid until:** 30 days (stable domain - Drain algorithm and MCP patterns unlikely to change)
