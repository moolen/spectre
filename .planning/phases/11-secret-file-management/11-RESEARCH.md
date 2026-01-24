# Phase 11: Secret File Management - Research

**Researched:** 2026-01-22
**Domain:** Kubernetes secret watching and hot-reload with client-go
**Confidence:** HIGH

## Summary

Phase 11 implements Kubernetes-native secret management with hot-reload capabilities. Instead of mounting secrets as files, Spectre will fetch secrets directly from the Kubernetes API server using client-go's SharedInformerFactory. The standard approach uses informers (not raw Watch) for automatic caching, reconnection, and event handling. Secrets are watched via the Kubernetes Watch API, which provides immediate notification on changes without requiring pod restarts.

The project already uses client-go v0.34.0 (corresponding to Kubernetes 1.34), which provides the complete informer infrastructure needed. The standard pattern is: create SharedInformerFactory → get secret informer → add event handlers → start factory → wait for cache sync. Thread-safety is achieved via sync.RWMutex (standard for token storage with high read-to-write ratio). Secret redaction uses custom wrapper types or regex-based sanitization to ensure tokens never appear in logs.

**Primary recommendation:** Use SharedInformerFactory with namespace-scoped secret informer, ResourceEventHandlerFuncs for Add/Update/Delete events, sync.RWMutex for token storage, and custom String() method on token type for automatic redaction.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| k8s.io/client-go | v0.34.0 | Kubernetes API client | Official Go client, used by all Kubernetes operators and controllers |
| k8s.io/api | v0.34.0 | Kubernetes API types | Official type definitions for Secret, Pod, etc. |
| k8s.io/apimachinery | v0.34.0 | API machinery (meta, watch) | Core types for Watch, ListOptions, ObjectMeta |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/cenkalti/backoff/v4 | v4.3.0 | Exponential backoff | Already in project, use for watch reconnection retry |
| go.uber.org/goleak | latest | Goroutine leak detection | Testing only - verify informer cleanup |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SharedInformerFactory | Raw Watch API | Raw watch requires manual reconnection, caching, and resync - only justified for extremely simple use cases |
| sync.RWMutex | atomic.Value | atomic.Value is ~3x faster but only works for simple types - RWMutex better for string token with validation logic |
| Informer | File mount + fsnotify | File mount requires kubelet propagation (up to 2min delay), can't detect missing secrets at startup |

**Installation:**
```bash
# Already in project (go.mod shows k8s.io/client-go v0.34.0)
# No additional dependencies needed
```

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/victorialogs/
├── victorialogs.go          # Main integration, holds secretWatcher
├── secret_watcher.go        # NEW: Secret watching and token management
├── secret_watcher_test.go   # NEW: Tests for token rotation
├── client.go                # HTTP client (uses token from secretWatcher)
└── types.go                 # Config types (add SecretRef)
```

### Pattern 1: SharedInformerFactory with Namespace Filter
**What:** Create a shared informer factory scoped to Spectre's namespace, get secret informer, add event handlers for Add/Update/Delete events.

**When to use:** Always prefer this over raw Watch - informers handle caching, reconnection, and resync automatically.

**Example:**
```go
// Source: https://pkg.go.dev/k8s.io/client-go/informers
import (
    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
)

// Create factory scoped to namespace
factory := informers.NewSharedInformerFactoryWithOptions(
    clientset,
    30*time.Second, // resync period
    informers.WithNamespace(namespace),
)

// Get secret informer
secretInformer := factory.Core().V1().Secrets().Informer()

// Add event handlers
secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        secret := obj.(*corev1.Secret)
        handleSecretUpdate(secret)
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
        secret := newObj.(*corev1.Secret)
        handleSecretUpdate(secret)
    },
    DeleteFunc: func(obj interface{}) {
        secret := obj.(*corev1.Secret)
        handleSecretDelete(secret)
    },
})

