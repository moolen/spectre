package tracing

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TracingProvider wraps OpenTelemetry TracerProvider and implements lifecycle.Component
type TracingProvider struct {
	tracerProvider *sdktrace.TracerProvider
	logger         *logging.Logger
	enabled        bool
}

// Config holds tracing configuration
type Config struct {
	Enabled     bool
	Endpoint    string // OTLP gRPC endpoint (e.g., "victorialogs:4317")
	TLSCAPath   string // Path to CA certificate for TLS verification (optional)
	TLSInsecure bool   // Skip TLS certificate verification (insecure)
}

// NewTracingProvider creates and initializes the tracing provider
func NewTracingProvider(cfg Config) (*TracingProvider, error) {
	logger := logging.GetLogger("tracing")

	if !cfg.Enabled {
		logger.Info("Tracing disabled")
		return &TracingProvider{
			logger:  logger,
			enabled: false,
		}, nil
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("tracing enabled but endpoint not configured")
	}

	// Create OTLP gRPC exporter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Configure TLS if CA path is provided
	var dialOptions []grpc.DialOption
	var otlpOptions []otlptracegrpc.Option

	if cfg.TLSCAPath != "" || cfg.TLSInsecure {
		// TLS configuration
		var tlsConfig *tls.Config

		if cfg.TLSInsecure {
			// Skip certificate verification
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			}
			logger.Info("TLS enabled for tracing with certificate verification disabled (insecure mode)")
		} else {
			// Load CA certificate
			caCert, err := os.ReadFile(cfg.TLSCAPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA certificate: %w", err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to append CA certificate to pool")
			}

			tlsConfig = &tls.Config{
				RootCAs:    certPool,
				MinVersion: tls.VersionTLS12,
			}
			logger.Info("TLS enabled for tracing with CA from: %s", cfg.TLSCAPath)
		}

		creds := credentials.NewTLS(tlsConfig)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	} else {
		// Use insecure connection (no TLS)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
		otlpOptions = append(otlpOptions, otlptracegrpc.WithInsecure())
		logger.Info("TLS disabled for tracing (insecure mode)")
	}

	otlpOptions = append(otlpOptions,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithDialOption(dialOptions...),
	)

	exporter, err := otlptracegrpc.New(ctx, otlpOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("spectre"),
			semconv.ServiceVersion("0.1.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider with always-on sampling
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // 100% sampling
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	logger.Info("Tracing initialized with endpoint: %s", cfg.Endpoint)

	return &TracingProvider{
		tracerProvider: tracerProvider,
		logger:         logger,
		enabled:        true,
	}, nil
}

// Start implements lifecycle.Component interface
func (tp *TracingProvider) Start(ctx context.Context) error {
	if !tp.enabled {
		tp.logger.Info("Tracing provider starting (disabled mode)")
		return nil
	}
	tp.logger.Info("Tracing provider started")
	return nil
}

// Stop implements lifecycle.Component interface
func (tp *TracingProvider) Stop(ctx context.Context) error {
	if !tp.enabled {
		return nil
	}

	tp.logger.Info("Shutting down tracing provider...")

	// Flush remaining spans
	if err := tp.tracerProvider.Shutdown(ctx); err != nil {
		tp.logger.Error("Error shutting down tracer provider: %v", err)
		return err
	}

	tp.logger.Info("Tracing provider stopped")
	return nil
}

// Name implements lifecycle.Component interface
func (tp *TracingProvider) Name() string {
	return "Tracing Provider"
}

// GetTracer returns a tracer for instrumenting code
func (tp *TracingProvider) GetTracer(name string) trace.Tracer {
	if !tp.enabled {
		return otel.GetTracerProvider().Tracer(name)
	}
	return otel.GetTracerProvider().Tracer(name)
}

// IsEnabled returns whether tracing is enabled
func (tp *TracingProvider) IsEnabled() bool {
	return tp.enabled
}
