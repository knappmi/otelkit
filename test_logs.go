package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/knappmi/otelkit"
)

func main() {
	// Configure OTelKit to test logs with stdout exporter
	config := otelkit.Config{
		ServiceName: "test-logs",
		ServiceVersion: "1.0.0",
		Environment: "development",
		EnableLogs: true,
		LogsExporterType: otelkit.ExporterStdout, // Use stdout to see what's happening
		Debug: true,
		LogLevel: slog.LevelDebug,
	}

	// Initialize OTelKit
	kit, err := otelkit.New(config)
	if err != nil {
		panic(err)
	}
	defer kit.Shutdown(context.Background())

	ctx := context.Background()
	
	// Test different log levels
	kit.LogInfo(ctx, "This is an info message", slog.String("test", "value"))
	kit.LogWarn(ctx, "This is a warning message", slog.Int("count", 42))
	kit.LogError(ctx, "This is an error message", nil, slog.Bool("critical", true))
	kit.LogDebug(ctx, "This is a debug message")
	
	// Give time for logs to be processed
	time.Sleep(2 * time.Second)
	
	println("Test completed - check output above")
}
