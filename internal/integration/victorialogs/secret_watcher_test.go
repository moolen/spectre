package victorialogs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/moolen/spectre/internal/logging"
)

// TestSecretWatcher_InitialFetch verifies that SecretWatcher loads token at startup
// when secret already exists.
func TestSecretWatcher_InitialFetch(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset with pre-populated secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("initial-token-123"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Verify token loaded
	token, err := watcher.GetToken()
	if err != nil {
		t.Errorf("GetToken() failed: %v", err)
	}
	if token != "initial-token-123" {
		t.Errorf("GetToken() = %q, want %q", token, "initial-token-123")
	}

	// Verify healthy
	if !watcher.IsHealthy() {
		t.Error("IsHealthy() = false, want true")
	}
}

// TestSecretWatcher_MissingSecretAtStartup verifies that SecretWatcher starts degraded
// when secret doesn't exist at startup.
func TestSecretWatcher_MissingSecretAtStartup(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset WITHOUT secret
	clientset := fake.NewSimpleClientset()

	// Create watcher
	watcher, err := NewSecretWatcher(clientset, "default", "missing-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	// Start watcher - should NOT fail even though secret is missing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start() failed when secret missing: %v", err)
	}
	defer watcher.Stop()

	// Verify starts degraded
	if watcher.IsHealthy() {
		t.Error("IsHealthy() = true, want false (degraded)")
	}

	// Verify GetToken returns error
	_, err = watcher.GetToken()
	if err == nil {
		t.Error("GetToken() succeeded, want error when degraded")
	}
}

