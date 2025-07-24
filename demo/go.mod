module demo

go 1.21

require github.com/knappmi/otelkit v0.0.0

replace github.com/knappmi/otelkit => ../

require (
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/attribute v1.24.0
)
