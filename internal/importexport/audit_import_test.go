package importexport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func TestParseImportPayload_AuditSingleEvent(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-1",
		"stage": "ResponseComplete",
		"stageTimestamp": "2024-01-02T03:04:05Z",
		"verb": "create",
		"requestURI": "/api/v1/namespaces/default/configmaps/cm-one",
		"objectRef": {
			"resource": "configmaps",
			"namespace": "default",
			"name": "cm-one",
			"uid": "cm-one-uid",
			"apiVersion": "v1"
		},
		"responseObject": {
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {
				"name": "cm-one",
				"namespace": "default",
				"uid": "cm-one-uid"
			}
		}
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() unexpected error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseImportPayload() got %d events, want 1", len(events))
	}

	if events[0].Type != models.EventTypeCreate {
		t.Fatalf("ParseImportPayload() event type = %q, want %q", events[0].Type, models.EventTypeCreate)
	}

	if events[0].Timestamp != 1704164645000000000 {
		t.Fatalf("ParseImportPayload() event timestamp = %d, want %d", events[0].Timestamp, int64(1704164645000000000))
	}

	if events[0].Resource.Namespace != "default" {
		t.Fatalf("ParseImportPayload() resource namespace = %q, want %q", events[0].Resource.Namespace, "default")
	}

	if events[0].Resource.Group != "" {
		t.Fatalf("ParseImportPayload() resource group = %q, want %q", events[0].Resource.Group, "")
	}

	if events[0].Resource.Version != "v1" {
		t.Fatalf("ParseImportPayload() resource version = %q, want %q", events[0].Resource.Version, "v1")
	}

	if events[0].Resource.Kind != "ConfigMap" {
		t.Fatalf("ParseImportPayload() resource kind = %q, want %q", events[0].Resource.Kind, "ConfigMap")
	}

	if events[0].Resource.Name != "cm-one" {
		t.Fatalf("ParseImportPayload() resource name = %q, want %q", events[0].Resource.Name, "cm-one")
	}

	if events[0].Resource.UID != "cm-one-uid" {
		t.Fatalf("ParseImportPayload() resource UID = %q, want %q", events[0].Resource.UID, "cm-one-uid")
	}

	if len(events[0].Data) == 0 {
		t.Fatalf("ParseImportPayload() event data is empty, expected responseObject content")
	}

	if !strings.Contains(string(events[0].Data), "ConfigMap") || !strings.Contains(string(events[0].Data), "cm-one") {
		t.Fatalf("ParseImportPayload() event data does not contain expected responseObject content: %s", string(events[0].Data))
	}

	if len(warnings) != 0 {
		t.Fatalf("ParseImportPayload() got %d warnings, want 0", len(warnings))
	}
}

func TestParseImportPayload_AuditEventList(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "EventList",
		"apiVersion": "audit.k8s.io/v1",
		"items": [
			{
				"kind": "Event",
				"apiVersion": "audit.k8s.io/v1",
				"auditID": "audit-2",
				"stage": "ResponseComplete",
				"stageTimestamp": "2024-01-02T03:04:06Z",
				"verb": "patch",
				"requestURI": "/api/v1/namespaces/default/configmaps/cm-from-request",
				"objectRef": {
					"resource": "configmaps",
					"namespace": "default",
					"name": "cm-from-request",
					"uid": "cm-request-uid",
					"apiVersion": "v1"
				},
				"requestObject": {
					"apiVersion": "v1",
					"kind": "ConfigMap",
					"metadata": {
						"name": "cm-from-request",
						"namespace": "default",
						"uid": "cm-request-uid"
					}
				}
			}
		]
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() unexpected error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseImportPayload() got %d events, want 1", len(events))
	}

	if events[0].Type != models.EventTypeUpdate {
		t.Fatalf("ParseImportPayload() event type = %q, want %q", events[0].Type, models.EventTypeUpdate)
	}

	if events[0].Timestamp != 1704164646000000000 {
		t.Fatalf("ParseImportPayload() event timestamp = %d, want %d", events[0].Timestamp, int64(1704164646000000000))
	}

	if events[0].Resource.Namespace != "default" {
		t.Fatalf("ParseImportPayload() resource namespace = %q, want %q", events[0].Resource.Namespace, "default")
	}

	if events[0].Resource.Group != "" {
		t.Fatalf("ParseImportPayload() resource group = %q, want %q", events[0].Resource.Group, "")
	}

	if events[0].Resource.Version != "v1" {
		t.Fatalf("ParseImportPayload() resource version = %q, want %q", events[0].Resource.Version, "v1")
	}

	if events[0].Resource.Kind != "ConfigMap" {
		t.Fatalf("ParseImportPayload() resource kind = %q, want %q", events[0].Resource.Kind, "ConfigMap")
	}

	if events[0].Resource.Name != "cm-from-request" {
		t.Fatalf("ParseImportPayload() resource name = %q, want %q", events[0].Resource.Name, "cm-from-request")
	}

	if events[0].Resource.UID != "cm-request-uid" {
		t.Fatalf("ParseImportPayload() resource UID = %q, want %q", events[0].Resource.UID, "cm-request-uid")
	}

	if len(events[0].Data) == 0 {
		t.Fatalf("ParseImportPayload() event data is empty, expected requestObject content")
	}

	if !strings.Contains(string(events[0].Data), "ConfigMap") || !strings.Contains(string(events[0].Data), "cm-from-request") {
		t.Fatalf("ParseImportPayload() event data does not contain expected requestObject content: %s", string(events[0].Data))
	}

	if len(warnings) != 0 {
		t.Fatalf("ParseImportPayload() got %d warnings, want 0", len(warnings))
	}
}

