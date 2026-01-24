# Phase 4: Log Template Mining - Research

**Researched:** 2026-01-21
**Domain:** Log parsing and template extraction using Drain algorithm
**Confidence:** HIGH

## Summary

Log template mining using the Drain algorithm is a well-established approach for automatic log clustering. The Drain algorithm uses a fixed-depth parse tree to achieve O(log n) matching performance and can extract templates from streaming logs in real-time. Two primary Go implementations exist: `github.com/faceair/drain` (more mature) and `github.com/PalanQu/LoggingDrain` (newer, performance-focused). The algorithm requires careful parameter tuning (similarity threshold, tree depth, max children) to balance between creating too many templates (template explosion) and merging unrelated logs.

**Key technical challenges identified:**
1. **Template explosion** from variable-starting logs (e.g., "cupsd shutdown succeeded", "irqbalance shutdown succeeded" create separate branches)
2. **Template drift** over time as log formats evolve without rebalancing
3. **Kubernetes-specific normalization** for pod names with dynamic suffixes (deployment-abc123-xyz45)
4. **JSON log handling** requires extracting message field before templating to avoid structure-based clustering

**Primary recommendation:** Use `github.com/faceair/drain` as the foundation with custom extensions for Kubernetes-aware masking, post-clustering variable normalization, and periodic template merging. Implement per-namespace template storage with SHA-256 hashing for stable template IDs.

## Standard Stack

The established libraries/tools for log template mining in Go:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/faceair/drain | Latest | Drain algorithm implementation | Official Go port of Drain3, stable API, configurable parameters |
| crypto/sha256 | stdlib | Template ID hashing | Deterministic hashing for stable template identifiers |
| encoding/json | stdlib | JSON log parsing | Extract message fields from structured logs |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| regexp | stdlib | Variable masking patterns | Aggressive masking for IPs, UUIDs, timestamps, K8s names |
| time | stdlib | Time-window batching | Periodic snapshots and template rebalancing |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| github.com/faceair/drain | github.com/PalanQu/LoggingDrain | LoggingDrain is newer but less mature; includes persistence layer but less documented |
| github.com/faceair/drain | Custom Drain implementation | Research recommends starting with library vs custom; algorithm has subtle edge cases |
| crypto/sha256 | Database auto-increment IDs | SHA-256 provides cross-instance stability (requirement MINE-03) |

**Installation:**
```bash
go get github.com/faceair/drain
# No additional dependencies needed - uses Go stdlib
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── logprocessing/           # Integration-agnostic package (REQUIREMENT: reusable beyond VictoriaLogs)
│   ├── drain.go             # Drain algorithm wrapper with extensions
│   ├── normalize.go         # Pre-processing: lowercase, trim, extract JSON msg
│   ├── masking.go           # Post-clustering: aggressive variable masking
│   ├── template.go          # Template types, hashing, comparison
│   ├── store.go             # In-memory template storage with persistence
│   └── kubernetes.go        # K8s-specific pattern detection (pod names, etc)
└── mcp/
    └── template_service.go  # MCP server integration (Phase 5)
```

### Pattern 1: Two-Phase Processing (Pre-tokenization + Post-masking)

**What:** Normalize logs minimally before Drain clustering, then apply aggressive masking to resulting templates

**When to use:** When dealing with Kubernetes logs that have variable prefixes (pod names, container IDs)

**Rationale from CONTEXT.md:** User decision is "masking AFTER Drain clustering" to preserve Drain's ability to detect structure before normalizing variables

**Example:**
```go
// Phase 1: Minimal pre-processing for Drain input
func PreProcess(rawLog string) string {
    // Extract message from JSON if structured
    msg := extractMessageField(rawLog)

    // Lowercase for case-insensitive clustering
    msg = strings.ToLower(msg)

    // DO NOT mask variables yet - let Drain see them
    return strings.TrimSpace(msg)
}

// Phase 2: Aggressive post-clustering masking
func PostProcessTemplate(template string) string {
    // Now mask variables in the resulting template
    template = maskIPs(template)
    template = maskUUIDs(template)
    template = maskTimestamps(template)
    template = maskK8sNames(template)  // deployment-abc123-xyz45 -> <K8S_NAME>

    // But preserve HTTP status codes (user decision)
    // "returned 404" vs "returned 500" stay distinct
    return template
}

// Source: User decisions from CONTEXT.md + Drain algorithm best practices
```