// Start factory
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
factory.Start(ctx.Done())

// Wait for cache sync
if !cache.WaitForCacheSync(ctx.Done(), secretInformer.HasSynced) {
    return fmt.Errorf("failed to sync secret cache")
}
```

### Pattern 2: Thread-Safe Token Storage with RWMutex
**What:** Store token in struct with sync.RWMutex, use RLock for reads (concurrent), Lock for writes (exclusive).

**When to use:** Token reads are frequent (every API call), writes are rare (only on rotation) - RWMutex is optimal for this pattern.

**Example:**
```go
// Source: https://medium.com/@anto_rayen/understanding-locks-rwmutex-in-golang-3c468c65062a
type SecretWatcher struct {
    mu    sync.RWMutex
    token string

    // Other fields: clientset, informer, namespace, secretName, key
}

// GetToken is called on every API request (high frequency)
func (w *SecretWatcher) GetToken() (string, error) {
    w.mu.RLock()
    defer w.mu.RUnlock()

    if w.token == "" {
        return "", fmt.Errorf("no token available")
    }
    return w.token, nil
}

// setToken is called only on secret rotation (low frequency)
func (w *SecretWatcher) setToken(newToken string) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.token = newToken
}
```

### Pattern 3: In-Cluster Config with RBAC
**What:** Use rest.InClusterConfig() to authenticate as ServiceAccount, configure RBAC to allow get/watch on secrets in same namespace.

**When to use:** Always when running inside Kubernetes - more secure than kubeconfig file.

**Example:**
```go
// Source: client-go documentation
import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

// In-cluster config (uses ServiceAccount token)
config, err := rest.InClusterConfig()
if err != nil {
    return fmt.Errorf("failed to get in-cluster config: %w", err)
}

clientset, err := kubernetes.NewForConfig(config)
if err != nil {
    return fmt.Errorf("failed to create clientset: %w", err)
}
```

**Required RBAC (deploy with Helm chart):**
```yaml
# Source: https://medium.com/@subhampradhan966/configuring-kubernetes-rbac-a-comprehensive-guide-b6d40ac7b257
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: spectre-secret-reader
  namespace: {{ .Release.Namespace }}
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "watch", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: spectre-secret-reader
  namespace: {{ .Release.Namespace }}
subjects:
- kind: ServiceAccount
  name: spectre
  namespace: {{ .Release.Namespace }}
roleRef:
  kind: Role
  name: spectre-secret-reader
  apiGroup: rbac.authorization.k8s.io
```

### Pattern 4: Secret Data Decoding
**What:** client-go automatically decodes base64 - Secret.Data field is `map[string][]byte` with raw decoded values.

**When to use:** Always - do NOT manually base64-decode Secret.Data, it's already decoded.

**Example:**
```go
// Source: https://github.com/kubernetes/client-go/issues/651
secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
if err != nil {
    return fmt.Errorf("failed to get secret: %w", err)
}

// Data is already base64-decoded by client-go
tokenBytes, ok := secret.Data[key]
if !ok {
    return fmt.Errorf("key %q not found in secret %q", key, secretName)
}

// Trim whitespace (Kubernetes secrets often have trailing newlines)
token := strings.TrimSpace(string(tokenBytes))
```

### Pattern 5: Token Redaction via Custom Type
**What:** Wrap token in custom type with String() method that returns "[REDACTED]" - prevents accidental logging.

**When to use:** Always for sensitive values - Go's fmt package calls String() automatically.

**Example:**
```go
// Source: https://medium.com/hackernoon/keep-passwords-and-secrets-out-of-your-logs-with-go-a2294a9546ce
type SecretToken string

func (t SecretToken) String() string {
    return "[REDACTED]"
}

func (t SecretToken) Value() string {
    return string(t)
}

// Usage
type SecretWatcher struct {
    mu    sync.RWMutex
    token SecretToken  // Not string
}

