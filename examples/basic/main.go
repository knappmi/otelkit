package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/knappmi/otelkit"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// Initialize OTelKit with default configuration
	config := otelkit.DefaultConfig()
	config.ServiceName = "example-service"
	config.ServiceVersion = "1.0.0"
	config.ExporterType = otelkit.ExporterStdout // Use stdout for demo
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
			log.Printf("Failed to shutdown OTelKit: %v", err)
		}
	}()

	// Example 1: Basic function tracing
	ctx := context.Background()
	err = kit.TraceFunction(ctx, "example.basic_operation", func(ctx context.Context) error {
		// Simulate some work
		time.Sleep(100 * time.Millisecond)
		kit.AddEvent(ctx, "processing_started")
		
		// Simulate more work
		time.Sleep(50 * time.Millisecond)
		kit.SetAttributes(ctx, attribute.String("processed_items", "42"))
		
		return nil
	})
	if err != nil {
		log.Printf("Error in basic operation: %v", err)
	}

	// Example 2: Database operation simulation
	err = kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
		// Simulate database query
		time.Sleep(25 * time.Millisecond)
		kit.AddEvent(ctx, "query_executed", attribute.String("query", "SELECT * FROM users WHERE active = true"))
		return nil
	})
	if err != nil {
		log.Printf("Error in database operation: %v", err)
	}

	// Example 3: Cache operation simulation
	err = kit.CacheOperation(ctx, "GET", "user:123", func(ctx context.Context) error {
		// Simulate cache lookup
		time.Sleep(5 * time.Millisecond)
		kit.AddEvent(ctx, "cache_hit")
		return nil
	})
	if err != nil {
		log.Printf("Error in cache operation: %v", err)
	}

	// Example 4: External service call simulation
	err = kit.ExternalServiceCall(ctx, "payment-service", "charge", func(ctx context.Context) error {
		// Simulate external API call
		time.Sleep(200 * time.Millisecond)
		kit.SetAttributes(ctx, 
			attribute.String("payment.method", "credit_card"),
			attribute.Float64("payment.amount", 99.99),
		)
		return nil
	})
	if err != nil {
		log.Printf("Error in external service call: %v", err)
	}

	// Example 5: Timed operation
	duration, err := kit.TimedOperation(ctx, "complex_calculation", func(ctx context.Context) error {
		// Simulate complex calculation
		time.Sleep(150 * time.Millisecond)
		return nil
	})
	if err != nil {
		log.Printf("Error in timed operation: %v", err)
	} else {
		fmt.Printf("Complex calculation took: %v\n", duration)
	}

	// Example 6: Batch operation
	err = kit.BatchOperation(ctx, "process_orders", 100, func(ctx context.Context) error {
		// Simulate batch processing
		time.Sleep(300 * time.Millisecond)
		kit.AddEvent(ctx, "batch_processing_complete")
		return nil
	})
	if err != nil {
		log.Printf("Error in batch operation: %v", err)
	}

	// Example 7: HTTP Server with middleware
	setupHTTPServer(kit)
}

func setupHTTPServer(kit *otelkit.OTelKit) {
	mux := http.NewServeMux()
	
	// Add a simple handler
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Add custom attributes to the span created by middleware
		kit.SetAttributes(ctx, attribute.String("user.name", "example_user"))
		
		// Simulate some processing
		err := kit.TraceFunction(ctx, "process_hello_request", func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			kit.AddEvent(ctx, "greeting_generated")
			return nil
		})
		
		if err != nil {
			kit.RecordError(ctx, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Hello, World!"}`))
	})

	// Wrap with OTel middleware
	handler := kit.HTTPMiddleware(mux)

	fmt.Println("Starting HTTP server on :8080")
	fmt.Println("Try: curl http://localhost:8080/hello")
	
	// Start server in a goroutine so example can continue
	go func() {
		if err := http.ListenAndServe(":8080", handler); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	
	// Let the server run for a bit
	time.Sleep(2 * time.Second)
	fmt.Println("HTTP server example complete")
}
