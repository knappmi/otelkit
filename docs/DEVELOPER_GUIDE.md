# OTelKit Developer Guide

## Quick Start

```bash
go get github.com/knappmi/otelkit
```

```go
package main

import (
    "context"
    "github.com/knappmi/otelkit"
)

func main() {
    // Initialize
    config := otelkit.DefaultConfig()
    config.ServiceName = "my-service"
    
    kit, _ := otelkit.New(config)
    defer kit.Shutdown(context.Background())
    
    // Trace any function
    ctx := context.Background()
    kit.TraceFunction(ctx, "my_operation", func(ctx context.Context) error {
        // Your business logic here
        return nil
    })
}
```

## Configuration Patterns

### Environment-Based Configuration
```go
// Reads from environment variables
config := otelkit.DefaultConfig()
kit, err := otelkit.New(config)
```

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
```

### Testing Configuration
```go
config := otelkit.Config{
    ServiceName:  "test-service",
    ExporterType: otelkit.ExporterNone, // No overhead
}
```

## Instrumentation Patterns

### Basic Function Tracing
```go
err := kit.TraceFunction(ctx, "calculate_total", func(ctx context.Context) error {
    // Add attributes
    kit.SetAttributes(ctx, 
        attribute.String("customer_id", "123"),
        attribute.Float64("amount", 99.99),
    )
    
    // Add events
    kit.AddEvent(ctx, "calculation_started")
    
    // Your logic
    result := doCalculation()
    
    kit.AddEvent(ctx, "calculation_completed",
        attribute.Float64("result", result))
    
    return nil
})
```

### HTTP Server Instrumentation
```go
mux := http.NewServeMux()

mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // Span created by middleware
    
    // Add custom attributes
    kit.SetAttributes(ctx, attribute.String("user.role", "admin"))
    
    // Trace sub-operations
    err := kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
        // Database logic
        return nil
    })
    
    if err != nil {
        kit.RecordError(ctx, err)
        http.Error(w, "Error", 500)
        return
    }
    
    w.WriteHeader(200)
})

// Apply middleware
handler := kit.HTTPMiddleware(mux)
http.ListenAndServe(":8080", handler)
```

### Database Operations
```go
err := kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
    // Automatically adds:
    // - db.operation: "SELECT"
    // - db.table: "users"
    
    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    // Process results...
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

### Batch Processing
```go
items := getItemsToProcess()

err := kit.BatchOperation(ctx, "process_orders", len(items), func(ctx context.Context) error {
    // Automatically adds:
    // - batch.operation: "process_orders"
    // - batch.item_count: len(items)
    
    for i, item := range items {
        if i%10 == 0 {
            kit.AddEvent(ctx, "progress", attribute.Int("processed", i))
        }
        processItem(item)
    }
    return nil
})
```

### Manual Span Management
```go
ctx, span := kit.StartSpan(ctx, "complex_operation")
defer span.End()

// Add attributes
kit.SetAttributes(ctx, attribute.String("operation.type", "complex"))

// Add events
kit.AddEvent(ctx, "phase_1_complete")

// Record errors
if err != nil {
    kit.RecordError(ctx, err)
    return err
}
```

## Best Practices

### 1. Span Naming
- Use descriptive, hierarchical names: `user_service.get_user`
- Include operation type: `db.SELECT`, `cache.GET`, `http.POST`
- Be consistent across your codebase

### 2. Attributes
- Add relevant business context: user IDs, order amounts, etc.
- Use semantic conventions when possible
- Don't add high-cardinality values (like timestamps)

### 3. Error Handling
```go
err := kit.TraceFunction(ctx, "risky_operation", func(ctx context.Context) error {
    result, err := riskyCall()
    if err != nil {
        // Error automatically recorded by TraceFunction
        return fmt.Errorf("risky call failed: %w", err)
    }
    
    kit.SetAttributes(ctx, attribute.String("result", result))
    return nil
})

// Or manually:
if err != nil {
    kit.RecordError(ctx, err)
    return err
}
```

### 4. Sampling
- Use high sampling (100%) in development
- Use lower sampling (1-10%) in production
- Consider business-critical paths for higher sampling

