package grafana

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/moolen/spectre/internal/logging"
)

// SecretWatcher watches a Kubernetes Secret and maintains a local cache of the API token.
// It uses client-go's SharedInformerFactory for automatic caching, reconnection, and event handling.
// Thread-safe for concurrent access via sync.RWMutex.
type SecretWatcher struct {
	mu      sync.RWMutex
	token   string
	healthy bool

	namespace  string
	secretName string
	key        string

	clientset kubernetes.Interface
	factory   informers.SharedInformerFactory
	cancel    context.CancelFunc
	logger    *logging.Logger
}

// NewSecretWatcher creates a new SecretWatcher instance.
// Parameters:
// - clientset: Kubernetes clientset (use rest.InClusterConfig() to create)
// - namespace: Kubernetes namespace containing the secret
// - secretName: Name of the secret to watch
// - key: Key within secret.Data to extract token from
// - logger: Logger for observability
func NewSecretWatcher(clientset kubernetes.Interface, namespace, secretName, key string, logger *logging.Logger) (*SecretWatcher, error) {
	if clientset == nil {
		return nil, fmt.Errorf("clientset cannot be nil")
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if secretName == "" {
		return nil, fmt.Errorf("secretName cannot be empty")
	}
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &SecretWatcher{
		clientset:  clientset,
		namespace:  namespace,
		secretName: secretName,
		key:        key,
		logger:     logger,
		healthy:    false,
	}, nil
}

// NewInClusterSecretWatcher creates a SecretWatcher using in-cluster Kubernetes configuration.
// This is the recommended constructor for production use.
func NewInClusterSecretWatcher(namespace, secretName, key string, logger *logging.Logger) (*SecretWatcher, error) {
	// Use ServiceAccount token mounted at /var/run/secrets/kubernetes.io/serviceaccount/token
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return NewSecretWatcher(clientset, namespace, secretName, key, logger)
}

// Start initializes the informer and begins watching the secret.
// It creates a SharedInformerFactory scoped to the namespace, sets up event handlers,
// and performs an initial fetch from the cache.
// Returns error if cache sync fails, but does NOT fail if secret is missing at startup
// (starts in degraded mode instead).
func (w *SecretWatcher) Start(ctx context.Context) error {
	// Create cancellable context for informer lifecycle
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	// Create factory scoped to namespace (more efficient than cluster-wide)
	// Resync every 30 seconds to ensure cache stays fresh
	w.factory = informers.NewSharedInformerFactoryWithOptions(
		w.clientset,
		30*time.Second,
		informers.WithNamespace(w.namespace),
	)

	// Get secret informer
	secretInformer := w.factory.Core().V1().Secrets().Informer()

	// Add event handlers - these fire when secrets change
	// Note: handlers receive ALL secrets in namespace, so we filter by name
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			if secret.Name == w.secretName {
				w.handleSecretUpdate(secret)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			secret := newObj.(*corev1.Secret)
			if secret.Name == w.secretName {
				w.handleSecretUpdate(secret)
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			if secret.Name == w.secretName {
				w.handleSecretDelete(secret)
			}
		},
	})

	// Start informer (spawns background goroutines)
	w.factory.Start(ctx.Done())

	// Wait for cache to sync (blocks until initial list completes)
	if !cache.WaitForCacheSync(ctx.Done(), secretInformer.HasSynced) {
		return fmt.Errorf("failed to sync secret cache")
	}

	// Initial fetch from cache (does NOT fail startup if secret missing)
	if err := w.initialFetch(); err != nil {
		w.logger.Warn("Initial fetch failed (will retry on watch events): %v", err)
	}

	w.logger.Info("SecretWatcher started for secret %s/%s (key: %s)", w.namespace, w.secretName, w.key)
	return nil
}

// Stop gracefully shuts down the informer and waits for goroutines to exit.
// Prevents goroutine leaks by cancelling context and calling factory.Shutdown().
func (w *SecretWatcher) Stop() error {
	w.logger.Info("Stopping SecretWatcher for secret %s/%s", w.namespace, w.secretName)

	if w.cancel != nil {
		w.cancel() // Cancel context to stop informer goroutines
	}

	if w.factory != nil {
		w.factory.Shutdown() // Wait for goroutines to exit
	}

	return nil
}

// GetToken returns the current API token.
// Thread-safe with RLock for concurrent reads.
// Returns error if integration is degraded (no valid token available).
func (w *SecretWatcher) GetToken() (string, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.healthy || w.token == "" {
		return "", fmt.Errorf("integration degraded: missing API token")
	}

	return w.token, nil
}

// IsHealthy returns true if a valid token is available.
// Thread-safe with RLock.
func (w *SecretWatcher) IsHealthy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.healthy
}

// handleSecretUpdate processes secret update events.
// Extracts the token from secret.Data[key], validates it, and updates internal state.
// Logs rotation events but NEVER logs token values (security).
func (w *SecretWatcher) handleSecretUpdate(secret *corev1.Secret) {
	// Extract token bytes from secret data
	tokenBytes, ok := secret.Data[w.key]
	if !ok {
		// Key not found - log available keys for debugging
		availableKeys := make([]string, 0, len(secret.Data))
		for k := range secret.Data {
			availableKeys = append(availableKeys, k)
		}
		w.logger.Warn("Key %q not found in Secret %s/%s, available keys: %v",
			w.key, w.namespace, w.secretName, availableKeys)
		w.markDegraded()
		return
	}

	// client-go already base64-decodes Secret.Data
	// Trim whitespace (secrets often have trailing newlines)
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		w.logger.Warn("Token is empty after trimming whitespace in Secret %s/%s key %q",
			w.namespace, w.secretName, w.key)
		w.markDegraded()
		return
	}

	// Update token (thread-safe with Lock for exclusive write)
	w.mu.Lock()
	oldToken := w.token
	w.token = token
	w.healthy = true
	w.mu.Unlock()

	// Log rotation (NEVER log token values)
	if oldToken != "" && oldToken != token {
		w.logger.Info("Token rotated for integration (secret: %s/%s)", w.namespace, w.secretName)
	} else if oldToken == "" {
		w.logger.Info("Token loaded for integration (secret: %s/%s)", w.namespace, w.secretName)
	}
}

// handleSecretDelete processes secret deletion events.
// Marks integration as degraded - watch will auto-recover if secret is recreated.
func (w *SecretWatcher) handleSecretDelete(secret *corev1.Secret) {
	w.logger.Warn("Secret %s/%s deleted - integration degraded", w.namespace, w.secretName)
	w.markDegraded()
}

// markDegraded marks the integration as unhealthy.
// Thread-safe with Lock.
func (w *SecretWatcher) markDegraded() {
	w.mu.Lock()
	w.healthy = false
	w.mu.Unlock()
}

// initialFetch performs initial token fetch from the informer's cache.
// Uses lister (local cache, no API call) for efficiency.
// Does NOT fail startup if secret is missing - starts degraded instead.
// Watch will pick up secret when it's created.
func (w *SecretWatcher) initialFetch() error {
	// Use informer's lister (reads from local cache, no API call)
	lister := w.factory.Core().V1().Secrets().Lister().Secrets(w.namespace)
	secret, err := lister.Get(w.secretName)
	if err != nil {
		// Secret doesn't exist - start degraded, watch will pick it up when created
		w.logger.Warn("Secret %s/%s not found at startup - starting degraded: %v",
			w.namespace, w.secretName, err)
		w.markDegraded()
		return nil // Don't fail startup
	}

	// Secret exists - process it
	w.handleSecretUpdate(secret)
	return nil
}
