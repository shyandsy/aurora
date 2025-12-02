.PHONY: test test-verbose test-coverage help

# 默认目标
.DEFAULT_GOAL := help

# 帮助信息
help:
	@echo "可用的 Make 目标:"
	@echo "  test          - 执行所有单元测试"
	@echo "  test-verbose  - 执行所有单元测试（详细输出）"
	@echo "  test-coverage - 执行所有单元测试并生成覆盖率报告"

# 执行所有单元测试
test:
	@echo "========== 运行单元测试 =========="
	@go test ./...

# 执行所有单元测试（详细输出）
test-verbose:
	@echo "========== 运行单元测试（详细输出）=========="
	@go test -v ./...

# 执行所有单元测试并生成覆盖率报告
test-coverage:
	@echo "========== 运行单元测试并生成覆盖率报告 =========="
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"
	@go tool cover -func=coverage.out

