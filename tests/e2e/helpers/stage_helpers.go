package helpers

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BaseContext provides common testing utilities and infrastructure shared across scenario tests.
// It embeds the standard test assertions and provides access to K8s and API clients.
type BaseContext struct {
	T         *testing.T
	Require   *require.Assertions
	Assert    *assert.Assertions
	TestCtx   *TestContext
	K8sClient *K8sClient
	APIClient *APIClient
}

// NewBaseContext creates a new BaseContext for a test.
// This should be called once per test stage.
func NewBaseContext(t *testing.T, testCtx *TestContext) *BaseContext {
	return &BaseContext{
		T:         t,
		Require:   require.New(t),
		Assert:    assert.New(t),
		TestCtx:   testCtx,
		K8sClient: testCtx.K8sClient,
		APIClient: testCtx.APIClient,
	}
}

// ContextHelper provides convenient methods for creating contexts with standard timeouts.
type ContextHelper struct {
	t *testing.T
}

// NewContextHelper creates a new ContextHelper for a test.
func NewContextHelper(t *testing.T) *ContextHelper {
	return &ContextHelper{t: t}
}

// WithDefaultTimeout returns a context with a 2-minute timeout, suitable for most operations.
// The cancel function should be called with defer to ensure the context is cancelled.
func (h *ContextHelper) WithDefaultTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(h.t.Context(), 2*time.Minute)
}

// WithLongTimeout returns a context with a 5-minute timeout, suitable for slower operations.
// The cancel function should be called with defer to ensure the context is cancelled.
func (h *ContextHelper) WithLongTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(h.t.Context(), 5*time.Minute)
}

// WithTimeout returns a context with a custom timeout.
// The cancel function should be called with defer to ensure the context is cancelled.
func (h *ContextHelper) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(h.t.Context(), timeout)
}

// NamespaceManager manages the creation and cleanup of test namespaces.
type NamespaceManager struct {
	t         *testing.T
	k8sClient *K8sClient
}

// NewNamespaceManager creates a new NamespaceManager for a test.
func NewNamespaceManager(t *testing.T, k8sClient *K8sClient) *NamespaceManager {
	return &NamespaceManager{
		t:         t,
		k8sClient: k8sClient,
	}
}

// CreateNamespace creates a single namespace with a random suffix and registers it for cleanup.
// The namespace name will be: "<prefix>-<random 6-digit suffix>"
// Returns the actual namespace name created.
func (m *NamespaceManager) CreateNamespace(ctx context.Context, namePrefix string) (string, error) {
	suffix := rand.Intn(999999)
	namespaceName := fmt.Sprintf("%s-%d", namePrefix, suffix)

	err := m.k8sClient.CreateNamespace(ctx, namespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to create namespace %s: %w", namespaceName, err)
	}

	// Register cleanup
	m.t.Cleanup(func() {
		if err := m.k8sClient.DeleteNamespace(m.t.Context(), namespaceName); err != nil {
			m.t.Logf("Warning: failed to delete namespace %s: %v", namespaceName, err)
		}
	})

	return namespaceName, nil
}

// CreateMultipleNamespaces creates multiple namespaces with random suffixes.
// Returns a map of prefix -> actual namespace name created.
// All namespaces are registered for cleanup.
func (m *NamespaceManager) CreateMultipleNamespaces(ctx context.Context, namePrefixes []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, prefix := range namePrefixes {
		namespaceName, err := m.CreateNamespace(ctx, prefix)
		if err != nil {
			return nil, err
		}
		result[prefix] = namespaceName
	}

	return result, nil
}

// WaitHelper provides utilities for waiting and sleeping in tests.
type WaitHelper struct {
	t *testing.T
}

// NewWaitHelper creates a new WaitHelper for a test.
func NewWaitHelper(t *testing.T) *WaitHelper {
	return &WaitHelper{t: t}
}

// Sleep pauses for the specified duration and logs what we're waiting for.
// This makes test output clearer about why the test is pausing.
func (h *WaitHelper) Sleep(duration time.Duration, reason string) {
	h.t.Logf("⏳ Waiting %v for: %s", duration, reason)
	time.Sleep(duration)
}
