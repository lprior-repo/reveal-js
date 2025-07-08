# Makefile for ghorg-wrapper
# High-performance GitHub organization cloner

.PHONY: help build test run dev coverage clean

# Default target
help: ## Show available commands
	@echo "GitHub Organization Cloner Wrapper"
	@echo "================================="
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Essential commands
build: ## Build the application
	@echo "Building ghorg-wrapper..."
	@go build -o ghorg-wrapper .

test: ## Run all tests (81% coverage)
	@echo "Running comprehensive test suite..."
	@go test -v -cover ./...

run: build ## Build and run with .env file
	@echo "Running ghorg-wrapper..."
	@./ghorg-wrapper .env

# Development workflow
dev: ## Full dev workflow (deps, format, test)
	@echo "Running development workflow..."
	@go mod tidy
	@go fmt ./...
	@go test ./...
	@echo "✅ Development workflow complete!"

coverage: ## Generate detailed coverage report
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1 | awk '{print "Total Coverage: " $$3}'
	@echo "📊 Coverage report: coverage.html"

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -f ghorg-wrapper coverage.out coverage.html
	@go clean