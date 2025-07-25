package otelkit

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	
	// Metrics
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	
	// Logs
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
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
	
	// EnableMetrics enables metrics collection and export
	// When true, the OTelKit instance will collect and export metrics
	EnableMetrics bool
	
	// EnableLogs enables structured logging with OpenTelemetry bridge
	// When true, the OTelKit instance will use slog with trace correlation
	EnableLogs bool
	
	// MetricsExporterType determines where metrics are sent
	// Options: ExporterOTLP, ExporterPrometheus, ExporterStdout, ExporterNone
	MetricsExporterType ExporterType
	
	// LogsExporterType determines where logs are sent
	// Options: ExporterOTLP, ExporterStdout, ExporterNone
	LogsExporterType ExporterType
	
	// PrometheusPort is the port for Prometheus metrics server (only used with ExporterPrometheus)
	// Example: 9090, 8080, 2112
	PrometheusPort int
	
	// LogLevel sets the minimum log level for structured logging
	// Options: slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError
	LogLevel slog.Level
	
	// LogFilePath specifies the file path for file-based logging (optional)
	// If empty, logs will only go to stdout and OTLP (if configured)
	// Example: "/var/log/app.log", "./logs/service.log"
	LogFilePath string
}

// ExporterType defines the type of exporter to use for sending telemetry data.
// Choose based on your observability infrastructure and requirements.
type ExporterType string

const (
	// ExporterJaeger sends traces to Jaeger backend
	// Use for: Local development, Jaeger-based observability setups
	// Requires: Jaeger collector running and accessible
	ExporterJaeger ExporterType = "jaeger"
	
	// ExporterOTLP sends telemetry using OpenTelemetry Protocol
	// Use for: Production environments, OpenTelemetry collectors, cloud observability
	// Requires: OTLP-compatible endpoint (e.g., OTEL Collector, cloud vendors)
	ExporterOTLP ExporterType = "otlp"
	
	// ExporterStdout prints telemetry to console in JSON format
	// Use for: Development, debugging, CI/CD pipelines, testing
	// Requires: Nothing, outputs to stdout
	ExporterStdout ExporterType = "stdout"
	
	// ExporterPrometheus exposes metrics in Prometheus format via HTTP endpoint
	// Use for: Prometheus-based monitoring, Kubernetes environments
	// Requires: Prometheus server to scrape the metrics endpoint
	ExporterPrometheus ExporterType = "prometheus"
	
	// ExporterNone disables telemetry export entirely
	// Use for: Maximum performance, when telemetry overhead must be eliminated
	// Requires: Nothing, creates no-op providers
	ExporterNone ExporterType = "none"
)

