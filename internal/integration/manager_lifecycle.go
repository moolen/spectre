package integration

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/config"
)

// Start initializes the manager and starts all enabled integration instances.
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting integration manager")

	integrationsFile, err := config.LoadIntegrationsFile(m.config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load integrations config: %w", err)
	}
	if err := m.startInstances(ctx, integrationsFile); err != nil {
		return err
	}

	watcherConfig := config.IntegrationWatcherConfig{
		FilePath:       m.config.ConfigPath,
		DebounceMillis: 500,
	}
	m.watcher, err = config.NewIntegrationWatcher(watcherConfig, m.handleConfigReload)
	if err != nil {
		m.stopAllInstances(ctx)
		return fmt.Errorf("failed to create config watcher: %w", err)
	}
	if err := m.watcher.Start(ctx); err != nil {
		m.stopAllInstances(ctx)
		return fmt.Errorf("failed to start config watcher: %w", err)
	}

	healthCtx, cancel := context.WithCancel(context.Background())
	m.healthCancel = cancel
	go m.runHealthChecks(healthCtx)

	m.logger.Info("Integration manager started successfully with %d instances", len(m.registry.List()))
	return nil
}

// Stop gracefully stops the manager, config watcher, and all integration instances.
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Stopping integration manager")

	if m.healthCancel != nil {
		m.healthCancel()
	}
	if m.watcher != nil {
		if err := m.watcher.Stop(); err != nil {
			m.logger.Warn("Error stopping config watcher: %v", err)
		}
	}

	m.stopAllInstances(ctx)
	close(m.stopped)

	m.logger.Info("Integration manager stopped")
	return nil
}
