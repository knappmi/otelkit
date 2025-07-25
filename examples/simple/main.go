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
	// Configure OTelKit for OTLP export (to OpenTelemetry Collector)
	config := otelkit.Config{
		ServiceName: "otelkit-simple-demo",
		ServiceVersion: "1.0.0",
		Environment: "development",
		OTLPEndpoint: "localhost:4318", // OpenTelemetry Collector HTTP endpoint (no protocol)
	}
	
	// Use OTLP exporter to send to OpenTelemetry Collector
	config.ExporterType = otelkit.ExporterOTLP
	config.EnableMetrics = true
	config.EnableLogs = true
	config.MetricsExporterType = otelkit.ExporterOTLP  // Use OTLP for metrics through collector
	config.LogsExporterType = otelkit.ExporterOTLP
	config.Debug = true
	config.LogLevel = slog.LevelDebug

	// Initialize OTelKit
	kit, err := otelkit.New(config)
	if err != nil {
		panic(err)
	}
	defer kit.Shutdown(context.Background())

	// Set up HTTP server
	mux := http.NewServeMux()

	// Simple users endpoint
	mux.Handle("/api/users", kit.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Log the start of request
		kit.LogInfo(ctx, "Processing users request")

		// Record a custom metric
		kit.RecordMetric(ctx, "users_requested", 1)

		// Simulate some work with tracing
		err := kit.TraceFunction(ctx, "fetch_users", func(ctx context.Context) error {
			// Simulate database delay
			time.Sleep(50 * time.Millisecond)
			
			kit.SetAttributes(ctx, 
				attribute.String("query", "SELECT * FROM users"),
				attribute.Int("count", 25))
			
			kit.LogDebug(ctx, "Users fetched from database", 
				slog.Int("count", 25))
			
			return nil
		})

		if err != nil {
			kit.LogError(ctx, "Failed to fetch users", err)
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			return
		}

		// Success response
		kit.LogInfo(ctx, "Users request completed successfully")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`))
	})))

	// Health endpoint
	mux.Handle("/health", kit.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		kit.LogInfo(ctx, "Health check requested")
		kit.RecordMetric(ctx, "health_check", 1)
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})))

	// Error endpoint for testing
	mux.Handle("/error", kit.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		kit.LogWarn(ctx, "Error endpoint requested - will generate test error")
		
		// Record error metric
		kit.RecordMetric(ctx, "test_errors", 1, 
			attribute.Bool("intentional", true))
		
		kit.LogError(ctx, "This is a test error", fmt.Errorf("simulated error"))
		
		http.Error(w, "Test Error", http.StatusInternalServerError)
	})))

	// Start server
	fmt.Println("🚀 OTelKit Simple Demo Server starting on :8080")
	fmt.Println("📊 Observability stack running on:")
	fmt.Println("   - Grafana:    http://localhost:3000")
	fmt.Println("   - Jaeger:     http://localhost:16686")
	fmt.Println("   - Prometheus: http://localhost:9090")
	fmt.Println("   - Loki:       http://localhost:3100")
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
