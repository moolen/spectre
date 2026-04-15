# Sensitive Data Scrubbing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `spectre server --scrub-sensitive-data` so live watcher ingestion and startup JSON imports scrub sensitive values before they are written to storage while preserving enough readability for debugging.

**Architecture:** Introduce a small `internal/scrub` package that operates on structured JSON and is called at ingest time. Wire the boolean flag into runtime config, pass a shared scrubber into the watcher event handler and CLI import path, and leave storage/query/export/timeline code unchanged because persisted data is already scrubbed.

**Tech Stack:** Go, Cobra CLI, existing Spectre watcher/storage/import packages, `go test`

---

## File Structure

- Create: `internal/scrub/scrubber.go`
- Create: `internal/scrub/scrubber_test.go`
- Create: `internal/watcher/event_handler_test.go`
- Create: `cmd/spectre/commands/server_scrub_flag_test.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/spectre/commands/server.go`
- Modify: `internal/watcher/event_handler.go`
- Modify: `internal/importexport/json_import.go`
- Modify: `internal/importexport/json_import_test.go`
- Modify: `docs/docs/configuration/storage-settings.md`

### Task 1: Add CLI And Config Plumbing

**Files:**
- Create: `cmd/spectre/commands/server_scrub_flag_test.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/spectre/commands/server.go`

- [ ] **Step 1: Write the failing CLI flag test**

```go
package commands

import "testing"

func TestServerScrubSensitiveDataFlagDefaultsToFalse(t *testing.T) {
	flag := serverCmd.Flags().Lookup("scrub-sensitive-data")
	if flag == nil {
		t.Fatalf("expected scrub-sensitive-data flag to be registered")
	}

	if flag.DefValue != "false" {
		t.Fatalf("expected default false, got %q", flag.DefValue)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/spectre/commands -run TestServerScrubSensitiveDataFlagDefaultsToFalse -count=1`
Expected: FAIL with `expected scrub-sensitive-data flag to be registered`

- [ ] **Step 3: Add config field and server flag**

```go
// internal/config/config.go
type Config struct {
	DataDir               string
	APIPort               int
	LogLevel              string
	WatcherConfigPath     string
	SegmentSize           int64
	MaxConcurrentRequests int
	BlockCacheMaxMB       int64
	BlockCacheEnabled     bool
	TracingEnabled        bool
	TracingEndpoint       string
	TracingTLSCAPath      string
	TracingTLSInsecure    bool
	ScrubSensitiveData    bool
}

func LoadConfig(
	dataDir string,
	apiPort int,
	logLevel, watcherConfigPath string,
	segmentSize int64,
	maxConcurrentRequests int,
	blockCacheMaxMB int64,
	blockCacheEnabled, tracingEnabled bool,
	tracingEndpoint, tracingTLSCAPath string,
	tracingTLSInsecure bool,
	scrubSensitiveData bool,
) *Config {
	return &Config{
		DataDir:               dataDir,
		APIPort:               apiPort,
		LogLevel:              logLevel,
		WatcherConfigPath:     watcherConfigPath,
		SegmentSize:           segmentSize,
		MaxConcurrentRequests: maxConcurrentRequests,
		BlockCacheMaxMB:       blockCacheMaxMB,
		BlockCacheEnabled:     blockCacheEnabled,
		TracingEnabled:        tracingEnabled,
		TracingEndpoint:       tracingEndpoint,
		TracingTLSCAPath:      tracingTLSCAPath,
		TracingTLSInsecure:    tracingTLSInsecure,
		ScrubSensitiveData:    scrubSensitiveData,
	}
}
```

