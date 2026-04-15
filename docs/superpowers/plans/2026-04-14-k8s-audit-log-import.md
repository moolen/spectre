# Kubernetes Audit Log Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend startup JSON import and shared JSON parsing so Spectre accepts both the existing `{"events":[...]}`
format and official Kubernetes audit log files, normalizes mutating audit records into existing `models.Event`
objects, and clearly warns users when relevant audit payload data is unavailable.

**Architecture:** Keep the persisted storage model unchanged. Add an import-time format detector plus an audit-log
normalizer in `internal/importexport` that converts `audit.k8s.io` records into the existing Spectre event shape,
skips non-mutating verbs (`get`, `list`, `watch`), and returns structured warnings alongside converted events. Wire
the warnings through CLI startup import and the HTTP JSON import endpoint so missing request/response object data is
visible to users without failing the whole import.

**Tech Stack:** Go, Cobra CLI, existing Spectre import/storage packages, Kubernetes audit JSON schema, `go test`

---

## File Structure

- Modify: `internal/importexport/json_import.go`
  Purpose: replace single-format parsing with format detection, recursive file handling, and warning propagation.
- Create: `internal/importexport/audit_import.go`
  Purpose: detect and normalize official Kubernetes audit `Event` / `EventList` payloads into `[]*models.Event`.
- Create: `internal/importexport/audit_import_test.go`
  Purpose: focused tests for audit normalization, skipped verbs, missing object data, and warning generation.
- Modify: `internal/importexport/json_import_test.go`
  Purpose: preserve existing import behavior while adding format detection and directory recursion coverage.
- Modify: `internal/importexport/report.go`
  Purpose: render warnings separately from errors in CLI-visible import summaries.
- Modify: `cmd/spectre/commands/server.go`
  Purpose: surface warnings during `--import` startup for both single-file and recursive directory import.
- Modify: `internal/api/import_handler.go`
  Purpose: include warnings in JSON import responses when the request body contains official audit log data.
- Modify: `docs/docs/configuration/storage-settings.md`
  Purpose: document accepted import formats, skipped verbs, and warning behavior for missing payload data.
- Optional modify: `README.md`
  Purpose: add one short example of importing official Kubernetes audit logs if the repo keeps user-facing import docs there.

## Normalization Rules

- Accept current Spectre envelope unchanged: `{"events":[models.Event...]}`.
- Accept official Kubernetes audit payloads in these shapes:
  - single `audit.k8s.io/*` `Event`
  - `audit.k8s.io/*` `EventList`
  - newline-delimited JSON where each line is an audit `Event`
- Only convert mutating verbs:
  - `create` -> `models.EventTypeCreate`
  - `update`, `patch`, `apply` -> `models.EventTypeUpdate`
  - `delete`, `deletecollection` -> `models.EventTypeDelete`
- Skip read-only verbs:
  - `get`, `list`, `watch`, `proxy`, `connect`
- Prefer object payloads in this order:
  - `responseObject`
  - `requestObject`
- Timestamp source:
  - prefer `stageTimestamp`
  - fallback to `requestReceivedTimestamp`
- Resource identity source:
  - derive `group`, `version`, `resource`, `subresource`, `namespace`, `name` from `objectRef`
  - set `kind` from `objectRef.resource` title-cased when a concrete kind is unavailable
  - synthesize a stable UID when audit logs do not include one
- Warning policy:
  - do not fail the import when a mutating audit record lacks object payload data
  - log and report the issue
  - skip records that cannot produce minimum `models.ResourceMetadata`

## Task 1: Lock The Parser Contract With Tests

**Files:**
- Modify: `internal/importexport/json_import_test.go`
- Create: `internal/importexport/audit_import_test.go`

- [ ] **Step 1: Add a failing test for the existing Spectre envelope path**