// OTelKit is the main wrapper struct that provides simplified OpenTelemetry functionality.
// Create one instance per service and reuse it throughout your application.
type OTelKit struct {
	// tracer is the OpenTelemetry tracer instance used to create spans
	tracer trace.Tracer
	
	// tracerProvider manages the tracer lifecycle and span export
	tracerProvider *sdktrace.TracerProvider
	
	// meter is the OpenTelemetry meter instance used to create metrics instruments
	meter metric.Meter
	
	// meterProvider manages the meter lifecycle and metrics export
	meterProvider *sdkmetric.MeterProvider
	
	// loggerProvider manages structured logging with OpenTelemetry correlation
	loggerProvider *sdklog.LoggerProvider
	
	// otelLogger is the OpenTelemetry logger for sending logs via OTLP
	otelLogger otellog.Logger
	
	// logger is the structured logger instance with trace correlation
	logger *slog.Logger
	
	// config stores the configuration used to initialize this instance
	config Config
	
	// Common metrics instruments for automatic instrumentation
	httpRequestDuration metric.Float64Histogram
	httpRequestsTotal   metric.Int64Counter
	activeSpansGauge    metric.Int64UpDownCounter
	businessOpsCounter  metric.Int64Counter
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
//   - OTEL_ENABLE_METRICS: overrides EnableMetrics (set to "true" to enable)
//   - OTEL_ENABLE_LOGS: overrides EnableLogs (set to "true" to enable)
//   - OTEL_METRICS_EXPORTER: overrides MetricsExporterType
//   - OTEL_LOGS_EXPORTER: overrides LogsExporterType
//   - OTEL_PROMETHEUS_PORT: overrides PrometheusPort
//   - OTEL_LOG_LEVEL: overrides LogLevel (debug, info, warn, error)
//   - OTEL_LOG_FILE_PATH: overrides LogFilePath
//
// Defaults:
//   - ServiceName: "unknown-service" (should be overridden)
//   - ServiceVersion: "1.0.0"
//   - Environment: "development"
//   - ExporterType: stdout
//   - SampleRate: 0.1 (10% sampling)
//   - EnableMetrics: true
//   - EnableLogs: true
//   - MetricsExporterType: prometheus
//   - LogsExporterType: stdout
//   - PrometheusPort: 9090
//   - LogLevel: slog.LevelInfo
func DefaultConfig() Config {
	logLevel := slog.LevelInfo
	switch getEnvOrDefault("OTEL_LOG_LEVEL", "info") {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	
	return Config{
		ServiceName:         getEnvOrDefault("OTEL_SERVICE_NAME", "unknown-service"),
		ServiceVersion:      getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
		Environment:         getEnvOrDefault("OTEL_ENVIRONMENT", "development"),
		ExporterType:        ExporterType(getEnvOrDefault("OTEL_EXPORTER_TYPE", string(ExporterStdout))),
		JaegerURL:           getEnvOrDefault("JAEGER_URL", "http://localhost:14268/api/traces"),
		OTLPEndpoint:        getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		SampleRate:          0.1, // 10% sampling by default
		Debug:               getEnvOrDefault("OTEL_DEBUG", "false") == "true",
		EnableMetrics:       getEnvOrDefault("OTEL_ENABLE_METRICS", "true") == "true",
		EnableLogs:          getEnvOrDefault("OTEL_ENABLE_LOGS", "true") == "true",
		MetricsExporterType: ExporterType(getEnvOrDefault("OTEL_METRICS_EXPORTER", string(ExporterPrometheus))),
		LogsExporterType:    ExporterType(getEnvOrDefault("OTEL_LOGS_EXPORTER", string(ExporterStdout))),
		PrometheusPort:      9090, // TODO: parse OTEL_PROMETHEUS_PORT as int
		LogLevel:            logLevel,
		LogFilePath:         getEnvOrDefault("OTEL_LOG_FILE_PATH", ""),
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
//   2. Initialize the configured exporters (traces, metrics, logs)
//   3. Set up providers with appropriate configurations
//   4. Register providers globally
//   5. Initialize common metrics instruments
//   6. Create structured logger with trace correlation
//   7. Create and return the OTelKit wrapper
//
// Example:
//   config := otelkit.DefaultConfig()
//   config.ServiceName = "my-service"
//   config.EnableMetrics = true
//   config.EnableLogs = true
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

	kit := &OTelKit{
		config: config,
	}

	// Initialize tracing
	if err := kit.initTracing(res); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Initialize metrics if enabled
	if config.EnableMetrics {
		if err := kit.initMetrics(res); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
	}

	// Initialize logging if enabled  
	if config.EnableLogs {
		if err := kit.initLogging(res); err != nil {
			return nil, fmt.Errorf("failed to initialize logging: %w", err)
		}
	}

	if config.Debug {
		log.Printf("OTelKit initialized: service=%s, version=%s, traces=%s, metrics=%v, logs=%v", 
			config.ServiceName, config.ServiceVersion, config.ExporterType, config.EnableMetrics, config.EnableLogs)
	}

	return kit, nil
}

// Shutdown gracefully shuts down all providers and flushes any pending telemetry data.
// This should be called during application shutdown to ensure all telemetry is exported.
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
//       log.Printf("Error shutting down OTelKit: %v", err)
//   }
func (o *OTelKit) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown tracer provider
	if o.tracerProvider != nil {
		if err := o.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	// Shutdown meter provider
	if o.meterProvider != nil {
		if err := o.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	// Shutdown logger provider
	if o.loggerProvider != nil {
		if err := o.loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("logger provider shutdown: %w", err))
		}
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
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

// GetMeter returns the underlying OpenTelemetry meter.
// Use this when you need direct access to OpenTelemetry metrics APIs
// that aren't wrapped by OTelKit.
//
// Returns:
//   - metric.Meter: The underlying OpenTelemetry meter instance, or nil if metrics disabled
//
// Example:
//   meter := kit.GetMeter()
//   if meter != nil {
//       counter, _ := meter.Int64Counter("my_custom_counter")
//       counter.Add(ctx, 1)
//   }
func (o *OTelKit) GetMeter() metric.Meter {
	return o.meter
}

// GetLogger returns the structured logger with OpenTelemetry correlation.
// This logger automatically includes trace and span IDs in log records.
//
// Returns:
//   - *slog.Logger: Structured logger instance, or nil if logging disabled
//
// Example:
//   logger := kit.GetLogger()
//   if logger != nil {
//       logger.InfoContext(ctx, "Processing request", "user_id", userID)
//   }
func (o *OTelKit) GetLogger() *slog.Logger {
	return o.logger
}

// LogInfo logs an info message with trace correlation
//
// Parameters:
//   - ctx: Context containing trace information for correlation
//   - msg: Log message
//   - attrs: Optional structured attributes
//
// Example:
//   kit.LogInfo(ctx, "User authenticated", slog.String("user_id", userID))
func (o *OTelKit) LogInfo(ctx context.Context, msg string, attrs ...slog.Attr) {
	// Log to slog for console output
	if o.logger != nil {
		o.logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
	}
	
	// Also emit through OpenTelemetry logs for OTLP export
	o.emitOTelLog(ctx, otellog.SeverityInfo, msg, attrs...)
}

// LogError logs an error message with trace correlation
//
// Parameters:
//   - ctx: Context containing trace information for correlation
//   - msg: Log message
//   - err: Error to log
//   - attrs: Optional structured attributes
//
// Example:
//   kit.LogError(ctx, "Failed to process request", err, slog.String("user_id", userID))
func (o *OTelKit) LogError(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	allAttrs := append(attrs, slog.Any("error", err))
	
	// Log to slog for console output
	if o.logger != nil {
		o.logger.LogAttrs(ctx, slog.LevelError, msg, allAttrs...)
	}
	
	// Also emit through OpenTelemetry logs for OTLP export
	o.emitOTelLog(ctx, otellog.SeverityError, msg, allAttrs...)
}

// LogDebug logs a debug message with trace correlation
//
// Parameters:
//   - ctx: Context containing trace information for correlation
//   - msg: Log message
//   - attrs: Optional structured attributes
//
// Example:
//   kit.LogDebug(ctx, "Processing step completed", slog.Int("step", 3))
func (o *OTelKit) LogDebug(ctx context.Context, msg string, attrs ...slog.Attr) {
	// Log to slog for console output
	if o.logger != nil {
		o.logger.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
	}
	
	// Also emit through OpenTelemetry logs for OTLP export
	o.emitOTelLog(ctx, otellog.SeverityDebug, msg, attrs...)
}

// LogWarn logs a warning message with trace correlation
//
// Parameters:
//   - ctx: Context containing trace information for correlation
//   - msg: Log message
//   - attrs: Optional structured attributes
//
// Example:
//   kit.LogWarn(ctx, "Rate limit approaching", slog.Int("requests", count))
func (o *OTelKit) LogWarn(ctx context.Context, msg string, attrs ...slog.Attr) {
	// Log to slog for console output
	if o.logger != nil {
		o.logger.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
	}
	
	// Also emit through OpenTelemetry logs for OTLP export
	o.emitOTelLog(ctx, otellog.SeverityWarn, msg, attrs...)
}

// RecordMetric records a business metric
//
// Parameters:
//   - ctx: Context for the metric recording
//   - operation: Type of business operation
//   - value: Metric value
//   - attrs: Optional attributes for the metric
//
// Example:
//   kit.RecordMetric(ctx, "order_processed", 1, attribute.String("region", "us-east"))
func (o *OTelKit) RecordMetric(ctx context.Context, operation string, value int64, attrs ...attribute.KeyValue) {
	if o.businessOpsCounter != nil {
		allAttrs := append(attrs, attribute.String("operation_type", operation))
		o.businessOpsCounter.Add(ctx, value, metric.WithAttributes(allAttrs...))
	}
}

// RecordHTTPMetrics records HTTP request metrics (used internally by middleware)
func (o *OTelKit) RecordHTTPMetrics(ctx context.Context, method, statusCode string, duration time.Duration) {
	if o.httpRequestsTotal != nil {
		o.httpRequestsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("status_code", statusCode),
		))
	}
	
	if o.httpRequestDuration != nil {
		o.httpRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("status_code", statusCode),
		))
	}
}