// Logging automatically redacts
logger.Info("Token updated: %v", watcher.token) // Logs: "Token updated: [REDACTED]"

// Get actual value when needed
actualToken := watcher.token.Value()
```

### Anti-Patterns to Avoid

- **Using raw Watch API instead of Informer:** Requires manual reconnection on 410 Gone errors, manual caching, manual resync logic - complex and error-prone.

- **Not scoping informer to namespace:** Watching all secrets in all namespaces requires ClusterRole (security risk) and caches unnecessary data (memory waste).

- **Blocking in event handlers:** Event handlers run synchronously - long operations block the informer. Use channels/goroutines for heavy work.

- **Not waiting for cache sync:** Querying lister before WaitForCacheSync completes returns stale/empty data.

- **Forgetting to close stop channel:** Informer goroutines leak if stop channel never closes - always defer close() or use context cancellation.

- **Manual base64 decoding of Secret.Data:** client-go already decodes it - double-decoding causes errors.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Watching Kubernetes resources | Custom HTTP watch loop with JSON parsing | SharedInformerFactory from client-go | Handles 410 Gone errors, reconnection, exponential backoff, caching, resync - 1000+ lines of complex logic |
| Handling 410 Gone errors | Manual resourceVersion tracking and re-list | Informer's automatic resync | 410 Gone means resourceVersion too old - informer re-lists automatically, you'll get it wrong |
| Kubernetes authentication | Reading ServiceAccount token file manually | rest.InClusterConfig() | Handles token rotation, CA cert loading, API server discovery - security-critical code |
| Secret rotation detection | Polling Get() every N seconds | Watch API via Informer | Watch provides push notifications within ~2 seconds, polling wastes API calls and delays updates |
| Token cache management | Custom cache with expiry logic | Informer's built-in cache (Lister) | Informer cache is thread-safe, automatically updated, indexed - don't reinvent |
| Exponential backoff for retries | Custom backoff with jitter | github.com/cenkalti/backoff (already in project) | Prevents thundering herd, tested formula, configurable limits |

**Key insight:** Kubernetes operators are complex distributed systems. client-go's informer pattern is the result of years of production experience and bug fixes. Custom watch implementations inevitably rediscover the same edge cases (network partitions, stale caches, goroutine leaks, API throttling) that informers already handle.

## Common Pitfalls

### Pitfall 1: Informer Goroutine Leaks on Shutdown
**What goes wrong:** Informer starts background goroutines that run until stop channel closes. If stop channel never closes (or context never cancels), goroutines leak, causing memory growth over time.

**Why it happens:** factory.Start(stopCh) spawns goroutines for each informer, but returns immediately. Easy to forget to close stopCh on application shutdown.

**How to avoid:**
- Always use context.WithCancel() and defer cancel()
- Or create stop channel and defer close(stopCh)
- Call factory.Shutdown() in Stop() method (blocks until all goroutines exit)

**Warning signs:**
- Increasing goroutine count in pprof (net/http/pprof)
- Memory growth without corresponding resource increase
- Test failures with goleak.VerifyNone() showing leaked goroutines

**Example:**
```go
// Source: https://medium.com/uckey/memory-goroutine-leak-with-rancher-kubernetes-custom-controller-with-client-go-9e296c815209
// WRONG - stop channel never closed
func (i *Integration) Start(ctx context.Context) error {
    factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
    stopCh := make(chan struct{})
    factory.Start(stopCh) // Goroutines run forever
    return nil
}

// RIGHT - context cancellation stops informer
func (i *Integration) Start(ctx context.Context) error {
    factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
    factory.Start(ctx.Done()) // Goroutines stop when ctx cancelled
    return nil
}

