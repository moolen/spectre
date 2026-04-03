package integrationadmin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/integration"
)

func (s *Service) TriggerSync(ctx context.Context, name string) (*integration.IntegrationStatus, error) {
	instance, found := s.registry().Get(name)
	if !found {
		return nil, newError(codeNotFound, fmt.Sprintf("Integration %q not found or not started", name), nil)
	}

	integrationSyncer, ok := instance.(syncer)
	if !ok {
		return nil, newError(codeNotSupported, "Sync only supported for Grafana integrations", nil)
	}

	if err := integrationSyncer.TriggerSync(ctx); err != nil {
		if err.Error() == "sync already in progress" {
			return nil, newError(codeSyncInProgress, err.Error(), err)
		}
		return nil, newError(codeInternalError, fmt.Sprintf("Sync failed: %v", err), err)
	}

	status := integrationSyncer.Status()
	return &status, nil
}

func (s *Service) TestConnection(testReq TestConnectionRequest) TestConnectionResponse {
	testFile := &config.IntegrationsFile{
		SchemaVersion: "v1",
		Instances: []config.IntegrationConfig{
			{
				Name:    testReq.Name,
				Type:    testReq.Type,
				Enabled: testReq.Enabled,
				Config:  testReq.Config,
			},
		},
	}
	if err := testFile.Validate(); err != nil {
		return TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Validation failed: %v", err),
		}
	}

	factory, ok := integration.GetFactory(testReq.Type)
	if !ok {
		return TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Unknown integration type: %s", testReq.Type),
		}
	}

	success, message := s.testConnection(factory, testReq)
	return TestConnectionResponse{
		Success: success,
		Message: message,
	}
}

func (s *Service) TriggerSignalValidation(ctx context.Context, name string, fullRun bool) (*SignalValidationResponse, error) {
	instance, found := s.registry().Get(name)
	if !found {
		return nil, newError(codeNotFound, fmt.Sprintf("Integration %q not found or not started", name), nil)
	}

	validator, ok := instance.(signalValidator)
	if !ok {
		return nil, newError(codeNotSupported, "Signal validation only supported for Grafana integrations with Prometheus configured", nil)
	}

	job := validator.SignalValidationJob()
	if job == nil {
		return nil, newError(codeNotConfigured, "Signal validation job not configured. Ensure Prometheus URL is set and signal validation is enabled.", nil)
	}

	runner, ok := job.(validationRunner)
	if !ok {
		return nil, newError(codeInternalError, "Signal validation job does not support running", nil)
	}

	var err error
	if fullRun {
		err = runner.RunFull(ctx)
	} else {
		err = runner.RunNow(ctx)
	}
	if err != nil {
		if strings.Contains(err.Error(), "already in progress") {
			return nil, newError(codeValidationInProgress, err.Error(), err)
		}
		s.logger.Warn("Signal validation completed with errors: %v", err)
	}

	message := "Signal validation triggered successfully"
	if fullRun {
		message = "Full signal validation backfill triggered successfully"
	}

	return &SignalValidationResponse{Message: message}, nil
}

func (s *Service) testConnection(factory integration.IntegrationFactory, testReq TestConnectionRequest) (success bool, message string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			success = false
			message = fmt.Sprintf("Test panicked: %v", recovered)
			s.logger.Error("Integration test panicked: %v", recovered)
		}
	}()

	instance, err := factory(testReq.Name, testReq.Config)
	if err != nil {
		return false, fmt.Sprintf("Failed to create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := instance.Start(ctx); err != nil {
		return false, fmt.Sprintf("Failed to start: %v", err)
	}

	healthCtx, healthCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer healthCancel()

	healthStatus := instance.Health(healthCtx)
	if healthStatus != integration.Healthy {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = instance.Stop(stopCtx)

		return false, fmt.Sprintf("Health check failed: %s", healthStatus.String())
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := instance.Stop(stopCtx); err != nil {
		s.logger.Warn("Failed to stop test instance cleanly: %v", err)
	}

	return true, "Connection successful"
}
