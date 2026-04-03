package victorialogs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logprocessing"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Start initializes the integration and validates connectivity.
func (v *VictoriaLogsIntegration) Start(ctx context.Context) error {
	v.logger.Info("Starting VictoriaLogs integration: %s (url: %s)", v.name, v.config.URL)

	v.metrics = NewMetrics(prometheus.DefaultRegisterer, v.name)

	if v.config.UsesSecretRef() {
		v.logger.Info("Creating SecretWatcher for secret: %s, key: %s",
			v.config.APITokenRef.SecretName, v.config.APITokenRef.Key)

		k8sConfig, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
		}
		namespace, err := getCurrentNamespace()
		if err != nil {
			return fmt.Errorf("failed to determine namespace: %w", err)
		}

		secretWatcher, err := NewSecretWatcher(
			clientset,
			namespace,
			v.config.APITokenRef.SecretName,
			v.config.APITokenRef.Key,
			v.logger,
		)
		if err != nil {
			return fmt.Errorf("failed to create secret watcher: %w", err)
		}
		if err := secretWatcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start secret watcher: %w", err)
		}

		v.secretWatcher = secretWatcher
		v.logger.Info("SecretWatcher started successfully")
	}

	v.client = NewClient(v.config.URL, 60*time.Second, v.secretWatcher)

	v.pipeline = NewPipeline(v.client, v.metrics, v.name)
	if err := v.pipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	drainConfig := logprocessing.DrainConfig{
		LogClusterDepth: 4,
		SimTh:           0.4,
		MaxChildren:     100,
	}
	v.templateStore = logprocessing.NewTemplateStore(drainConfig)
	v.logger.Info("Template store initialized with Drain config: depth=%d, simTh=%.2f", drainConfig.LogClusterDepth, drainConfig.SimTh)

	if err := v.testConnection(ctx); err != nil {
		v.logger.Warn("Failed initial connectivity test (will retry on health checks): %v", err)
		v.setHealthStatus(integration.Degraded)
	} else {
		v.setHealthStatus(integration.Healthy)
	}

	v.logger.Info("VictoriaLogs integration started successfully (health: %s)", v.getHealthStatus().String())
	return nil
}

// Stop gracefully shuts down the integration.
func (v *VictoriaLogsIntegration) Stop(ctx context.Context) error {
	v.logger.Info("Stopping VictoriaLogs integration: %s", v.name)

	if v.pipeline != nil {
		if err := v.pipeline.Stop(ctx); err != nil {
			v.logger.Error("Error stopping pipeline: %v", err)
		}
	}
	if v.secretWatcher != nil {
		if err := v.secretWatcher.Stop(); err != nil {
			v.logger.Error("Error stopping secret watcher: %v", err)
		}
	}
	if v.metrics != nil {
		v.metrics.Unregister()
	}

	v.client = nil
	v.pipeline = nil
	v.metrics = nil
	v.templateStore = nil
	v.secretWatcher = nil
	v.setHealthStatus(integration.Stopped)

	v.logger.Info("VictoriaLogs integration stopped")
	return nil
}

// testConnection tests connectivity to VictoriaLogs by executing a minimal query.
func (v *VictoriaLogsIntegration) testConnection(ctx context.Context) error {
	params := QueryParams{
		TimeRange: DefaultTimeRange(),
		Limit:     1,
	}

	_, err := v.client.QueryLogs(ctx, params)
	if err != nil {
		return fmt.Errorf("connectivity test failed: %w", err)
	}

	return nil
}

// getCurrentNamespace reads the namespace from the ServiceAccount mount.
func getCurrentNamespace() (string, error) {
	const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