func (i *Integration) Stop(ctx context.Context) error {
    i.cancel() // Cancel context from Start()
    i.factory.Shutdown() // Wait for goroutines to exit
    return nil
}
```

### Pitfall 2: Watch Reconnection After 410 Gone Error
**What goes wrong:** Kubernetes watch connections can expire if resourceVersion becomes too old (API server has compacted history). Watch returns 410 Gone error. If not handled, watch stops receiving updates permanently.

**Why it happens:** Kubernetes API server only keeps a limited history of resource versions. If watch disconnects for too long (network partition, API server restart), the old resourceVersion is gone when reconnecting.

**How to avoid:** Use Informer instead of raw Watch - informer automatically handles 410 Gone by re-listing all resources and restarting watch with fresh resourceVersion.

**Warning signs:**
- Secret rotations stop being detected after Spectre pod restart or network issue
- Logs show "resourceVersion too old" or "410 Gone" errors
- Integration remains in degraded state despite valid secret existing

**Example:**
```go
// Source: https://github.com/kubernetes/kubernetes/issues/25151
// WRONG - raw Watch doesn't handle 410 Gone
watcher, err := clientset.CoreV1().Secrets(namespace).Watch(ctx, metav1.ListOptions{})
for event := range watcher.ResultChan() {
    // If watch connection expires, this loop ends and never restarts
}

// RIGHT - Informer handles 410 Gone automatically
factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
secretInformer := factory.Core().V1().Secrets().Informer()
secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    UpdateFunc: func(old, new interface{}) {
        // Always receives updates, even after 410 Gone (informer re-lists)
    },
})
factory.Start(ctx.Done())
```

### Pitfall 3: Blocking Operations in Event Handlers
**What goes wrong:** Event handlers (AddFunc, UpdateFunc, DeleteFunc) run synchronously in the informer's goroutine. Long-running operations (API calls, database writes, heavy computation) block the handler, preventing other events from processing.

**Why it happens:** Informer delivers events one-by-one to handlers. If handler takes 10 seconds, next event waits 10 seconds - creates cascading delays.

**How to avoid:**
- Keep handlers fast (<1ms) - just validate and copy data
- Use buffered channel to queue work for background goroutine
- Or spawn goroutine in handler (but beware unbounded goroutine growth)

**Warning signs:**
- Slow secret rotation detection (>5 seconds when should be <2 seconds)
- Logs showing "cache sync took 30s" warnings
- Other resources (pods, configmaps) also slow to update

**Example:**
```go
// WRONG - blocks informer for 5 seconds per secret
secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    UpdateFunc: func(old, new interface{}) {
        secret := new.(*corev1.Secret)
        validateToken(secret) // Calls external API - 5 seconds
        updateDatabase(secret) // Database write - 2 seconds
    },
})

// RIGHT - handler returns immediately, work happens async
type SecretWatcher struct {
    workQueue chan *corev1.Secret
}

secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    UpdateFunc: func(old, new interface{}) {
        secret := new.(*corev1.Secret)
        // Non-blocking send (or use select with default)
        select {
        case w.workQueue <- secret:
        default:
            logger.Warn("Work queue full, dropping secret update")
        }
    },
})

// Background worker processes queue
go func() {
    for secret := range w.workQueue {
        validateToken(secret)
        updateDatabase(secret)
    }
}()
```

### Pitfall 4: Race Condition Between Token Read and Update
**What goes wrong:** Multiple goroutines read token (API calls) while one goroutine updates token (secret rotation). Without proper locking, reads can see partial writes (empty string, corrupted value) causing API auth failures.

**Why it happens:** Go strings are not atomic - even simple assignment can be observed mid-write by concurrent reader on different CPU core.

**How to avoid:**
- Use sync.RWMutex - RLock for reads (concurrent), Lock for writes (exclusive)
- Or use atomic.Value if token storage is simple (just string, no validation)
- Test with race detector: go test -race

**Warning signs:**
- Intermittent "invalid token" errors during secret rotation
- Race detector warnings in tests: "WARNING: DATA RACE"
- Auth failures that resolve after retrying

**Example:**
```go
// WRONG - no synchronization
type SecretWatcher struct {
    token string // RACE: concurrent read/write
}

