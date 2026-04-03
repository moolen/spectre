package watcher

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/config"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// loadAndStartWatchers loads the config and starts watchers for all resources.
func (w *Watcher) loadAndStartWatchers(ctx context.Context) error {
	watcherConfig, err := config.LoadWatcherConfig(w.configPath)
	if err != nil {
		return fmt.Errorf("failed to load watcher config: %w", err)
	}

	w.logger.Info("Loaded %d resource configurations", len(watcherConfig.Resources))

	w.resetWatcherState()

	gvrMap := w.collectConfiguredResources(watcherConfig.Resources)
	w.startConfiguredWatchers(ctx, gvrMap)
	w.markInitialLoadComplete()

	return nil
}

// resolveGVR resolves a GroupVersionKind to a GroupVersionResource using the discovery client.
func (w *Watcher) resolveGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool, error) {
	apiVersion := gvk.GroupVersion().String()
	if gvk.Group == "" {
		apiVersion = gvk.Version
	}

	apiResourceList, err := w.discoveryClient.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("failed to get server resources for %s: %w", apiVersion, err)
	}

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

func (w *Watcher) resetWatcherState() {
	w.stopExistingWatchersForReload()

	w.namespaceMutex.Lock()
	w.namespaceFilters = make(map[string]map[string]bool)
	w.namespaceMutex.Unlock()

	w.pendingMutex.Lock()
	w.pendingResources = nil
	w.pendingMutex.Unlock()
}

func (w *Watcher) stopExistingWatchersForReload() {
	w.watchersMutex.Lock()
	defer w.watchersMutex.Unlock()

	for key, cancel := range w.watchers {
		w.logger.Debug("Stopping existing watcher: %s", key)
		cancel()
		delete(w.watchers, key)
	}
}

func (w *Watcher) collectConfiguredResources(resources []config.Resource) map[gvrKey]*gvrInfo {
	gvrMap := make(map[gvrKey]*gvrInfo)

	for _, resource := range resources {
		gvr, namespaced, err := w.resolveGVR(schema.GroupVersionKind{
			Group:   resource.Group,
			Version: resource.Version,
			Kind:    resource.Kind,
		})
		if err != nil {
			w.logger.Warn("Failed to resolve GVR for %s/%s/%s: %v (will retry periodically)", resource.Group, resource.Version, resource.Kind, err)
			w.pendingMutex.Lock()
			w.pendingResources = append(w.pendingResources, pendingResource{
				Group:     resource.Group,
				Version:   resource.Version,
				Kind:      resource.Kind,
				Namespace: resource.Namespace,
			})
			w.pendingMutex.Unlock()
			continue
		}

		key := gvrKey{
			group:    gvr.Group,
			version:  gvr.Version,
			kind:     resource.Kind,
			resource: gvr.Resource,
		}

		info, exists := gvrMap[key]
		if !exists {
			info = &gvrInfo{
				gvr:        gvr,
				namespaced: namespaced,
				namespaces: make(map[string]bool),
				kind:       resource.Kind,
			}
			gvrMap[key] = info
		}

		if namespaced && resource.Namespace != "" {
			info.namespaces[resource.Namespace] = true
		}
	}

	return gvrMap
}

func (w *Watcher) startConfiguredWatchers(ctx context.Context, gvrMap map[gvrKey]*gvrInfo) {
	for key, info := range gvrMap {
		gvrString := formatGVRString(key.group, key.version, key.resource)
		w.storeNamespaceFilter(gvrString, info.namespaces)

		if err := w.startGVRWatcher(ctx, info.gvr, info.namespaced, info.kind); err != nil {
			w.logger.Error("Failed to start watcher for %s: %v", gvrString, err)
		}
	}
}

func (w *Watcher) storeNamespaceFilter(gvrString string, namespaces map[string]bool) {
	w.namespaceMutex.Lock()
	defer w.namespaceMutex.Unlock()

	if len(namespaces) > 0 {
		w.namespaceFilters[gvrString] = namespaces
		return
	}

	w.namespaceFilters[gvrString] = make(map[string]bool)
}

func (w *Watcher) markInitialLoadComplete() {
	w.readinessMutex.Lock()
	w.initialLoadComplete = true
	w.readinessMutex.Unlock()
}

func formatGVRString(group, version, resource string) string {
	return fmt.Sprintf("%s/%s/%s", group, version, resource)
}

func watcherKeyForGVR(gvr schema.GroupVersionResource) string {
	return formatGVRString(gvr.Group, gvr.Version, gvr.Resource)
}
