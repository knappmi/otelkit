package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/knappmi/otelkit"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// Configure OTelKit with comprehensive observability
	config := otelkit.DefaultConfig()
	config.ServiceName = "comprehensive-demo"
	config.ServiceVersion = "1.0.0"
	config.Environment = "development"
	config.ExporterType = otelkit.ExporterOTLP
	config.EnableMetrics = true
	config.EnableLogs = true
	config.MetricsExporterType = otelkit.ExporterOTLP
	config.LogsExporterType = otelkit.ExporterOTLP
	config.Debug = true
	config.LogLevel = slog.LevelDebug

	// Initialize OTelKit
	kit, err := otelkit.New(config)
	if err != nil {
		panic(err)
	}
	defer kit.Shutdown(context.Background())

	// Set up HTTP server with comprehensive observability middleware
	mux := http.NewServeMux()

	// Example handler that demonstrates all three pillars of observability
	mux.Handle("/api/users", kit.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Log the start of user processing
		kit.LogInfo(ctx, "Starting user request processing",
			slog.String("endpoint", "/api/users"),
		)

		// Simulate database operation with tracing and logging
		err := kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
			// Simulate database query time
			time.Sleep(50 * time.Millisecond)
			
			// Add custom span attributes
			kit.SetAttributes(ctx,
				attribute.String("query", "SELECT * FROM users WHERE active = true"),
				attribute.Int("rows_returned", 25),
			)
			
			kit.LogDebug(ctx, "Database query executed successfully",
				slog.Int("rows_returned", 25),
			)
			
			return nil
		})

		if err != nil {
			kit.LogError(ctx, "Database operation failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Record business metrics
		kit.RecordMetric(ctx, "user_lookup", 1,
			attribute.String("endpoint", "/api/users"),
			attribute.Bool("cache_hit", false),
		)

		// Simulate some business logic with custom tracing
		err = kit.TraceFunction(ctx, "process_user_data", func(ctx context.Context) error {
			kit.LogDebug(ctx, "Processing user data")
			
			// Simulate processing time
			time.Sleep(30 * time.Millisecond)
			
			kit.AddEvent(ctx, "data_validation_complete")
			
			return nil
		})

		if err != nil {
			kit.LogError(ctx, "User data processing failed", err)
			http.Error(w, "Processing Error", http.StatusInternalServerError)
			return
		}

		// Success response
		kit.LogInfo(ctx, "User request completed successfully",
			slog.Int("user_count", 25),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users": [], "count": 25, "status": "success"}`))
	})))

	// Health check endpoint
	mux.Handle("/health", kit.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		kit.LogDebug(ctx, "Health check requested")
		kit.RecordMetric(ctx, "health_check", 1)
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})))

	// Error endpoint to demonstrate error handling
	mux.Handle("/error", kit.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		kit.LogWarn(ctx, "Error endpoint requested - will generate test error")
		
		// Simulate an error in tracing
		err := kit.TraceFunction(ctx, "simulate_error", func(ctx context.Context) error {
			kit.LogError(ctx, "Simulated error occurred", 
				fmt.Errorf("this is a test error"))
			return fmt.Errorf("this is a test error")
		})

		kit.RecordMetric(ctx, "error_endpoint", 1,
			attribute.Bool("intentional_error", true),
		)

		if err != nil {
			http.Error(w, "Simulated Error", http.StatusInternalServerError)
			return
		}
	})))

	kit.LogInfo(context.Background(), "Starting HTTP server with comprehensive observability",
		slog.String("port", "8080"),
		slog.String("service", config.ServiceName),
	)

	// Start server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	kit.LogInfo(context.Background(), "Server started - visit http://localhost:8080/api/users for traces/logs/metrics")
	
	if err := server.ListenAndServe(); err != nil {
		kit.LogError(context.Background(), "Server failed to start", err)
	}
}
