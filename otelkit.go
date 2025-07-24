package otelkit

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for the OTel wrapper.
// All fields are optional and will use defaults if not specified.
type Config struct {
	// ServiceName identifies your service in traces (required for meaningful observability)
	// Example: "user-api", "payment-service", "order-processor"
	ServiceName string
	
	// ServiceVersion helps track deployments and correlate issues with releases
	// Example: "1.2.3", "v2.0.0-beta", "commit-abc123"
	ServiceVersion string
	
	// Environment distinguishes between different deployment environments
	// Example: "development", "staging", "production"
	Environment string
	
	// ExporterType determines where traces are sent
	// Options: ExporterJaeger, ExporterOTLP, ExporterStdout, ExporterNone
	ExporterType ExporterType
	
	// JaegerURL is the endpoint for Jaeger collector (only used with ExporterJaeger)
	// Example: "http://localhost:14268/api/traces", "http://jaeger-collector:14268/api/traces"
	JaegerURL string
	
	// OTLPEndpoint is the endpoint for OTLP exporter (only used with ExporterOTLP)
	// Example: "http://localhost:4318", "http://otel-collector:4318"
	OTLPEndpoint string
	
	// SampleRate controls what percentage of traces are exported (0.0 to 1.0)
	// 0.1 = 10% sampling, 1.0 = 100% sampling, 0.0 = no sampling
	// Lower values reduce overhead but may miss issues
	SampleRate float64
	
	// Debug enables verbose logging of OTelKit operations
	// Useful for troubleshooting configuration and export issues
	Debug bool
}

// ExporterType defines the type of exporter to use for sending traces.
// Choose based on your observability infrastructure and requirements.
type ExporterType string

const (
	// ExporterJaeger sends traces to Jaeger backend
	// Use for: Local development, Jaeger-based observability setups
	// Requires: Jaeger collector running and accessible
	ExporterJaeger ExporterType = "jaeger"
	
	// ExporterOTLP sends traces using OpenTelemetry Protocol
	// Use for: Production environments, OpenTelemetry collectors, cloud observability
	// Requires: OTLP-compatible endpoint (e.g., OTEL Collector, cloud vendors)
	ExporterOTLP ExporterType = "otlp"
	
	// ExporterStdout prints traces to console in JSON format
	// Use for: Development, debugging, CI/CD pipelines, testing
	// Requires: Nothing, outputs to stdout
	ExporterStdout ExporterType = "stdout"
	
	// ExporterNone disables trace export entirely
	// Use for: Maximum performance, when tracing overhead must be eliminated
	// Requires: Nothing, creates no-op spans
	ExporterNone ExporterType = "none"
)

// OTelKit is the main wrapper struct that provides simplified OpenTelemetry functionality.
// Create one instance per service and reuse it throughout your application.
type OTelKit struct {
	// tracer is the OpenTelemetry tracer instance used to create spans
	tracer trace.Tracer
	
	// tracerProvider manages the tracer lifecycle and span export
	tracerProvider *sdktrace.TracerProvider
	
	// config stores the configuration used to initialize this instance
	config Config
}

// DefaultConfig returns a default configuration with sensible defaults.
// Values can be overridden by environment variables or programmatically.
//
// Returns:
//   - Config: Configuration struct with default values populated
//
// Environment variable overrides:
//   - OTEL_SERVICE_NAME: overrides ServiceName
//   - OTEL_SERVICE_VERSION: overrides ServiceVersion  
//   - OTEL_ENVIRONMENT: overrides Environment
//   - OTEL_EXPORTER_TYPE: overrides ExporterType
//   - JAEGER_URL: overrides JaegerURL
//   - OTEL_EXPORTER_OTLP_ENDPOINT: overrides OTLPEndpoint
//   - OTEL_DEBUG: overrides Debug (set to "true" to enable)
//
// Defaults:
//   - ServiceName: "unknown-service" (should be overridden)
//   - ServiceVersion: "1.0.0"
//   - Environment: "development"
//   - ExporterType: stdout
//   - SampleRate: 0.1 (10% sampling)
func DefaultConfig() Config {
	return Config{
		ServiceName:    getEnvOrDefault("OTEL_SERVICE_NAME", "unknown-service"),
		ServiceVersion: getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
		Environment:    getEnvOrDefault("OTEL_ENVIRONMENT", "development"),
		ExporterType:   ExporterType(getEnvOrDefault("OTEL_EXPORTER_TYPE", string(ExporterStdout))),
		JaegerURL:      getEnvOrDefault("JAEGER_URL", "http://localhost:14268/api/traces"),
		OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		SampleRate:     0.1, // 10% sampling by default
		Debug:          getEnvOrDefault("OTEL_DEBUG", "false") == "true",
	}
}

