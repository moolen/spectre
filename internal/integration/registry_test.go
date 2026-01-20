package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockIntegration implements Integration for testing
type mockIntegration struct {
	name string
}

func (m *mockIntegration) Metadata() IntegrationMetadata {
	return IntegrationMetadata{
		Name:        m.name,
		Version:     "1.0.0",
		Description: "Mock integration for testing",
		Type:        "mock",
	}
}

func (m *mockIntegration) Start(ctx context.Context) error {
	return nil
}

func (m *mockIntegration) Stop(ctx context.Context) error {
	return nil
}

func (m *mockIntegration) Health(ctx context.Context) HealthStatus {
	return Healthy
}

func (m *mockIntegration) RegisterTools(registry ToolRegistry) error {
	return nil
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	// Register first instance - should succeed
	instance1 := &mockIntegration{name: "test-1"}
	err := r.Register("test-1", instance1)
	assert.NoError(t, err)

	// Register with empty name - should fail
	instance2 := &mockIntegration{name: ""}
	err = r.Register("", instance2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Register duplicate name - should fail
	instance3 := &mockIntegration{name: "test-1"}
	err = r.Register("test-1", instance3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	// Get non-existent instance - should return false
	_, exists := r.Get("nonexistent")
	assert.False(t, exists)

	// Register and retrieve instance - should succeed
	instance := &mockIntegration{name: "test-instance"}
	err := r.Register("test-instance", instance)
	assert.NoError(t, err)

	retrieved, exists := r.Get("test-instance")
	assert.True(t, exists)
	assert.Equal(t, instance, retrieved)
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	// Empty registry - should return empty slice
	names := r.List()
	assert.Empty(t, names)

	// Register multiple instances
	err := r.Register("instance-c", &mockIntegration{name: "instance-c"})
	assert.NoError(t, err)
	err = r.Register("instance-a", &mockIntegration{name: "instance-a"})
	assert.NoError(t, err)
	err = r.Register("instance-b", &mockIntegration{name: "instance-b"})
	assert.NoError(t, err)

	// List should return sorted names
	names = r.List()
	assert.Equal(t, []string{"instance-a", "instance-b", "instance-c"}, names)
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()

	// Remove non-existent instance - should return false
	removed := r.Remove("nonexistent")
	assert.False(t, removed)

	// Register instance
	instance := &mockIntegration{name: "test-instance"}
	err := r.Register("test-instance", instance)
	assert.NoError(t, err)

	// Remove existing instance - should return true
	removed = r.Remove("test-instance")
	assert.True(t, removed)

	// Verify instance is gone
	_, exists := r.Get("test-instance")
	assert.False(t, exists)

	// Remove again - should return false
	removed = r.Remove("test-instance")
	assert.False(t, removed)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Concurrent Register operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				name := fmt.Sprintf("instance-%d-%d", id, j)
				instance := &mockIntegration{name: name}
				_ = r.Register(name, instance)
			}
		}(i)
	}

	// Concurrent Get operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				name := fmt.Sprintf("instance-%d-%d", id, j)
				_, _ = r.Get(name)
			}
		}(i)
	}

	// Concurrent List operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = r.List()
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify registry is in consistent state
	names := r.List()
	assert.Equal(t, numGoroutines*numOperations, len(names))

	// Verify all instances can be retrieved
	for _, name := range names {
		instance, exists := r.Get(name)
		assert.True(t, exists)
		assert.NotNil(t, instance)
	}
}
