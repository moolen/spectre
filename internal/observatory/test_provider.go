package observatory

import (
	"context"
	"time"
)

// TestProvider is a mock Provider for testing Observatory core logic.
// It implements the Provider interface with in-memory data storage.
//
// This provider is useful for:
//   - Unit testing Observatory services without real integrations
//   - Integration tests with controlled test data
//   - Golden tests with deterministic inputs
type TestProvider struct {
	name          string
	signals       []SignalAnchor
	baselines     map[string]*SignalBaseline // metric|ns|workload -> baseline
	currentValues map[string]float64         // metric|ns|workload -> value
	alertStates   map[string]string          // metric|ns|workload -> state
}

// NewTestProvider creates a new TestProvider with the given name.
func NewTestProvider(name string) *TestProvider {
	return &TestProvider{
		name:          name,
		signals:       make([]SignalAnchor, 0),
		baselines:     make(map[string]*SignalBaseline),
		currentValues: make(map[string]float64),
		alertStates:   make(map[string]string),
	}
}

// Name returns the provider's unique identifier.
func (p *TestProvider) Name() string {
	return p.name
}

// ListSignalAnchors returns all signals matching the filter options.
func (p *TestProvider) ListSignalAnchors(ctx context.Context, opts SignalListOptions) ([]SignalAnchor, error) {
	var result []SignalAnchor

	for _, signal := range p.signals {
		// Apply filters
		if opts.Namespace != "" && signal.WorkloadNamespace != opts.Namespace {
			continue
		}
		if opts.WorkloadName != "" && signal.WorkloadName != opts.WorkloadName {
			continue
		}
		if opts.Role != "" && signal.Role != opts.Role {
			continue
		}

		// Check expiry
		if signal.ExpiresAt > 0 && signal.ExpiresAt < time.Now().Unix() {
			continue
		}

		result = append(result, signal)
	}

	return result, nil
}

// GetCurrentValue returns the configured current value for a signal.
func (p *TestProvider) GetCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, bool, error) {
	key := signalKey(metricName, namespace, workload)
	value, found := p.currentValues[key]
	return value, found, nil
}

// GetBaseline returns the configured baseline for a signal.
func (p *TestProvider) GetBaseline(ctx context.Context, metricName, namespace, workload string) (*SignalBaseline, error) {
	key := signalKey(metricName, namespace, workload)
	return p.baselines[key], nil
}

// GetAlertState returns the configured alert state for a signal.
func (p *TestProvider) GetAlertState(ctx context.Context, metricName, namespace, workload string) (string, error) {
	key := signalKey(metricName, namespace, workload)
	return p.alertStates[key], nil
}

// --- Test Helper Methods ---

// AddSignal adds a signal anchor to the provider.
// The SourceProvider field is automatically set to this provider's name.
func (p *TestProvider) AddSignal(signal SignalAnchor) {
	signal.SourceProvider = p.name
	if signal.ExpiresAt == 0 {
		signal.ExpiresAt = time.Now().Add(7 * 24 * time.Hour).Unix()
	}
	p.signals = append(p.signals, signal)
}

// SetBaseline sets the baseline for a specific signal.
func (p *TestProvider) SetBaseline(metricName, namespace, workload string, baseline *SignalBaseline) {
	key := signalKey(metricName, namespace, workload)
	if baseline != nil {
		baseline.SourceProvider = p.name
	}
	p.baselines[key] = baseline
}

// SetCurrentValue sets the current value for a specific signal.
func (p *TestProvider) SetCurrentValue(metricName, namespace, workload string, value float64) {
	key := signalKey(metricName, namespace, workload)
	p.currentValues[key] = value
}

// SetAlertState sets the alert state for a specific signal.
func (p *TestProvider) SetAlertState(metricName, namespace, workload, state string) {
	key := signalKey(metricName, namespace, workload)
	p.alertStates[key] = state
}

// ClearAll resets all data in the provider.
func (p *TestProvider) ClearAll() {
	p.signals = make([]SignalAnchor, 0)
	p.baselines = make(map[string]*SignalBaseline)
	p.currentValues = make(map[string]float64)
	p.alertStates = make(map[string]string)
}

// SignalCount returns the number of signals in the provider.
func (p *TestProvider) SignalCount() int {
	return len(p.signals)
}