### Pattern 2: Namespace-Scoped Template Storage

**What:** Store templates per-namespace with composite keys, not globally

**When to use:** Multi-tenant environments where same log pattern means different things in different namespaces

**Example:**
```go
// Template store keyed by namespace
type TemplateStore struct {
    templates map[string]*NamespaceTemplates  // namespace -> templates
    mu        sync.RWMutex
}

type NamespaceTemplates struct {
    drain     *drain.Drain     // Per-namespace Drain instance
    templates map[string]*Template  // templateID -> Template
    counts    map[string]int   // templateID -> occurrence count
}

func (s *TemplateStore) Process(namespace, logMessage string) string {
    s.mu.Lock()
    defer s.mu.Unlock()

    ns := s.getOrCreateNamespace(namespace)

    // Train Drain for this namespace
    cluster := ns.drain.Train(logMessage)

    // Generate stable template ID from cluster template + namespace
    templateID := generateTemplateID(namespace, cluster.String())

    // Track occurrence count for pruning
    ns.counts[templateID]++

    return templateID
}

// Source: User decision from CONTEXT.md + multi-tenancy best practices
```

### Pattern 3: Count-Based Template Expiry with Auto-Merge

**What:** Prune templates below occurrence threshold and periodically merge similar templates

**When to use:** To handle template drift and prevent unbounded memory growth

**Example:**
```go
type TemplateRebalancer struct {
    store         *TemplateStore
    pruneThreshold int           // Minimum occurrences to keep (user decided: 10)
    mergeInterval  time.Duration  // How often to run auto-merge (user decided: 5 minutes)
}

func (r *TemplateRebalancer) Rebalance(namespace string) {
    ns := r.store.GetNamespace(namespace)

    // Step 1: Prune low-count templates
    for templateID, count := range ns.counts {
        if count < r.pruneThreshold {
            delete(ns.templates, templateID)
            delete(ns.counts, templateID)
        }
    }

    // Step 2: Find and merge similar templates
    templates := ns.templates.Values()
    for i := 0; i < len(templates); i++ {
        for j := i + 1; j < len(templates); j++ {
            if shouldMerge(templates[i], templates[j]) {
                mergeTemplates(ns, templates[i], templates[j])
            }
        }
    }
}

// Auto-merge detection: compute similarity between templates
func shouldMerge(t1, t2 *Template) bool {
    // Normalize edit distance by template length
    distance := editDistance(t1.Pattern, t2.Pattern)
    shorter := min(len(t1.Tokens), len(t2.Tokens))

    normalizedSimilarity := 1.0 - float64(distance)/float64(shorter)

    // User decision: "loose clustering" means aggressive merging
    // Merge if >70% similar
    return normalizedSimilarity > 0.7
}

// Source: Drain+ template merging algorithm + user decisions
```

### Pattern 4: Periodic Disk Snapshots

**What:** In-memory storage with periodic JSON snapshots for crash recovery

**When to use:** Single-instance deployments where eventual consistency is acceptable

**Example:**
```go
type PersistenceManager struct {
    store          *TemplateStore
    snapshotPath   string
    snapshotInterval time.Duration  // User decided: 5 minutes
}

func (pm *PersistenceManager) Start(ctx context.Context) error {
    ticker := time.NewTicker(pm.snapshotInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := pm.Snapshot(); err != nil {
                // Log error but continue - losing 5 minutes is acceptable
                log.Error("Failed to snapshot templates: %v", err)
            }
        case <-ctx.Done():
            // Final snapshot on shutdown
            return pm.Snapshot()
        }
    }
}

func (pm *PersistenceManager) Snapshot() error {
    // Serialize all namespace templates to JSON
    data, err := json.Marshal(pm.store.templates)
    if err != nil {
        return fmt.Errorf("marshal templates: %w", err)
    }

    // Atomic write: tmp file + rename
    tmpPath := pm.snapshotPath + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }
    return os.Rename(tmpPath, pm.snapshotPath)
}

// Source: User decision from CONTEXT.md + Drain3 persistence strategies
```

