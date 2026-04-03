package integrationadmin

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/config"
)

func (s *Service) List(ctx context.Context) ([]Instance, error) {
	integrationsFile, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	responses := make([]Instance, 0, len(integrationsFile.Instances))
	for _, instance := range integrationsFile.Instances {
		responses = append(responses, s.buildInstance(ctx, instance))
	}

	return responses, nil
}

func (s *Service) Get(ctx context.Context, name string) (*Instance, error) {
	integrationsFile, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	instance, found := findInstance(integrationsFile.Instances, name)
	if !found {
		return nil, newError(codeNotFound, fmt.Sprintf("Integration %q not found", name), nil)
	}

	response := s.buildInstance(ctx, instance)
	return &response, nil
}

func (s *Service) Create(newInstance config.IntegrationConfig) (*Instance, error) {
	integrationsFile, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	for _, instance := range integrationsFile.Instances {
		if instance.Name == newInstance.Name {
			return nil, newError(codeDuplicateName, fmt.Sprintf("Integration %q already exists", newInstance.Name), nil)
		}
	}

	testFile := &config.IntegrationsFile{
		SchemaVersion: integrationsFile.SchemaVersion,
		Instances:     append(integrationsFile.Instances, newInstance),
	}
	if err := testFile.Validate(); err != nil {
		return nil, newError(codeInvalidConfig, fmt.Sprintf("Validation failed: %v", err), err)
	}

	integrationsFile.Instances = append(integrationsFile.Instances, newInstance)
	if err := s.writeConfig(integrationsFile); err != nil {
		return nil, err
	}

	s.logger.Info("Created integration instance: %s (type: %s)", newInstance.Name, newInstance.Type)

	response := newConfigBackedInstance(newInstance)
	return &response, nil
}

func (s *Service) Update(name string, updatedInstance config.IntegrationConfig) (*Instance, error) {
	integrationsFile, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	found := false
	for i := range integrationsFile.Instances {
		if integrationsFile.Instances[i].Name == name {
			updatedInstance.Name = name
			integrationsFile.Instances[i] = updatedInstance
			found = true
			break
		}
	}

	if !found {
		return nil, newError(codeNotFound, fmt.Sprintf("Integration %q not found", name), nil)
	}

	if err := integrationsFile.Validate(); err != nil {
		return nil, newError(codeInvalidConfig, fmt.Sprintf("Validation failed: %v", err), err)
	}

	if err := s.writeConfig(integrationsFile); err != nil {
		return nil, err
	}

	s.logger.Info("Updated integration instance: %s", name)

	response := newConfigBackedInstance(updatedInstance)
	return &response, nil
}

func (s *Service) Delete(name string) error {
	integrationsFile, err := s.loadConfig()
	if err != nil {
		return err
	}

	found := false
	newInstances := make([]config.IntegrationConfig, 0, len(integrationsFile.Instances))
	for _, instance := range integrationsFile.Instances {
		if instance.Name == name {
			found = true
			continue
		}
		newInstances = append(newInstances, instance)
	}

	if !found {
		return newError(codeNotFound, fmt.Sprintf("Integration %q not found", name), nil)
	}

	integrationsFile.Instances = newInstances
	if err := s.writeConfig(integrationsFile); err != nil {
		return err
	}

	s.logger.Info("Deleted integration instance: %s", name)
	return nil
}

func (s *Service) loadConfig() (*config.IntegrationsFile, error) {
	integrationsFile, err := config.LoadIntegrationsFile(s.configPath)
	if err != nil {
		s.logger.Error("Failed to load integrations config: %v", err)
		return nil, newError(codeLoadError, fmt.Sprintf("Failed to load config: %v", err), err)
	}
	return integrationsFile, nil
}

func (s *Service) writeConfig(integrationsFile *config.IntegrationsFile) error {
	if err := config.WriteIntegrationsFile(s.configPath, integrationsFile); err != nil {
		s.logger.Error("Failed to write integrations config: %v", err)
		return newError(codeWriteError, fmt.Sprintf("Failed to save config: %v", err), err)
	}
	return nil
}

func newConfigBackedInstance(instance config.IntegrationConfig) Instance {
	return Instance{
		Name:      instance.Name,
		Type:      instance.Type,
		Enabled:   instance.Enabled,
		Config:    instance.Config,
		Health:    "not_started",
		DateAdded: time.Now().Format(time.RFC3339),
	}
}