### 5. Performance
- OTelKit has minimal overhead when using sampling
- Use `ExporterNone` in tests to eliminate overhead
- Prefer batch operations for high-volume scenarios

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | "unknown-service" | Service name |
| `OTEL_SERVICE_VERSION` | "1.0.0" | Service version |
| `OTEL_ENVIRONMENT` | "development" | Environment |
| `OTEL_EXPORTER_TYPE` | "stdout" | Exporter type |
| `JAEGER_URL` | "http://localhost:14268/api/traces" | Jaeger endpoint |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | "http://localhost:4318" | OTLP endpoint |
| `OTEL_DEBUG` | "false" | Debug logging |

## Deployment

### Docker Compose with Jaeger
```yaml
version: '3.8'
services:
  my-app:
    build: .
    environment:
      - OTEL_SERVICE_NAME=my-app
      - OTEL_EXPORTER_TYPE=jaeger
      - JAEGER_URL=http://jaeger:14268/api/traces
    depends_on:
      - jaeger
      
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "14268:14268"
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: my-app
        image: my-app:latest
        env:
        - name: OTEL_SERVICE_NAME
          value: "my-app"
        - name: OTEL_EXPORTER_TYPE
          value: "otlp"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://otel-collector:4318"
```

## Testing

### Unit Tests
```go
func TestMyFunction(t *testing.T) {
    config := otelkit.DefaultConfig()
    config.ExporterType = otelkit.ExporterNone // No tracing overhead
    
    kit, _ := otelkit.New(config)
    defer kit.Shutdown(context.Background())
    
    ctx := context.Background()
    err := myFunction(ctx, kit)
    assert.NoError(t, err)
}
```

### Integration Tests
```go
func TestIntegration(t *testing.T) {
    config := otelkit.DefaultConfig()
    config.ExporterType = otelkit.ExporterStdout // See traces in test output
    config.Debug = true
    
    kit, _ := otelkit.New(config)
    defer kit.Shutdown(context.Background())
    
    // Test with real tracing enabled
}
```

## Common Patterns

### Service Layer
```go
type UserService struct {
    db  *sql.DB
    kit *otelkit.OTelKit
}

func (s *UserService) GetUser(ctx context.Context, id int) (*User, error) {
    return s.kit.TraceFunction(ctx, "user_service.get_user", func(ctx context.Context) error {
        s.kit.SetAttributes(ctx, attribute.Int("user.id", id))
        
        return s.kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
            // Database logic
            return nil
        })
    })
}
```

### Repository Pattern
```go
type UserRepository struct {
    db  *sql.DB
    kit *otelkit.OTelKit
}

func (r *UserRepository) FindByID(ctx context.Context, id int) (*User, error) {
    return r.kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
        // Database-specific logic
        return nil
    })
}
```

### Error Patterns
```go
// Business errors vs system errors
err := kit.TraceFunction(ctx, "validate_user", func(ctx context.Context) error {
    if user.Email == "" {
        // Business validation error - not span error
        kit.AddEvent(ctx, "validation_failed", 
            attribute.String("field", "email"))
        return ErrInvalidEmail // Don't record as span error
    }
    
    // System error - should be recorded
    if err := externalValidation(user); err != nil {
        return err // This will be recorded as span error
    }
    
    return nil
})
```

## Migration Guide

### From Manual OpenTelemetry
```go
// Before:
tracer := otel.Tracer("my-service")
ctx, span := tracer.Start(ctx, "operation")
defer span.End()
span.SetAttributes(attribute.String("key", "value"))

// After:
ctx, span := kit.StartSpan(ctx, "operation")
defer span.End()
kit.SetAttributes(ctx, attribute.String("key", "value"))

// Or even simpler:
kit.TraceFunction(ctx, "operation", func(ctx context.Context) error {
    kit.SetAttributes(ctx, attribute.String("key", "value"))
    return nil
})
```

### From Other Tracing Libraries
OTelKit provides a higher-level API that reduces boilerplate while maintaining OpenTelemetry compatibility. Most tracing concepts translate directly:

- Spans → `TraceFunction` or `StartSpan`
- Tags/Labels → `SetAttributes`
- Logs → `AddEvent`
- Errors → `RecordError`
