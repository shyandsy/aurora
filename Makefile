.PHONY: test test-verbose test-coverage help

.DEFAULT_GOAL := help

help:
	@echo "Available Make targets:"
	@echo "  test          - Run all unit tests"
	@echo "  test-verbose  - Run all unit tests (verbose output)"
	@echo "  test-coverage - Run all unit tests and generate coverage report"

test:
	@echo "========== Running unit tests =========="
	@go test ./...

test-verbose:
	@echo "========== Running unit tests (verbose) =========="
	@go test -v ./...

test-coverage:
	@echo "========== Running unit tests with coverage =========="
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out