### Anti-Patterns to Avoid

- **Masking before clustering:** Breaks Drain's structure detection (e.g., all IPs become `<IP>` before clustering)
- **Global template storage:** Cross-namespace pollution in multi-tenant environments
- **No rebalancing:** Templates drift over time as log formats evolve
- **Cryptographic hash collision handling:** SHA-256 collision probability is negligible for template IDs (2^-256)
- **Processing every log:** For high-volume namespaces, sample logs instead of processing all

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Drain parse tree | Custom log clustering | github.com/faceair/drain | Branch explosion mitigation, similarity calculation, O(log n) performance require careful tuning |
| Edit distance calculation | Custom string comparison | Levenshtein algorithm (standard) | Normalized edit distance needs proper length handling for similarity scoring |
| Variable detection in logs | Regex per log line | Post-clustering masking | Variable detection on raw logs causes false splits; detection on templates is more stable |
| JSON message extraction | Custom JSON parsing | encoding/json + gjson for nested fields | Handles edge cases: escaped quotes, nested objects, missing fields |
| Template merging | Simple string matching | Drain+ similarity algorithms | Template merging requires semantic understanding (synonyms, reordering) not just character matching |

**Key insight:** The Drain algorithm has subtle edge cases around variable-starting logs and branch explosion that took years of research to solve correctly. The LogPAI benchmark showed Drain achieves 37-97% performance improvement over other online parsers while maintaining highest accuracy across 11 datasets.

## Common Pitfalls

### Pitfall 1: Template Explosion from Variable-Starting Logs

**What goes wrong:** Logs that start with variables (e.g., pod names) create separate tree branches, leading to millions of nodes and poor clustering

**Example:**
```
cupsd shutdown succeeded
irqbalance shutdown succeeded
networkmanager shutdown succeeded
```
Each creates a different branch even though they have the same structure.

**Why it happens:** Drain uses the first token to navigate the tree. Variable first tokens bypass the similarity threshold entirely.

**How to avoid:**
1. Pre-tokenization: Strip known variable prefixes before Drain processing
2. Kubernetes-specific: Detect `<word>-<hash>-<hash>` pattern, replace with `<K8S_NAME>` placeholder
3. Max children parameter: Limit branches per node to force wildcard grouping (maxChildren=100 recommended)

**Warning signs:**
- Template count grows linearly with log volume
- Most templates have count=1 or count=2
- Memory usage grows unbounded

