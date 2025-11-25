package watcher

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/moritz/rpk/internal/logging"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Watcher monitors Kubernetes resources for changes
type Watcher struct {
	clientset     kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	stopChan      chan struct{}
	wg            sync.WaitGroup
	logger        *logging.Logger
	eventHandler  EventHandler
	resourceTypes []string
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
func New(handler EventHandler, resourceTypes []string) (*Watcher, error) {
	logger := logging.GetLogger("watcher")

	// Create Kubernetes client config
	config, err := buildClientConfig()
	if err != nil {
		logger.Error("Failed to build Kubernetes client config: %v", err)
		return nil, err
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("Failed to create Kubernetes clientset: %v", err)
		return nil, err
	}

	// Create informer factory
	informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	w := &Watcher{
		clientset:       clientset,
		informerFactory: informerFactory,
		stopChan:        make(chan struct{}),
		logger:          logger,
		eventHandler:    handler,
		resourceTypes:   resourceTypes,
	}

	logger.Info("Watcher created successfully")
	return w, nil
}

// Start begins monitoring the configured resource types
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("Starting watchers for resource types: %v", w.resourceTypes)

	// Register informers for built-in resource types
	for _, resourceType := range w.resourceTypes {
		if err := w.registerResourceWatcher(resourceType); err != nil {
			w.logger.Error("Failed to register watcher for %s: %v", resourceType, err)
			// Continue with other resource types even if one fails
		}
	}

	// Start all informers
	w.informerFactory.Start(w.stopChan)

	// Wait for cache to sync
	w.logger.Info("Waiting for informer cache to sync...")
	if !cache.WaitForCacheSync(w.stopChan, w.informerFactory.WaitForCacheSync()...) {
		w.logger.Error("Failed to wait for cache sync")
		return fmt.Errorf("failed to wait for informer cache sync")
	}

	w.logger.Info("Informer cache synchronized, watchers are active")
	return nil
}

// Stop implements the lifecycle.Component interface
// Gracefully shuts down the watcher component
func (w *Watcher) Stop(ctx context.Context) error {
	w.logger.Info("Stopping watcher component")

	done := make(chan struct{}, 1)

	go func() {
		close(w.stopChan)
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

// registerResourceWatcher registers a watcher for a specific resource type
func (w *Watcher) registerResourceWatcher(resourceType string) error {
	w.logger.Debug("Registering watcher for resource type: %s", resourceType)

	switch resourceType {
	case "Pod":
		informer := w.informerFactory.Core().V1().Pods().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("Pod", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("Pod", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("Pod", obj) },
		})

	case "Deployment":
		informer := w.informerFactory.Apps().V1().Deployments().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("Deployment", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("Deployment", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("Deployment", obj) },
		})

	case "Service":
		informer := w.informerFactory.Core().V1().Services().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("Service", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("Service", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("Service", obj) },
		})

	case "Node":
		informer := w.informerFactory.Core().V1().Nodes().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("Node", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("Node", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("Node", obj) },
		})

	case "StatefulSet":
		informer := w.informerFactory.Apps().V1().StatefulSets().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("StatefulSet", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("StatefulSet", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("StatefulSet", obj) },
		})

	case "DaemonSet":
		informer := w.informerFactory.Apps().V1().DaemonSets().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("DaemonSet", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("DaemonSet", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("DaemonSet", obj) },
		})

	case "ConfigMap":
		informer := w.informerFactory.Core().V1().ConfigMaps().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("ConfigMap", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("ConfigMap", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("ConfigMap", obj) },
		})

	case "Secret":
		informer := w.informerFactory.Core().V1().Secrets().Informer()
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { w.handleAdd("Secret", obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { w.handleUpdate("Secret", oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { w.handleDelete("Secret", obj) },
		})

	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	w.logger.Debug("Registered watcher for %s", resourceType)
	return nil
}

// handleAdd is called when a resource is created
func (w *Watcher) handleAdd(kind string, obj interface{}) {
	if err := w.eventHandler.OnAdd(obj.(runtime.Object)); err != nil {
		w.logger.Error("Error handling Add event for %s: %v", kind, err)
	}
}

// handleUpdate is called when a resource is updated
func (w *Watcher) handleUpdate(kind string, oldObj, newObj interface{}) {
	if err := w.eventHandler.OnUpdate(oldObj.(runtime.Object), newObj.(runtime.Object)); err != nil {
		w.logger.Error("Error handling Update event for %s: %v", kind, err)
	}
}

// handleDelete is called when a resource is deleted
func (w *Watcher) handleDelete(kind string, obj interface{}) {
	if err := w.eventHandler.OnDelete(obj.(runtime.Object)); err != nil {
		w.logger.Error("Error handling Delete event for %s: %v", kind, err)
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
