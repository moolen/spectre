package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func TestWriteIntegrationsFile_Success(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "writer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetPath := filepath.Join(tmpDir, "integrations.yaml")

	// Create test config
	config := &IntegrationsFile{
		SchemaVersion: "v1",
		Instances: []IntegrationConfig{
			{
				Name:    "test-instance",
				Type:    "victorialogs",
				Enabled: true,
				Config: map[string]interface{}{
					"url": "http://localhost:9428",
				},
			},
		},
	}

	// Write config
	if err := WriteIntegrationsFile(targetPath, config); err != nil {
		t.Fatalf("WriteIntegrationsFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Fatalf("Target file was not created")
	}

	// Read back and verify contents
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}

	// Verify schema_version is present
	content := string(data)
	if len(content) == 0 {
		t.Fatalf("Written file is empty")
	}

	// Basic validation that YAML contains expected fields
	if !contains(content, "schema_version") {
		t.Errorf("Expected schema_version in output, got: %s", content)
	}
	if !contains(content, "instances") {
		t.Errorf("Expected instances in output, got: %s", content)
	}
	if !contains(content, "test-instance") {
		t.Errorf("Expected test-instance in output, got: %s", content)
	}
}

func TestWriteIntegrationsFile_InvalidPath(t *testing.T) {
	// Test with invalid path (directory doesn't exist)
	invalidPath := "/nonexistent/directory/integrations.yaml"

	config := &IntegrationsFile{
		SchemaVersion: "v1",
		Instances: []IntegrationConfig{
			{
				Name:    "test",
				Type:    "test",
				Enabled: true,
				Config: map[string]interface{}{
					"url": "http://localhost:9428",
				},
			},
		},
	}

	// Write should fail
	err := WriteIntegrationsFile(invalidPath, config)
	if err == nil {
		t.Fatalf("Expected error when writing to invalid path, got nil")
	}
}

func TestWriteIntegrationsFile_ReadBack(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "writer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetPath := filepath.Join(tmpDir, "integrations.yaml")

	// Create test config with multiple instances
	originalConfig := &IntegrationsFile{
		SchemaVersion: "v1",
		Instances: []IntegrationConfig{
			{
				Name:    "victorialogs-prod",
				Type:    "victorialogs",
				Enabled: true,
				Config: map[string]interface{}{
					"url": "http://prod.example.com:9428",
				},
			},
			{
				Name:    "victorialogs-staging",
				Type:    "victorialogs",
				Enabled: false,
				Config: map[string]interface{}{
					"url": "http://staging.example.com:9428",
				},
			},
		},
	}

	// Write config
	if err := WriteIntegrationsFile(targetPath, originalConfig); err != nil {
		t.Fatalf("WriteIntegrationsFile failed: %v", err)
	}

	// Load using Koanf (same loader as Phase 1)
	k := koanf.New(".")
	if err := k.Load(file.Provider(targetPath), yaml.Parser()); err != nil {
		t.Fatalf("Failed to load with Koanf: %v", err)
	}

	var loadedConfig IntegrationsFile
	if err := k.UnmarshalWithConf("", &loadedConfig, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		t.Fatalf("Failed to unmarshal with Koanf: %v", err)
	}

	// Verify round-trip
	if loadedConfig.SchemaVersion != originalConfig.SchemaVersion {
		t.Errorf("SchemaVersion mismatch: got %q, want %q", loadedConfig.SchemaVersion, originalConfig.SchemaVersion)
	}

	if len(loadedConfig.Instances) != len(originalConfig.Instances) {
		t.Fatalf("Instance count mismatch: got %d, want %d", len(loadedConfig.Instances), len(originalConfig.Instances))
	}

	// Verify first instance
	inst1 := loadedConfig.Instances[0]
	if inst1.Name != "victorialogs-prod" {
		t.Errorf("Instance 0 name mismatch: got %q, want %q", inst1.Name, "victorialogs-prod")
	}
	if inst1.Type != "victorialogs" {
		t.Errorf("Instance 0 type mismatch: got %q, want %q", inst1.Type, "victorialogs")
	}
	if !inst1.Enabled {
		t.Errorf("Instance 0 should be enabled")
	}
	if url, ok := inst1.Config["url"].(string); !ok || url != "http://prod.example.com:9428" {
		t.Errorf("Instance 0 URL mismatch: got %v", inst1.Config["url"])
	}

	// Verify second instance
	inst2 := loadedConfig.Instances[1]
	if inst2.Name != "victorialogs-staging" {
		t.Errorf("Instance 1 name mismatch: got %q, want %q", inst2.Name, "victorialogs-staging")
	}
	if inst2.Enabled {
		t.Errorf("Instance 1 should be disabled")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