func (w *SecretWatcher) GetToken() string {
    return w.token // RACE: reads while Update() writes
}

func (w *SecretWatcher) Update(secret *corev1.Secret) {
    w.token = parseToken(secret) // RACE: writes while GetToken() reads
}

// RIGHT - RWMutex protects token
type SecretWatcher struct {
    mu    sync.RWMutex
    token string
}

func (w *SecretWatcher) GetToken() (string, error) {
    w.mu.RLock()
    defer w.mu.RUnlock()
    if w.token == "" {
        return "", fmt.Errorf("no token available")
    }
    return w.token, nil
}

func (w *SecretWatcher) Update(secret *corev1.Secret) {
    newToken := parseToken(secret)
    w.mu.Lock()
    w.token = newToken
    w.mu.Unlock()
}
```

### Pitfall 5: Not Trimming Whitespace from Secret Values
**What goes wrong:** Kubernetes secrets often have trailing newlines when created via kubectl or YAML (common editor behavior). Token comparison fails: "token123\n" != "token123".

**Why it happens:** Users create secrets like: `kubectl create secret generic my-secret --from-literal=token="$(cat token.txt)"` where token.txt has trailing newline. Or YAML editors add newlines.

**How to avoid:** Always strings.TrimSpace() after decoding Secret.Data - removes leading/trailing whitespace including newlines.

**Warning signs:**
- Secret exists with correct value in kubectl output
- Integration remains degraded with "invalid token" error
- Token length differs from expected (len("token123\n") == 9, not 8)

**Example:**
```go
// Source: Common kubectl secret creation pattern
// WRONG - uses raw bytes including whitespace
tokenBytes := secret.Data[key]
token := string(tokenBytes) // May be "token123\n"
client.SetToken(token) // Fails: API expects "token123"

// RIGHT - trim whitespace
tokenBytes := secret.Data[key]
token := strings.TrimSpace(string(tokenBytes)) // Now "token123"
if token == "" {
    return fmt.Errorf("token is empty after trimming whitespace")
}
client.SetToken(token) // Success
```

### Pitfall 6: Informer Resync Storms During Network Partition
**What goes wrong:** If resync period is too short (e.g., 1 second) and network is flaky, informer constantly re-lists all secrets, flooding API server and causing throttling (HTTP 429).

**Why it happens:** Resync period triggers full re-list of all resources in namespace. If network drops during re-list, informer retries immediately - exponential API load.

**How to avoid:**
- Use resync period ≥30 seconds (30s is common default)
- Don't set resync to 0 (disables resync entirely - stale cache risk)
- Monitor API server metrics for high secret list request rate

**Warning signs:**
- API server logs show HTTP 429 (Too Many Requests) from Spectre
- Spectre logs show "rate limited" or "throttled" messages
- Secret updates delayed during high API server load

**Example:**
```go
// WRONG - 1 second resync floods API server
factory := informers.NewSharedInformerFactory(clientset, 1*time.Second)

// RIGHT - 30 second resync (standard)
factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

// ALSO RIGHT - namespace-scoped reduces blast radius
factory := informers.NewSharedInformerFactoryWithOptions(
    clientset,
    30*time.Second,
    informers.WithNamespace(namespace), // Only secrets in Spectre's namespace
)
```

## Code Examples

Verified patterns from official sources:

### Creating In-Cluster Kubernetes Client
```go
// Source: k8s.io/client-go documentation
package secretwatcher

import (
    "context"
    "fmt"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func NewKubernetesClient() (*kubernetes.Clientset, error) {
    // InClusterConfig uses ServiceAccount token from:
    // /var/run/secrets/kubernetes.io/serviceaccount/token
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create clientset: %w", err)
    }

    return clientset, nil
}
```

### Setting Up Secret Informer with Event Handlers
```go
// Source: https://github.com/feiskyer/kubernetes-handbook/blob/master/examples/client/informer/informer.go
package secretwatcher