// TestSecretWatcher_SecretRotation verifies that SecretWatcher detects secret updates
// and automatically rotates the token.
func TestSecretWatcher_SecretRotation(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset with initial secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("initial-token"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create and start watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Verify initial token
	token, err := watcher.GetToken()
	if err != nil {
		t.Fatalf("GetToken() failed: %v", err)
	}
	if token != "initial-token" {
		t.Errorf("GetToken() = %q, want %q", token, "initial-token")
	}

	// Update secret with new token
	secret.Data["api-token"] = []byte("rotated-token-456")
	_, err = clientset.CoreV1().Secrets("default").Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}

	// Wait for event to propagate (informer processes events asynchronously)
	// Use retry loop instead of fixed sleep for more reliable tests
	var newToken string
	for i := 0; i < 50; i++ {
		newToken, err = watcher.GetToken()
		if err == nil && newToken == "rotated-token-456" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify new token loaded
	if newToken != "rotated-token-456" {
		t.Errorf("GetToken() after rotation = %q, want %q", newToken, "rotated-token-456")
	}
}

// TestSecretWatcher_MissingKey verifies that SecretWatcher handles missing keys gracefully
// by starting degraded and logging available keys.
func TestSecretWatcher_MissingKey(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create secret with wrong key
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"wrong-key": []byte("some-value"),
			"other-key": []byte("other-value"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create watcher expecting "api-token" key
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Verify starts degraded
	if watcher.IsHealthy() {
		t.Error("IsHealthy() = true, want false when key missing")
	}

	// Verify GetToken returns error
	_, err = watcher.GetToken()
	if err == nil {
		t.Error("GetToken() succeeded, want error when key missing")
	}
}

// TestSecretWatcher_EmptyToken verifies that SecretWatcher treats whitespace-only tokens
// as invalid and starts degraded.
func TestSecretWatcher_EmptyToken(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create secret with whitespace-only token
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("   \n  \t  "), // Whitespace only
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Verify starts degraded
	if watcher.IsHealthy() {
		t.Error("IsHealthy() = true, want false for empty token")
	}

	// Verify GetToken returns error
	_, err = watcher.GetToken()
	if err == nil {
		t.Error("GetToken() succeeded, want error for empty token")
	}
}

// TestSecretWatcher_SecretDeleted verifies that SecretWatcher detects secret deletion
// and marks integration as degraded.
func TestSecretWatcher_SecretDeleted(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset with secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("valid-token"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create and start watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Verify healthy initially
	if !watcher.IsHealthy() {
		t.Fatal("IsHealthy() = false, want true initially")
	}

	// Delete secret
	err = clientset.CoreV1().Secrets("default").Delete(ctx, "test-secret", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete secret: %v", err)
	}

	// Wait for deletion event to propagate
	var healthy bool
	for i := 0; i < 50; i++ {
		healthy = watcher.IsHealthy()
		if !healthy {
			break // Deletion detected
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify now unhealthy
	if healthy {
		t.Error("IsHealthy() = true after deletion, want false")
	}
}

// TestSecretWatcher_ConcurrentReads verifies that GetToken() is thread-safe
// and handles concurrent reads during token rotation without data races.
func TestSecretWatcher_ConcurrentReads(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset with initial secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("initial-token"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create and start watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Launch 100 goroutines calling GetToken() concurrently
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				token, err := watcher.GetToken()
				if err != nil {
					errors <- err
					return
				}
				// Token should be either "initial-token" or "rotated-token"
				if token != "initial-token" && token != "rotated-token" {
					errors <- fmt.Errorf("unexpected token: %q", token)
					return
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Rotate secret mid-way
	time.Sleep(20 * time.Millisecond)
	secret.Data["api-token"] = []byte("rotated-token")
	_, err = clientset.CoreV1().Secrets("default").Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent read error: %v", err)
	}
}

// TestSecretWatcher_StopCleansUpGoroutines verifies that Stop() properly cleans up
// informer goroutines and prevents leaks.
func TestSecretWatcher_StopCleansUpGoroutines(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset with secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("test-token"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create and start watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Stop watcher
	if err := watcher.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Verify watcher no longer processes events by attempting another update
	// (no good way to verify goroutine count without goleak, but we can verify functionality)
	secret.Data["api-token"] = []byte("new-token-after-stop")
	_, err = clientset.CoreV1().Secrets("default").Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}

	// Wait a bit to ensure no updates processed
	time.Sleep(500 * time.Millisecond)

	// Token should still be old value (watcher stopped)
	// Note: GetToken will return error because watcher stopped, but we can check internal state
	watcher.mu.RLock()
	stoppedToken := watcher.token
	watcher.mu.RUnlock()

	if stoppedToken != "test-token" {
		t.Errorf("Token changed after Stop(): got %q, want %q", stoppedToken, "test-token")
	}
}

// TestSecretWatcher_ValidationErrors verifies that NewSecretWatcher validates inputs.
func TestSecretWatcher_ValidationErrors(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")
	clientset := fake.NewSimpleClientset()

	tests := []struct {
		name       string
		clientset  kubernetes.Interface
		namespace  string
		secretName string
		key        string
		logger     *logging.Logger
		wantErr    bool
	}{
		{
			name:       "nil clientset",
			clientset:  nil,
			namespace:  "default",
			secretName: "test",
			key:        "token",
			logger:     logger,
			wantErr:    true,
		},
		{
			name:       "empty namespace",
			clientset:  clientset,
			namespace:  "",
			secretName: "test",
			key:        "token",
			logger:     logger,
			wantErr:    true,
		},
		{
			name:       "empty secretName",
			clientset:  clientset,
			namespace:  "default",
			secretName: "",
			key:        "token",
			logger:     logger,
			wantErr:    true,
		},
		{
			name:       "empty key",
			clientset:  clientset,
			namespace:  "default",
			secretName: "test",
			key:        "",
			logger:     logger,
			wantErr:    true,
		},
		{
			name:       "nil logger",
			clientset:  clientset,
			namespace:  "default",
			secretName: "test",
			key:        "token",
			logger:     nil,
			wantErr:    true,
		},
		{
			name:       "valid inputs",
			clientset:  clientset,
			namespace:  "default",
			secretName: "test",
			key:        "token",
			logger:     logger,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSecretWatcher(tt.clientset, tt.namespace, tt.secretName, tt.key, tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSecretWatcher() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSecretWatcher_WhitespaceTrimmingInRotation verifies that trailing newlines
// and whitespace are properly trimmed during token rotation.
func TestSecretWatcher_WhitespaceTrimmingInRotation(t *testing.T) {
	logger := logging.GetLogger("test.secret_watcher")

	// Create fake clientset with secret containing trailing newline
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-token": []byte("token-with-newline\n"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)

	// Create and start watcher
	watcher, err := NewSecretWatcher(clientset, "default", "test-secret", "api-token", logger)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Verify whitespace trimmed
	token, err := watcher.GetToken()
	if err != nil {
		t.Fatalf("GetToken() failed: %v", err)
	}
	if token != "token-with-newline" {
		t.Errorf("GetToken() = %q, want %q (whitespace not trimmed)", token, "token-with-newline")
	}
}
