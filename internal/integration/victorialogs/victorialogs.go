// Package victorialogs provides VictoriaLogs integration for Spectre.
package victorialogs

import (
	"sync"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/logprocessing"
)

func init() {
	if err := integration.RegisterFactory("victorialogs", NewVictoriaLogsIntegration); err != nil {
		logger := logging.GetLogger("integration.victorialogs")
		logger.Warn("Failed to register victorialogs factory: %v", err)
	}
}

// VictoriaLogsIntegration implements the Integration interface for VictoriaLogs.
type VictoriaLogsIntegration struct {
	name          string
	config        Config
	client        *Client
	pipeline      *Pipeline
	metrics       *Metrics
	logger        *logging.Logger
	registry      integration.ToolRegistry
	templateStore *logprocessing.TemplateStore
	secretWatcher *SecretWatcher
	healthStatus  integration.HealthStatus
	mu            sync.RWMutex
}
