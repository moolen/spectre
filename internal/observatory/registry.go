package observatory

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages multiple Observatory providers and aggregates their data.
// It provides a unified view of signals across all registered providers.
//
// Thread-safe: All operations are protected by a read-write mutex.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new Observatory registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
// Returns an error if a provider with the same name is already registered.
func (r *Registry) Register(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// Unregister removes a provider from the registry.
// No-op if the provider is not registered.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// GetProvider returns a specific provider by name.
// Returns (provider, true) if found, (nil, false) if not.
func (r *Registry) GetProvider(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// Providers returns the names of all registered providers.
func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ProviderCount returns the number of registered providers.
func (r *Registry) ProviderCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// ListAllSignalAnchors aggregates signals from all registered providers.
// Signals are merged by composite key (metric_name|namespace|workload).
// When the same signal exists in multiple providers, highest QualityScore wins.
//
// The returned signals have their SourceProvider field set to indicate origin.
func (r *Registry) ListAllSignalAnchors(ctx context.Context, opts SignalListOptions) ([]SignalAnchor, error) {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	// Collect signals from all providers
	signalMap := make(map[string]SignalAnchor) // key: metric|namespace|workload

	for _, provider := range providers {
		signals, err := provider.ListSignalAnchors(ctx, opts)
		if err != nil {
			// Log error but continue with other providers (graceful degradation)
			continue
		}

		for _, signal := range signals {
			key := signalKey(signal.MetricName, signal.WorkloadNamespace, signal.WorkloadName)

			existing, exists := signalMap[key]
			if !exists {
				signalMap[key] = signal
				continue
			}

			// Conflict resolution: highest QualityScore wins
			if signal.QualityScore > existing.QualityScore {
				signalMap[key] = signal
			} else if signal.QualityScore == existing.QualityScore {
				// Tiebreaker: highest Confidence
				if signal.Confidence > existing.Confidence {
					signalMap[key] = signal
				} else if signal.Confidence == existing.Confidence {
					// Final tiebreaker: most recently seen
					if signal.LastSeen > existing.LastSeen {
						signalMap[key] = signal
					}
				}
			}
		}
	}

	// Convert map to slice
	result := make([]SignalAnchor, 0, len(signalMap))
	for _, signal := range signalMap {
		result = append(result, signal)
	}

	return result, nil
}

// GetSignalBaseline retrieves the baseline for a signal, checking all providers.
// Returns the first baseline found (signals should only exist in one provider).
func (r *Registry) GetSignalBaseline(ctx context.Context, metricName, namespace, workload string) (*SignalBaseline, error) {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	for _, provider := range providers {
		baseline, err := provider.GetBaseline(ctx, metricName, namespace, workload)
		if err != nil {
			continue
		}
		if baseline != nil {
			return baseline, nil
		}
	}

	return nil, nil
}

// GetSignalCurrentValue retrieves the current value for a signal.
// Returns the first value found from any provider.
func (r *Registry) GetSignalCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, bool, error) {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	for _, provider := range providers {
		value, found, err := provider.GetCurrentValue(ctx, metricName, namespace, workload)
		if err != nil {
			continue
		}
		if found {
			return value, true, nil
		}
	}

	return 0, false, nil
}

// GetSignalAlertState retrieves the alert state for a signal.
// Returns the first non-empty state found from any provider.
// Prioritizes "firing" state if multiple providers report different states.
func (r *Registry) GetSignalAlertState(ctx context.Context, metricName, namespace, workload string) (string, error) {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	var bestState string
	for _, provider := range providers {
		state, err := provider.GetAlertState(ctx, metricName, namespace, workload)
		if err != nil {
			continue
		}
		if state == "firing" {
			return "firing", nil // Firing takes priority
		}
		if state != "" && bestState == "" {
			bestState = state
		}
	}

	return bestState, nil
}

// ForEachProvider calls the given function for each registered provider.
// Iteration stops if the function returns an error.
func (r *Registry) ForEachProvider(fn func(Provider) error) error {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	for _, provider := range providers {
		if err := fn(provider); err != nil {
			return err
		}
	}
	return nil
}

// signalKey generates the composite key for signal deduplication.
func signalKey(metricName, namespace, workload string) string {
	return metricName + "|" + namespace + "|" + workload
}