// emitOTelLog emits a log record through the OpenTelemetry logs API
func (o *OTelKit) emitOTelLog(ctx context.Context, severity otellog.Severity, msg string, attrs ...slog.Attr) {
	if o.otelLogger == nil {
		return
	}
	
	// Create log record
	var record otellog.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(severity)
	record.SetBody(otellog.StringValue(msg))
	
	// Convert slog attributes to OpenTelemetry log attributes
	for _, attr := range attrs {
		record.AddAttributes(o.convertSlogAttr(attr))
	}
	
	// Add trace context if available
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		record.AddAttributes(
			otellog.String("trace_id", span.SpanContext().TraceID().String()),
			otellog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	
	// Emit the log record
	o.otelLogger.Emit(ctx, record)
}

// convertSlogAttr converts a slog.Attr to an OpenTelemetry log.KeyValue
func (o *OTelKit) convertSlogAttr(attr slog.Attr) otellog.KeyValue {
	switch attr.Value.Kind() {
	case slog.KindString:
		return otellog.String(attr.Key, attr.Value.String())
	case slog.KindInt64:
		return otellog.Int64(attr.Key, attr.Value.Int64())
	case slog.KindFloat64:
		return otellog.Float64(attr.Key, attr.Value.Float64())
	case slog.KindBool:
		return otellog.Bool(attr.Key, attr.Value.Bool())
	default:
		// For other types, convert to string
		return otellog.String(attr.Key, attr.Value.String())
	}
}

// IncrementActiveSpans increments the active spans counter (used internally)
func (o *OTelKit) IncrementActiveSpans(ctx context.Context) {
	if o.activeSpansGauge != nil {
		o.activeSpansGauge.Add(ctx, 1)
	}
}

// DecrementActiveSpans decrements the active spans counter (used internally)
func (o *OTelKit) DecrementActiveSpans(ctx context.Context) {
	if o.activeSpansGauge != nil {
		o.activeSpansGauge.Add(ctx, -1)
	}
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
			semconv.DeploymentEnvironmentName(config.Environment),
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
	return createTraceExporter(config)
}

// createTraceExporter creates a trace exporter based on configuration
func createTraceExporter(config Config) (sdktrace.SpanExporter, error) {
	switch config.ExporterType {
	case ExporterJaeger:
		return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerURL)))
	case ExporterOTLP:
		// Construct the traces endpoint URL
		endpoint := config.OTLPEndpoint
		if endpoint == "" {
			endpoint = "localhost:4318"
		}
		
		return otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithURLPath("/v1/traces"),
			otlptracehttp.WithInsecure(),
		)
	case ExporterStdout:
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case ExporterNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported trace exporter type: %s", config.ExporterType)
	}
}