```go
func TestParseImportPayload_SpectreEnvelope(t *testing.T) {
	input := strings.NewReader(`{
		"events": [{
			"id": "event1",
			"timestamp": 1234567890000000000,
			"type": "CREATE",
			"resource": {
				"group": "apps",
				"version": "v1",
				"kind": "Deployment",
				"namespace": "default",
				"name": "demo",
				"uid": "demo-uid"
			},
			"data": {"apiVersion":"apps/v1","kind":"Deployment"}
		}]
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}
```

- [ ] **Step 2: Add a failing test for a single official audit event**

```go
func TestParseImportPayload_AuditSingleEvent(t *testing.T) {
	input := strings.NewReader(`{
		"kind":"Event",
		"apiVersion":"audit.k8s.io/v1",
		"level":"RequestResponse",
		"auditID":"audit-1",
		"stage":"ResponseComplete",
		"verb":"create",
		"requestReceivedTimestamp":"2026-04-14T10:11:12Z",
		"stageTimestamp":"2026-04-14T10:11:13Z",
		"objectRef":{
			"resource":"deployments",
			"namespace":"default",
			"name":"demo",
			"apiGroup":"apps",
			"apiVersion":"v1"
		},
		"responseObject":{
			"apiVersion":"apps/v1",
			"kind":"Deployment",
			"metadata":{"name":"demo","namespace":"default"}
		}
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != models.EventTypeCreate {
		t.Fatalf("expected CREATE, got %s", events[0].Type)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}
```

- [ ] **Step 3: Add a failing test for `EventList` input**

```go
func TestParseImportPayload_AuditEventList(t *testing.T) {
	input := strings.NewReader(`{
		"kind":"EventList",
		"apiVersion":"audit.k8s.io/v1",
		"items":[
			{
				"kind":"Event",
				"apiVersion":"audit.k8s.io/v1",
				"auditID":"audit-1",
				"stage":"ResponseComplete",
				"verb":"patch",
				"stageTimestamp":"2026-04-14T10:11:13Z",
				"objectRef":{
					"resource":"configmaps",
					"namespace":"default",
					"name":"demo",
					"apiVersion":"v1"
				},
				"requestObject":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"demo","namespace":"default"}}
			}
		]
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() error = %v", err)
	}
	if len(events) != 1 || events[0].Type != models.EventTypeUpdate {
		t.Fatalf("expected 1 UPDATE event, got %#v", events)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}
```

- [ ] **Step 4: Add a failing test for JSONL audit input**

```go
func TestImportJSONFile_AuditJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.log")
	content := `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"a1","stage":"ResponseComplete","verb":"delete","stageTimestamp":"2026-04-14T10:11:13Z","objectRef":{"resource":"pods","namespace":"default","name":"demo","apiVersion":"v1"}}
{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"a2","stage":"ResponseComplete","verb":"get","stageTimestamp":"2026-04-14T10:11:14Z","objectRef":{"resource":"pods","namespace":"default","name":"demo","apiVersion":"v1"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	events, warnings, err := ImportJSONFile(path)
	if err != nil {
		t.Fatalf("ImportJSONFile() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected only mutating event to be imported, got %d", len(events))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}
```

- [ ] **Step 5: Add a failing test for missing object payload warnings**

