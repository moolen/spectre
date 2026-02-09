package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/observatory"
)

// TestObservatoryServiceAdapter_ImplementsInterface verifies the adapter implements the interface.
func TestObservatoryServiceAdapter_ImplementsInterface(t *testing.T) {
	// This is a compile-time check - if it compiles, the interface is implemented
	var _ ObservatoryServiceInterface = (*ObservatoryServiceAdapter)(nil)
}

// TestObservatoryInvestigateServiceAdapter_ImplementsInterface verifies the adapter implements the interface.
func TestObservatoryInvestigateServiceAdapter_ImplementsInterface(t *testing.T) {
	// This is a compile-time check - if it compiles, the interface is implemented
	var _ ObservatoryInvestigateServiceInterface = (*ObservatoryInvestigateServiceAdapter)(nil)
}

// TestObservatoryServiceAdapter_GetClusterAnomalies tests the GetClusterAnomalies adapter method.
func TestObservatoryServiceAdapter_GetClusterAnomalies(t *testing.T) {
	// Create a test provider from the observatory package
	provider := observatory.NewTestProvider("test-provider")

	// Add a signal with anomalous current value
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "http_requests_total",
		Role:              observatory.SignalAvailability,
		Confidence:        0.9,
		QualityScore:      0.9,
		WorkloadNamespace: "default",
		WorkloadName:      "api-server",
	})

	// Set current value significantly higher than baseline
	provider.SetCurrentValue("http_requests_total", "default", "api-server", 500.0)

	// Set baseline with enough samples
	provider.SetBaseline("http_requests_total", "default", "api-server", &observatory.SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		P50:         100.0,
		P90:         120.0,
		P99:         150.0,
		Min:         80.0,
		Max:         150.0,
		SampleCount: 100,
	})

	// Create registry and register provider
	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create service and adapter
	service := observatory.NewService(reg)
	adapter := NewObservatoryServiceAdapter(service)

	// Test GetClusterAnomalies
	result, err := adapter.GetClusterAnomalies(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetClusterAnomalies failed: %v", err)
	}

	// Verify the result structure
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	t.Logf("Got %d hotspots, %d total anomalous signals",
		len(result.TopHotspots), result.TotalAnomalousSignals)
}

// TestObservatoryServiceAdapter_GetNamespaceAnomalies tests namespace-scoped anomaly retrieval.
func TestObservatoryServiceAdapter_GetNamespaceAnomalies(t *testing.T) {
	// Create a test provider
	provider := observatory.NewTestProvider("test-provider")

	// Add a signal
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "request_latency",
		Role:              observatory.SignalLatency,
		Confidence:        0.85,
		QualityScore:      0.85,
		WorkloadNamespace: "production",
		WorkloadName:      "frontend",
	})

	// Set high current value (anomalous)
	provider.SetCurrentValue("request_latency", "production", "frontend", 500.0)

	// Set baseline
	provider.SetBaseline("request_latency", "production", "frontend", &observatory.SignalBaseline{
		Mean:        100.0,
		StdDev:      20.0,
		P50:         95.0,
		P90:         130.0,
		P99:         150.0,
		Min:         50.0,
		Max:         150.0,
		SampleCount: 50,
	})

	// Create registry and register provider
	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create service and adapter
	service := observatory.NewService(reg)
	adapter := NewObservatoryServiceAdapter(service)

	// Test GetNamespaceAnomalies
	result, err := adapter.GetNamespaceAnomalies(context.Background(), "production")
	if err != nil {
		t.Fatalf("GetNamespaceAnomalies failed: %v", err)
	}

	// Verify the result structure
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", result.Namespace)
	}
}

// TestObservatoryServiceAdapter_GetWorkloadAnomalyDetail tests workload-level detail retrieval.
func TestObservatoryServiceAdapter_GetWorkloadAnomalyDetail(t *testing.T) {
	// Create a test provider
	provider := observatory.NewTestProvider("test-provider")

	// Add a signal with high error rate (anomalous)
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "error_rate",
		Role:              observatory.SignalErrors,
		Confidence:        0.95,
		QualityScore:      0.95,
		WorkloadNamespace: "staging",
		WorkloadName:      "backend",
	})

	// Set high error rate (anomalous)
	provider.SetCurrentValue("error_rate", "staging", "backend", 0.15)

	// Set baseline
	provider.SetBaseline("error_rate", "staging", "backend", &observatory.SignalBaseline{
		Mean:        0.02,
		StdDev:      0.01,
		P50:         0.02,
		P90:         0.03,
		P99:         0.05,
		Min:         0.01,
		Max:         0.05,
		SampleCount: 200,
	})

	// Create registry and register provider
	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create service and adapter
	service := observatory.NewService(reg)
	adapter := NewObservatoryServiceAdapter(service)

	// Test GetWorkloadAnomalyDetail
	result, err := adapter.GetWorkloadAnomalyDetail(context.Background(), "staging", "backend")
	if err != nil {
		t.Fatalf("GetWorkloadAnomalyDetail failed: %v", err)
	}

	// Verify the result structure
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Namespace != "staging" {
		t.Errorf("expected namespace 'staging', got %q", result.Namespace)
	}
	if result.Workload != "backend" {
		t.Errorf("expected workload 'backend', got %q", result.Workload)
	}
}