// New creates a new OTelKit instance with the provided configuration.
// This should be called once during application startup.
//
// Parameters:
//   - config: Configuration struct with desired settings
//
// Returns:
//   - *OTelKit: Configured OTelKit instance ready for use
//   - error: Any error that occurred during initialization
//
// The function will:
//   1. Create OpenTelemetry resource with service metadata
//   2. Initialize the configured exporter (Jaeger, OTLP, stdout, or none)
//   3. Set up tracer provider with sampling configuration
//   4. Register the provider globally
//   5. Create and return the OTelKit wrapper
//
// Example:
//   config := otelkit.DefaultConfig()
//   config.ServiceName = "my-service"
//   kit, err := otelkit.New(config)
//   if err != nil {
//       log.Fatal(err)
//   }
//   defer kit.Shutdown(context.Background())
func New(config Config) (*OTelKit, error) {
	// Create resource
	res, err := newResource(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	exporter, err := createExporter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create tracer provider
	var tracerProvider *sdktrace.TracerProvider
	if exporter != nil {
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
		)
	} else {
		// No-op tracer provider for when exporter is none
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.NeverSample()),
		)
	}

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Create tracer
	tracer := tracerProvider.Tracer(
		config.ServiceName,
		trace.WithInstrumentationVersion(config.ServiceVersion),
	)

	kit := &OTelKit{
		tracer:         tracer,
		tracerProvider: tracerProvider,
		config:         config,
	}

	if config.Debug {
		log.Printf("OTelKit initialized: service=%s, version=%s, exporter=%s", 
			config.ServiceName, config.ServiceVersion, config.ExporterType)
	}

	return kit, nil
}

// Shutdown gracefully shuts down the tracer provider and flushes any pending spans.
// This should be called during application shutdown to ensure all traces are exported.
//
// Parameters:
//   - ctx: Context with timeout for the shutdown operation (recommended: 5-10 seconds)
//
// Returns:
//   - error: Any error that occurred during shutdown
//
// Example:
//   ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//   defer cancel()
//   if err := kit.Shutdown(ctx); err != nil {
//       log.Printf("Error shutting down tracer: %v", err)
//   }
func (o *OTelKit) Shutdown(ctx context.Context) error {
	if o.tracerProvider != nil {
		return o.tracerProvider.Shutdown(ctx)
	}
	return nil
}

// StartSpan starts a new span with the given name and options.
// Use this for manual span creation when TraceFunction doesn't fit your needs.
//
// Parameters:
//   - ctx: Parent context (may contain parent span)
//   - spanName: Descriptive name for the span (e.g., "user.create", "db.query")
//   - opts: Optional span start options (attributes, links, etc.)
//
// Returns:
//   - context.Context: New context containing the span
//   - trace.Span: The created span (must call span.End() when operation completes)
//
// Example:
//   ctx, span := kit.StartSpan(ctx, "calculate_total")
//   defer span.End()
//   // ... do work ...
//   span.SetAttributes(attribute.Int("items.count", count))
func (o *OTelKit) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return o.tracer.Start(ctx, spanName, opts...)
}

// TraceFunction is a convenient wrapper to trace a function execution.
// This is the recommended way to add tracing to most operations.
//
// Parameters:
//   - ctx: Context for the operation (may contain parent span)
//   - functionName: Descriptive name for the span (should describe what the function does)
//   - fn: The function to execute (receives the span context)
//   - attrs: Optional attributes to add to the span
//
// Returns:
//   - error: Any error returned by the fn function
//
// The function automatically:
//   - Creates and ends the span
//   - Records any error returned by fn
//   - Sets span status to error if fn returns an error
//   - Adds provided attributes to the span
//
// Example:
//   err := kit.TraceFunction(ctx, "process_order", func(ctx context.Context) error {
//       kit.AddEvent(ctx, "validation_started")
//       return processOrder(ctx, orderID)
//   }, attribute.String("order.id", orderID))
func (o *OTelKit) TraceFunction(ctx context.Context, functionName string, fn func(ctx context.Context) error, attrs ...attribute.KeyValue) error {
	ctx, span := o.StartSpan(ctx, functionName)
	defer span.End()

	// Add attributes
	span.SetAttributes(attrs...)

	// Execute function
	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// TraceHTTPHandler wraps an HTTP handler with tracing.
// This is an alternative to the HTTPMiddleware for more granular control.
//
// Parameters:
//   - handlerName: Name for the handler (will be prefixed with "http.")
//   - handler: The handler function to wrap
//
// Returns:
//   - func(ctx context.Context) error: Wrapped handler function
//
// Example:
//   wrappedHandler := kit.TraceHTTPHandler("get_user", func(ctx context.Context) error {
//       // handler logic here
//       return nil
//   })
func (o *OTelKit) TraceHTTPHandler(handlerName string, handler func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return o.TraceFunction(ctx, fmt.Sprintf("http.%s", handlerName), handler)
	}
}

