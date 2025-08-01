.PHONY: build test lint clean install fmt vet help dev ci deps deps-update deps-verify security proto-gen proto-clean build-all pre-commit pre-commit-install

# Build variables
BINARY_NAME=ag-ui-cli
BUILD_DIR=bin
CMD_DIR=./cmd/ag-ui-cli
PROTO_DIR=proto
PROTO_OUT=pkg/proto

# Cross-platform build targets
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Versioning
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Tool commands
GOLANGCI_LINT=golangci-lint
GOSEC=gosec
PROTOC=protoc
PROTOC_GEN_GO=protoc-gen-go

# Build flags
LDFLAGS=-ldflags="-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

## help: Show this help message
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

## test: Run tests with coverage
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -timeout=10m -coverprofile=coverage.out ./...
	@echo "Coverage report:"
	$(GOCMD) tool cover -func=coverage.out

## test-short: Run tests without coverage
test-short:
	@echo "Running tests (short mode)..."
	$(GOTEST) -v -short ./...

## test-chaos: Run chaos engineering tests
test-chaos:
	@echo "Running chaos engineering tests..."
	$(GOTEST) -v -tags="chaos integration" -timeout=30m ./pkg/state/

## test-error-injection: Run error injection tests
test-error-injection:
	@echo "Running error injection tests..."
	$(GOTEST) -v -run="TestStateManager_WithErrors" ./pkg/state/

## test-resilience: Run all resilience tests (error injection + chaos)
test-resilience:
	@echo "Running resilience tests..."
	$(GOTEST) -v -tags="chaos integration" -run="TestStateManager_(WithErrors|ChaosEngineering)" ./pkg/state/

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config=.golangci.yml; \
	else \
		echo "golangci-lint not found. Please install it: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

## deps: Download and tidy dependencies
deps:
	@echo "Managing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	$(GOMOD) verify

## install: Install binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) $(CMD_DIR)

## dev: Development target - format, vet, test
dev: fmt vet test-short

## ci: CI target - all checks including linting and type safety
ci: fmt vet lint-all test build

## check: Quick validation (fmt, vet, build)
check: fmt vet build
	@echo "Basic checks passed ✓"

## coverage: Generate and open coverage report
coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## run: Run the CLI tool
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

## version: Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

## deps-update: Update all dependencies to latest versions
deps-update:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	$(GOMOD) verify

## deps-verify: Verify dependency integrity and security
deps-verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "Checking for known vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

## security: Run security analysis
security:
	@echo "Running security analysis..."
	@if command -v $(GOSEC) >/dev/null 2>&1; then \
		$(GOSEC) -fmt json -out gosec-report.json -stdout ./...; \
	else \
		echo "gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi

## proto-gen: Generate Go code from protocol buffer files
proto-gen:
	@echo "Generating protobuf code..."
	@if [ ! -d "$(PROTO_DIR)" ]; then \
		echo "Proto directory $(PROTO_DIR) not found. Creating..."; \
		mkdir -p $(PROTO_DIR); \
	fi
	@if [ ! -d "$(PROTO_OUT)/generated" ]; then \
		mkdir -p $(PROTO_OUT)/generated; \
	fi
	@if command -v $(PROTOC) >/dev/null 2>&1 && command -v $(PROTOC_GEN_GO) >/dev/null 2>&1; then \
		$(PROTOC) \
			--go_out=$(PROTO_OUT)/generated \
			--go_opt=paths=source_relative \
			--proto_path=$(PROTO_DIR) \
			$(PROTO_DIR)/*.proto; \
		echo "Protobuf code generation completed"; \
		echo "Generated files:"; \
		ls -la $(PROTO_OUT)/generated/*.pb.go 2>/dev/null || echo "No .pb.go files found"; \
	else \
		echo "protoc or protoc-gen-go not found. Install protoc and run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		exit 1; \
	fi

## proto-clean: Clean generated protobuf files
proto-clean:
	@echo "Cleaning generated protobuf files..."
	find $(PROTO_OUT)/generated -name "*.pb.go" -delete 2>/dev/null || true
	@echo "Cleaned protobuf files"

## build-all: Build for all supported platforms
build-all: clean
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		platform_split=($${platform//\// }); \
		GOOS=$${platform_split[0]}; \
		GOARCH=$${platform_split[1]}; \
		output_name=$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ $$GOOS = "windows" ]; then output_name+='.exe'; fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		env GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$$output_name $(CMD_DIR); \
		if [ $$? -ne 0 ]; then \
			echo "Failed to build for $$GOOS/$$GOARCH"; \
			exit 1; \
		fi; \
	done
	@echo "Cross-platform build completed ✓"

## lint-fix: Run linter with auto-fix where possible
lint-fix:
	@echo "Running linter with auto-fix..."
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		$(GOLANGCI_LINT) run --config=.golangci.yml --fix; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

## lint-typesafety: Check for interface{} usage and type safety violations
lint-typesafety:
	@echo "Running type safety checks..."
	@chmod +x scripts/pre-commit-hooks/check-typesafety.sh
	@./scripts/pre-commit-hooks/check-typesafety.sh

## lint-migration: Analyze migration progress and suggest improvements
lint-migration:
	@echo "Running migration analysis..."
	@$(GOCMD) run ./lint/migration_rules.go ./...

## lint-suggest: Get type-safe alternatives for current files
lint-suggest:
	@echo "Generating type-safe suggestions..."
	@chmod +x scripts/pre-commit-hooks/suggest-alternatives.sh
	@./scripts/pre-commit-hooks/suggest-alternatives.sh

## lint-format-migration: Auto-format migrated code
lint-format-migration:
	@echo "Formatting migrated code..."
	@chmod +x scripts/pre-commit-hooks/format-migration.sh
	@./scripts/pre-commit-hooks/format-migration.sh

## lint-docs: Generate linting rules documentation
lint-docs:
	@echo "Generating linting documentation..."
	@mkdir -p docs/linting
	@$(GOCMD) run ./lint/generate_rules_doc.go -output docs/linting
	@echo "Documentation generated in docs/linting/"

## lint-all: Run all linting and type safety checks
lint-all: lint lint-typesafety lint-migration
	@echo "All linting checks completed ✓"

## tools-install: Install all required development tools
tools-install:
	@echo "Installing development tools..."
	$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install golang.org/x/tools/cmd/goimports@latest
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOCMD) install go.uber.org/mock/mockgen@latest
	$(GOCMD) install golang.org/x/tools/go/analysis/passes/...@latest
	@echo "Development tools installed ✓"
	@echo "Setting up pre-commit hooks..."
	@chmod +x scripts/pre-commit-hooks/*.sh
	@echo "Pre-commit hooks configured ✓"

## full-check: Run comprehensive quality checks
full-check: deps-verify security lint-all test
	@echo "Full quality check completed ✓"

## pre-commit: Run pre-commit framework hooks with type safety
pre-commit:
	@echo "Running pre-commit hooks..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit run --all-files; \
	else \
		echo "pre-commit not found. Run ./scripts/setup-pre-commit.sh to install"; \
		echo "Falling back to manual checks..."; \
		$(MAKE) fmt lint-fix lint-typesafety test-short; \
	fi
	@echo "Pre-commit checks completed ✓"

## pre-commit-full: Run comprehensive pre-commit checks
pre-commit-full: fmt lint-all test
	@echo "Comprehensive pre-commit checks completed ✓"

## pre-commit-install: Install pre-commit hooks
pre-commit-install:
	@echo "Installing pre-commit hooks..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install; \
		pre-commit install --hook-type commit-msg; \
		pre-commit install --hook-type pre-push; \
		echo "Pre-commit hooks installed ✓"; \
	else \
		echo "pre-commit not found. Run ./scripts/setup-pre-commit.sh to install"; \
		exit 1; \
	fi

# Default target
all: ci 