// TestObservatoryInvestigateServiceAdapter_GetWorkloadSignals tests signal listing.
func TestObservatoryInvestigateServiceAdapter_GetWorkloadSignals(t *testing.T) {
	// Create a test provider with multiple signals
	provider := observatory.NewTestProvider("test-provider")

	// Add two signals for the same workload
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "http_requests",
		Role:              observatory.SignalAvailability,
		Confidence:        0.9,
		QualityScore:      0.9,
		WorkloadNamespace: "default",
		WorkloadName:      "api",
	})
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "response_time",
		Role:              observatory.SignalLatency,
		Confidence:        0.85,
		QualityScore:      0.85,
		WorkloadNamespace: "default",
		WorkloadName:      "api",
	})

	// Set current values
	provider.SetCurrentValue("http_requests", "default", "api", 100.0)
	provider.SetCurrentValue("response_time", "default", "api", 50.0)

	// Set baselines
	provider.SetBaseline("http_requests", "default", "api", &observatory.SignalBaseline{
		Mean: 100.0, StdDev: 10.0, P50: 100.0, P90: 120.0, P99: 150.0, Min: 80.0, Max: 150.0, SampleCount: 100,
	})
	provider.SetBaseline("response_time", "default", "api", &observatory.SignalBaseline{
		Mean: 45.0, StdDev: 5.0, P50: 45.0, P90: 55.0, P99: 60.0, Min: 30.0, Max: 60.0, SampleCount: 100,
	})

	// Create registry and register provider
	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create service and adapter
	service := observatory.NewInvestigateService(reg)
	adapter := NewObservatoryInvestigateServiceAdapter(service)

	// Test GetWorkloadSignals
	result, err := adapter.GetWorkloadSignals(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("GetWorkloadSignals failed: %v", err)
	}

	// Verify the result structure
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(result.Signals))
	}
}

// TestObservatoryInvestigateServiceAdapter_GetSignalDetail tests detailed signal retrieval.
func TestObservatoryInvestigateServiceAdapter_GetSignalDetail(t *testing.T) {
	// Create a test provider
	provider := observatory.NewTestProvider("test-provider")

	// Add a signal
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "cpu_usage",
		Role:              observatory.SignalSaturation,
		Confidence:        0.8,
		QualityScore:      0.8,
		WorkloadNamespace: "prod",
		WorkloadName:      "service",
	})

	// Set current value
	provider.SetCurrentValue("cpu_usage", "prod", "service", 75.0)

	// Set baseline with percentile data
	provider.SetBaseline("cpu_usage", "prod", "service", &observatory.SignalBaseline{
		Mean:        50.0,
		StdDev:      10.0,
		P50:         48.0,
		P90:         65.0,
		P99:         72.0,
		Min:         30.0,
		Max:         75.0,
		SampleCount: 500,
	})

	// Create registry and register provider
	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create service and adapter
	service := observatory.NewInvestigateService(reg)
	adapter := NewObservatoryInvestigateServiceAdapter(service)

	// Test GetSignalDetail
	result, err := adapter.GetSignalDetail(context.Background(), "prod", "service", "cpu_usage")
	if err != nil {
		t.Fatalf("GetSignalDetail failed: %v", err)
	}

	// Verify the result structure
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.MetricName != "cpu_usage" {
		t.Errorf("expected metric 'cpu_usage', got %q", result.MetricName)
	}
	if result.CurrentValue != 75.0 {
		t.Errorf("expected current value 75.0, got %f", result.CurrentValue)
	}
	if result.Baseline.Mean != 50.0 {
		t.Errorf("expected baseline mean 50.0, got %f", result.Baseline.Mean)
	}
}