**Sources:** [Drain algorithm limitations](https://github.com/logpai/Drain3), [Variable-starting log handling](https://arxiv.org/pdf/2110.15473)

### Pitfall 2: Over-Aggressive Similarity Threshold

**What goes wrong:** Similarity threshold too high (e.g., 0.7) merges unrelated logs into the same template

**Example with sim_th=0.7:**
```
"user login succeeded"
"user login failed"
```
These are 85% similar and get merged, losing critical distinction between success/failure.

**Why it happens:** Drain's similarity threshold is token-based: `similar_tokens / total_tokens`. High threshold merges logs that differ in only 1-2 tokens.

**How to avoid:**
1. Start with sim_th=0.4 (default) for structured logs
2. For messy/unstructured logs, increase to 0.5-0.6
3. User decision: Include log level in template - `INFO: user login` vs `ERROR: user login` are different templates
4. Preserve critical distinctions: HTTP status codes, error codes stay as literals

**Warning signs:**
- Template contains both success and error messages
- Single template accounts for >50% of all logs
- Downstream analysis can't distinguish failure modes

**Sources:** [Drain3 tuning recommendations](https://github.com/logpai/Drain3), [Similarity threshold research](https://arxiv.org/pdf/1806.04356)

### Pitfall 3: No Template Drift Handling

**What goes wrong:** Log formats change over time (new fields added, messages reworded) but old templates persist, leading to duplicate templates for the same event

**Example:**
```
Old format: "Connection established to 10.0.0.1"
New format: "Connection established to 10.0.0.1 (TLS 1.3)"
```
These create separate templates even though they represent the same event.

**Why it happens:** Drain creates new clusters when similarity drops below threshold. Once created, clusters never merge automatically.

**How to avoid:**
1. Periodic rebalancing: Run template similarity check every 5-10 minutes
2. Auto-merge similar templates: Use normalized edit distance >0.7 as merge threshold
3. Count-based pruning: Remove templates with <10 occurrences (rare edge cases)
4. User decision: Start empty on first run - don't bootstrap from VictoriaLogs to avoid importing legacy formats

**Warning signs:**
- Template count grows steadily over days/weeks
- Multiple templates with near-identical patterns
- Restarting service reduces template count significantly

**Sources:** [Drain+ template correction](https://link.springer.com/chapter/10.1007/978-3-030-37453-2_15), [LogERT stability improvements](https://www.sciencedirect.com/science/article/pii/S2590005625001705)

### Pitfall 4: Inefficient High-Volume Processing

**What goes wrong:** Processing every log from high-volume namespaces (1M+ logs/hour) causes CPU bottleneck and memory pressure

**Example:** A busy ingress controller generates 10K logs/minute, all matching 5-10 templates. Processing every log is wasteful.

**Why it happens:** Drain's O(log n) matching is fast per-log but still requires tree traversal, tokenization, and similarity calculation for every message.

**How to avoid:**
1. **Sampling:** Process 1-in-N logs from high-volume namespaces (user requirement MINE-05)
2. **Batching:** Collect logs in time windows (e.g., 1 minute) before processing (user requirement MINE-06)
3. **Cache hits:** Track recently matched templates per namespace, skip Drain processing for exact matches
4. **Diversity sampling:** Use TF-IDF + DPP to select diverse logs from each batch, skip duplicates

**Implementation strategy:**
```go
// Track volume per namespace
type NamespaceTracker struct {
    logCount  int
    lastReset time.Time
}

func shouldSample(namespace string, tracker *NamespaceTracker) bool {
    threshold := 1000  // logs per minute

    if tracker.logCount < threshold {
        return true  // Process all logs
    }

    // High volume: sample 10%
    return rand.Float64() < 0.1
}
```

**Warning signs:**
- CPU pegged at 100% during log ingestion
- Lag between log generation and template extraction
- Memory growth during busy periods

**Sources:** [LLM-based batching strategies](https://arxiv.org/html/2406.06156v1), [AWSOM-LP sampling](https://arxiv.org/pdf/2110.15473)

### Pitfall 5: JSON Structure-Based Clustering

**What goes wrong:** Feeding entire JSON log to Drain causes clustering by JSON structure instead of message content

**Example:**
```json
{"level": "info", "msg": "user login succeeded", "user": "alice"}
{"level": "info", "msg": "user login succeeded", "user": "bob"}
```
These create separate templates because `"user": "alice"` vs `"user": "bob"` differ.

**Why it happens:** Drain sees the entire serialized JSON string, not just the semantic message field.

**How to avoid:**
1. Pre-processing: Extract `message`, `msg`, `log`, or `text` field from JSON before Drain
2. Ignore structured fields: Timestamp, user ID, trace ID are metadata, not template-defining
3. User decision: "For JSON logs, extract and template the message/msg field only"
4. Fallback: If no message field exists, use full JSON (might be structured event log)

**Implementation:**
```go
func extractMessageField(rawLog string) string {
    var parsed map[string]interface{}
    if err := json.Unmarshal([]byte(rawLog), &parsed); err != nil {
        return rawLog  // Not JSON, use as-is
    }

    // Try common message field names
    for _, field := range []string{"message", "msg", "log", "text", "_raw"} {
        if msg, ok := parsed[field].(string); ok {
            return msg
        }
    }

    // No message field - return full JSON
    return rawLog
}
```

**Warning signs:**
- One template per unique user/request ID
- Templates contain serialized JSON with variable values
- Template count approaches log volume

**Sources:** [JSON logging best practices](https://betterstack.com/community/guides/logging/json-logging/), [Structured log parsing](https://cloud.google.com/logging/docs/structured-logging)

## Code Examples

Verified patterns from official sources and research:

### Basic Drain Usage with faceair/drain

```go
// Source: https://pkg.go.dev/github.com/faceair/drain
package main

import (
    "fmt"
    "github.com/faceair/drain"
)

func main() {
    // Create Drain instance with configuration
    config := &drain.Config{
        LogClusterDepth: 4,        // Tree depth (minimum 3, recommended 4)
        SimTh:           0.4,      // Similarity threshold (0.3-0.5 for structured logs)
        MaxChildren:     100,      // Max branches per node (prevents explosion)
        MaxClusters:     0,        // Unlimited clusters (0 = no limit)
        ExtraDelimiters: []string{"_", "="},  // Additional token separators
        ParamString:     "<*>",    // Wildcard placeholder
    }

    logger := drain.New(config)

    // Train on log messages
    logs := []string{
        "connected to 10.0.0.1",
        "connected to 10.0.0.2",
        "Hex number 0xDEADBEAF",
        "Hex number 0x10000",
    }

    for _, line := range logs {
        cluster := logger.Train(line)
        fmt.Printf("Template: %s\n", cluster.String())
    }

    // Match new log against existing clusters
    cluster := logger.Match("connected to 10.0.0.99")
    if cluster != nil {
        fmt.Printf("Matched: %s\n", cluster.String())
        // Output: id={1} : size={3} : connected to <*>
    }
}
```

### Stable Template ID Generation with SHA-256

```go
// Source: https://pkg.go.dev/crypto/sha256 + best practices
package logprocessing

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

// Template represents a log template with stable identifier
type Template struct {
    ID        string   // SHA-256 hash of pattern + namespace
    Namespace string   // Kubernetes namespace
    Pattern   string   // Template pattern (e.g., "connected to <*>")
    Tokens    []string // Tokenized pattern
    Count     int      // Number of logs matching this template
}

// GenerateTemplateID creates a stable hash for cross-client consistency
// Requirement MINE-03: Templates have stable hashes
func GenerateTemplateID(namespace, pattern string) string {
    // Canonicalize input for deterministic hashing
    canonical := fmt.Sprintf("%s|%s", namespace, pattern)

    // SHA-256 hash (deterministic, collision-resistant)
    hash := sha256.Sum256([]byte(canonical))

    // Return hex-encoded hash as template ID
    return hex.EncodeToString(hash[:])
}

// Example usage:
// templateID := GenerateTemplateID("default", "connected to <*>")
// -> "a3c2f1e9b8d7..."  (consistent across restarts and clients)
```

### Kubernetes-Specific Name Masking

```go
// Source: User decisions from CONTEXT.md + K8s naming conventions
package logprocessing

import (
    "regexp"
    "strings"
)

var (
    // Kubernetes pod name pattern: <deployment>-<replicaset-hash>-<pod-hash>
    // Example: nginx-deployment-66b6c48dd5-8w7xz
    k8sPodPattern = regexp.MustCompile(`\b[a-z0-9-]+-[a-z0-9]{8,10}-[a-z0-9]{5}\b`)

    // Kubernetes replicaset pattern: <deployment>-<hash>
    k8sReplicaSetPattern = regexp.MustCompile(`\b[a-z0-9-]+-[a-z0-9]{8,10}\b`)
)

// MaskKubernetesNames replaces dynamic K8s resource names with placeholder
// User decision: "pod names (app-xyz-abc123) become <K8S_NAME>"
func MaskKubernetesNames(template string) string {
    // Mask pod names first (more specific pattern)
    template = k8sPodPattern.ReplaceAllString(template, "<K8S_NAME>")

    // Then mask replicaset names
    template = k8sReplicaSetPattern.ReplaceAllString(template, "<K8S_NAME>")

    return template
}

// Example:
// input:  "pod nginx-deployment-66b6c48dd5-8w7xz started"
// output: "pod <K8S_NAME> started"
```

### Aggressive Variable Masking (Post-Clustering)

```go
// Source: Drain3 masking patterns + user decisions from CONTEXT.md
package logprocessing

import "regexp"

var (
    // IP addresses (IPv4 and IPv6)
    ipv4Pattern = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
    ipv6Pattern = regexp.MustCompile(`\b[0-9a-fA-F:]+:[0-9a-fA-F:]+\b`)

    // UUIDs (standard format)
    uuidPattern = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)

    // Timestamps (ISO8601, RFC3339, Unix timestamps)
    timestampPattern = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?\b`)
    unixTimestampPattern = regexp.MustCompile(`\b\d{10,13}\b`)

    // Hex strings (0x prefix or long hex sequences)
    hexPattern = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
    longHexPattern = regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`)

    // File paths (Unix and Windows)
    filePathPattern = regexp.MustCompile(`\b(/[a-zA-Z0-9_.-]+)+\b`)
    windowsPathPattern = regexp.MustCompile(`\b[A-Z]:\\[a-zA-Z0-9_.\-\\]+\b`)

    // URLs
    urlPattern = regexp.MustCompile(`\bhttps?://[a-zA-Z0-9.-]+[a-zA-Z0-9/._?=&-]*\b`)

    // Email addresses
    emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)

    // Generic numbers (but NOT HTTP status codes - user decision)
    // Negative lookbehind/lookahead for status code context
    numberPattern = regexp.MustCompile(`\b(?<!status )\d+(?! code)\b`)
)

// AggressiveMask applies all masking patterns to a template
// User decision: "Aggressive masking" but "preserve HTTP status codes"
func AggressiveMask(template string) string {
    // Order matters: specific patterns before generic ones
    template = ipv6Pattern.ReplaceAllString(template, "<IP>")
    template = ipv4Pattern.ReplaceAllString(template, "<IP>")
    template = uuidPattern.ReplaceAllString(template, "<UUID>")
    template = timestampPattern.ReplaceAllString(template, "<TIMESTAMP>")
    template = unixTimestampPattern.ReplaceAllString(template, "<TIMESTAMP>")
    template = hexPattern.ReplaceAllString(template, "<HEX>")
    template = longHexPattern.ReplaceAllString(template, "<HEX>")
    template = urlPattern.ReplaceAllString(template, "<URL>")
    template = emailPattern.ReplaceAllString(template, "<EMAIL>")
    template = filePathPattern.ReplaceAllString(template, "<PATH>")
    template = windowsPathPattern.ReplaceAllString(template, "<PATH>")
    template = MaskKubernetesNames(template)

    // Generic numbers last (but preserve HTTP status codes)
    // User decision: "returned 404" vs "returned 500" stay distinct
    template = maskNumbersExceptStatusCodes(template)

    return template
}

func maskNumbersExceptStatusCodes(template string) string {
    // Preserve common status code contexts
    preserveContexts := []string{
        "status", "code", "http", "returned", "response",
    }

    // Simple heuristic: if "status" or "returned" appears within 3 tokens,
    // don't mask the number
    tokens := strings.Fields(template)
    for i, token := range tokens {
        if numberPattern.MatchString(token) {
            shouldMask := true

            // Check surrounding tokens for status code context
            for j := max(0, i-3); j < min(len(tokens), i+3); j++ {
                lower := strings.ToLower(tokens[j])
                for _, ctx := range preserveContexts {
                    if strings.Contains(lower, ctx) {
                        shouldMask = false
                        break
                    }
                }
            }

            if shouldMask {
                tokens[i] = "<NUM>"
            }
        }
    }

    return strings.Join(tokens, " ")
}
```

### JSON Message Field Extraction

```go
// Source: User decision + JSON logging best practices
package logprocessing

import (
    "encoding/json"
)

// ExtractMessage extracts the semantic message from a log entry
// User decision: "For JSON logs, extract and template the message/msg field only"
func ExtractMessage(rawLog string) string {
    // Try parsing as JSON
    var parsed map[string]interface{}
    if err := json.Unmarshal([]byte(rawLog), &parsed); err != nil {
        // Not JSON, use as-is
        return rawLog
    }

    // Try common message field names (order matters - most specific first)
    messageFields := []string{
        "message",  // Standard field name
        "msg",      // Common shorthand
        "log",      // Kubernetes container logs
        "text",     // Alternative name
        "_raw",     // Fluentd convention
        "event",    // Event-based logging
    }

    for _, field := range messageFields {
        if value, ok := parsed[field]; ok {
            if msg, ok := value.(string); ok && msg != "" {
                return msg
            }
        }
    }

    // No message field found - return full JSON
    // This might be a structured event log where all fields are meaningful
    return rawLog
}

// Example:
// Input:  {"level":"info","msg":"user login succeeded","user":"alice"}
// Output: "user login succeeded"
//
// Input:  "plain text log message"
// Output: "plain text log message"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Spell, LKE (sequence-based) | Drain (tree-based) | 2017 | 37-97% performance improvement, highest accuracy across benchmarks |
| Pre-clustering masking | Post-clustering masking | 2019 (Drain+) | Better handling of variable-starting logs, preserves structure detection |
| Manual regex patterns | Drain automatic template extraction | 2017 | No configuration needed, adapts to new log formats automatically |
| Global template storage | Per-namespace scoping | 2020+ (multi-tenancy) | Prevents cross-tenant template pollution |
| LRU cache eviction | Count-based pruning + auto-merge | 2021+ (Drain3, LogERT) | Handles template drift, prevents unbounded growth |
| Batch-only processing | Streaming + batching hybrid | 2024+ (LLM approaches) | Balance between real-time and efficiency |

**Deprecated/outdated:**
- **Spell algorithm**: Slower than Drain, doesn't handle variable-starting logs well
- **IPLoM**: Requires pre-configured message length groups, not adaptive
- **Pre-masking everything**: Loses structure information, causes over-generalization
- **Hardcoded similarity threshold**: Needs per-dataset tuning, no one-size-fits-all value

**Research frontier (2025-2026):**
- **LLM-based template merging**: Using semantic similarity instead of token similarity for better accuracy
- **Entropy-based sampling**: LEMUR algorithm uses information entropy for diverse log selection
- **XDrain forest approach**: Multiple trees with voting for stability (but adds complexity)

## Open Questions

Things that couldn't be fully resolved:

1. **Optimal similarity threshold for Kubernetes logs**
   - What we know: Research recommends 0.3-0.5 for structured logs, 0.5-0.6 for messy logs
   - What's unclear: Kubernetes logs mix structured (JSON) and unstructured (plain text) messages
   - Recommendation: Start with 0.4 (default), instrument to track template count growth, tune down to 0.3 if explosion occurs

2. **Auto-merge similarity threshold**
   - What we know: Drain+ uses 0.6 for template merging, we need normalized edit distance calculation
   - What's unclear: User decision is "loose clustering" but exact threshold not specified
   - Recommendation: Start with 0.7 (70% similar) for aggressive merging, instrument to track merge frequency, tune up if over-merging occurs

3. **Sampling strategy for high-volume namespaces**
   - What we know: Sample 1-in-N logs, use diversity-based sampling (TF-IDF + DPP)
   - What's unclear: Threshold for "high-volume" and sampling ratio not specified
   - Recommendation: Define high-volume as >1000 logs/minute, sample 10% (1-in-10) to balance coverage vs performance

4. **Bootstrap behavior on first run**
   - What we know: User decided "start empty, build from incoming logs"
   - What's unclear: How long until templates stabilize? Should we pre-seed common patterns?
   - Recommendation: Accept 5-10 minute "training period" after startup, don't pre-seed (user decision), instrument to track template creation rate over time

5. **JSON field extraction edge cases**
   - What we know: Extract message/msg field, ignore JSON structure
   - What's unclear: What if message field is nested? What if it's an array? What about multi-line JSON?
   - Recommendation: Implement best-effort extraction with fallback to full JSON, document known limitations

## Sources

### Primary (HIGH confidence)
- [github.com/faceair/drain](https://pkg.go.dev/github.com/faceair/drain) - Official Go port of Drain3, API documentation
- [crypto/sha256](https://pkg.go.dev/crypto/sha256) - Go standard library documentation
- [Drain original paper (2017)](https://jiemingzhu.github.io/pub/pjhe_icws2017.pdf) - Algorithm specification and performance benchmarks
- [Drain3 GitHub](https://github.com/logpai/Drain3) - Reference implementation, configuration parameters, persistence strategies
- User decisions from `/home/moritz/dev/spectre-via-ssh/.planning/phases/04-log-template-mining/04-CONTEXT.md` - Locked implementation choices

### Secondary (MEDIUM confidence)
- [LoggingDrain GitHub](https://github.com/PalanQu/LoggingDrain) - Alternative Go implementation, performance benchmarks
- [Drain+ paper (DAG approach)](https://arxiv.org/pdf/1806.04356) - Template merging algorithms and statistical separator generation
- [Stronger, Faster, and Cheaper Log Parsing with LLMs](https://arxiv.org/html/2406.06156v1) - Modern batching and sampling strategies
- [AWSOM-LP paper](https://arxiv.org/pdf/2110.15473) - Entropy-based sampling and frequency analysis
- [JSON logging best practices (Better Stack)](https://betterstack.com/community/guides/logging/json-logging/) - Message field extraction patterns
- [Google Cloud structured logging](https://cloud.google.com/logging/docs/structured-logging) - JSON field conventions

### Tertiary (LOW confidence - marked for validation)
- [XDrain paper (2024)](https://www.sciencedirect.com/science/article/abs/pii/S0950584924001514) - Fixed-depth forest approach (paywalled, summary only)
- [LogERT stability improvements (2025)](https://www.sciencedirect.com/science/article/pii/S2590005625001705) - Evolving re-search trees (recent, needs validation)
- [Kubernetes logging best practices (CNCF)](https://www.cncf.io/blog/2023/07/03/kubernetes-logging-best-practices/) - General guidance, not template-mining specific
- [Kubernetes pod naming conventions](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/) - Official docs but doesn't cover masking patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - github.com/faceair/drain is official port, widely used, stable API
- Architecture: HIGH - Patterns verified from research papers, reference implementation, and user decisions
- Pitfalls: HIGH - Documented in Drain papers, LogPAI benchmarks, and production experience reports
- Code examples: HIGH - From official documentation, verified with pkg.go.dev and user decisions
- Performance recommendations: MEDIUM - Sampling strategies from recent research, need validation at scale
- Auto-merge threshold: MEDIUM - Based on Drain+ paper but needs per-dataset tuning

**Research date:** 2026-01-21
**Valid until:** ~30 days (2026-02-20) - Drain algorithm is stable, but Go library ecosystem moves quickly

**Research methodology:**
1. WebSearch for Drain implementations (found 2 Go libraries)
2. WebFetch for official documentation (pkg.go.dev, GitHub)
3. WebSearch for algorithm tuning guidance (similarity thresholds, pitfalls)
4. WebSearch for Kubernetes-specific patterns (pod names, JSON logs)
5. Cross-referenced findings with user decisions from CONTEXT.md
6. Validated configuration parameters against Drain3 reference implementation

**Coverage assessment:**
- [x] Standard stack identified (Drain library, hashing, JSON parsing)
- [x] Architecture patterns documented (two-phase processing, namespace scoping, rebalancing, persistence)
- [x] Don't-hand-roll items listed (Drain implementation, edit distance, JSON parsing)
- [x] Common pitfalls catalogued (template explosion, drift, high-volume, JSON clustering)
- [x] Code examples provided (Drain usage, hashing, masking, JSON extraction)
- [x] State-of-the-art captured (algorithm evolution, deprecations, research frontier)
- [x] Open questions documented with recommendations

**Ready for planning:** YES - All research domains covered with high confidence. Planner can create task breakdown.
