.PHONY: test test-verbose test-coverage doc-check help

.DEFAULT_GOAL := help

help:
	@echo "Available Make targets:"
	@echo "  test          - Run all unit tests"
	@echo "  test-verbose  - Run all unit tests (verbose output)"
	@echo "  test-coverage - Run all unit tests and generate coverage report"
	@echo "  doc-check     - 校验文档与 user.Config 一致(防文档抄错/抄漏字段;已含在 test 里)"

doc-check:
	@echo "========== 校验 user 模块文档与 Config 一致 =========="
	@go test ./modules/user/ -run TestDocs_ -count=1 -v

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

