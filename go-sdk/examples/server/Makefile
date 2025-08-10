.PHONY: help run build test lint fmt vet tidy check dev clean

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

run: ## Run the server
	go run ./cmd/server

build: ## Build the server binary
	@mkdir -p bin
	go build -o bin/server ./cmd/server

test: ## Run tests with coverage
	go test ./... -cover

fmt: ## Format Go code
	go fmt ./...

vet: ## Run Go vet
	go vet ./...

lint: ## Run golangci-lint (if available)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "or visit: https://golangci-lint.run/usage/install/"; \
	fi

tidy: ## Clean up go.mod and go.sum
	go mod tidy

check: fmt vet test lint ## Run all checks (fmt, vet, test, lint)

dev: ## Start development server with live reload
	@./scripts/dev.sh

clean: ## Clean build artifacts and test files
	rm -rf bin/
	rm -f *.out *.prof *.test coverage.html server