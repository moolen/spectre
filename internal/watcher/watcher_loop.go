package watcher

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// startGVRWatcher starts a watcher for a single GVR (watching all namespaces for namespaced resources).
//
//nolint:unparam // error return kept for interface consistency, errors are logged instead
func (w *Watcher) startGVRWatcher(ctx context.Context, gvr schema.GroupVersionResource, namespaced bool, kind string) error {
	watcherKey := watcherKeyForGVR(gvr)
	watcherCtx, cancel := context.WithCancel(ctx)

	w.watchersMutex.Lock()
	w.watchers[watcherKey] = cancel
	w.watchersMutex.Unlock()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer cancel()

		namespace := ""
		if namespaced {
			w.logger.Info("Starting watcher for %s (all namespaces, filtering client-side)", watcherKey)
		} else {
			w.logger.Info("Starting watcher for %s (cluster-scoped)", watcherKey)
		}

		if err := w.watchLoop(watcherCtx, gvr, namespace, kind, namespaced); err != nil && watcherCtx.Err() == nil {
			w.logger.Error("Watcher for %s failed: %v", watcherKey, err)
		}
	}()

	return nil
}

// watchLoop performs a raw List/Watch loop for a resource without caching.
func (w *Watcher) watchLoop(ctx context.Context, gvr schema.GroupVersionResource, namespace, kind string, namespaced bool) error {
	w.logger.Debug("Starting watchLoop for kind=%s gvr=%s namespace=%q namespaced=%v",
		kind, gvr.String(), namespace, namespaced)

	resourceInterface := w.resourceInterfaceForNamespace(gvr, namespace)
	gvk := schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    kind,
	}

	allowedNamespaces := w.getAllowedNamespaces(gvr)
	shouldProcess := namespaceFilter(namespaced, allowedNamespaces)

	for {
		if err := w.checkWatchContext(ctx); err != nil {
			return err
		}

		resourceVersion, err := w.listAndProcessResources(ctx, resourceInterface, gvr, gvk, shouldProcess)
		if err != nil {
			w.logger.Error("Failed to list resources %s: %v, retrying in 5s", gvr.String(), err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err := w.watchResourceStream(ctx, resourceInterface, gvr, gvk, resourceVersion, shouldProcess); err != nil {
			if err == errWatcherStopped || err == ctx.Err() {
				return err
			}
			w.logger.Error("Watch loop for %s failed: %v", gvr.String(), err)
		}

		time.Sleep(1 * time.Second)
	}
}

func (w *Watcher) resourceInterfaceForNamespace(gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return w.dynamicClient.Resource(gvr)
	}
	return w.dynamicClient.Resource(gvr).Namespace(namespace)
}

func (w *Watcher) getAllowedNamespaces(gvr schema.GroupVersionResource) map[string]bool {
	gvrString := watcherKeyForGVR(gvr)
	w.namespaceMutex.RLock()
	defer w.namespaceMutex.RUnlock()
	return w.namespaceFilters[gvrString]
}

func namespaceFilter(namespaced bool, allowedNamespaces map[string]bool) func(string) bool {
	return func(objNamespace string) bool {
		if !namespaced {
			return true
		}
		if len(allowedNamespaces) == 0 {
			return true
		}
		return allowedNamespaces[objNamespace]
	}
}

var errWatcherStopped = fmt.Errorf("watcher stopped")

func (w *Watcher) checkWatchContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.stopChan:
		return errWatcherStopped
	default:
		return nil
	}
}

func (w *Watcher) listAndProcessResources(
	ctx context.Context,
	resourceInterface dynamic.ResourceInterface,
	gvr schema.GroupVersionResource,
	gvk schema.GroupVersionKind,
	shouldProcess func(string) bool,
) (string, error) {
	list, err := resourceInterface.List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return "", err
	}

	if err := w.processListedItems(ctx, list.Items, gvk, "initial", shouldProcess); err != nil {
		return "", err
	}

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

		if err := w.processListedItems(ctx, list.Items, gvk, "paginated", shouldProcess); err != nil {
			return "", err
		}

		resourceVersion = list.GetResourceVersion()
	}

	return resourceVersion, nil
}

func (w *Watcher) processListedItems(
	ctx context.Context,
	items []unstructured.Unstructured,
	gvk schema.GroupVersionKind,
	source string,
	shouldProcess func(string) bool,
) error {
	for i := range items {
		if err := w.checkWatchContext(ctx); err != nil {
			return err
		}

		if !shouldProcess(items[i].GetNamespace()) {
			continue
		}

		items[i].SetGroupVersionKind(gvk)
		w.logger.Debug("Processing %s List item: kind=%s name=%s namespace=%s",
			source, gvk.Kind, items[i].GetName(), items[i].GetNamespace())

		if err := w.eventHandler.OnAdd(&items[i]); err != nil {
			w.logger.Error("Error handling Add event: %v", err)
		}
	}

	return nil
}

func (w *Watcher) watchResourceStream(
	ctx context.Context,
	resourceInterface dynamic.ResourceInterface,
	gvr schema.GroupVersionResource,
	gvk schema.GroupVersionKind,
	resourceVersion string,
	shouldProcess func(string) bool,
) error {
	watcher, err := resourceInterface.Watch(ctx, metav1.ListOptions{ResourceVersion: resourceVersion})
	if err != nil {
		return fmt.Errorf("failed to start watch for %s: %w", gvr.String(), err)
	}
	defer watcher.Stop()

	watchCh := watcher.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopChan:
			return errWatcherStopped
		case event, ok := <-watchCh:
			if !ok {
				w.logger.Debug("Watch channel closed for %s, restarting", gvr.String())
				return nil
			}
			if event.Type == watch.Error {
				w.logger.Error("Watch error for %s: %v", gvr.String(), event.Object)
				return nil
			}
			w.processWatchEvent(event, gvk, shouldProcess)
		}
	}
}

func (w *Watcher) processWatchEvent(
	event watch.Event,
	gvk schema.GroupVersionKind,
	shouldProcess func(string) bool,
) {
	unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		w.logger.Warn("Received non-unstructured object in watch event")
		return
	}

	if !shouldProcess(unstructuredObj.GetNamespace()) {
		w.logger.Debug("Skipping event: %s %s/%s", event.Type, unstructuredObj.GetNamespace(), unstructuredObj.GetName())
		return
	}

	unstructuredObj.SetGroupVersionKind(gvk)

	switch event.Type {
	case watch.Added:
		if err := w.eventHandler.OnAdd(unstructuredObj); err != nil {
			w.logger.Error("Error handling Add event: %v", err)
		}
	case watch.Modified:
		if err := w.eventHandler.OnUpdate(unstructuredObj, unstructuredObj); err != nil {
			w.logger.Error("Error handling Update event: %v", err)
		}
	case watch.Deleted:
		if err := w.eventHandler.OnDelete(unstructuredObj); err != nil {
			w.logger.Error("Error handling Delete event: %v", err)
		}
	case watch.Bookmark:
	case watch.Error:
		w.logger.Error("Watch error event received: %v", event.Object)
	}
}