// TestObservatoryInvestigateServiceAdapter_CompareSignal tests time-based comparison.
func TestObservatoryInvestigateServiceAdapter_CompareSignal(t *testing.T) {
	// Create a test provider
	provider := observatory.NewTestProvider("test-provider")

	// Add a signal
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "requests",
		Role:              observatory.SignalAvailability,
		Confidence:        0.9,
		QualityScore:      0.9,
		WorkloadNamespace: "test",
		WorkloadName:      "app",
	})

	// Set current value (higher than baseline mean = anomalous)
	provider.SetCurrentValue("requests", "test", "app", 200.0)

	// Set baseline
	provider.SetBaseline("requests", "test", "app", &observatory.SignalBaseline{
		Mean: 100.0, StdDev: 20.0, P50: 100.0, P90: 130.0, P99: 150.0, Min: 60.0, Max: 150.0, SampleCount: 100,
	})

	// Create registry and register provider
	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create service and adapter
	service := observatory.NewInvestigateService(reg)
	adapter := NewObservatoryInvestigateServiceAdapter(service)

	// Test CompareSignal
	result, err := adapter.CompareSignal(context.Background(), "test", "app", "requests", 24*time.Hour)
	if err != nil {
		t.Fatalf("CompareSignal failed: %v", err)
	}

	// Verify the result structure
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.MetricName != "requests" {
		t.Errorf("expected metric 'requests', got %q", result.MetricName)
	}
	if result.LookbackHours != 24 {
		t.Errorf("expected lookback 24 hours, got %d", result.LookbackHours)
	}
}

// TestObservatoryServiceAdapter_NilOptions tests handling of nil options.
func TestObservatoryServiceAdapter_NilOptions(t *testing.T) {
	// Create a test provider
	provider := observatory.NewTestProvider("test-provider")
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "metric1",
		Role:              observatory.SignalAvailability,
		Confidence:        0.9,
		QualityScore:      0.9,
		WorkloadNamespace: "ns1",
		WorkloadName:      "wl1",
	})
	provider.SetCurrentValue("metric1", "ns1", "wl1", 100.0)
	provider.SetBaseline("metric1", "ns1", "wl1", &observatory.SignalBaseline{
		Mean: 100.0, StdDev: 10.0, P50: 100.0, P90: 120.0, P99: 150.0, Min: 80.0, Max: 150.0, SampleCount: 100,
	})

	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	service := observatory.NewService(reg)
	adapter := NewObservatoryServiceAdapter(service)

	// Test with nil options (should work)
	result, err := adapter.GetClusterAnomalies(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetClusterAnomalies with nil options failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestObservatoryServiceAdapter_WithScopeOptions tests filtering with scope options.
func TestObservatoryServiceAdapter_WithScopeOptions(t *testing.T) {
	// Create a test provider
	provider := observatory.NewTestProvider("test-provider")

	// Add signals in different namespaces
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "metric1",
		Role:              observatory.SignalAvailability,
		Confidence:        0.9,
		QualityScore:      0.9,
		WorkloadNamespace: "ns1",
		WorkloadName:      "wl1",
	})
	provider.AddSignal(observatory.SignalAnchor{
		MetricName:        "metric2",
		Role:              observatory.SignalLatency,
		Confidence:        0.9,
		QualityScore:      0.9,
		WorkloadNamespace: "ns2",
		WorkloadName:      "wl2",
	})

	// Set current values
	provider.SetCurrentValue("metric1", "ns1", "wl1", 100.0)
	provider.SetCurrentValue("metric2", "ns2", "wl2", 100.0)

	// Set baselines
	provider.SetBaseline("metric1", "ns1", "wl1", &observatory.SignalBaseline{
		Mean: 100.0, StdDev: 10.0, P50: 100.0, P90: 120.0, P99: 150.0, Min: 80.0, Max: 150.0, SampleCount: 100,
	})
	provider.SetBaseline("metric2", "ns2", "wl2", &observatory.SignalBaseline{
		Mean: 100.0, StdDev: 10.0, P50: 100.0, P90: 120.0, P99: 150.0, Min: 80.0, Max: 150.0, SampleCount: 100,
	})

	reg := observatory.NewRegistry()
	if err := reg.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	service := observatory.NewService(reg)
	adapter := NewObservatoryServiceAdapter(service)

	// Test with namespace filter
	result, err := adapter.GetClusterAnomalies(context.Background(), &ScopeOptions{
		Namespace: "ns1",
	})
	if err != nil {
		t.Fatalf("GetClusterAnomalies with scope options failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
