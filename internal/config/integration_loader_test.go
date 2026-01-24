package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadIntegrationsFile_Valid(t *testing.T) {
	// Create temporary test file with valid config
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "valid.yaml")

	content := `schema_version: v1
instances:
  - name: test-instance
    type: test
    enabled: true
    config:
      url: "http://localhost:9428"
      timeout: 30
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load and verify
	cfg, err := LoadIntegrationsFile(tmpFile)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify schema version
	assert.Equal(t, "v1", cfg.SchemaVersion)

	// Verify instances
	require.Len(t, cfg.Instances, 1)
	instance := cfg.Instances[0]
	assert.Equal(t, "test-instance", instance.Name)
	assert.Equal(t, "test", instance.Type)
	assert.True(t, instance.Enabled)
	assert.Equal(t, "http://localhost:9428", instance.Config["url"])
}

func TestLoadIntegrationsFile_MultipleInstances(t *testing.T) {
	// Create temporary test file with multiple instances
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "multiple.yaml")

	content := `schema_version: v1
instances:
  - name: instance-a
    type: typeA
    enabled: true
    config:
      setting: "value-a"
  - name: instance-b
    type: typeB
    enabled: false
    config:
      setting: "value-b"
  - name: instance-c
    type: typeA
    enabled: true
    config:
      setting: "value-c"
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load and verify
	cfg, err := LoadIntegrationsFile(tmpFile)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify instances count
	require.Len(t, cfg.Instances, 3)

	// Verify each instance
	assert.Equal(t, "instance-a", cfg.Instances[0].Name)
	assert.Equal(t, "typeA", cfg.Instances[0].Type)
	assert.True(t, cfg.Instances[0].Enabled)

	assert.Equal(t, "instance-b", cfg.Instances[1].Name)
	assert.Equal(t, "typeB", cfg.Instances[1].Type)
	assert.False(t, cfg.Instances[1].Enabled)

	assert.Equal(t, "instance-c", cfg.Instances[2].Name)
	assert.Equal(t, "typeA", cfg.Instances[2].Type)
	assert.True(t, cfg.Instances[2].Enabled)
}

func TestLoadIntegrationsFile_InvalidSchemaVersion(t *testing.T) {
	// Create temporary test file with invalid schema version
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid-schema.yaml")

	content := `schema_version: v2
instances:
  - name: test-instance
    type: test
    enabled: true
    config:
      url: "http://localhost:9428"
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load and expect validation error
	cfg, err := LoadIntegrationsFile(tmpFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "schema_version")
}

func TestLoadIntegrationsFile_FileNotFound(t *testing.T) {
	// Try to load non-existent file
	cfg, err := LoadIntegrationsFile("/nonexistent/path/to/file.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to load")
}

func TestLoadIntegrationsFile_InvalidYAML(t *testing.T) {
	// Create temporary test file with invalid YAML syntax
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid-yaml.yaml")

	content := `schema_version: v1
instances:
  - name: test-instance
    type: test
    enabled: true
    config:
      url: "http://localhost:9428
      # Missing closing quote above causes syntax error
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load and expect parsing error
	cfg, err := LoadIntegrationsFile(tmpFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to")
}

func TestLoadIntegrationsFile_DuplicateInstanceNames(t *testing.T) {
	// Create temporary test file with duplicate instance names
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "duplicate-names.yaml")

	content := `schema_version: v1
instances:
  - name: duplicate
    type: typeA
    enabled: true
    config:
      setting: "value-a"
  - name: duplicate
    type: typeB
    enabled: true
    config:
      setting: "value-b"
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load and expect validation error
	cfg, err := LoadIntegrationsFile(tmpFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoadIntegrationsFile_MissingRequiredFields(t *testing.T) {
	// Create temporary test file with missing required fields
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "missing-fields.yaml")

	content := `schema_version: v1
instances:
  - name: ""
    type: test
    enabled: true
    config:
      url: "http://localhost:9428"
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load and expect validation error
	cfg, err := LoadIntegrationsFile(tmpFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "validation failed")
}
