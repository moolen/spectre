package victorialogs

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/integration"
)

// Health returns the current cached health status.
func (v *VictoriaLogsIntegration) Health(ctx context.Context) integration.HealthStatus {
	if v.client == nil {
		return integration.Stopped
	}
	if v.secretWatcher != nil && !v.secretWatcher.IsHealthy() {
		v.setHealthStatus(integration.Degraded)
		return integration.Degraded
	}

	return v.getHealthStatus()
}

// CheckConnectivity implements integration.ConnectivityChecker.
func (v *VictoriaLogsIntegration) CheckConnectivity(ctx context.Context) error {
	if v.client == nil {
		v.setHealthStatus(integration.Stopped)
		return fmt.Errorf("client not initialized")
	}
	if err := v.testConnection(ctx); err != nil {
		v.setHealthStatus(integration.Degraded)
		return err
	}

	v.setHealthStatus(integration.Healthy)
	return nil
}

// setHealthStatus updates the health status in a thread-safe manner.
func (v *VictoriaLogsIntegration) setHealthStatus(status integration.HealthStatus) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.healthStatus = status
}

// getHealthStatus retrieves the health status in a thread-safe manner.
func (v *VictoriaLogsIntegration) getHealthStatus() integration.HealthStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.healthStatus
}