// AddEvent adds an event to the current span in the context.
// Events represent important moments during span execution.
//
// Parameters:
//   - ctx: Context containing the span to add the event to
//   - name: Descriptive name for the event (e.g., "cache_miss", "validation_failed")
//   - attrs: Optional attributes providing additional context for the event
//
// If no span is present in the context, this function does nothing.
//
// Example:
//   kit.AddEvent(ctx, "user_validation_started")
//   kit.AddEvent(ctx, "cache_hit", attribute.String("cache.key", key))
func (o *OTelKit) AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span in the context.
// Attributes provide metadata about the operation being traced.
//
// Parameters:
//   - ctx: Context containing the span to add attributes to
//   - attrs: Key-value pairs to add as span attributes
//
// If no span is present in the context, this function does nothing.
//
// Common attribute patterns:
//   - HTTP: http.method, http.status_code, http.url
//   - Database: db.system, db.operation, db.table
//   - User: user.id, user.role
//   - Business: order.id, payment.amount, product.category
//
// Example:
//   kit.SetAttributes(ctx,
//       attribute.String("user.id", userID),
//       attribute.Int("order.amount", amount),
//       attribute.Bool("payment.successful", true),
//   )
func (o *OTelKit) SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span and sets the span status to error.
// This should be called whenever an error occurs during span execution.
//
// Parameters:
//   - ctx: Context containing the span to record the error on
//   - err: The error that occurred
//
// If no span is present in the context, this function does nothing.
// The error message will be set as the span status description.
//
// Example:
//   if err := someOperation(); err != nil {
//       kit.RecordError(ctx, err)
//       return err
//   }
func (o *OTelKit) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// GetTracer returns the underlying OpenTelemetry tracer.
// Use this only when you need direct access to OpenTelemetry APIs
// that aren't wrapped by OTelKit.
//
// Returns:
//   - trace.Tracer: The underlying OpenTelemetry tracer instance
//
// Most users should use the OTelKit wrapper methods instead of this.
// Direct tracer access is useful for:
//   - Custom span options not supported by OTelKit
//   - Integration with other OpenTelemetry libraries
//   - Advanced tracing patterns
//
// Example:
//   tracer := kit.GetTracer()
//   ctx, span := tracer.Start(ctx, "custom_span", trace.WithSpanKind(trace.SpanKindClient))
//   defer span.End()
func (o *OTelKit) GetTracer() trace.Tracer {
	return o.tracer
}

// Helper functions

// newResource creates an OpenTelemetry resource with service metadata.
// Resources identify the service, version, and environment in telemetry data.
//
// Parameters:
//   - config: Configuration containing service metadata
//
// Returns:
//   - *resource.Resource: Resource instance with service identification attributes
//   - error: Any error that occurred during resource creation
//
// The resource includes:
//   - service.name: From config.ServiceName
//   - service.version: From config.ServiceVersion  
//   - deployment.environment: From config.Environment
//   - Plus default SDK and runtime attributes
func newResource(config Config) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
}

// createExporter creates the appropriate span exporter based on configuration.
//
// Parameters:
//   - config: Configuration specifying which exporter type to create
//
// Returns:
//   - sdktrace.SpanExporter: The configured exporter, or nil for ExporterNone
//   - error: Any error that occurred during exporter creation
//
// Exporter types:
//   - ExporterJaeger: Creates Jaeger exporter using config.JaegerURL
//   - ExporterOTLP: Creates OTLP HTTP exporter using config.OTLPEndpoint
//   - ExporterStdout: Creates stdout exporter with pretty-printing
//   - ExporterNone: Returns nil (no-op mode)
//
// Errors can occur if:
//   - Network endpoints are unreachable
//   - Invalid URLs are provided
//   - Unsupported exporter type is specified
func createExporter(config Config) (sdktrace.SpanExporter, error) {
	switch config.ExporterType {
	case ExporterJaeger:
		return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerURL)))
	case ExporterOTLP:
		return otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(config.OTLPEndpoint),
			otlptracehttp.WithInsecure(),
		)
	case ExporterStdout:
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case ExporterNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", config.ExporterType)
	}
}

// getEnvOrDefault retrieves an environment variable value or returns a default.
// This utility function is used throughout the configuration system.
//
// Parameters:
//   - key: Environment variable name to look up
//   - defaultValue: Value to return if environment variable is not set or empty
//
// Returns:
//   - string: Environment variable value if set and non-empty, otherwise defaultValue
//
// This function treats empty string environment variables as unset.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