```go
func TestParseImportPayload_AuditMissingObjectPayloadWarns(t *testing.T) {
	input := strings.NewReader(`{
		"kind":"Event",
		"apiVersion":"audit.k8s.io/v1",
		"auditID":"audit-1",
		"stage":"ResponseComplete",
		"verb":"update",
		"stageTimestamp":"2026-04-14T10:11:13Z",
		"objectRef":{
			"resource":"deployments",
			"namespace":"default",
			"name":"demo",
			"apiGroup":"apps",
			"apiVersion":"v1"
		}
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected event to be skipped without payload, got %d", len(events))
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for missing object payload")
	}
}
```

- [ ] **Step 6: Add a failing test for recursive directory import over mixed `.json` and `.log` files**

```go
func TestWalkAndImportJSON_MixedSpectreAndAuditFiles(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	spectreFile := filepath.Join(tmpDir, "events.json")
	if err := os.WriteFile(spectreFile, []byte(`{
		"events": [{
			"id": "event1",
			"timestamp": 1234567890000000000,
			"type": "CREATE",
			"resource": {
				"group": "apps",
				"version": "v1",
				"kind": "Deployment",
				"namespace": "default",
				"name": "demo",
				"uid": "demo-uid"
			},
			"data": {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"demo","namespace":"default"}}
		}]
	}`), 0644); err != nil {
		t.Fatalf("write spectre file: %v", err)
	}

	auditJSONFile := filepath.Join(nested, "audit.json")
	if err := os.WriteFile(auditJSONFile, []byte(`{
		"kind":"Event",
		"apiVersion":"audit.k8s.io/v1",
		"auditID":"audit-1",
		"stage":"ResponseComplete",
		"verb":"patch",
		"stageTimestamp":"2026-04-14T10:11:13Z",
		"objectRef":{"resource":"configmaps","namespace":"default","name":"demo","apiVersion":"v1"},
		"requestObject":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"demo","namespace":"default"}}
	}`), 0644); err != nil {
		t.Fatalf("write audit json file: %v", err)
	}

	auditLogFile := filepath.Join(nested, "audit.log")
	if err := os.WriteFile(auditLogFile, []byte(`{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-2","stage":"ResponseComplete","verb":"delete","stageTimestamp":"2026-04-14T10:11:14Z","objectRef":{"resource":"pods","namespace":"default","name":"demo","apiVersion":"v1"},"requestObject":{"apiVersion":"v1","kind":"Pod","metadata":{"name":"demo","namespace":"default"}}}`), 0644); err != nil {
		t.Fatalf("write audit log file: %v", err)
	}

	storageDir := t.TempDir()
	st, err := storage.New(storageDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}
	defer st.Close()

	report, err := WalkAndImportJSON(tmpDir, st, storage.ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}, nil)
	if err != nil {
		t.Fatalf("WalkAndImportJSON() error = %v", err)
	}
	if report.TotalFiles != 3 {
		t.Fatalf("expected 3 files, got %d", report.TotalFiles)
	}
	if report.TotalEvents != 3 {
		t.Fatalf("expected 3 imported events, got %d", report.TotalEvents)
	}
}
```

- [ ] **Step 7: Run the import package tests to verify failure**

Run: `go test ./internal/importexport -count=1`

Expected: FAIL in the new audit import tests because `ParseImportPayload`, warning propagation, and audit normalization do not exist yet.

- [ ] **Step 8: Commit the failing tests**

```bash
git add internal/importexport/json_import_test.go internal/importexport/audit_import_test.go
git commit -m "test: cover official kubernetes audit import"
```

## Task 2: Introduce A Shared Import Parsing Result

**Files:**
- Modify: `internal/importexport/json_import.go`
- Modify: `internal/importexport/report.go`

- [ ] **Step 1: Add an internal parse result type and warning-aware public signatures**

```go
type ParseResult struct {
	Events   []*models.Event
	Warnings []string
}

func ImportJSONFile(filePath string) ([]*models.Event, []string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return ParseImportPayload(file)
}

func ParseImportPayload(r io.Reader) ([]*models.Event, []string, error) {
	panic("not implemented")
}
```

- [ ] **Step 2: Keep backward compatibility by updating all local callers in the same patch**

```go
events, warnings, err := ImportJSONFile(filePath)
if err != nil {
	return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
}
report.Warnings = append(report.Warnings, warnings...)
```

- [ ] **Step 3: Extend `ImportReport` with warnings**

```go
type ImportReport struct {
	TotalFiles    int
	ImportedFiles int
	MergedHours   int
	SkippedFiles  int
	FailedFiles   int
	TotalEvents   int64
	Errors        []string
	Warnings      []string
	Duration      time.Duration
}
```

- [ ] **Step 4: Render warnings separately in terminal summaries**

```go
if len(report.Warnings) > 0 {
	sb.WriteString("\nWarnings:\n")
	for _, warning := range report.Warnings {
		sb.WriteString(fmt.Sprintf("  - %s\n", warning))
	}
}
```

- [ ] **Step 5: Update `WalkAndImportJSON` to aggregate warnings instead of treating them as failures**

```go
var allWarnings []string

for _, filePath := range matchedFiles {
	events, warnings, err := ImportJSONFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}
	allWarnings = append(allWarnings, warnings...)
	allEvents = append(allEvents, events...)
}

