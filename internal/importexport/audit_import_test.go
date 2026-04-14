package importexport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/models"
)

func TestParseImportPayload_AuditSingleEvent(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-1",
		"stage": "ResponseComplete",
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

	if len(warnings) != 0 {
		t.Fatalf("ParseImportPayload() got %d warnings, want 0", len(warnings))
	}
}

func TestImportJSONFile_AuditJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.log")
	data := `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-delete","stage":"ResponseComplete","verb":"delete","requestURI":"/api/v1/namespaces/default/pods/p1","objectRef":{"resource":"pods","namespace":"default","name":"p1","uid":"pod-1-uid","apiVersion":"v1"}}
{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-get","stage":"ResponseComplete","verb":"get","requestURI":"/api/v1/namespaces/default/pods/p1","objectRef":{"resource":"pods","namespace":"default","name":"p1","uid":"pod-1-uid","apiVersion":"v1"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	events, warnings, err := ImportJSONFile(path)
	if err != nil {
		t.Fatalf("ImportJSONFile() unexpected error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ImportJSONFile() got %d events, want 1", len(events))
	}

	if len(warnings) != 0 {
		t.Fatalf("ImportJSONFile() got %d warnings, want 0", len(warnings))
	}

	if events[0].Type != models.EventTypeDelete {
		t.Fatalf("ImportJSONFile() event type = %q, want %q", events[0].Type, models.EventTypeDelete)
	}
}

func TestParseImportPayload_AuditMissingObjectPayloadWarns(t *testing.T) {
	input := strings.NewReader(`{
		"kind": "Event",
		"apiVersion": "audit.k8s.io/v1",
		"auditID": "audit-3",
		"stage": "ResponseComplete",
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
}
