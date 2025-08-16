# Makefile for AG-UI client tests

.PHONY: test test-verbose test-short test-coverage clean

# Default test timeout
TEST_TIMEOUT ?= 30s

# Run all tests
test:
	go test -timeout $(TEST_TIMEOUT) ./...

# Run tests with verbose output
test-verbose:
	go test -v -timeout $(TEST_TIMEOUT) ./...

# Run short tests only
test-short:
	go test -short -timeout 10s ./...

# Run tests with coverage
test-coverage:
	go test -timeout $(TEST_TIMEOUT) -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run specific test
test-one:
	go test -v -timeout $(TEST_TIMEOUT) -run $(TEST) ./...

# Check for goroutine leaks
test-leaks:
	go test -v -timeout $(TEST_TIMEOUT) -run "TestNoGoroutineLeaks|TestGoroutineLeaksAfterChat|TestMultipleCommandsSequential" ./...

# Clean test artifacts
clean:
	rm -f coverage.out coverage.html

# Help
help:
	@echo "Available targets:"
	@echo "  test          - Run all tests (timeout: $(TEST_TIMEOUT))"
	@echo "  test-verbose  - Run tests with verbose output"
	@echo "  test-short    - Run short tests only"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-one      - Run specific test (use TEST=TestName)"
	@echo "  test-leaks    - Check for goroutine leaks"
	@echo "  clean         - Remove test artifacts"
	@echo ""
	@echo "Environment variables:"
	@echo "  TEST_TIMEOUT  - Test timeout (default: 30s)"
	@echo "  TEST          - Specific test to run with test-one"