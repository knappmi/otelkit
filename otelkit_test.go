package otelkit

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestOTelKitBasicFunctionality(t *testing.T) {
	// Create a test configuration
	config := DefaultConfig()
	config.ServiceName = "test-service"
	config.ExporterType = ExporterNone // No exporter for tests
	config.Debug = false

	kit, err := New(config)
	if err != nil {
		t.Fatalf("Failed to initialize OTelKit: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		kit.Shutdown(ctx)
	}()

	ctx := context.Background()

	t.Run("TraceFunction", func(t *testing.T) {
		executed := false
		err := kit.TraceFunction(ctx, "test_function", func(ctx context.Context) error {
			executed = true
			kit.AddEvent(ctx, "test_event")
			kit.SetAttributes(ctx, attribute.String("test", "value"))
			return nil
		})

		if err != nil {
			t.Errorf("TraceFunction failed: %v", err)
		}
		if !executed {
			t.Error("Function was not executed")
		}
	})

	t.Run("DatabaseOperation", func(t *testing.T) {
		executed := false
		err := kit.DatabaseOperation(ctx, "SELECT", "test_table", func(ctx context.Context) error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("DatabaseOperation failed: %v", err)
		}
		if !executed {
			t.Error("Database operation was not executed")
		}
	})

	t.Run("CacheOperation", func(t *testing.T) {
		executed := false
		err := kit.CacheOperation(ctx, "GET", "test_key", func(ctx context.Context) error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("CacheOperation failed: %v", err)
		}
		if !executed {
			t.Error("Cache operation was not executed")
		}
	})

	t.Run("TimedOperation", func(t *testing.T) {
		executed := false
		duration, err := kit.TimedOperation(ctx, "timed_test", func(ctx context.Context) error {
			executed = true
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("TimedOperation failed: %v", err)
		}
		if !executed {
			t.Error("Timed operation was not executed")
		}
		if duration < 10*time.Millisecond {
			t.Errorf("Duration should be at least 10ms, got %v", duration)
		}
	})

	t.Run("BatchOperation", func(t *testing.T) {
		executed := false
		err := kit.BatchOperation(ctx, "test_batch", 42, func(ctx context.Context) error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("BatchOperation failed: %v", err)
		}
		if !executed {
			t.Error("Batch operation was not executed")
		}
	})
}

func TestOTelKitConfiguration(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()
		
		if config.ServiceName == "" {
			t.Error("ServiceName should not be empty")
		}
		if config.ServiceVersion == "" {
			t.Error("ServiceVersion should not be empty")
		}
		if config.Environment == "" {
			t.Error("Environment should not be empty")
		}
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := Config{
			ServiceName:    "custom-service",
			ServiceVersion: "2.0.0",
			Environment:    "test",
			ExporterType:   ExporterNone,
			SampleRate:     1.0,
			Debug:          false,
		}

		kit, err := New(config)
		if err != nil {
			t.Fatalf("Failed to initialize OTelKit with custom config: %v", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			kit.Shutdown(ctx)
		}()

		if kit.config.ServiceName != "custom-service" {
			t.Errorf("Expected service name 'custom-service', got '%s'", kit.config.ServiceName)
		}
	})
}

func BenchmarkOTelKitOverhead(b *testing.B) {
	config := DefaultConfig()
	config.ExporterType = ExporterNone
	config.Debug = false

	kit, err := New(config)
	if err != nil {
		b.Fatalf("Failed to initialize OTelKit: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		kit.Shutdown(ctx)
	}()

	ctx := context.Background()

	b.Run("TraceFunction", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			kit.TraceFunction(ctx, "benchmark_function", func(ctx context.Context) error {
				// Minimal work to measure tracing overhead
				return nil
			})
		}
	})

	b.Run("StartSpan", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, span := kit.StartSpan(ctx, "benchmark_span")
			span.End()
		}
	})
}
