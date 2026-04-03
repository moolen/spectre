package integration

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/moolen/spectre/internal/config"
)

// startInstances validates versions and starts all enabled instances from config.
func (m *Manager) startInstances(ctx context.Context, integrationsFile *config.IntegrationsFile) error {
	m.logger.Info("Starting %d integration instance(s)", len(integrationsFile.Instances))

	for _, instanceConfig := range integrationsFile.Instances {
		if !instanceConfig.Enabled {
			m.logger.Debug("Skipping disabled instance: %s", instanceConfig.Name)
			continue
		}

		factory, ok := GetFactory(instanceConfig.Type)
		if !ok {
			m.logger.Error("No factory registered for integration type %q (instance: %s)",
				instanceConfig.Type, instanceConfig.Name)
			continue
		}

		instance, err := factory(instanceConfig.Name, instanceConfig.Config)
		if err != nil {
			m.logger.Error("Failed to create instance %s (type: %s): %v",
				instanceConfig.Name, instanceConfig.Type, err)
			continue
		}

		if err := m.validateInstanceVersion(instance); err != nil {
			return err
		}

		if m.graphClient != nil {
			if setter, ok := instance.(GraphClientSetter); ok {
				setter.SetGraphClient(m.graphClient)
				m.logger.Debug("Injected graph client into instance: %s", instanceConfig.Name)
			}
		}

		if err := m.registry.Register(instanceConfig.Name, instance); err != nil {
			m.logger.Error("Failed to register instance %s: %v", instanceConfig.Name, err)
			continue
		}

		if err := instance.Start(ctx); err != nil {
			m.logger.Error("Failed to start instance %s: %v (marking as degraded)", instanceConfig.Name, err)
		} else {
			m.logger.Info("Started instance: %s (type: %s, version: %s)",
				instanceConfig.Name, instanceConfig.Type, instance.Metadata().Version)
		}

		if m.mcpRegistry != nil {
			if err := instance.RegisterTools(m.mcpRegistry); err != nil {
				m.logger.Error("Failed to register tools for %s: %v", instanceConfig.Name, err)
			}
		}
	}

	return nil
}

// validateInstanceVersion checks if instance version meets minimum requirements.
func (m *Manager) validateInstanceVersion(instance Integration) error {
	if m.minVersion == nil {
		return nil
	}

	metadata := instance.Metadata()
	instanceVer, err := version.NewVersion(metadata.Version)
	if err != nil {
		return fmt.Errorf("instance %s has invalid version %q: %w",
			metadata.Name, metadata.Version, err)
	}
	if instanceVer.LessThan(m.minVersion) {
		return fmt.Errorf("instance %s version %s is below minimum required version %s",
			metadata.Name, metadata.Version, m.minVersion.String())
	}

	m.logger.Debug("Instance %s version %s validated (>= %s)",
		metadata.Name, metadata.Version, m.minVersion.String())
	return nil
}
