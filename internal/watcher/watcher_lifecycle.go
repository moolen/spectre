package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/moolen/spectre/internal/logging"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// New creates a new Watcher instance.
func New(handler EventHandler, configPath string) (*Watcher, error) {
	logger := logging.GetLogger("watcher")

	restConfig, err := buildClientConfig()
	if err != nil {
		logger.Error("Failed to build Kubernetes client config: %v", err)
		return nil, err
	}

	logger.Info("restConfig.ServerName: %s", restConfig.ServerName)

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		logger.Error("Failed to create dynamic client: %v", err)
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		logger.Error("Failed to create discovery client: %v", err)
		return nil, err
	}

	w := &Watcher{
		dynamicClient:       dynamicClient,
		discoveryClient:     discoveryClient,
		restConfig:          restConfig,
		configPath:          configPath,
		stopChan:            make(chan struct{}),
		logger:              logger,
		eventHandler:        handler,
		watchers:            make(map[string]context.CancelFunc),
		namespaceFilters:    make(map[string]map[string]bool),
		initialLoadComplete: false,
	}

	logger.Info("Watcher created successfully")
	return w, nil
}

// Start begins monitoring the configured resource types.
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("Starting watchers from config file: %s", w.configPath)

	w.wg.Add(1)
	go w.hotReloadLoop(ctx)

	w.wg.Add(1)
	go w.crdDiscoveryRetryLoop(ctx)

	if err := w.loadAndStartWatchers(ctx); err != nil {
		return fmt.Errorf("failed to load initial watchers: %w", err)
	}

	return nil
}

// Stop implements the lifecycle.Component interface.
func (w *Watcher) Stop(ctx context.Context) error {
	w.logger.Info("Stopping watcher component")

	w.stopActiveWatchers()
	close(w.stopChan)

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

// GetRestConfig returns the Kubernetes REST config used by this watcher.
func (w *Watcher) GetRestConfig() *rest.Config {
	return w.restConfig
}

// Name implements the lifecycle.Component interface.
func (w *Watcher) Name() string {
	return "Watcher"
}

// IsReady returns true when all expected watchers are started and have completed initial List processing.
func (w *Watcher) IsReady() bool {
	w.readinessMutex.RLock()
	defer w.readinessMutex.RUnlock()
	return w.initialLoadComplete
}

func (w *Watcher) stopActiveWatchers() {
	w.watchersMutex.Lock()
	defer w.watchersMutex.Unlock()

	for key, cancel := range w.watchers {
		w.logger.Debug("Stopping watcher: %s", key)
		cancel()
	}
	w.watchers = make(map[string]context.CancelFunc)
}

func hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func buildClientConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

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
