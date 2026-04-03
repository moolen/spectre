package integrationadmin

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
)

const (
	codeDuplicateName        = "DUPLICATE_NAME"
	codeInvalidConfig        = "INVALID_CONFIG"
	codeInternalError        = "INTERNAL_ERROR"
	codeLoadError            = "LOAD_ERROR"
	codeNotConfigured        = "NOT_CONFIGURED"
	codeNotFound             = "NOT_FOUND"
	codeNotSupported         = "NOT_SUPPORTED"
	codeSyncInProgress       = "SYNC_IN_PROGRESS"
	codeValidationInProgress = "VALIDATION_IN_PROGRESS"
	codeWriteError           = "WRITE_ERROR"
)

type Service struct {
	configPath string
	manager    *integration.Manager
	logger     *logging.Logger
}

type statusProvider interface {
	Status() integration.IntegrationStatus
}

type syncer interface {
	TriggerSync(ctx context.Context) error
	Status() integration.IntegrationStatus
}

type signalValidator interface {
	SignalValidationJob() interface{}
}

type validationRunner interface {
	RunNow(ctx context.Context) error
	RunFull(ctx context.Context) error
}

func NewService(configPath string, manager *integration.Manager, logger *logging.Logger) *Service {
	return &Service{
		configPath: configPath,
		manager:    manager,
		logger:     logger,
	}
}

func (s *Service) buildInstance(ctx context.Context, instance config.IntegrationConfig) Instance {
	health, syncStatus := s.runtimeStatus(ctx, instance.Name)
	return Instance{
		Name:       instance.Name,
		Type:       instance.Type,
		Enabled:    instance.Enabled,
		Config:     instance.Config,
		Health:     health,
		DateAdded:  time.Now().Format(time.RFC3339),
		SyncStatus: syncStatus,
	}
}

func (s *Service) runtimeStatus(ctx context.Context, name string) (string, *integration.SyncStatus) {
	registry := s.registry()
	if registry == nil {
		return "not_started", nil
	}

	runtimeInstance, ok := registry.Get(name)
	if !ok {
		return "not_started", nil
	}

	healthCtx, cancel := context.WithTimeout(defaultContext(ctx), 2*time.Second)
	defer cancel()

	health := runtimeInstance.Health(healthCtx).String()

	var syncStatus *integration.SyncStatus
	if provider, ok := runtimeInstance.(statusProvider); ok {
		status := provider.Status()
		syncStatus = status.SyncStatus
	}

	return health, syncStatus
}

func (s *Service) registry() *integration.Registry {
	if s.manager == nil {
		return nil
	}
	return s.manager.GetRegistry()
}

func findInstance(instances []config.IntegrationConfig, name string) (config.IntegrationConfig, bool) {
	for _, instance := range instances {
		if instance.Name == name {
			return instance, true
		}
	}
	return config.IntegrationConfig{}, false
}

func defaultContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
