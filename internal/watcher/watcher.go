package watcher

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/moritz/rpk/internal/config"
	"github.com/moritz/rpk/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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

// New creates a new Watcher instance
func New(handler EventHandler, configPath string) (*Watcher, error) {
	logger := logging.GetLogger("watcher")

	// Create Kubernetes client config
	restConfig, err := buildClientConfig()
	if err != nil {
		logger.Error("Failed to build Kubernetes client config: %v", err)
		return nil, err
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		logger.Error("Failed to create dynamic client: %v", err)
		return nil, err
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		logger.Error("Failed to create discovery client: %v", err)
		return nil, err
	}

	w := &Watcher{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		restConfig:      restConfig,
		configPath:      configPath,
		stopChan:        make(chan struct{}),
		logger:          logger,
		eventHandler:    handler,
		watchers:        make(map[string]context.CancelFunc),
	}

	logger.Info("Watcher created successfully")
	return w, nil
}

// Start begins monitoring the configured resource types
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("Starting watchers from config file: %s", w.configPath)

	// Start the hot-reload goroutine
	w.wg.Add(1)
	go w.hotReloadLoop(ctx)

	// Initial load
	if err := w.loadAndStartWatchers(ctx); err != nil {
		return fmt.Errorf("failed to load initial watchers: %w", err)
	}

	return nil
}

// Stop implements the lifecycle.Component interface
// Gracefully shuts down the watcher component
func (w *Watcher) Stop(ctx context.Context) error {
	w.logger.Info("Stopping watcher component")

	// Stop all active watchers
	w.watchersMutex.Lock()
	for key, cancel := range w.watchers {
		w.logger.Debug("Stopping watcher: %s", key)
		cancel()
	}
	w.watchers = make(map[string]context.CancelFunc)
	w.watchersMutex.Unlock()

	// Close stop channel
	close(w.stopChan)

	// Wait for all goroutines to finish
	done := make(chan struct{}, 1)
	go func() {
		w.wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		w.logger.Info("Watcher component stopped")
		return nil
	case <-ctx.Done():
		w.logger.Warn("Watcher component shutdown timeout")
		return ctx.Err()
	}
}

// loadAndStartWatchers loads the config and starts watchers for all resources
func (w *Watcher) loadAndStartWatchers(ctx context.Context) error {
	// Load watcher config
	watcherConfig, err := config.LoadWatcherConfig(w.configPath)
	if err != nil {
		return fmt.Errorf("failed to load watcher config: %w", err)
	}

	w.logger.Info("Loaded %d resource configurations", len(watcherConfig.Resources))

	// Stop existing watchers
	w.watchersMutex.Lock()
	for key, cancel := range w.watchers {
		w.logger.Debug("Stopping existing watcher: %s", key)
		cancel()
		delete(w.watchers, key)
	}
	w.watchersMutex.Unlock()

	// Start watchers for each resource
	for _, resource := range watcherConfig.Resources {
		if err := w.startResourceWatcher(ctx, resource); err != nil {
			w.logger.Error("Failed to start watcher for %s/%s/%s: %v", resource.Group, resource.Version, resource.Kind, err)
			// Continue with other resources even if one fails
		}
	}

	return nil
}

// startResourceWatcher starts a watcher for a single resource configuration
func (w *Watcher) startResourceWatcher(ctx context.Context, resource config.Resource) error {
	// Resolve GVK to GVR
	gvr, namespaced, err := w.resolveGVR(schema.GroupVersionKind{
		Group:   resource.Group,
		Version: resource.Version,
		Kind:    resource.Kind,
	})
	if err != nil {
		return fmt.Errorf("failed to resolve GVR: %w", err)
	}

	// Create watcher key for tracking
	watcherKey := fmt.Sprintf("%s/%s/%s", resource.Group, resource.Version, resource.Kind)
	if resource.Namespace != "" {
		watcherKey = fmt.Sprintf("%s/%s/%s/%s", resource.Group, resource.Version, resource.Kind, resource.Namespace)
	}

	// Create context for this watcher
	watcherCtx, cancel := context.WithCancel(ctx)

	// Store cancel function
	w.watchersMutex.Lock()
	w.watchers[watcherKey] = cancel
	w.watchersMutex.Unlock()

	// Start the watch loop in a goroutine
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer cancel()

		namespace := resource.Namespace
		if !namespaced {
			// Cluster-scoped resources ignore namespace
			namespace = ""
		}

		w.logger.Info("Starting watcher for %s (namespace: %s)", watcherKey, namespace)
		if err := w.watchLoop(watcherCtx, gvr, namespace, resource.Kind); err != nil {
			if watcherCtx.Err() == nil {
				w.logger.Error("Watcher for %s failed: %v", watcherKey, err)
			}
		}
	}()

	return nil
}

// resolveGVR resolves a GroupVersionKind to a GroupVersionResource using the discovery client
func (w *Watcher) resolveGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool, error) {
	// Determine the API version string
	var apiVersion string
	if gvk.Group == "" {
		// Core resources use just the version
		apiVersion = gvk.Version
	} else {
		apiVersion = gvk.GroupVersion().String()
	}

	// Get API resource list
	apiResourceList, err := w.discoveryClient.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("failed to get server resources for %s: %w", apiVersion, err)
	}

	// Find the resource that matches the kind
	for _, apiResource := range apiResourceList.APIResources {
		if apiResource.Kind == gvk.Kind {
			return schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: apiResource.Name,
			}, apiResource.Namespaced, nil
		}
	}

	return schema.GroupVersionResource{}, false, fmt.Errorf("resource kind %s not found in API group %s/%s", gvk.Kind, gvk.Group, gvk.Version)
}

