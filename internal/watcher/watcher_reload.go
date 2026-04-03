package watcher

import (
	"context"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// hotReloadLoop polls the config file for changes and reloads watchers.
func (w *Watcher) hotReloadLoop(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastContentHash string
	if content, err := os.ReadFile(w.configPath); err == nil {
		lastContentHash = hashContent(content)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			content, err := os.ReadFile(w.configPath)
			if err != nil {
				w.logger.Warn("Failed to read config file %s: %v", w.configPath, err)
				continue
			}

			currentHash := hashContent(content)
			if currentHash == lastContentHash {
				continue
			}

			w.logger.Info("Config file changed, reloading watchers")
			lastContentHash = currentHash

			if err := w.loadAndStartWatchers(ctx); err != nil {
				w.logger.Error("Failed to reload watchers: %v", err)
			} else {
				w.logger.Info("Watchers reloaded successfully")
			}
		}
	}
}

// crdDiscoveryRetryLoop periodically retries starting watchers for resources whose CRDs were not available at startup.
func (w *Watcher) crdDiscoveryRetryLoop(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.retryPendingResources(ctx)
		}
	}
}

// retryPendingResources attempts to resolve and start watchers for pending resources.
func (w *Watcher) retryPendingResources(ctx context.Context) {
	pending := w.drainPendingResources()
	if len(pending) == 0 {
		return
	}

	w.logger.Info("Retrying CRD discovery for %d pending resources", len(pending))

	gvrMap := make(map[gvrKey]*gvrInfo)
	stillPending := make([]pendingResource, 0, len(pending))

	for _, resource := range pending {
		gvr, namespaced, err := w.resolveGVR(schema.GroupVersionKind{
			Group:   resource.Group,
			Version: resource.Version,
			Kind:    resource.Kind,
		})
		if err != nil {
			stillPending = append(stillPending, resource)
			continue
		}

		w.logger.Info("CRD now available: %s/%s/%s", resource.Group, resource.Version, resource.Kind)

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

	w.restorePendingResources(stillPending)
	w.startNewlyAvailableWatchers(ctx, gvrMap)
}

func (w *Watcher) drainPendingResources() []pendingResource {
	w.pendingMutex.Lock()
	defer w.pendingMutex.Unlock()

	if len(w.pendingResources) == 0 {
		return nil
	}

	pending := make([]pendingResource, len(w.pendingResources))
	copy(pending, w.pendingResources)
	w.pendingResources = nil
	return pending
}

func (w *Watcher) restorePendingResources(stillPending []pendingResource) {
	if len(stillPending) == 0 {
		return
	}

	w.pendingMutex.Lock()
	w.pendingResources = append(w.pendingResources, stillPending...)
	w.pendingMutex.Unlock()
	w.logger.Debug("Still waiting for %d CRDs to become available", len(stillPending))
}

func (w *Watcher) startNewlyAvailableWatchers(ctx context.Context, gvrMap map[gvrKey]*gvrInfo) {
	for key, info := range gvrMap {
		watcherKey := formatGVRString(key.group, key.version, key.resource)
		if w.hasActiveWatcher(watcherKey) {
			w.logger.Debug("Watcher already exists for %s, skipping", watcherKey)
			continue
		}

		gvrString := formatGVRString(key.group, key.version, key.resource)
		w.storeNamespaceFilter(gvrString, info.namespaces)

		if err := w.startGVRWatcher(ctx, info.gvr, info.namespaced, info.kind); err != nil {
			w.logger.Error("Failed to start watcher for %s: %v", gvrString, err)
		} else {
			w.logger.Info("Started watcher for newly available CRD: %s", gvrString)
		}
	}
}

func (w *Watcher) hasActiveWatcher(watcherKey string) bool {
	w.watchersMutex.RLock()
	defer w.watchersMutex.RUnlock()
	_, exists := w.watchers[watcherKey]
	return exists
}
