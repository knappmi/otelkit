# OTelKit

A Go OpenTelemetry wrapper that simplifies instrumenting your Go applications with minimal effort.

## 📚 Documentation

- **[Getting Started Guide](README.md)** - Quick setup and basic usage
- **[OpenTelemetry Overview](OPENTELEMETRY_OVERVIEW.md)** - Complete guide to OpenTelemetry concepts and architecture  
- **[Developer Guide](DEVELOPER_GUIDE.md)** - Advanced usage patterns and best practices
- **[Examples Output](EXAMPLES_OUTPUT_REAL.md)** - Real trace output and demonstrations
- **[API Reference](https://pkg.go.dev/github.com/knappmi/otelkit)** - Complete API documentation

## Features

- 🚀 **Easy Setup**: Initialize with a single function call
- 🔧 **Multiple Exporters**: Support for Jaeger, OTLP, Stdout, and No-op
- 🌐 **HTTP Middleware**: Automatic HTTP request tracing
- 📊 **Built-in Patterns**: Database, cache, external service, and batch operation tracing
- ⚡ **Performance**: Minimal overhead with configurable sampling
- 🧪 **Test-Friendly**: Easy to disable tracing in tests
- 📝 **Rich Context**: Automatic span attributes and events

## Installation

```bash
go get github.com/knappmi/otelkit
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/knappmi/otelkit"
)

func main() {
    // Initialize with default configuration
    config := otelkit.DefaultConfig()
    config.ServiceName = "my-service"
    config.ExporterType = otelkit.ExporterStdout
    
    kit, err := otelkit.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer kit.Shutdown(context.Background())
    
    // Trace any function
    ctx := context.Background()
    err = kit.TraceFunction(ctx, "my_operation", func(ctx context.Context) error {
        // Your business logic here
        return nil
    })
}
```

## Configuration

OTelKit can be configured through environment variables or programmatically:

### Environment Variables

- `OTEL_SERVICE_NAME`: Service name (default: "unknown-service")
- `OTEL_SERVICE_VERSION`: Service version (default: "1.0.0")
- `OTEL_ENVIRONMENT`: Environment (default: "development")
- `OTEL_EXPORTER_TYPE`: Exporter type - "jaeger", "otlp", "stdout", "none" (default: "stdout")
- `JAEGER_URL`: Jaeger collector URL (default: "http://localhost:14268/api/traces")
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint (default: "http://localhost:4318")
- `OTEL_DEBUG`: Enable debug logging (default: "false")

### Programmatic Configuration

```go
config := otelkit.Config{
    ServiceName:    "my-api",
    ServiceVersion: "1.2.3",
    Environment:    "production",
    ExporterType:   otelkit.ExporterJaeger,
    JaegerURL:      "http://jaeger:14268/api/traces",
    SampleRate:     0.1, // 10% sampling
    Debug:          false,
}

kit, err := otelkit.New(config)
```

## Usage Examples

### Basic Function Tracing

```go
err := kit.TraceFunction(ctx, "calculate_total", func(ctx context.Context) error {
    // Add custom attributes
    kit.SetAttributes(ctx, 
        attribute.String("customer_id", "123"),
        attribute.Float64("amount", 99.99),
    )
    
    // Add events
    kit.AddEvent(ctx, "calculation_started")
    
    // Your logic here
    total := calculateTotal()
    
    kit.AddEvent(ctx, "calculation_completed", 
        attribute.Float64("result", total))
    
    return nil
})
```

### HTTP Middleware

```go
mux := http.NewServeMux()
mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // The span is automatically created by middleware
    kit.SetAttributes(ctx, attribute.String("user.role", "admin"))
    
    // Trace internal operations
    err := kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
        // Database query logic
        return nil
    })
    
    if err != nil {
        kit.RecordError(ctx, err)
        http.Error(w, "Internal Error", 500)
        return
    }
    
    w.WriteHeader(200)
})

// Wrap with OTel middleware
handler := kit.HTTPMiddleware(mux)
http.ListenAndServe(":8080", handler)
```

### Database Operations

```go
err := kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
    // Additional attributes are automatically added:
    // - db.operation: "SELECT"
    // - db.table: "users"
    // - db.type: "unknown" (can be overridden)
    
    rows, err := db.QueryContext(ctx, "SELECT * FROM users WHERE active = ?", true)
    // ... handle query
    return err
})
```

### Cache Operations

```go
err := kit.CacheOperation(ctx, "GET", "user:123", func(ctx context.Context) error {
    // Automatically adds:
    // - cache.operation: "GET"
    // - cache.key: "user:123"
    
    value, err := cache.Get("user:123")
    if err == nil {
        kit.AddEvent(ctx, "cache_hit")
    } else {
        kit.AddEvent(ctx, "cache_miss")
    }
    return err
})
```

### External Service Calls

```go
err := kit.ExternalServiceCall(ctx, "payment-service", "charge", func(ctx context.Context) error {
    // Automatically adds:
    // - service.name: "payment-service"
    // - service.operation: "charge"
    
    response, err := paymentClient.Charge(ctx, request)
    if err != nil {
        return err
    }
    
    kit.SetAttributes(ctx, 
        attribute.String("payment.id", response.ID),
        attribute.String("payment.status", response.Status),
    )
    
    return nil
})
```

### Batch Operations

```go
items := []Item{...} // 100 items

err := kit.BatchOperation(ctx, "process_orders", len(items), func(ctx context.Context) error {
    // Automatically adds:
    // - batch.operation: "process_orders" 
    // - batch.item_count: 100
    
    for i, item := range items {
        if i%10 == 0 {
            kit.AddEvent(ctx, "batch_progress", 
                attribute.Int("processed", i))
        }
        // Process item...
    }
    
    return nil
})
```

### Timed Operations

```go
duration, err := kit.TimedOperation(ctx, "complex_calculation", func(ctx context.Context) error {
    // This automatically measures and records the duration
    time.Sleep(100 * time.Millisecond) // Simulate work
    return nil
})

fmt.Printf("Operation took: %v\n", duration)
// The span will also have an "operation.duration_ms" attribute
```

### Conditional Tracing

```go
// Only trace in debug mode
err := kit.ConditionalTrace(ctx, debugMode, "debug_operation", func(ctx context.Context) error {
    // This only creates a span if debugMode is true
    return performDebugOperation()
})
```

## Testing

For testing, you can disable tracing to avoid overhead:

```go
func setupTestKit() *otelkit.OTelKit {
    config := otelkit.DefaultConfig()
    config.ExporterType = otelkit.ExporterNone // No tracing in tests
    
    kit, _ := otelkit.New(config)
    return kit
}
```

## Exporters

### Jaeger

```go
config.ExporterType = otelkit.ExporterJaeger
config.JaegerURL = "http://localhost:14268/api/traces"
```

### OTLP (OpenTelemetry Protocol)

```go
config.ExporterType = otelkit.ExporterOTLP
config.OTLPEndpoint = "http://localhost:4318"
```

### Stdout (Development)

```go
config.ExporterType = otelkit.ExporterStdout
```

### None (Testing/Disabled)

```go
config.ExporterType = otelkit.ExporterNone
```

## Performance

OTelKit is designed with minimal overhead:

- Configurable sampling rates (default: 10%)
- No-op mode for production if needed
- Efficient span creation and management
- Minimal allocations in hot paths

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Examples

See the [examples](examples/) directory for complete working examples:

- [Basic Usage](examples/basic/main.go) - Comprehensive example showing all features
- HTTP server with middleware
- Database and cache operations
- External service calls
- Batch processing

## Roadmap

- [ ] Metrics support with OpenTelemetry metrics
- [ ] Logging integration
- [ ] Additional exporters (AWS X-Ray, Google Cloud Trace)
- [ ] Automatic database driver instrumentation
- [ ] gRPC middleware
- [ ] Gin/Echo framework integration
