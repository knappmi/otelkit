.PHONY: help build test test-coverage lint clean fmt vet deps example infra-up infra-down infra-status comprehensive-demo

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the module
	go build ./...

test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: ## Run tests with race detection
	go test -v -race ./...

lint: ## Run linting
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

clean: ## Clean build artifacts
	go clean
	rm -f coverage.out coverage.html

deps: ## Download dependencies
	go mod download
	go mod tidy

# Infrastructure Management
infra-up: ## Start complete observability infrastructure (Jaeger, Prometheus, Loki, Grafana, OTel Collector)
	@echo "Starting comprehensive observability infrastructure..."
	docker-compose up -d
	@echo "Services starting up..."
	@echo "Jaeger UI:     http://localhost:16686"
	@echo "Prometheus:    http://localhost:9090" 
	@echo "Grafana:       http://localhost:3000 (admin/admin)"
	@echo "OTel Collector health: http://localhost:13133"
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Infrastructure is ready!"

infra-down: ## Stop observability infrastructure
	@echo "Stopping observability infrastructure..."
	docker-compose down

infra-status: ## Check status of observability infrastructure
	@echo "Observability Infrastructure Status:"
	@docker-compose ps

infra-logs: ## Show logs from observability infrastructure
	docker-compose logs -f

# Examples
example: ## Run the basic example
	cd examples/basic && go run main.go

comprehensive-demo: ## Run comprehensive observability demo
	@echo "Starting comprehensive observability demo..."
	@echo "Make sure infrastructure is running: make infra-up"
	@echo "Then visit:"
	@echo "  - http://localhost:8080/api/users (generates traces/logs/metrics)"
	@echo "  - http://localhost:8080/health (health check)"
	@echo "  - http://localhost:8080/error (error simulation)"
	@echo ""
	@echo "View telemetry at:"
	@echo "  - Traces: http://localhost:16686"
	@echo "  - Metrics: http://localhost:3000"
	@echo "  - Logs: http://localhost:3000"
	@echo ""
	cd examples/comprehensive && go run main.go

benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

check: fmt vet lint test ## Run all checks (format, vet, lint, test)

update-deps: ## Update dependencies
	go get -u ./...
	go mod tidy

install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Complete observability workflow
demo-full: infra-up ## Start infrastructure and run comprehensive demo
	@echo "Complete observability demo starting..."
	@echo "Infrastructure started, waiting for readiness..."
	@sleep 15
	@echo "Starting demo application..."
	@make comprehensive-demo