func TestImportJSONFile_AuditJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.log")
	data := `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-delete","stage":"ResponseComplete","stageTimestamp":"2024-01-02T03:04:08Z","verb":"delete","requestURI":"/api/v1/namespaces/default/pods/p1","objectRef":{"resource":"pods","namespace":"default","name":"p1","uid":"pod-1-uid","apiVersion":"v1"}}
{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-get","stage":"ResponseComplete","stageTimestamp":"2024-01-02T03:04:09Z","verb":"get","requestURI":"/api/v1/namespaces/default/pods/p1","objectRef":{"resource":"pods","namespace":"default","name":"p1","uid":"pod-1-uid","apiVersion":"v1"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	events, err := Import(FromFile(path), WithLogger(logging.GetLogger("test")))
	if err != nil {
		t.Fatalf("Import() unexpected error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Import() got %d events, want 1", len(events))
	}

	if events[0].Type != models.EventTypeDelete {
		t.Fatalf("Import() event type = %q, want %q", events[0].Type, models.EventTypeDelete)
	}
}

func TestImportFromDirectory_MixedSpectreAndAuditFiles(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	spectrePath := filepath.Join(tmpDir, "spectre.json")
	spectreEnvelope := `{
		"events": [
			{
				"id": "event-s-1",
				"timestamp": 1234567890000000000,
				"type": "CREATE",
				"resource": {
					"group": "apps",
					"version": "v1",
					"kind": "Deployment",
					"namespace": "default",
					"name": "s1",
					"uid": "s1-uid"
				}
			}
		]
	}`
	if err := os.WriteFile(spectrePath, []byte(spectreEnvelope), 0644); err != nil {
		t.Fatalf("Failed to create spectre file: %v", err)
	}

	auditJSONPath := filepath.Join(tmpDir, "audit.json")
	auditJSON := `{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-dir-json",
		"stage": "ResponseComplete",
		"stageTimestamp": "2024-01-02T03:04:10Z",
		"verb": "create",
		"requestURI": "/api/v1/namespaces/default/configmaps/cm-dir",
		"objectRef": {
			"resource": "configmaps",
			"namespace": "default",
			"name": "cm-dir",
			"uid": "cm-dir-uid",
			"apiVersion": "v1"
		},
		"responseObject": {
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {
				"name": "cm-dir",
				"namespace": "default",
				"uid": "cm-dir-uid"
			}
		}
	}`
	if err := os.WriteFile(auditJSONPath, []byte(auditJSON), 0644); err != nil {
		t.Fatalf("Failed to create audit JSON file: %v", err)
	}

	auditLogPath := filepath.Join(nestedDir, "audit.log")
	auditLog := `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-dir-log","stage":"ResponseComplete","stageTimestamp":"2024-01-02T03:04:11Z","verb":"patch","requestURI":"/apis/apps/v1/namespaces/default/deployments/d1","objectRef":{"resource":"deployments","namespace":"default","name":"d1","uid":"d1-uid","apiGroup":"apps","apiVersion":"v1"},"responseObject":{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d1","namespace":"default","uid":"d1-uid"}}}`
	if err := os.WriteFile(auditLogPath, []byte(auditLog), 0644); err != nil {
		t.Fatalf("Failed to create audit log file: %v", err)
	}

	events, err := Import(FromDirectory(tmpDir), WithLogger(logging.GetLogger("test")))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Import() got %d events, want 3", len(events))
	}
}

func TestParseImportPayload_AuditMissingObjectPayloadWarns(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-3",
		"stage": "ResponseComplete",
		"stageTimestamp": "2024-01-02T03:04:07Z",
		"verb": "create",
		"requestURI": "/api/v1/namespaces/default/configmaps/no-payload",
		"objectRef": {
			"resource": "configmaps",
			"namespace": "default",
			"name": "no-payload",
			"uid": "no-payload-uid",
			"apiVersion": "v1"
		}
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() unexpected error = %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("ParseImportPayload() got %d events, want 0", len(events))
	}

	if len(warnings) < 1 {
		t.Fatalf("ParseImportPayload() got %d warnings, want at least 1", len(warnings))
	}

	hasIdentifyingWarning := false
	for _, warning := range warnings {
		if strings.Contains(warning, "no-payload") {
			hasIdentifyingWarning = true
			break
		}
	}
	if !hasIdentifyingWarning {
		t.Fatalf("ParseImportPayload() warnings = %v, want at least one warning containing %q", warnings, "no-payload")
	}
}

func TestParseImportPayload_NonAuditEventObjectDoesNotUseAuditPath(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "v1"
	}`)

	_, _, err := ParseImportPayload(input)
	if err == nil {
		t.Fatalf("ParseImportPayload() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "events array is empty") {
		t.Fatalf("ParseImportPayload() error = %v, want error containing %q", err, "events array is empty")
	}
}