import (
    "context"
    "fmt"
    "strings"
    "sync"
    "time"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
)

type SecretWatcher struct {
    mu          sync.RWMutex
    token       string
    healthy     bool

    namespace   string
    secretName  string
    key         string

    clientset   *kubernetes.Clientset
    factory     informers.SharedInformerFactory
    cancel      context.CancelFunc
}

func NewSecretWatcher(clientset *kubernetes.Clientset, namespace, secretName, key string) *SecretWatcher {
    return &SecretWatcher{
        clientset:  clientset,
        namespace:  namespace,
        secretName: secretName,
        key:        key,
    }
}

func (w *SecretWatcher) Start(ctx context.Context) error {
    // Create cancellable context for informer lifecycle
    ctx, cancel := context.WithCancel(ctx)
    w.cancel = cancel

    // Create factory scoped to namespace (more efficient than cluster-wide)
    w.factory = informers.NewSharedInformerFactoryWithOptions(
        w.clientset,
        30*time.Second, // Resync every 30 seconds
        informers.WithNamespace(w.namespace),
    )

    // Get secret informer
    secretInformer := w.factory.Core().V1().Secrets().Informer()

    // Add event handlers
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

    // Start informer
    w.factory.Start(ctx.Done())

    // Wait for cache to sync (blocks until initial list completes)
    if !cache.WaitForCacheSync(ctx.Done(), secretInformer.HasSynced) {
        return fmt.Errorf("failed to sync secret cache")
    }

    // Initial fetch (informer cache is now populated)
    return w.initialFetch()
}

func (w *SecretWatcher) Stop(ctx context.Context) error {
    if w.cancel != nil {
        w.cancel() // Stop informer goroutines
    }
    if w.factory != nil {
        w.factory.Shutdown() // Wait for goroutines to exit
    }
    return nil
}

func (w *SecretWatcher) handleSecretUpdate(secret *corev1.Secret) {
    tokenBytes, ok := secret.Data[w.key]
    if !ok {
        availableKeys := make([]string, 0, len(secret.Data))
        for k := range secret.Data {
            availableKeys = append(availableKeys, k)
        }
        // Clear error message helps user debug config
        logger.Warn("Key %q not found in Secret %q, available keys: %v",
            w.key, w.secretName, availableKeys)
        w.markDegraded()
        return
    }

    // client-go already base64-decodes Secret.Data
    token := strings.TrimSpace(string(tokenBytes))
    if token == "" {
        logger.Warn("Token is empty in Secret %q key %q", w.secretName, w.key)
        w.markDegraded()
        return
    }

    // Update token (thread-safe)
    w.mu.Lock()
    oldToken := w.token
    w.token = token
    w.healthy = true
    w.mu.Unlock()

    if oldToken != "" && oldToken != token {
        logger.Info("Token rotated for integration (secret: %s)", w.secretName)
    } else {
        logger.Info("Token loaded for integration (secret: %s)", w.secretName)
    }
}

func (w *SecretWatcher) handleSecretDelete(secret *corev1.Secret) {
    logger.Warn("Secret %q deleted - integration degraded", w.secretName)
    w.markDegraded()
}

func (w *SecretWatcher) markDegraded() {
    w.mu.Lock()
    w.healthy = false
    w.mu.Unlock()
}

func (w *SecretWatcher) initialFetch() error {
    // Use informer's lister (reads from local cache, no API call)
    lister := w.factory.Core().V1().Secrets().Lister().Secrets(w.namespace)
    secret, err := lister.Get(w.secretName)
    if err != nil {
        // Secret doesn't exist - start degraded, watch will pick it up when created
        logger.Warn("Secret %q not found at startup - starting degraded: %v", w.secretName, err)
        w.markDegraded()
        return nil // Don't fail startup
    }

    w.handleSecretUpdate(secret)
    return nil
}