// watchLoop performs a raw List/Watch loop for a resource without caching
func (w *Watcher) watchLoop(ctx context.Context, gvr schema.GroupVersionResource, namespace, kind string) error {
	// Get the resource interface
	var resourceInterface dynamic.ResourceInterface
	if namespace == "" {
		resourceInterface = w.dynamicClient.Resource(gvr)
	} else {
		resourceInterface = w.dynamicClient.Resource(gvr).Namespace(namespace)
	}

	// Retry loop for handling connection drops
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopChan:
			return fmt.Errorf("watcher stopped")
		default:
		}

		// Perform List to get initial state and resource version
		list, err := resourceInterface.List(ctx, metav1.ListOptions{
			Limit: 500, // Use pagination for large lists
		})
		if err != nil {
			w.logger.Error("Failed to list resources %s: %v, retrying in 5s", gvr.String(), err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Process initial list as Add events
		items := list.Items
		for i := range items {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-w.stopChan:
				return fmt.Errorf("watcher stopped")
			default:
			}

			if err := w.eventHandler.OnAdd(&items[i]); err != nil {
				w.logger.Error("Error handling Add event: %v", err)
			}
		}

		// Handle pagination if needed
		resourceVersion := list.GetResourceVersion()
		for list.GetContinue() != "" {
			list, err = resourceInterface.List(ctx, metav1.ListOptions{
				Limit:    500,
				Continue: list.GetContinue(),
			})
			if err != nil {
				w.logger.Error("Failed to list resources (pagination) %s: %v, retrying", gvr.String(), err)
				break
			}

			items = list.Items
			for i := range items {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-w.stopChan:
					return fmt.Errorf("watcher stopped")
				default:
				}

				if err := w.eventHandler.OnAdd(&items[i]); err != nil {
					w.logger.Error("Error handling Add event: %v", err)
				}
			}

			resourceVersion = list.GetResourceVersion()
		}

		// Start watching from the resource version
		watcher, err := resourceInterface.Watch(ctx, metav1.ListOptions{
			ResourceVersion: resourceVersion,
		})
		if err != nil {
			w.logger.Error("Failed to start watch for %s: %v, retrying in 5s", gvr.String(), err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Process watch events
		watchCh := watcher.ResultChan()
		watchActive := true
		for watchActive {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return ctx.Err()
			case <-w.stopChan:
				watcher.Stop()
				return fmt.Errorf("watcher stopped")
			case event, ok := <-watchCh:
				if !ok {
					// Channel closed, restart watch
					w.logger.Debug("Watch channel closed for %s, restarting", gvr.String())
					watcher.Stop()
					watchActive = false
					break
				}

				if event.Type == watch.Error {
					w.logger.Error("Watch error for %s: %v", gvr.String(), event.Object)
					watcher.Stop()
					watchActive = false
					break
				}

				// Process the event
				unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					w.logger.Warn("Received non-unstructured object in watch event")
					continue
				}

				switch event.Type {
				case watch.Added:
					if err := w.eventHandler.OnAdd(unstructuredObj); err != nil {
						w.logger.Error("Error handling Add event: %v", err)
					}
				case watch.Modified:
					// For Modified events, we need both old and new objects
					// Since we don't cache, we'll pass the new object as both
					// The event handler should handle this appropriately
					if err := w.eventHandler.OnUpdate(unstructuredObj, unstructuredObj); err != nil {
						w.logger.Error("Error handling Update event: %v", err)
					}
				case watch.Deleted:
					if err := w.eventHandler.OnDelete(unstructuredObj); err != nil {
						w.logger.Error("Error handling Delete event: %v", err)
					}
				}
			}
		}

		// Small delay before restarting
		time.Sleep(1 * time.Second)
	}
}

// hotReloadLoop polls the config file for changes and reloads watchers
func (w *Watcher) hotReloadLoop(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time

	// Get initial mod time
	if info, err := os.Stat(w.configPath); err == nil {
		lastModTime = info.ModTime()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			// Check if file has changed
			info, err := os.Stat(w.configPath)
			if err != nil {
				w.logger.Warn("Failed to stat config file %s: %v", w.configPath, err)
				continue
			}

			if info.ModTime().After(lastModTime) {
				w.logger.Info("Config file changed, reloading watchers")
				lastModTime = info.ModTime()

				if err := w.loadAndStartWatchers(ctx); err != nil {
					w.logger.Error("Failed to reload watchers: %v", err)
				} else {
					w.logger.Info("Watchers reloaded successfully")
				}
			}
		}
	}
}

// Name implements the lifecycle.Component interface
// Returns the human-readable name of the watcher component
func (w *Watcher) Name() string {
	return "Watcher"
}

// buildClientConfig builds the Kubernetes client config
func buildClientConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	kubeconfig := ""
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = fmt.Sprintf("%s/.kube/config", home)
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build client config: %w", err)
	}

	return config, nil
}
