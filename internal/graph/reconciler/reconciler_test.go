package reconciler

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("Expected Enabled to be true by default")
	}

	if config.Interval != 5*time.Minute {
		t.Errorf("Expected Interval to be 5m, got %v", config.Interval)
	}

	if config.BatchSize != 100 {
		t.Errorf("Expected BatchSize to be 100, got %d", config.BatchSize)
	}
}

func TestReconcileInput(t *testing.T) {
	input := ReconcileInput{
		Resources: []GraphResource{
			{UID: "uid-1", Kind: "Pod", Namespace: "default", Name: "pod-1"},
			{UID: "uid-2", Kind: "Pod", Namespace: "default", Name: "pod-2"},
		},
		BatchSize: 100,
	}

	if len(input.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(input.Resources))
	}
}

func TestReconcileOutput(t *testing.T) {
	output := ReconcileOutput{
		ResourcesChecked:    10,
		ResourcesDeleted:    []string{"uid-1", "uid-2"},
		ResourcesStillExist: []string{"uid-3", "uid-4", "uid-5"},
		Errors:              nil,
	}

	if output.ResourcesChecked != 10 {
		t.Errorf("Expected ResourcesChecked to be 10, got %d", output.ResourcesChecked)
	}

	if len(output.ResourcesDeleted) != 2 {
		t.Errorf("Expected 2 deleted resources, got %d", len(output.ResourcesDeleted))
	}

	if len(output.ResourcesStillExist) != 3 {
		t.Errorf("Expected 3 existing resources, got %d", len(output.ResourcesStillExist))
	}
}

func TestPodTerminationReconcilerInterface(t *testing.T) {
	// Test that PodTerminationReconciler implements ReconcileHandler
	var _ ReconcileHandler = (*PodTerminationReconciler)(nil)
}

func TestPodTerminationReconcilerName(t *testing.T) {
	reconciler := &PodTerminationReconciler{}

	if name := reconciler.Name(); name != "PodTerminationReconciler" {
		t.Errorf("Expected name 'PodTerminationReconciler', got %q", name)
	}
}

func TestPodTerminationReconcilerResourceKind(t *testing.T) {
	reconciler := &PodTerminationReconciler{}

	if kind := reconciler.ResourceKind(); kind != "Pod" {
		t.Errorf("Expected resource kind 'Pod', got %q", kind)
	}
}

// MockReconcileHandler is a mock implementation for testing
type MockReconcileHandler struct {
	name         string
	resourceKind string
	reconcileFn  func(ctx context.Context, input ReconcileInput) (*ReconcileOutput, error)
}

func (m *MockReconcileHandler) Name() string {
	return m.name
}

func (m *MockReconcileHandler) ResourceKind() string {
	return m.resourceKind
}

func (m *MockReconcileHandler) Reconcile(ctx context.Context, input ReconcileInput) (*ReconcileOutput, error) {
	if m.reconcileFn != nil {
		return m.reconcileFn(ctx, input)
	}
	return &ReconcileOutput{}, nil
}

func TestMockReconcileHandler(t *testing.T) {
	mockHandler := &MockReconcileHandler{
		name:         "TestHandler",
		resourceKind: "TestResource",
		reconcileFn: func(ctx context.Context, input ReconcileInput) (*ReconcileOutput, error) {
			return &ReconcileOutput{
				ResourcesChecked:    len(input.Resources),
				ResourcesDeleted:    []string{"uid-1"},
				ResourcesStillExist: []string{},
			}, nil
		},
	}

	// Verify interface compliance
	var _ ReconcileHandler = mockHandler

	if mockHandler.Name() != "TestHandler" {
		t.Errorf("Expected name 'TestHandler', got %q", mockHandler.Name())
	}

	if mockHandler.ResourceKind() != "TestResource" {
		t.Errorf("Expected resource kind 'TestResource', got %q", mockHandler.ResourceKind())
	}

	input := ReconcileInput{
		Resources: []GraphResource{
			{UID: "uid-1"},
			{UID: "uid-2"},
		},
	}

	output, err := mockHandler.Reconcile(context.Background(), input)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if output.ResourcesChecked != 2 {
		t.Errorf("Expected 2 resources checked, got %d", output.ResourcesChecked)
	}

	if len(output.ResourcesDeleted) != 1 {
		t.Errorf("Expected 1 deleted resource, got %d", len(output.ResourcesDeleted))
	}
}
