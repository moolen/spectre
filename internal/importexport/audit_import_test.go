package importexport

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

func TestParseImportPayload_AuditSingleEvent(t *testing.T) {
	input := []byte(`{
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
	input := []byte(`{
		"kind": "EventList",
		"apiVersion": "audit.k8s.io/v1",
		"items": [
			{
				"kind": "Event",
				"apiVersion": "audit.k8s.io/v1",
				"auditID": "audit-2",
				"stage": "ResponseComplete",
				"verb": "update",
				"requestURI": "/apis/apps/v1/namespaces/default/deployments/web",
				"objectRef": {
					"resource": "deployments",
					"namespace": "default",
					"name": "web",
					"uid": "deploy-web-uid",
					"apiGroup": "apps",
					"apiVersion": "v1"
				},
				"responseObject": {
					"apiVersion": "apps/v1",
					"kind": "Deployment",
					"metadata": {
						"name": "web",
						"namespace": "default",
						"uid": "deploy-web-uid"
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
	data := `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-get","stage":"ResponseComplete","verb":"get","requestURI":"/api/v1/namespaces/default/pods/p1","objectRef":{"resource":"pods","namespace":"default","name":"p1","uid":"pod-1-uid","apiVersion":"v1"}}
{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-patch","stage":"ResponseComplete","verb":"patch","requestURI":"/api/v1/namespaces/default/pods/p1","objectRef":{"resource":"pods","namespace":"default","name":"p1","uid":"pod-1-uid","apiVersion":"v1"},"responseObject":{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p1","namespace":"default","uid":"pod-1-uid"}}}`
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

	if events[0].Type != models.EventTypeUpdate {
		t.Fatalf("ImportJSONFile() event type = %q, want %q", events[0].Type, models.EventTypeUpdate)
	}
}

func TestParseImportPayload_AuditMissingObjectPayloadWarns(t *testing.T) {
	input := []byte(`{
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

func TestWalkAndImportJSON_MixedSpectreAndAuditFiles(t *testing.T) {
	tmpDir := t.TempDir()

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
				},
				"data": {
					"apiVersion": "apps/v1",
					"kind": "Deployment",
					"metadata": {
						"name": "s1",
						"namespace": "default",
						"uid": "s1-uid"
					}
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

	auditLogPath := filepath.Join(tmpDir, "audit.log")
	auditLog := `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"audit-dir-log","stage":"ResponseComplete","verb":"patch","requestURI":"/apis/apps/v1/namespaces/default/deployments/d1","objectRef":{"resource":"deployments","namespace":"default","name":"d1","uid":"d1-uid","apiGroup":"apps","apiVersion":"v1"},"responseObject":{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d1","namespace":"default","uid":"d1-uid"}}}`
	if err := os.WriteFile(auditLogPath, []byte(auditLog), 0644); err != nil {
		t.Fatalf("Failed to create audit log file: %v", err)
	}

	storageDir := t.TempDir()
	st, err := storage.New(storageDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	opts := storage.ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}

	report, err := WalkAndImportJSON(tmpDir, st, opts, nil)
	if err != nil {
		t.Fatalf("WalkAndImportJSON() error = %v", err)
	}

	if report.TotalFiles != 3 {
		t.Fatalf("WalkAndImportJSON() total files = %d, want 3", report.TotalFiles)
	}

	if report.TotalEvents != 3 {
		t.Fatalf("WalkAndImportJSON() total events = %d, want 3", report.TotalEvents)
	}
}