func (w *SecretWatcher) GetToken() (string, error) {
    w.mu.RLock()
    defer w.mu.RUnlock()

    if !w.healthy || w.token == "" {
        return "", fmt.Errorf("integration degraded: missing API token")
    }

    return w.token, nil
}

func (w *SecretWatcher) IsHealthy() bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.healthy
}
```

### Token Redaction Pattern
```go
// Source: https://medium.com/hackernoon/keep-passwords-and-secrets-out-of-your-logs-with-go-a2294a9546ce
package secretwatcher

import "fmt"

// SecretToken wraps a token string to prevent logging
type SecretToken string

// String implements fmt.Stringer - called by fmt.Printf, logger.Info, etc.
func (t SecretToken) String() string {
    return "[REDACTED]"
}

// Value returns the actual token value (use only when needed for API calls)
func (t SecretToken) Value() string {
    return string(t)
}

// Example usage in SecretWatcher
type SecretWatcher struct {
    mu    sync.RWMutex
    token SecretToken // Not string
}

func (w *SecretWatcher) handleSecretUpdate(secret *corev1.Secret) {
    tokenBytes := secret.Data[w.key]
    newToken := SecretToken(strings.TrimSpace(string(tokenBytes)))

    w.mu.Lock()
    w.token = newToken
    w.mu.Unlock()

    // Logs: "Token updated: [REDACTED]"
    logger.Info("Token updated: %v", w.token)
}

