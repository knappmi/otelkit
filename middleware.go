package otelkit

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns an HTTP middleware that automatically traces HTTP requests.
// 
// Parameters:
//   - next: The next HTTP handler in the middleware chain
//
// Returns:
//   - http.Handler: A wrapped handler that creates a span for each HTTP request
//
// The middleware automatically captures:
//   - HTTP method (GET, POST, etc.)
//   - Full request URL
//   - Route path
//   - User agent string
//   - Remote client address
//   - Response status code and status text
//   - Request duration
//   - Error status for 4xx/5xx responses
func (o *OTelKit) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := o.StartSpan(r.Context(), r.Method+" "+r.URL.Path,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.route", r.URL.Path),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
			),
		)
		defer span.End()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		// Execute the handler with the traced context
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", wrapped.statusCode),
			attribute.String("http.status_text", http.StatusText(wrapped.statusCode)),
		)

		// Set span status based on HTTP status code
		if wrapped.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
// This is necessary because the standard http.ResponseWriter doesn't expose
// the status code after it's written, but we need it for tracing purposes.
//
// Fields:
//   - ResponseWriter: The underlying http.ResponseWriter
//   - statusCode: The HTTP status code (defaults to 200)
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before forwarding to the underlying writer.
//
// Parameters:
//   - statusCode: The HTTP status code to write (200, 404, 500, etc.)
func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// DatabaseOperation traces a database operation with standardized attributes.
//
// Parameters:
//   - ctx: Context for the operation (will be enriched with span context)
//   - operation: The database operation type (SELECT, INSERT, UPDATE, DELETE, etc.)
//   - table: The database table name being operated on
//   - fn: The function to execute that performs the database operation
//
// Returns:
//   - error: Any error returned by the fn function
//
// This function automatically adds the following span attributes:
//   - db.operation: The operation type
//   - db.table: The table name
//   - db.type: Database type (defaults to "unknown", can be overridden in fn)
func (o *OTelKit) DatabaseOperation(ctx context.Context, operation, table string, fn func(ctx context.Context) error) error {
	return o.TraceFunction(ctx, "db."+operation,
		fn,
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
		attribute.String("db.type", "unknown"), // Can be overridden
	)
}

// CacheOperation traces a cache operation (get, set, delete, etc.).
//
// Parameters:
//   - ctx: Context for the operation (will be enriched with span context)
//   - operation: The cache operation type (get, set, delete, flush, etc.)
//   - key: The cache key being operated on
//   - fn: The function to execute that performs the cache operation
//
// Returns:
//   - error: Any error returned by the fn function
//
// This function automatically adds the following span attributes:
//   - cache.operation: The operation type
//   - cache.key: The cache key
func (o *OTelKit) CacheOperation(ctx context.Context, operation, key string, fn func(ctx context.Context) error) error {
	return o.TraceFunction(ctx, "cache."+operation,
		fn,
		attribute.String("cache.operation", operation),
		attribute.String("cache.key", key),
	)
}

// ExternalServiceCall traces a call to an external service or API.
//
// Parameters:
//   - ctx: Context for the operation (will be enriched with span context)
//   - serviceName: Name of the external service (e.g., "payment-api", "user-service")
//   - operation: The operation being performed (e.g., "get_user", "process_payment")
//   - fn: The function to execute that makes the external service call
//
// Returns:
//   - error: Any error returned by the fn function
//
// This function automatically adds the following span attributes:
//   - service.name: The external service name
//   - service.operation: The operation being performed
//
// The span name will be formatted as "external.{serviceName}.{operation}"
func (o *OTelKit) ExternalServiceCall(ctx context.Context, serviceName, operation string, fn func(ctx context.Context) error) error {
	return o.TraceFunction(ctx, "external."+serviceName+"."+operation,
		fn,
		attribute.String("service.name", serviceName),
		attribute.String("service.operation", operation),
	)
}

// TimedOperation executes a function and records its duration as a span attribute.
//
// Parameters:
//   - ctx: Context for the operation (will be enriched with span context)
//   - operationName: Name for the span (should be descriptive of the operation)
//   - fn: The function to execute and time
//
// Returns:
//   - time.Duration: The actual duration the operation took to complete
//   - error: Any error returned by the fn function
//
// This function automatically adds the following span attributes:
//   - operation.duration_ms: The operation duration in milliseconds
//
// Use this when you need to measure and return the execution time of an operation.
func (o *OTelKit) TimedOperation(ctx context.Context, operationName string, fn func(ctx context.Context) error) (time.Duration, error) {
	start := time.Now()
	
	err := o.TraceFunction(ctx, operationName, func(ctx context.Context) error {
		return fn(ctx)
	})
	
	duration := time.Since(start)
	
	// Add duration to span if it exists
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int64("operation.duration_ms", duration.Milliseconds()))
	
	return duration, err
}

// BatchOperation traces a batch operation with item count tracking.
//
// Parameters:
//   - ctx: Context for the operation (will be enriched with span context)
//   - operationName: Name of the batch operation (e.g., "process_orders", "send_emails")
//   - itemCount: Number of items being processed in the batch
//   - fn: The function to execute that performs the batch operation
//
// Returns:
//   - error: Any error returned by the fn function
//
// This function automatically adds the following span attributes:
//   - batch.operation: The operation name
//   - batch.item_count: The number of items in the batch
//
// The span name will be formatted as "batch.{operationName}"
// Use this for operations that process multiple items at once.
func (o *OTelKit) BatchOperation(ctx context.Context, operationName string, itemCount int, fn func(ctx context.Context) error) error {
	return o.TraceFunction(ctx, "batch."+operationName,
		fn,
		attribute.String("batch.operation", operationName),
		attribute.Int("batch.item_count", itemCount),
	)
}

// ConditionalTrace only creates a span if the condition is true, otherwise executes fn directly.
//
// Parameters:
//   - ctx: Context for the operation (will be enriched with span context if condition is true)
//   - condition: Boolean condition that determines whether to create a span
//   - spanName: Name for the span (only used if condition is true)
//   - fn: The function to execute
//
// Returns:
//   - error: Any error returned by the fn function
//
// This is useful for conditional tracing based on runtime conditions such as:
//   - Debug mode enabled
//   - Specific user types
//   - Feature flags
//   - Performance sensitive paths where tracing overhead should be avoided
func (o *OTelKit) ConditionalTrace(ctx context.Context, condition bool, spanName string, fn func(ctx context.Context) error) error {
	if condition {
		return o.TraceFunction(ctx, spanName, fn)
	}
	return fn(ctx)
}
