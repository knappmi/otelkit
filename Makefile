.PHONY: help build test test-coverage lint clean fmt vet deps example

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

example: ## Run the basic example
	cd examples/basic && go run main.go

benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

check: fmt vet lint test ## Run all checks (format, vet, lint, test)

update-deps: ## Update dependencies
	go get -u ./...
	go mod tidy

install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
