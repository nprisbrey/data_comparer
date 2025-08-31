# Makefile for Directory Comparison Tool

# Variables
BINARY_NAME=dir-compare
GO_FILES=$(wildcard *.go)
TEST_FILES=$(wildcard *_test.go)

# Default target
.PHONY: all
all: test build

# Build the binary
.PHONY: build
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .
	@echo "✅ Build complete: $(BINARY_NAME)"

# Run all tests
.PHONY: test
test:
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out .
	@echo "✅ Tests complete"

# Run tests with coverage report
.PHONY: test-coverage
test-coverage: test
	@echo "📊 Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Run the comprehensive test suite
.PHONY: test-all
test-all:
	@echo "🧪 Running comprehensive test suite..."
	chmod +x test_runner.sh
	./test_runner.sh

# Run benchmarks
.PHONY: bench
bench:
	@echo "⚡ Running benchmarks..."
	go test -bench=. -benchmem -run=^$ .

# Run tests with race detection
.PHONY: test-race
test-race:
	@echo "🏃 Running race condition tests..."
	go test -race .

# Static analysis
.PHONY: vet
vet:
	@echo "🔍 Running static analysis..."
	go vet .

# Format code
.PHONY: fmt
fmt:
	@echo "📝 Formatting code..."
	go fmt .

# Check formatting
.PHONY: fmt-check
fmt-check:
	@echo "📝 Checking code formatting..."
	@if [ -n "$(shell gofmt -l .)" ]; then \
		echo "❌ Code formatting issues found:"; \
		gofmt -l .; \
		exit 1; \
	else \
		echo "✅ Code formatting is correct"; \
	fi

# Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -f coverage.html
	@echo "✅ Clean complete"

# Install dependencies (if any)
.PHONY: deps
deps:
	@echo "📦 Installing dependencies..."
	go mod tidy
	@echo "✅ Dependencies installed"

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Security check (requires gosec)
.PHONY: security
security:
	@echo "🔒 Running security check..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "⚠️  gosec not installed. Run: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Pre-commit hooks
.PHONY: pre-commit-install
pre-commit-install:
	@echo "🪝 Installing pre-commit hooks..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install; \
		echo "✅ Pre-commit hooks installed"; \
	else \
		echo "❌ pre-commit not found. Install with: pip install pre-commit"; \
		exit 1; \
	fi

.PHONY: pre-commit-run
pre-commit-run:
	@echo "🪝 Running pre-commit hooks on all files..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit run --all-files; \
	else \
		echo "❌ pre-commit not found. Install with: pip install pre-commit"; \
		exit 1; \
	fi

.PHONY: pre-commit-update
pre-commit-update:
	@echo "🪝 Updating pre-commit hooks..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit autoupdate; \
	else \
		echo "❌ pre-commit not found. Install with: pip install pre-commit"; \
		exit 1; \
	fi

# Full quality check
.PHONY: check
check: fmt-check vet lint security test-race
	@echo "✅ All quality checks passed!"

# Full quality check with pre-commit
.PHONY: check-all
check-all: pre-commit-run check
	@echo "✅ All quality checks and pre-commit hooks passed!"

# Install the binary to $GOPATH/bin
.PHONY: install
install: build
	@echo "📦 Installing $(BINARY_NAME) to GOPATH/bin..."
	go install .
	@echo "✅ Installation complete"

# Create a release build
.PHONY: release
release: clean check test-coverage
	@echo "🚀 Creating release build..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o $(BINARY_NAME)-linux-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o $(BINARY_NAME)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o $(BINARY_NAME)-windows-amd64.exe .
	@echo "✅ Release builds complete"

# Example usage
.PHONY: example
example: build
	@echo "📚 Running example comparison..."
	@mkdir -p example/set1 example/set2
	@echo "File 1 content" > example/set1/file1.txt
	@echo "Common content" > example/set1/common.txt
	@echo "File 1 modified" > example/set2/file1.txt
	@echo "Common content" > example/set2/common.txt
	@echo "Unique file" > example/set2/unique.txt
	@echo ""
	@echo "Running: ./$(BINARY_NAME) example/set1 example/set2 --details"
	@./$(BINARY_NAME) example/set1 example/set2 --details || true
	@rm -rf example

# Show help
.PHONY: help
help:
	@echo "Directory Comparison Tool - Available Make Targets:"
	@echo ""
	@echo "Building:"
	@echo "  build          Build the binary"
	@echo "  install        Install the binary to GOPATH/bin"
	@echo "  release        Create release builds for multiple platforms"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run unit tests"
	@echo "  test-coverage  Run tests and generate coverage report"
	@echo "  test-all       Run comprehensive test suite with coverage"
	@echo "  test-race      Run tests with race detection"
	@echo "  bench          Run benchmark tests"
	@echo ""
	@echo "Quality:"
	@echo "  fmt            Format code"
	@echo "  fmt-check      Check code formatting"
	@echo "  vet            Run static analysis"
	@echo "  lint           Run linter (requires golangci-lint)"
	@echo "  security       Run security check (requires gosec)"
	@echo "  check          Run all quality checks"
	@echo "  check-all      Run quality checks and pre-commit hooks"
	@echo ""
	@echo "Pre-commit:"
	@echo "  pre-commit-install  Install pre-commit hooks"
	@echo "  pre-commit-run      Run pre-commit hooks on all files"
	@echo "  pre-commit-update   Update pre-commit hook versions"
	@echo ""
	@echo "Utilities:"
	@echo "  clean          Clean build artifacts"
	@echo "  deps           Install/update dependencies"
	@echo "  example        Run example comparison"
	@echo "  help           Show this help message"

# Default to showing help if no target specified
.DEFAULT_GOAL := help