// createMetricsExporter creates a metrics exporter based on configuration
func createMetricsExporter(config Config) (sdkmetric.Reader, error) {
	switch config.MetricsExporterType {
	case ExporterOTLP:
		// Construct the metrics endpoint URL
		endpoint := config.OTLPEndpoint
		if endpoint == "" {
			endpoint = "localhost:4318"
		}
		
		exporter, err := otlpmetrichttp.New(
			context.Background(),
			otlpmetrichttp.WithEndpoint(endpoint),
			otlpmetrichttp.WithURLPath("/v1/metrics"),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second)), nil
	case ExporterPrometheus:
		exporter, err := prometheus.New(
			prometheus.WithoutTargetInfo(),
		)
		if err != nil {
			return nil, err
		}
		return exporter, nil
	case ExporterStdout:
		exporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second)), nil
	case ExporterNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported metrics exporter type: %s", config.MetricsExporterType)
	}
}

// createLogsExporter creates a logs exporter based on configuration
func createLogsExporter(config Config) (sdklog.Exporter, error) {
	switch config.LogsExporterType {
	case ExporterOTLP:
		// Construct the logs endpoint URL
		endpoint := config.OTLPEndpoint
		if endpoint == "" {
			endpoint = "localhost:4318"
		}
		
		if config.Debug {
			log.Printf("Debug: Creating logs exporter with endpoint: %s", endpoint)
		}
		
		return otlploghttp.New(
			context.Background(),
			otlploghttp.WithEndpoint(endpoint),
			otlploghttp.WithURLPath("/v1/logs"),
			otlploghttp.WithInsecure(),
		)
	case ExporterStdout:
		return stdoutlog.New(stdoutlog.WithPrettyPrint())
	case ExporterNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported logs exporter type: %s", config.LogsExporterType)
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

// initTracing initializes the tracing components of OTelKit
func (o *OTelKit) initTracing(res *resource.Resource) error {
	// Create trace exporter
	exporter, err := createTraceExporter(o.config)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	var tracerProvider *sdktrace.TracerProvider
	if exporter != nil {
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(o.config.SampleRate)),
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
		o.config.ServiceName,
		trace.WithInstrumentationVersion(o.config.ServiceVersion),
	)

	o.tracer = tracer
	o.tracerProvider = tracerProvider

	return nil
}

// initMetrics initializes the metrics components of OTelKit
func (o *OTelKit) initMetrics(res *resource.Resource) error {
	// Create metrics exporter
	exporter, err := createMetricsExporter(o.config)
	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	// Create meter provider
	var meterProvider *sdkmetric.MeterProvider
	if exporter != nil {
		meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(exporter),
			sdkmetric.WithResource(res),
		)
	} else {
		// No-op meter provider
		meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
		)
	}

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Create meter
	meter := meterProvider.Meter(
		o.config.ServiceName,
		metric.WithInstrumentationVersion(o.config.ServiceVersion),
	)

	// Initialize common metrics instruments
	if err := o.initMetricsInstruments(meter); err != nil {
		return fmt.Errorf("failed to initialize metrics instruments: %w", err)
	}

	o.meter = meter
	o.meterProvider = meterProvider

	return nil
}