func TestParseImportPayload_AuditInvalidTimestampWarnsAndSkips(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-invalid-ts",
		"stage": "ResponseComplete",
		"stageTimestamp": "not-a-timestamp",
		"verb": "create",
		"requestURI": "/api/v1/namespaces/default/configmaps/cm-invalid-ts",
		"objectRef": {
			"resource": "configmaps",
			"namespace": "default",
			"name": "cm-invalid-ts",
			"uid": "cm-invalid-ts-uid",
			"apiVersion": "v1"
		},
		"responseObject": {
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {
				"name": "cm-invalid-ts",
				"namespace": "default",
				"uid": "cm-invalid-ts-uid"
			}
		}
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() unexpected error = %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("ParseImportPayload() got %d events, want 0", len(events))
	}

	if len(warnings) < 1 {
		t.Fatalf("ParseImportPayload() got %d warnings, want at least 1", len(warnings))
	}

	hasTimestampWarning := false
	for _, warning := range warnings {
		if strings.Contains(warning, "invalid stageTimestamp") || strings.Contains(warning, "stageTimestamp/requestReceivedTimestamp") {
			hasTimestampWarning = true
			break
		}
	}
	if !hasTimestampWarning {
		t.Fatalf("ParseImportPayload() warnings = %v, want timestamp warning", warnings)
	}
}

func TestParseImportPayload_AuditUsesPayloadUIDWhenObjectRefUIDMissing(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-payload-uid",
		"stage": "ResponseComplete",
		"stageTimestamp": "2024-01-02T03:04:09Z",
		"verb": "patch",
		"requestURI": "/api/v1/namespaces/default/configmaps/cm-payload-uid",
		"objectRef": {
			"resource": "configmaps",
			"namespace": "default",
			"name": "cm-payload-uid",
			"uid": "",
			"apiVersion": "v1"
		},
		"responseObject": {
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {
				"name": "cm-payload-uid",
				"namespace": "default",
				"uid": "uid-from-payload"
			}
		}
	}`)

	events, warnings, err := ParseImportPayload(input)
	if err != nil {
		t.Fatalf("ParseImportPayload() unexpected error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseImportPayload() got %d events, want 1", len(events))
	}
	if events[0].Resource.UID != "uid-from-payload" {
		t.Fatalf("ParseImportPayload() resource UID = %q, want %q", events[0].Resource.UID, "uid-from-payload")
	}
	if len(warnings) != 0 {
		t.Fatalf("ParseImportPayload() got %d warnings, want 0", len(warnings))
	}
}

func TestParseImportPayload_AuditEventListRejectsNonAuditItem(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "EventList",
		"apiVersion": "audit.k8s.io/v1",
		"items": [
			{
				"kind": "Event",
				"apiVersion": "v1",
				"verb": "create",
				"stage": "ResponseComplete"
			}
		]
	}`)

	_, _, err := ParseImportPayload(input)
	if err == nil {
		t.Fatalf("ParseImportPayload() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported audit EventList item") {
		t.Fatalf("ParseImportPayload() error = %v, want error containing %q", err, "unsupported audit EventList item")
	}
}
