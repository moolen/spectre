package integration

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/config"
)

// handleConfigReload is called when the config file changes.
func (m *Manager) handleConfigReload(newConfig *config.IntegrationsFile) error {
	m.logger.Info("Config reload triggered - restarting all integration instances")

	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), m.config.ShutdownTimeout)
	defer cancel()
	m.stopAllInstancesLocked(ctx)

	instanceNames := m.registry.List()
	for _, name := range instanceNames {
		m.registry.Remove(name)
	}

	if err := m.startInstances(context.Background(), newConfig); err != nil {
		m.logger.Error("Failed to start instances after config reload: %v", err)
		return err
	}

	m.logger.Info("Config reload complete - %d instances running", len(m.registry.List()))
	return nil
}

// runHealthChecks periodically checks instance health and attempts auto-recovery.
func (m *Manager) runHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	m.logger.Debug("Health check loop started (interval: %s)", m.config.HealthCheckInterval)

	for {
		select {
		case <-ctx.Done():
			m.logger.Debug("Health check loop stopped")
			return
		case <-ticker.C:
			m.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks checks health of all instances and attempts recovery.
func (m *Manager) performHealthChecks(ctx context.Context) {
	m.mu.RLock()
	instanceNames := m.registry.List()
	m.mu.RUnlock()

	for _, name := range instanceNames {
		m.mu.RLock()
		instance, ok := m.registry.Get(name)
		m.mu.RUnlock()
		if !ok {
			continue
		}

		if checker, ok := instance.(ConnectivityChecker); ok {
			if err := checker.CheckConnectivity(ctx); err != nil {
				m.logger.Debug("Connectivity check failed for instance %s: %v", name, err)
			}
		}

		if instance.Health(ctx) == Degraded {
			m.logger.Debug("Instance %s is degraded, attempting recovery", name)
			if err := instance.Start(ctx); err != nil {
				m.logger.Debug("Recovery failed for instance %s: %v", name, err)
			} else {
				m.logger.Info("Instance %s recovered successfully", name)
			}
		}
	}
}

// stopAllInstances stops all registered instances with timeout.
func (m *Manager) stopAllInstances(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopAllInstancesLocked(ctx)
}

// stopAllInstancesLocked stops all instances - caller must hold write lock.
func (m *Manager) stopAllInstancesLocked(ctx context.Context) {
	instanceNames := m.registry.List()
	m.logger.Debug("Stopping %d instance(s)", len(instanceNames))

	for _, name := range instanceNames {
		instance, ok := m.registry.Get(name)
		if !ok {
			continue
		}

		stopCtx, cancel := context.WithTimeout(ctx, m.config.ShutdownTimeout)
		if err := instance.Stop(stopCtx); err != nil {
			m.logger.Warn("Error stopping instance %s: %v", name, err)
		} else {
			m.logger.Debug("Stopped instance: %s", name)
		}
		cancel()
	}
}
