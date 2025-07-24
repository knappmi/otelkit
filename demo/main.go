package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/knappmi/otelkit"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// Initialize OTelKit with configuration from environment
	config := otelkit.DefaultConfig()
	config.ServiceName = "demo-service"
	config.ServiceVersion = "1.0.0"
	config.Debug = true

	kit, err := otelkit.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize OTelKit: %v", err)
	}

	// Ensure proper shutdown
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := kit.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	ctx := context.Background()

	// Example 1: Simple function tracing
	err = kit.TraceFunction(ctx, "calculate_total", func(ctx context.Context) error {
		kit.AddEvent(ctx, "calculation_started")
		time.Sleep(50 * time.Millisecond) // Simulate work
		kit.SetAttributes(ctx, 
			attribute.String("calculation.type", "total"),
			attribute.Int("items.count", 25),
		)
		return nil
	})
	if err != nil {
		log.Printf("Error in calculation: %v", err)
	}

	// Example 2: Database operation
	err = kit.TraceFunction(ctx, "db.query_users", func(ctx context.Context) error {
		kit.SetAttributes(ctx,
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "users"),
		)
		kit.AddEvent(ctx, "query_executed")
		time.Sleep(30 * time.Millisecond) // Simulate DB query
		kit.AddEvent(ctx, "results_processed", 
			attribute.Int("rows.count", 15))
		return nil
	})
	if err != nil {
		log.Printf("Error in database query: %v", err)
	}

	// Example 3: External API call
	err = kit.TraceFunction(ctx, "http.external_api_call", func(ctx context.Context) error {
		kit.SetAttributes(ctx,
			attribute.String("http.method", "GET"),
			attribute.String("http.url", "https://api.example.com/data"),
			attribute.String("service.name", "external-api"),
		)
		kit.AddEvent(ctx, "request_sent")
		time.Sleep(100 * time.Millisecond) // Simulate network call
		kit.SetAttributes(ctx, attribute.Int("http.status_code", 200))
		kit.AddEvent(ctx, "response_received")
		return nil
	})
	if err != nil {
		log.Printf("Error in API call: %v", err)
	}

	fmt.Printf("Demo complete. Traces sent to %s exporter.\n", config.ExporterType)
	fmt.Println("If using Jaeger, check http://localhost:16686")
	
	// Give time for traces to be exported
	time.Sleep(2 * time.Second)
}
