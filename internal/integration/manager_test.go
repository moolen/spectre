package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// managerMockIntegration is a test implementation of the Integration interface
// with additional tracking for manager tests
type managerMockIntegration struct {
	name       string
	version    string
	intType    string
	startErr   error
	stopErr    error
	health     HealthStatus
	startCalls int
	stopCalls  int
}

func (m *managerMockIntegration) Metadata() IntegrationMetadata {
	return IntegrationMetadata{
		Name:        m.name,
		Version:     m.version,
		Type:        m.intType,
		Description: "Mock integration for testing",
	}
}

func (m *managerMockIntegration) Start(ctx context.Context) error {
	m.startCalls++
	return m.startErr
}

func (m *managerMockIntegration) Stop(ctx context.Context) error {
	m.stopCalls++
	return m.stopErr
}

func (m *managerMockIntegration) Health(ctx context.Context) HealthStatus {
	return m.health
}

func (m *managerMockIntegration) RegisterTools(registry ToolRegistry) error {
	return nil
}

// createTestConfigFile creates a temporary YAML config file for testing
func createTestConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "integrations.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	return configPath
}

func TestManagerVersionValidation(t *testing.T) {
	// Register mock factory that returns old version
	RegisterFactory("mock", func(name string, config map[string]interface{}) (Integration, error) {
		return &managerMockIntegration{
			name:    name,
			version: "0.9.0", // Below minimum
			intType: "mock",
			health:  Healthy,
		}, nil
	})
	defer func() {
		// Clear factory for other tests
		defaultRegistry = NewFactoryRegistry()
	}()

	configContent := `schema_version: v1
instances:
  - name: test-instance
    type: mock
    enabled: true
    config: {}`

	configPath := createTestConfigFile(t, configContent)

	// Create manager with minimum version requirement
	mgr, err := NewManager(ManagerConfig{
		ConfigPath:            configPath,
		MinIntegrationVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Start should fail due to version mismatch
	ctx := context.Background()
	err = mgr.Start(ctx)
	if err == nil {
		t.Fatal("Expected version validation error, got nil")
	}

	// Check error message contains version information
	expectedMsg := "below minimum required version"
	if err.Error() == "" || !containsStr(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got: %v", expectedMsg, err)
	}
}

func TestManagerStartLoadsInstances(t *testing.T) {
	// Register mock factory
	RegisterFactory("mock", func(name string, config map[string]interface{}) (Integration, error) {
		return &managerMockIntegration{
			name:    name,
			version: "1.0.0",
			intType: "mock",
			health:  Healthy,
		}, nil
	})
	defer func() {
		defaultRegistry = NewFactoryRegistry()
	}()

	configContent := `schema_version: v1
instances:
  - name: instance-1
    type: mock
    enabled: true
    config: {}
  - name: instance-2
    type: mock
    enabled: true
    config: {}`

	configPath := createTestConfigFile(t, configContent)

	mgr, err := NewManager(ManagerConfig{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer mgr.Stop(ctx)

	// Verify both instances are in registry
	instances := mgr.GetRegistry().List()
	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instances))
	}

	// Verify instance names
	if !contains(instances, "instance-1") || !contains(instances, "instance-2") {
		t.Errorf("Expected instances [instance-1, instance-2], got %v", instances)
	}
}

func TestManagerFailedInstanceDegraded(t *testing.T) {
	// Track which instances were created
	createdInstances := make(map[string]*managerMockIntegration)

	RegisterFactory("mock", func(name string, config map[string]interface{}) (Integration, error) {
		mock := &managerMockIntegration{
			name:    name,
			version: "1.0.0",
			intType: "mock",
			health:  Healthy,
		}
		// Make instance-2 fail on start
		if name == "instance-2" {
			mock.startErr = fmt.Errorf("connection failed")
		}
		createdInstances[name] = mock
		return mock, nil
	})
	defer func() {
		defaultRegistry = NewFactoryRegistry()
	}()

	configContent := `schema_version: v1
instances:
  - name: instance-1
    type: mock
    enabled: true
    config: {}
  - name: instance-2
    type: mock
    enabled: true
    config: {}`

	configPath := createTestConfigFile(t, configContent)

	mgr, err := NewManager(ManagerConfig{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	// Start should succeed even though instance-2 fails
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Manager should continue despite instance failure: %v", err)
	}
	defer mgr.Stop(ctx)

	// Verify both instances are registered (degraded instance stays registered)
	instances := mgr.GetRegistry().List()
	if len(instances) != 2 {
		t.Errorf("Expected 2 instances (including degraded), got %d", len(instances))
	}

	// Verify instance-1 started successfully
	if createdInstances["instance-1"].startCalls != 1 {
		t.Errorf("Expected instance-1 to start once, got %d calls", createdInstances["instance-1"].startCalls)
	}

	// Verify instance-2 attempted to start
	if createdInstances["instance-2"].startCalls != 1 {
		t.Errorf("Expected instance-2 to attempt start, got %d calls", createdInstances["instance-2"].startCalls)
	}
}

func TestManagerConfigReload(t *testing.T) {
	createdInstances := make(map[string]*managerMockIntegration)

	RegisterFactory("mock", func(name string, config map[string]interface{}) (Integration, error) {
		mock := &managerMockIntegration{
			name:    name,
			version: "1.0.0",
			intType: "mock",
			health:  Healthy,
		}
		createdInstances[name] = mock
		return mock, nil
	})
	defer func() {
		defaultRegistry = NewFactoryRegistry()
	}()

	configContent1 := `schema_version: v1
instances:
  - name: instance-1
    type: mock
    enabled: true
    config: {}`

	configPath := createTestConfigFile(t, configContent1)

	mgr, err := NewManager(ManagerConfig{
		ConfigPath:          configPath,
		HealthCheckInterval: 1 * time.Hour, // Disable health checks for this test
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer mgr.Stop(ctx)

	// Verify initial instance
	instances := mgr.GetRegistry().List()
	if len(instances) != 1 || instances[0] != "instance-1" {
		t.Fatalf("Expected [instance-1], got %v", instances)
	}

	// Update config file with different instance
	configContent2 := `schema_version: v1
instances:
  - name: instance-2
    type: mock
    enabled: true
    config: {}`

	if err := os.WriteFile(configPath, []byte(configContent2), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for file watcher to detect change and reload (debounce is 500ms)
	time.Sleep(1500 * time.Millisecond)

	// Verify new instance loaded
	instances = mgr.GetRegistry().List()
	if len(instances) != 1 || instances[0] != "instance-2" {
		t.Errorf("Expected [instance-2] after reload, got %v", instances)
	}

	// Verify instance-1 was stopped during reload
	if createdInstances["instance-1"].stopCalls < 1 {
		t.Errorf("Expected instance-1 to be stopped at least once, got %d calls", createdInstances["instance-1"].stopCalls)
	}
}

func TestManagerHealthCheckRecovery(t *testing.T) {
	mock := &managerMockIntegration{
		name:    "test-instance",
		version: "1.0.0",
		intType: "mock",
		health:  Degraded, // Start as degraded
	}

	RegisterFactory("mock", func(name string, config map[string]interface{}) (Integration, error) {
		return mock, nil
	})
	defer func() {
		defaultRegistry = NewFactoryRegistry()
	}()

	configContent := `schema_version: v1
instances:
  - name: test-instance
    type: mock
    enabled: true
    config: {}`

	configPath := createTestConfigFile(t, configContent)

	mgr, err := NewManager(ManagerConfig{
		ConfigPath:          configPath,
		HealthCheckInterval: 100 * time.Millisecond, // Fast health checks for testing
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer mgr.Stop(ctx)

	// Initial start call
	initialStartCalls := mock.startCalls

	// Wait for health check cycle to run
	time.Sleep(300 * time.Millisecond)

	// Verify Start was called again for recovery attempt
	if mock.startCalls <= initialStartCalls {
		t.Errorf("Expected health check to attempt recovery (Start called again), got %d total calls", mock.startCalls)
	}
}

func TestManagerGracefulShutdown(t *testing.T) {
	mock := &managerMockIntegration{
		name:    "test-instance",
		version: "1.0.0",
		intType: "mock",
		health:  Healthy,
	}

	RegisterFactory("mock", func(name string, config map[string]interface{}) (Integration, error) {
		return mock, nil
	})
	defer func() {
		defaultRegistry = NewFactoryRegistry()
	}()

	configContent := `schema_version: v1
instances:
  - name: test-instance
    type: mock
    enabled: true
    config: {}`

	configPath := createTestConfigFile(t, configContent)

	mgr, err := NewManager(ManagerConfig{
		ConfigPath:          configPath,
		ShutdownTimeout:     5 * time.Second,
		HealthCheckInterval: 1 * time.Hour, // Disable health checks
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Stop manager
	if err := mgr.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	// Verify instance was stopped at least once (may be stopped during watcher callback + manager.Stop)
	if mock.stopCalls < 1 {
		t.Errorf("Expected instance to be stopped at least once, got %d calls", mock.stopCalls)
	}
}

// Helper function to check if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// Helper function to check if a string contains a substring
func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