report.Warnings = append(report.Warnings, allWarnings...)
```

- [ ] **Step 6: Run tests for the parser/report package**

Run: `go test ./internal/importexport -run 'TestParseImportPayload|TestImportJSONFile|TestWalkAndImportJSON|TestFormatImportReport' -count=1`

Expected: some tests still fail because audit detection and normalization are not implemented yet.

- [ ] **Step 7: Commit the warning-capable parser contract**

```bash
git add internal/importexport/json_import.go internal/importexport/report.go
git commit -m "refactor: add warning-aware import parsing contract"
```

## Task 3: Implement Official Audit Log Detection And Normalization

**Files:**
- Create: `internal/importexport/audit_import.go`
- Create: `internal/importexport/audit_import_test.go`
- Modify: `internal/importexport/json_import.go`

- [ ] **Step 1: Add small internal audit types for the fields Spectre cares about**

```go
type auditEvent struct {
	Kind                     string          `json:"kind"`
	APIVersion               string          `json:"apiVersion"`
	AuditID                  string          `json:"auditID"`
	Verb                     string          `json:"verb"`
	Stage                    string          `json:"stage"`
	RequestReceivedTimestamp string          `json:"requestReceivedTimestamp"`
	StageTimestamp           string          `json:"stageTimestamp"`
	ObjectRef                *auditObjectRef `json:"objectRef"`
	RequestObject            json.RawMessage `json:"requestObject"`
	ResponseObject           json.RawMessage `json:"responseObject"`
}

