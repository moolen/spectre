package victorialogs

import (
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
)

// NewVictoriaLogsIntegration creates a new VictoriaLogs integration instance.
func NewVictoriaLogsIntegration(name string, configMap map[string]interface{}) (integration.Integration, error) {
	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &VictoriaLogsIntegration{
		name:          name,
		config:        config,
		healthStatus:  integration.Stopped,
		logger:        logging.GetLogger("integration.victorialogs." + name),
		client:        nil,
		pipeline:      nil,
		metrics:       nil,
		templateStore: nil,
		secretWatcher: nil,
	}, nil
}

// Metadata returns the integration's identifying information.
func (v *VictoriaLogsIntegration) Metadata() integration.IntegrationMetadata {
	return integration.IntegrationMetadata{
		Name:        v.name,
		Version:     "0.1.0",
		Description: "VictoriaLogs log aggregation integration",
		Type:        "victorialogs",
	}
}
