package watcher

import (
	"context"
	"sync"

	"github.com/moolen/spectre/internal/logging"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// pendingResource represents a resource that failed CRD discovery and needs retry
type pendingResource struct {
	Group     string
	Version   string
	Kind      string
	Namespace string // Optional namespace filter
}

// Watcher monitors Kubernetes resources for changes
type Watcher struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	restConfig      *rest.Config
	configPath      string
	stopChan        chan struct{}
	wg              sync.WaitGroup
	logger          *logging.Logger
	eventHandler    EventHandler
	watchers        map[string]context.CancelFunc // Track active watchers by key
	watchersMutex   sync.RWMutex
	// namespaceFilters maps GVR string to set of allowed namespaces (empty set means all namespaces)
	namespaceFilters map[string]map[string]bool
	namespaceMutex   sync.RWMutex

	// Pending resources that failed CRD discovery (CRD not installed yet)
	pendingResources []pendingResource
	pendingMutex     sync.RWMutex

	// Readiness tracking
	readinessMutex      sync.RWMutex
	initialLoadComplete bool // Flag to indicate initial load is done (prevents reset on hot-reload)
}

// EventHandler is called when a resource event occurs
type EventHandler interface {
	// OnAdd is called when a resource is created
	OnAdd(obj runtime.Object) error

	// OnUpdate is called when a resource is updated
	OnUpdate(oldObj, newObj runtime.Object) error

	// OnDelete is called when a resource is deleted
	OnDelete(obj runtime.Object) error
}

type gvrKey struct {
	group    string
	version  string
	kind     string
	resource string
}

type gvrInfo struct {
	gvr        schema.GroupVersionResource
	namespaced bool
	namespaces map[string]bool
	kind       string
}