type auditObjectRef struct {
	Resource    string `json:"resource"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	APIGroup    string `json:"apiGroup"`
	APIVersion  string `json:"apiVersion"`
	Subresource string `json:"subresource"`
}

type auditEventList struct {
	Kind       string       `json:"kind"`
	APIVersion string       `json:"apiVersion"`
	Items      []auditEvent `json:"items"`
}
```

- [ ] **Step 2: Add a format detector that reads the full payload once and then branches**

```go
func ParseImportPayload(r io.Reader) ([]*models.Event, []string, error) {
	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read import payload: %w", err)
	}

	if result, ok, err := tryParseSpectreEnvelope(payload); ok || err != nil {
		return result.Events, result.Warnings, err
	}
	if result, ok, err := tryParseAuditPayload(payload); ok || err != nil {
		return result.Events, result.Warnings, err
	}

	return nil, nil, fmt.Errorf("unsupported JSON import format")
}
```

- [ ] **Step 3: Implement official audit payload parsing for object JSON and JSONL**

```go
func tryParseAuditPayload(payload []byte) (ParseResult, bool, error) {
	if result, ok, err := parseAuditJSONObject(payload); ok || err != nil {
		return result, ok, err
	}
	if result, ok, err := parseAuditJSONLines(payload); ok || err != nil {
		return result, ok, err
	}
	return ParseResult{}, false, nil
}
```

- [ ] **Step 4: Implement verb filtering and stage filtering**

```go
func normalizeAuditEvent(evt auditEvent) (*models.Event, string, bool) {
	verb := strings.ToLower(evt.Verb)
	switch verb {
	case "get", "list", "watch", "proxy", "connect":
		return nil, "", false
	case "create":
		// continue
	case "update", "patch", "apply":
		// continue
	case "delete", "deletecollection":
		// continue
	default:
		return nil, "", false
	}

	if evt.Stage != "" && evt.Stage != "ResponseComplete" && evt.Stage != "Panic" {
		return nil, "", false
	}

	// build event in next step
	return nil, "", true
}
```

- [ ] **Step 5: Normalize audit records into existing `models.Event` values**

```go
func normalizeAuditEvent(evt auditEvent) (*models.Event, string, bool) {
	// verb + stage filtering omitted here

	resource, warning, ok := buildResourceMetadata(evt)
	if !ok {
		return nil, warning, true
	}

	data := preferredAuditObject(evt)
	if len(data) == 0 {
		return nil, fmt.Sprintf("auditID=%s %s %s/%s skipped: mutating audit event missing requestObject/responseObject",
			evt.AuditID, evt.Verb, resource.Namespace, resource.Name), true
	}

	eventType := mapAuditVerb(evt.Verb)
	ts, err := parseAuditTimestamp(evt)
	if err != nil {
		return nil, fmt.Sprintf("auditID=%s skipped: invalid timestamp: %v", evt.AuditID, err), true
	}

	return &models.Event{
		ID:        stableAuditEventID(evt, resource),
		Timestamp: ts.UnixNano(),
		Type:      eventType,
		Resource:  resource,
		Data:      data,
		DataSize:  int32(len(data)),
	}, warning, true
}
```

- [ ] **Step 6: Synthesize stable resource identity when the audit payload lacks UID**

```go
func buildResourceMetadata(evt auditEvent) (models.ResourceMetadata, string, bool) {
	if evt.ObjectRef == nil || evt.ObjectRef.Resource == "" || evt.ObjectRef.Name == "" || evt.ObjectRef.APIVersion == "" {
		return models.ResourceMetadata{}, fmt.Sprintf("auditID=%s skipped: objectRef missing resource identity", evt.AuditID), false
	}

	kind := inferKind(evt.ObjectRef.Resource)
	uid := stableResourceUID(evt)
	warning := ""
	if uid != "" {
		warning = fmt.Sprintf("auditID=%s %s %s/%s imported with synthetic uid=%s", evt.AuditID, evt.Verb, evt.ObjectRef.Namespace, evt.ObjectRef.Name, uid)
	}

	return models.ResourceMetadata{
		Group:     evt.ObjectRef.APIGroup,
		Version:   evt.ObjectRef.APIVersion,
		Kind:      kind,
		Namespace: evt.ObjectRef.Namespace,
		Name:      evt.ObjectRef.Name,
		UID:       uid,
	}, warning, true
}
```

- [ ] **Step 7: Reduce warning noise by aggregating duplicate messages before returning them**

```go
func dedupeWarnings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, warning := range in {
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		out = append(out, warning)
	}
	return out
}
```

- [ ] **Step 8: Run the audit import tests**

Run: `go test ./internal/importexport -run 'TestParseImportPayload|TestImportJSONFile_AuditJSONL' -count=1`

Expected: PASS for audit normalization and warning behavior.

- [ ] **Step 9: Commit audit normalization**

```bash
git add internal/importexport/json_import.go internal/importexport/audit_import.go internal/importexport/audit_import_test.go
git commit -m "feat: normalize official kubernetes audit logs on import"
```

## Task 4: Extend Recursive File Import Matching

**Files:**
- Modify: `internal/importexport/json_import.go`
- Modify: `internal/importexport/json_import_test.go`

- [ ] **Step 1: Expand recursive matching beyond `.json` to include common audit log filenames**

```go
func isSupportedImportFile(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return true
	case strings.HasSuffix(lower, ".jsonl"):
		return true
	case strings.HasSuffix(lower, ".log"):
		return true
	default:
		return false
	}
}
```

- [ ] **Step 2: Use parser-first validation so filename is only a candidate filter**

```go
if isSupportedImportFile(info.Name()) {
	jsonFiles = append(jsonFiles, path)
}

// later:
events, warnings, err := ImportJSONFile(filePath)
if err != nil {
	return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
}
```

- [ ] **Step 3: Preserve existing strict failure behavior for malformed matched files**

```go
// Keep current behavior:
// if a matched file cannot be parsed as either Spectre JSON or audit JSON/audit JSONL,
// fail the import rather than silently skip it.
```

- [ ] **Step 4: Run recursive import tests**

Run: `go test ./internal/importexport -run 'TestWalkAndImportJSON' -count=1`

Expected: PASS, including mixed Spectre and audit files in nested directories.

- [ ] **Step 5: Commit recursive matching changes**

```bash
git add internal/importexport/json_import.go internal/importexport/json_import_test.go
git commit -m "feat: detect audit log files during recursive import"
```

## Task 5: Surface Warnings To Users In CLI And API Import Flows

**Files:**
- Modify: `cmd/spectre/commands/server.go`
- Modify: `internal/api/import_handler.go`
- Test: `internal/importexport/json_import_test.go`

- [ ] **Step 1: Update CLI single-file import to collect parser warnings**

```go
events, warnings, err := importexport.ImportJSONFile(importPath)
if err != nil {
	logger.Error("Failed to read file: %v", err)
	HandleError(err, "Import file error")
}

for _, warning := range warnings {
	logger.Warn("Import warning: %s", warning)
}
```

- [ ] **Step 2: Include warnings in the printed startup import report**

```go
report = &importexport.ImportReport{
	TotalFiles:    1,
	ImportedFiles: storageReport.ImportedFiles,
	MergedHours:   storageReport.MergedHours,
	SkippedFiles:  storageReport.SkippedFiles,
	FailedFiles:   storageReport.FailedFiles,
	TotalEvents:   storageReport.TotalEvents,
	Errors:        storageReport.Errors,
	Warnings:      warnings,
	Duration:      time.Since(startTime),
}
```

- [ ] **Step 3: Propagate warnings through the HTTP JSON import endpoint**

```go
events, warnings, err := importexport.ParseImportPayload(decompressedBody)
if err != nil {
	writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
	return
}

response := map[string]interface{}{
	"status":         "success",
	"total_events":   report.TotalEvents,
	"merged_hours":   report.MergedHours,
	"files_created":  report.MergedHours,
	"imported_files": report.ImportedFiles,
	"duration":       report.Duration.String(),
	"errors":         report.Errors,
	"warnings":       warnings,
}
```

- [ ] **Step 4: Log warning summaries in the API handler without flooding logs**

```go
if len(warnings) > 0 {
	h.logger.Warn("JSON import completed with %d warnings", len(warnings))
	for _, warning := range warnings {
		h.logger.Warn("Import warning: %s", warning)
	}
}
```

- [ ] **Step 5: Run targeted CLI/API-adjacent tests**

Run: `go test ./internal/importexport ./internal/api -count=1`

Expected: PASS, with import parser tests green and API compile/tests updated for the new parser signature.

- [ ] **Step 6: Commit user-visible warning propagation**

```bash
git add cmd/spectre/commands/server.go internal/api/import_handler.go internal/importexport/report.go
git commit -m "feat: surface audit import warnings in cli and api"
```

## Task 6: Document The New Import Behavior

**Files:**
- Modify: `docs/docs/configuration/storage-settings.md`
- Optional modify: `README.md`

- [ ] **Step 1: Document supported import formats**

```md
Spectre accepts these startup import formats:

- Native Spectre JSON: `{"events":[...]}`
- Kubernetes audit `Event`
- Kubernetes audit `EventList`
- Line-delimited Kubernetes audit JSON (`.jsonl` / `.log`)
```

- [ ] **Step 2: Document the normalization rules users will observe**

```md
When importing official Kubernetes audit logs, Spectre:

- keeps mutating requests only (`create`, `update`, `patch`, `apply`, `delete`, `deletecollection`)
- skips read-only requests (`get`, `list`, `watch`, `proxy`, `connect`)
- uses `responseObject` or `requestObject` as the best-effort resource snapshot
- emits warnings when mutating audit entries cannot provide enough object data to build a Spectre event
```

- [ ] **Step 3: Add one CLI example using audit log files**

```bash
spectre server --import=/backups/kube-apiserver-audit/
```

- [ ] **Step 4: Run markdown/docs sanity checks if the repo has them**

Run: `rg -n "Kubernetes audit" docs/docs/configuration/storage-settings.md README.md`

Expected: the new documentation appears in the import section with no leftover placeholders.

- [ ] **Step 5: Commit documentation**

```bash
git add docs/docs/configuration/storage-settings.md README.md
git commit -m "docs: describe kubernetes audit log import support"
```

## Task 7: Full Verification

**Files:**
- Modify only if verification reveals regressions.

- [ ] **Step 1: Run the focused import and API tests**

Run: `go test ./internal/importexport ./internal/api -count=1`

Expected: PASS

- [ ] **Step 2: Run the storage tests that validate imported event shape**

Run: `go test ./internal/storage -run 'TestAddEventsBatch|TestExportImport' -count=1`

Expected: PASS

- [ ] **Step 3: Run one end-to-end JSON import test**

Run: `go test ./tests/e2e -run TestJSONEventBatchImport -count=1`

Expected: PASS if an e2e cluster is available; otherwise record that this step was not run.

- [ ] **Step 4: Manually smoke-test startup import with a small audit fixture**

Run: `go test ./internal/importexport -run TestImportJSONFile_AuditJSONL -count=1 -v`

Expected: PASS and fixture demonstrates that only mutating audit records are converted.

- [ ] **Step 5: Commit any final verification fixes**

```bash
git add internal/importexport internal/api cmd/spectre/commands docs/docs/configuration/storage-settings.md README.md
git commit -m "test: verify kubernetes audit import end to end"
```

## Self-Review

- Spec coverage:
  - recursive file import support: covered in Task 4
  - official Kubernetes audit format support: covered in Task 3
  - ignore read-only verbs: covered in Task 3
  - warn user when audit object data is unavailable: covered in Tasks 2, 3, and 5
  - preserve existing Spectre import behavior: covered in Task 1 and Task 2
- Placeholder scan:
  - no placeholders remain
- Type consistency:
  - the plan assumes `ImportJSONFile` and `ParseImportPayload` return `([]*models.Event, []string, error)` everywhere
  - the plan keeps `models.Event` and `storage.AddEventsBatch` unchanged