```go
// cmd/spectre/commands/server.go
var (
	demo                  bool
	dataDir               string
	apiPort               int
	watcherConfigPath     string
	watcherEnabled        bool
	segmentSize           int64
	maxConcurrentRequests int
	cacheMaxMB            int64
	cacheEnabled          bool
	importPath            string
	scrubSensitiveData    bool
)

func init() {
	serverCmd.Flags().BoolVar(&scrubSensitiveData, "scrub-sensitive-data", false, "Scrub sensitive values before writing resource data to storage")
}

func runServer(cmd *cobra.Command, args []string) {
	cfg := config.LoadConfig(
		dataDir,
		apiPort,
		logLevel,
		watcherConfigPath,
		segmentSize,
		maxConcurrentRequests,
		cacheMaxMB,
		cacheEnabled,
		tracingEnabled,
		tracingEndpoint,
		tracingTLSCAPath,
		tracingTLSInsecure,
		scrubSensitiveData,
	)
}
```

- [ ] **Step 4: Run the targeted tests**

Run: `go test ./cmd/spectre/commands ./internal/config -run 'TestServerScrubSensitiveDataFlagDefaultsToFalse|TestNonExistent' -count=1`
Expected: `ok   github.com/moolen/spectre/cmd/spectre/commands`
Expected: `?    github.com/moolen/spectre/internal/config [no test files]`

- [ ] **Step 5: Commit**

```bash
git add cmd/spectre/commands/server.go cmd/spectre/commands/server_scrub_flag_test.go internal/config/config.go
git commit -m "feat: add scrub sensitive data server flag"
```

### Task 2: Build The JSON Scrubber With Unit Tests

**Files:**
- Create: `internal/scrub/scrubber.go`
- Create: `internal/scrub/scrubber_test.go`

- [ ] **Step 1: Write the failing scrubber tests**

```go
package scrub

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestScrubEventData_ConfigMapData(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key","LOG_LEVEL":"info"}}`)

	out, err := s.ScrubEventData("ConfigMap", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data := got["data"].(map[string]any)
	if data["JWT_SECRET"] == "demo_jwt_secret_key" {
		t.Fatalf("expected JWT_SECRET to be scrubbed")
	}
	if data["LOG_LEVEL"] == "info" {
		t.Fatalf("expected ConfigMap values to be scrubbed")
	}
}

