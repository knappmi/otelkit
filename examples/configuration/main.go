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
	// Example 1: Using environment variables
	fmt.Println("=== Configuration Examples ===")
	
	// Default configuration (reads from environment variables)
	config1 := otelkit.DefaultConfig()
	fmt.Printf("Default config: Service=%s, Version=%s, Exporter=%s\n", 
		config1.ServiceName, config1.ServiceVersion, config1.ExporterType)

	// Example 2: Custom configuration for development
	devConfig := otelkit.Config{
		ServiceName:    "my-dev-service",
		ServiceVersion: "0.1.0",
		Environment:    "development",
		ExporterType:   otelkit.ExporterStdout,
		SampleRate:     1.0, // 100% sampling in dev
		Debug:          true,
	}

	kit1, err := otelkit.New(devConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer kit1.Shutdown(context.Background())

	// Example 3: Production configuration with Jaeger
	prodConfig := otelkit.Config{
		ServiceName:    "my-prod-service", 
		ServiceVersion: "1.2.3",
		Environment:    "production",
		ExporterType:   otelkit.ExporterJaeger,
		JaegerURL:      "http://jaeger:14268/api/traces",
		SampleRate:     0.01, // 1% sampling in production
		Debug:          false,
	}

	kit2, err := otelkit.New(prodConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer kit2.Shutdown(context.Background())

	// Example 4: OTLP configuration
	otlpConfig := otelkit.Config{
		ServiceName:    "my-otlp-service",
		ServiceVersion: "2.0.0", 
		Environment:    "staging",
		ExporterType:   otelkit.ExporterOTLP,
		OTLPEndpoint:   "http://otel-collector:4318",
		SampleRate:     0.1, // 10% sampling
		Debug:          false,
	}

	kit3, err := otelkit.New(otlpConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer kit3.Shutdown(context.Background())

	// Example 5: Disabled tracing for testing
	testConfig := otelkit.Config{
		ServiceName:  "test-service",
		ExporterType: otelkit.ExporterNone, // No tracing overhead
		Debug:        false,
	}

	kit4, err := otelkit.New(testConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer kit4.Shutdown(context.Background())

	// Demonstrate different configurations
	ctx := context.Background()

	fmt.Println("\n=== Testing Different Configurations ===")

	// Development tracing (stdout, verbose)
	fmt.Println("Development tracing:")
	kit1.TraceFunction(ctx, "dev_operation", func(ctx context.Context) error {
		kit1.SetAttributes(ctx, attribute.String("config", "development"))
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	// Production tracing (would go to Jaeger in real deployment)
	fmt.Println("Production tracing:")
	kit2.TraceFunction(ctx, "prod_operation", func(ctx context.Context) error {
		kit2.SetAttributes(ctx, attribute.String("config", "production"))
		time.Sleep(5 * time.Millisecond)
		return nil
	})

	// No tracing overhead
	fmt.Println("Test tracing (no output expected):")
	kit4.TraceFunction(ctx, "test_operation", func(ctx context.Context) error {
		kit4.SetAttributes(ctx, attribute.String("config", "test"))
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	fmt.Println("\n=== Configuration Complete ===")
	fmt.Println("Set these environment variables to configure OTelKit:")
	fmt.Println("- OTEL_SERVICE_NAME=my-service")
	fmt.Println("- OTEL_SERVICE_VERSION=1.0.0")
	fmt.Println("- OTEL_ENVIRONMENT=production")
	fmt.Println("- OTEL_EXPORTER_TYPE=jaeger")
	fmt.Println("- JAEGER_URL=http://localhost:14268/api/traces")
	fmt.Println("- OTEL_DEBUG=false")
}