func (w *SecretWatcher) GetToken() (string, error) {
    w.mu.RLock()
    defer w.mu.RUnlock()

    if w.token == "" {
        return "", fmt.Errorf("no token available")
    }

    // Return actual value for API client
    return w.token.Value(), nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| File mount + fsnotify | Kubernetes Watch API + Informer | 2019+ | Watch provides <2s updates vs 1-2min kubelet propagation delay. Direct API access detects missing secrets at startup. |
| Raw Watch API | SharedInformerFactory | 2016+ (client-go v2.0) | Informer handles 410 Gone, reconnection, caching, resync - 1000+ lines of complex logic now built-in. |
| sync.Mutex for all locks | sync.RWMutex for read-heavy workloads | Always available | RWMutex allows concurrent reads (API calls don't block each other), only writes (rotation) are exclusive. |
| Manual base64 decode | client-go auto-decodes Secret.Data | Always | Secret.Data is map[string][]byte already decoded - manual decode causes double-decode errors. |
| String for tokens | Custom type with String() redaction | Best practice since ~2018 | Prevents accidental logging - fmt.Printf("%v", token) automatically redacts. |

**Deprecated/outdated:**
- **File mount pattern for hot-reload:** Kubernetes still supports it, but Watch API is better - faster updates, detects missing secrets, no kubelet delay.
- **NewFilteredSharedInformerFactory:** Deprecated in favor of NewSharedInformerFactoryWithOptions (WithNamespace option).
- **Informer.Run():** Deprecated in favor of factory.Start() - factory coordinates multiple informers.

## Open Questions

Things that couldn't be fully resolved:

1. **Optimal resync period for secrets**
   - What we know: 30 seconds is common default, 0 disables resync (stale cache risk), <10s can flood API server
   - What's unclear: Whether Spectre's specific workload justifies different value
   - Recommendation: Start with 30s (standard), monitor API server metrics, adjust if needed

2. **RWMutex vs atomic.Value for token storage**
   - What we know: atomic.Value is ~3x faster (0.5ns vs 48ns per read), RWMutex better for complex data structures
   - What's unclear: Whether token validation logic (empty check, whitespace trim) happens inside or outside lock
   - Recommendation: Use RWMutex (more flexible, validation can be inside lock), benchmark if performance issues arise

3. **Informer workqueue for async processing**
   - What we know: Event handlers should be fast (<1ms), heavy work needs async processing
   - What's unclear: Whether token update needs external validation (API call to test token)
   - Recommendation: Start with synchronous handler (token update is fast), add workqueue only if validation is needed

4. **Exponential backoff parameters for watch reconnection**
   - What we know: Informer has built-in reconnection, cenkalti/backoff provides configurable backoff
   - What's unclear: Whether informer's default backoff is sufficient or needs tuning
   - Recommendation: Use informer's built-in reconnection (already handles backoff), add custom backoff only if logs show excessive retries

## Sources

### Primary (HIGH confidence)
- [k8s.io/client-go/informers](https://pkg.go.dev/k8s.io/client-go/informers) - Official Go package documentation
- [kubernetes/client-go GitHub](https://github.com/kubernetes/client-go) - Official source code and examples
- [client-go Secret types](https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/core/v1/secret.go) - Secret client interface
- [client-go Secret informer](https://github.com/kubernetes/client-go/blob/master/informers/core/v1/secret.go) - SecretInformer implementation
- [Go sync package](https://pkg.go.dev/sync) - Official RWMutex documentation

### Secondary (MEDIUM confidence)
- [Extend Kubernetes via a shared informer (CNCF)](https://www.cncf.io/blog/2019/10/15/extend-kubernetes-via-a-shared-informer/) - 2019 official CNCF blog
- [Kubernetes Informer example code](https://github.com/feiskyer/kubernetes-handbook/blob/master/examples/client/informer/informer.go) - Community examples
- [Understanding Locks & RWMutex in Golang](https://medium.com/@anto_rayen/understanding-locks-rwmutex-in-golang-3c468c65062a) - Verified with Go docs
- [Atomic ConfigMap Updates via Symlinks (ITNEXT)](https://itnext.io/atomic-configmap-updates-in-kubernetes-how-symlinks-and-kubelet-make-it-happen-21a44338c247) - Kubernetes internals
- [Configuring Kubernetes RBAC Guide](https://medium.com/@subhampradhan966/configuring-kubernetes-rbac-a-comprehensive-guide-b6d40ac7b257) - RBAC patterns verified with k8s.io docs
- [Keep passwords and secrets out of logs (Medium)](https://medium.com/hackernoon/keep-passwords-and-secrets-out-of-your-logs-with-go-a2294a9546ce) - String() redaction pattern
- [How to Decode Kubernetes Secret (Baeldung)](https://www.baeldung.com/ops/kubernetes-decode-secret) - Verified with client-go behavior

### Tertiary (LOW confidence - WebSearch only)
- [Stakater Reloader GitHub](https://github.com/stakater/Reloader) - Example secret hot-reload operator
- [RWMutex performance comparison](https://gist.github.com/dim/152e6bf80e1384ea72e17ac717a5000a) - Benchmark gist, not official
- [Goroutine leak debugging](https://medium.com/uckey/memory-goroutine-leak-with-rancher-kubernetes-custom-controller-with-client-go-9e296c815209) - Community experience
- [Kubernetes watch 410 Gone handling](https://github.com/kubernetes/kubernetes/issues/25151) - GitHub issue discussion

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - client-go is official, project already uses v0.34.0, version compatibility verified
- Architecture: HIGH - SharedInformerFactory pattern is documented in official client-go docs and used by all k8s operators
- Pitfalls: HIGH - Informer goroutine leaks, 410 Gone, race conditions are well-documented in kubernetes/kubernetes issues
- Secret decoding: HIGH - client-go behavior verified in official GitHub issue #651 and code
- Token redaction: MEDIUM - String() pattern is idiomatic Go but not officially documented for secrets specifically
- Backoff parameters: LOW - Informer has built-in backoff but exact parameters not clearly documented

**Research date:** 2026-01-22
**Valid until:** 2026-03-22 (60 days - client-go is stable, informer pattern unchanged for years)