func TestScrubEventData_SecretDataReencodesBase64(t *testing.T) {
	s := New(true)
	encoded := base64.StdEncoding.EncodeToString([]byte("sk_test_fake_key_payment"))
	input := json.RawMessage(`{"kind":"Secret","data":{"token":"` + encoded + `"}}`)

	out, err := s.ScrubEventData("Secret", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(got.Data["token"])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if string(decoded) == "sk_test_fake_key_payment" {
		t.Fatalf("expected decoded secret to be scrubbed")
	}
}

func TestScrubEventData_LastAppliedConfigurationRecurses(t *testing.T) {
	s := New(true)
	input := json.RawMessage(`{
		"metadata": {
			"annotations": {
				"kubectl.kubernetes.io/last-applied-configuration": "{\"kind\":\"ConfigMap\",\"data\":{\"JWT_SECRET\":\"demo_jwt_secret_key\"}}"
			}
		}
	}`)

	out, err := s.ScrubEventData("Deployment", input)
	if err != nil {
		t.Fatalf("ScrubEventData() error = %v", err)
	}

	var got struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Metadata.Annotations["kubectl.kubernetes.io/last-applied-configuration"] == "{\"kind\":\"ConfigMap\",\"data\":{\"JWT_SECRET\":\"demo_jwt_secret_key\"}}" {
		t.Fatalf("expected annotation payload to be scrubbed")
	}
}

func TestMaskString_PreservesLength(t *testing.T) {
	if got := maskString("demo_jwt_secret_key"); len(got) != len("demo_jwt_secret_key") {
		t.Fatalf("expected preserved length, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scrub -count=1`
Expected: FAIL with `undefined: New` and `undefined: maskString`

- [ ] **Step 3: Implement the scrubber**

```go
package scrub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type Scrubber struct {
	enabled bool
}

func New(enabled bool) *Scrubber {
	return &Scrubber{enabled: enabled}
}

func (s *Scrubber) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Scrubber) ScrubEventData(kind string, data json.RawMessage) (json.RawMessage, error) {
	if !s.Enabled() || len(data) == 0 {
		return data, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parse scrub target: %w", err)
	}

	switch kind {
	case "Secret":
		s.scrubStringMap(obj, "stringData")
		s.scrubBase64Map(obj, "data")
	case "ConfigMap":
		s.scrubStringMap(obj, "data")
		s.scrubBase64Map(obj, "binaryData")
	}

	s.scrubWorkloadEnv(obj)
	s.scrubLastAppliedConfiguration(obj)

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal scrub target: %w", err)
	}
	return json.RawMessage(out), nil
}

func maskString(value string) string {
	n := len(value)
	if n == 0 {
		return value
	}
	if n <= 4 {
		return value[:1] + repeatMask(n-1)
	}
	if n <= 8 {
		return value[:1] + repeatMask(n-2) + value[n-1:]
	}
	return value[:3] + repeatMask(n-5) + value[n-2:]
}

func repeatMask(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = '*'
	}
	return string(buf)
}

func (s *Scrubber) scrubStringMap(obj map[string]any, field string) {
	values, ok := obj[field].(map[string]any)
	if !ok {
		return
	}
	for key, raw := range values {
		if text, ok := raw.(string); ok {
			values[key] = maskString(text)
		}
	}
}

func (s *Scrubber) scrubBase64Map(obj map[string]any, field string) {
	values, ok := obj[field].(map[string]any)
	if !ok {
		return
	}
	for key, raw := range values {
		text, ok := raw.(string)
		if !ok {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			values[key] = maskString(text)
			continue
		}
		values[key] = base64.StdEncoding.EncodeToString([]byte(maskString(string(decoded))))
	}
}
```

- [ ] **Step 4: Extend the implementation to cover env values and annotation recursion**

```go
func (s *Scrubber) scrubWorkloadEnv(obj map[string]any) {
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return
	}
	template, ok := spec["template"].(map[string]any)
	if !ok {
		return
	}
	templateSpec, ok := template["spec"].(map[string]any)
	if !ok {
		return
	}

	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		items, ok := templateSpec[field].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			container, ok := item.(map[string]any)
			if !ok {
				continue
			}
			envItems, ok := container["env"].([]any)
			if !ok {
				continue
			}
			for _, envItem := range envItems {
				envMap, ok := envItem.(map[string]any)
				if !ok {
					continue
				}
				value, ok := envMap["value"].(string)
				if !ok {
					continue
				}
				envMap["value"] = maskString(value)
			}
		}
	}
}

func (s *Scrubber) scrubLastAppliedConfiguration(obj map[string]any) {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok {
		return
	}
	raw, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"].(string)
	if !ok || raw == "" {
		return
	}

	var nested map[string]any
	if err := json.Unmarshal([]byte(raw), &nested); err != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = maskString(raw)
		return
	}

	nestedKind, _ := nested["kind"].(string)
	nestedJSON, err := json.Marshal(nested)
	if err != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = maskString(raw)
		return
	}

	scrubbed, err := s.ScrubEventData(nestedKind, nestedJSON)
	if err != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = maskString(raw)
		return
	}
	annotations["kubectl.kubernetes.io/last-applied-configuration"] = string(scrubbed)
}
```

- [ ] **Step 5: Run scrubber tests to verify they pass**

Run: `go test ./internal/scrub -count=1`
Expected: `ok   github.com/moolen/spectre/internal/scrub`

- [ ] **Step 6: Commit**

```bash
git add internal/scrub/scrubber.go internal/scrub/scrubber_test.go
git commit -m "feat: add ingest-time sensitive data scrubber"
```

### Task 3: Integrate Scrubbing Into Watcher Ingestion

**Files:**
- Create: `internal/watcher/event_handler_test.go`
- Modify: `internal/watcher/event_handler.go`
- Modify: `cmd/spectre/commands/server.go`

- [ ] **Step 1: Write the failing watcher integration tests**

```go
package watcher

import (
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/scrub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type recordingStorage struct {
	event *models.Event
}

func (s *recordingStorage) WriteEvent(event *models.Event) error {
	s.event = event
	return nil
}

func TestEventCaptureHandlerOnAddScrubsConfigMapDataWhenEnabled(t *testing.T) {
	store := &recordingStorage{}
	handler := NewEventCaptureHandler(store, scrub.New(true))

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default", UID: "uid-1"},
		Data: map[string]string{
			"JWT_SECRET": "demo_jwt_secret_key",
		},
	}
	obj.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	if err := handler.OnAdd(obj); err != nil {
		t.Fatalf("OnAdd() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(store.event.Data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	data := got["data"].(map[string]any)
	if data["JWT_SECRET"] == "demo_jwt_secret_key" {
		t.Fatalf("expected stored ConfigMap data to be scrubbed")
	}
}

func TestEventCaptureHandlerOnAddLeavesPayloadUntouchedWhenDisabled(t *testing.T) {
	store := &recordingStorage{}
	handler := NewEventCaptureHandler(store, scrub.New(false))

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default", UID: "uid-1"},
		Data: map[string]string{
			"JWT_SECRET": "demo_jwt_secret_key",
		},
	}
	obj.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	if err := handler.OnAdd(obj); err != nil {
		t.Fatalf("OnAdd() error = %v", err)
	}

	if !json.Valid(store.event.Data) {
		t.Fatalf("expected valid stored JSON")
	}
	if string(store.event.Data) == "" {
		t.Fatalf("expected stored payload")
	}
}
```

- [ ] **Step 2: Run watcher tests to verify they fail**

Run: `go test ./internal/watcher -run 'TestEventCaptureHandlerOnAdd' -count=1`
Expected: FAIL with `too many arguments in call to NewEventCaptureHandler`

- [ ] **Step 3: Inject the scrubber into the watcher event handler**

```go
// internal/watcher/event_handler.go
type EventCaptureHandler struct {
	storage  StorageWriter
	logger   *logging.Logger
	pruner   *ManagedFieldsPruner
	scrubber *scrub.Scrubber
}

func NewEventCaptureHandler(storage StorageWriter, scrubber *scrub.Scrubber) *EventCaptureHandler {
	return &EventCaptureHandler{
		storage:  storage,
		logger:   logging.GetLogger("event_handler"),
		pruner:   NewManagedFieldsPruner(),
		scrubber: scrubber,
	}
}

func (h *EventCaptureHandler) objectToJSON(obj runtime.Object) (json.RawMessage, int32, error) {
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal object to JSON: %w", err)
	}

	dataSize := int32(len(jsonData))
	jsonData, err = h.pruner.Prune(jsonData)
	if err != nil {
		h.logger.Warn("Failed to prune managedFields: %v", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	scrubbed, err := h.scrubber.ScrubEventData(gvk.Kind, jsonData)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to scrub object JSON: %w", err)
	}

	return json.RawMessage(scrubbed), dataSize, nil
}
```

- [ ] **Step 4: Wire the watcher scrubber in the server startup path**

```go
// cmd/spectre/commands/server.go
eventScrubber := scrub.New(cfg.ScrubSensitiveData)

if watcherEnabled {
	watcherComponent, err = watcher.New(
		watcher.NewEventCaptureHandler(storageComponent, eventScrubber),
		cfg.WatcherConfigPath,
	)
	if err != nil {
		logger.Error("Failed to create watcher component: %v", err)
		HandleError(err, "Watcher initialization error")
	}
}
```

- [ ] **Step 5: Run watcher and command tests to verify they pass**

Run: `go test ./internal/watcher ./cmd/spectre/commands -run 'TestEventCaptureHandlerOnAdd|TestServerScrubSensitiveDataFlagDefaultsToFalse' -count=1`
Expected: `ok   github.com/moolen/spectre/internal/watcher`
Expected: `ok   github.com/moolen/spectre/cmd/spectre/commands`

- [ ] **Step 6: Commit**

```bash
git add internal/watcher/event_handler.go internal/watcher/event_handler_test.go cmd/spectre/commands/server.go
git commit -m "feat: scrub watcher payloads before persistence"
```

### Task 4: Integrate Scrubbing Into CLI Import Ingestion

**Files:**
- Modify: `internal/importexport/json_import.go`
- Modify: `internal/importexport/json_import_test.go`
- Modify: `cmd/spectre/commands/server.go`

- [ ] **Step 1: Write the failing import scrubbing tests**

```go
func TestScrubEventsScrubsImportedConfigMapPayloads(t *testing.T) {
	events := []*models.Event{
		{
			ID:        "event-1",
			Timestamp: 1234567890000000000,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				Kind:      "ConfigMap",
				Namespace: "default",
				Name:      "cfg",
				UID:       "uid-1",
			},
			Data: json.RawMessage(`{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key"}}`),
		},
	}

	if err := ScrubEvents(events, scrub.New(true)); err != nil {
		t.Fatalf("ScrubEvents() error = %v", err)
	}

	if string(events[0].Data) == `{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key"}}` {
		t.Fatalf("expected imported payload to be scrubbed")
	}
}

func TestScrubEventsLeavesImportedPayloadUntouchedWhenDisabled(t *testing.T) {
	events := []*models.Event{
		{
			ID:        "event-1",
			Timestamp: 1234567890000000000,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				Kind:      "ConfigMap",
				Namespace: "default",
				Name:      "cfg",
				UID:       "uid-1",
			},
			Data: json.RawMessage(`{"kind":"ConfigMap","data":{"JWT_SECRET":"demo_jwt_secret_key"}}`),
		},
	}

	before := string(events[0].Data)
	if err := ScrubEvents(events, scrub.New(false)); err != nil {
		t.Fatalf("ScrubEvents() error = %v", err)
	}
	if string(events[0].Data) != before {
		t.Fatalf("expected payload to remain unchanged when scrubbing is disabled")
	}
}
```

- [ ] **Step 2: Run the import tests to verify they fail**

Run: `go test ./internal/importexport -run TestScrubEvents -count=1`
Expected: FAIL with `undefined: ScrubEvents`

- [ ] **Step 3: Add a reusable import scrubbing helper**

```go
// internal/importexport/json_import.go
func ScrubEvents(events []*models.Event, scrubber *scrub.Scrubber) error {
	if scrubber == nil || !scrubber.Enabled() {
		return nil
	}

	for _, event := range events {
		if event == nil || len(event.Data) == 0 {
			continue
		}

		scrubbed, err := scrubber.ScrubEventData(event.Resource.Kind, event.Data)
		if err != nil {
			return fmt.Errorf("scrub imported event %s: %w", event.ID, err)
		}
		event.Data = scrubbed
	}
	return nil
}
```

- [ ] **Step 4: Call the helper from both CLI import branches**

```go
// cmd/spectre/commands/server.go
eventScrubber := scrub.New(cfg.ScrubSensitiveData)

if info.IsDir() {
	report, err = importexport.WalkAndImportJSON(importPath, storageComponent, importOpts, progressCallback, eventScrubber)
	if err != nil {
		logger.Error("Import failed: %v", err)
		HandleError(err, "Import error")
	}
} else {
	events, err := importexport.ImportJSONFile(importPath)
	if err != nil {
		logger.Error("Failed to read file: %v", err)
		HandleError(err, "Import file error")
	}
	if err := importexport.ScrubEvents(events, eventScrubber); err != nil {
		logger.Error("Failed to scrub imported events: %v", err)
		HandleError(err, "Import scrub error")
	}
	storageReport, err := storageComponent.AddEventsBatch(events, importOpts)
	if err != nil {
		logger.Error("Import failed: %v", err)
		HandleError(err, "Import error")
	}
}
```

```go
// internal/importexport/json_import.go
func WalkAndImportJSON(
	dirPath string,
	st *storage.Storage,
	opts storage.ImportOptions,
	progress ProgressCallback,
	scrubber *scrub.Scrubber,
) (*ImportReport, error) {
	if err := ScrubEvents(allEvents, scrubber); err != nil {
		return nil, fmt.Errorf("failed to scrub imported events: %w", err)
	}
	storageReport, err := st.AddEventsBatch(allEvents, opts)
	if err != nil {
		return nil, fmt.Errorf("batch import failed: %w", err)
	}

	return &ImportReport{
		TotalFiles:    filesProcessed,
		ImportedFiles: storageReport.ImportedFiles,
		MergedHours:   storageReport.MergedHours,
		SkippedFiles:  storageReport.SkippedFiles,
		FailedFiles:   storageReport.FailedFiles,
		TotalEvents:   storageReport.TotalEvents,
		Errors:        storageReport.Errors,
		Duration:      storageReport.Duration,
	}, nil
}
```

```go
// internal/importexport/json_import_test.go
report, err := WalkAndImportJSON(tmpDir, st, opts, progressCallback, scrub.New(false))
if err != nil {
	t.Fatalf("WalkAndImportJSON() error = %v", err)
}
if report.TotalEvents == 0 {
	t.Fatalf("expected imported events")
}
```

- [ ] **Step 5: Run import tests to verify they pass**

Run: `go test ./internal/importexport -run 'TestScrubEvents|TestWalkAndImportJSON|TestParseJSONEvents' -count=1`
Expected: `ok   github.com/moolen/spectre/internal/importexport`

- [ ] **Step 6: Commit**

```bash
git add internal/importexport/json_import.go internal/importexport/json_import_test.go cmd/spectre/commands/server.go
git commit -m "feat: scrub imported event payloads before batch ingest"
```

### Task 5: Update Docs And Run End-To-End Verification

**Files:**
- Modify: `docs/docs/configuration/storage-settings.md`

- [ ] **Step 1: Add operator docs for the new flag**

Add this section to `docs/docs/configuration/storage-settings.md`:

```text
## Sensitive Data Scrubbing

**Flag:** `--scrub-sensitive-data`
**Type:** Boolean
**Default:** `false`

**Purpose:** Scrub sensitive values before Spectre writes resource payloads to storage.

**What gets scrubbed:**
- `Secret.data`
- `Secret.stringData`
- `ConfigMap.data`
- `ConfigMap.binaryData`
- explicit container `env[].value`
- `kubectl.kubernetes.io/last-applied-configuration`

**Example command:** `spectre server --data-dir=./data --scrub-sensitive-data=true`

When enabled, newly ingested watcher events and startup imports persist scrubbed values. Existing historical data is unchanged.
```

- [ ] **Step 2: Run the focused test suites**

Run: `go test ./internal/scrub ./internal/watcher ./internal/importexport ./cmd/spectre/commands -count=1`
Expected: all four packages report `ok`

- [ ] **Step 3: Run a broader regression pass for the touched subsystems**

Run: `go test ./internal/storage ./internal/api -count=1`
Expected: `ok   github.com/moolen/spectre/internal/storage`
Expected: `ok   github.com/moolen/spectre/internal/api`

- [ ] **Step 4: Inspect the final diff**

Run: `git diff --stat HEAD~5..HEAD`
Expected: diff includes only the scrubber package, watcher/import wiring, CLI/config wiring, tests, and the storage settings doc update

- [ ] **Step 5: Commit**

```bash
git add docs/docs/configuration/storage-settings.md
git commit -m "docs: document sensitive data scrubbing flag"
```

## Self-Review

### Spec coverage

- CLI boolean flag: covered by Task 1.
- Shared ingest-time scrubber package: covered by Task 2.
- Watcher ingestion before storage: covered by Task 3.
- Startup JSON import scrubbing: covered by Task 4.
- Masking semantics, base64 handling, and annotation recursion: covered by Task 2 tests and implementation.
- Operator documentation and verification: covered by Task 5.

### Placeholder scan

- No `TBD`, `TODO`, or deferred implementation notes remain in this plan.
- Each task includes exact files, concrete code, commands, and expected outcomes.

### Type consistency

- `scrub.New(bool)` and `(*Scrubber).ScrubEventData(string, json.RawMessage)` are defined in Task 2 and used consistently in Tasks 3 and 4.
- `importexport.ScrubEvents` is introduced in Task 4 before later verification steps reference import scrubbing.
- `Config.ScrubSensitiveData` is introduced in Task 1 before `server.go` wiring uses it in later tasks.