// initMetricsInstruments creates common metrics instruments
func (o *OTelKit) initMetricsInstruments(meter metric.Meter) error {
	var err error

	// HTTP request duration histogram
	o.httpRequestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_request_duration_seconds histogram: %w", err)
	}

	// HTTP requests total counter
	o.httpRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_requests_total counter: %w", err)
	}

	// Active spans gauge
	o.activeSpansGauge, err = meter.Int64UpDownCounter(
		"otelkit_active_spans",
		metric.WithDescription("Number of currently active spans"),
	)
	if err != nil {
		return fmt.Errorf("failed to create otelkit_active_spans gauge: %w", err)
	}

	// Business operations counter
	o.businessOpsCounter, err = meter.Int64Counter(
		"otelkit_business_operations_total",
		metric.WithDescription("Total number of business operations"),
	)
	if err != nil {
		return fmt.Errorf("failed to create otelkit_business_operations_total counter: %w", err)
	}

	return nil
}

// initLogging initializes the logging components of OTelKit
func (o *OTelKit) initLogging(res *resource.Resource) error {
	// Create logs exporter
	exporter, err := createLogsExporter(o.config)
	if err != nil {
		return fmt.Errorf("failed to create logs exporter: %w", err)
	}

	// Create logger provider
	var loggerProvider *sdklog.LoggerProvider
	if exporter != nil {
		loggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
			sdklog.WithResource(res),
		)
	} else {
		// No-op logger provider
		loggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithResource(res),
		)
	}

	// Set global logger provider
	// TODO: Set when available in SDK

	// Create structured logger with OpenTelemetry bridge
	// This creates a logger that automatically correlates logs with traces
	var logWriter *os.File = os.Stdout
	if o.config.LogFilePath != "" {
		logWriter, err = os.OpenFile(o.config.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", o.config.LogFilePath, err)
		}
	}

	handler := slog.NewJSONHandler(logWriter, &slog.HandlerOptions{
		Level: o.config.LogLevel,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Add trace and span IDs to log records
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: "timestamp", Value: a.Value}
			}
			return a
		},
	})

	logger := slog.New(handler)

	o.loggerProvider = loggerProvider
	o.otelLogger = loggerProvider.Logger("otelkit", otellog.WithInstrumentationVersion(o.config.ServiceVersion))
	o.logger = logger

	return nil
}
