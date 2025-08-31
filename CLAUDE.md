# Project Overview

This is a fast, intelligent directory comparison tool written in Go that identifies differences between file sets using content-based comparison (SHA256 hashing). The tool is designed as a single-file Go application with comprehensive testing and cross-platform support.

# Key Architecture

- **Single main.go**: All functionality is contained in one file (~1000+ lines) with well-defined data structures:
  - `FileInfo`: Represents file metadata including path, hash, size, and root directory
  - `FileSet`: Collection of files with lookup maps for efficient comparison
  - `ComparisonResult`: Holds comparison results between two file sets
  - `TreeNode`: Represents directory tree structure for formatted output

- **Core workflow**: File discovery → SHA256 hashing → intelligent comparison → tree building → formatted output
- **Concurrency**: Uses goroutines and worker pools for parallel file processing with CPU-optimized batching
- **No external dependencies**: Uses only Go standard library

# Common Development Commands

## Building and Testing
```bash
# Build the binary
make build

# Run all tests with race detection and coverage
make test

# Run comprehensive test suite (includes coverage, race detection, benchmarks)
make test-all

# Run tests with coverage report generation
make test-coverage
```

## Code Quality
```bash
# Format code
make fmt

# Static analysis
make vet

# Run all quality checks (formatting, vet, lint, security, race tests)
make check

# Run quality checks with pre-commit hooks
make check-all
```

## Pre-commit Hooks
The project uses pre-commit hooks with Go-specific checks:
```bash
# Install pre-commit hooks
make pre-commit-install

# Run pre-commit on all files
make pre-commit-run
```

## Release and Cross-platform
```bash
# Create release builds for Linux, macOS, Windows
make release

# Install to $GOPATH/bin
make install
```

## Testing Individual Components
```bash
# Run specific test functions
go test -run TestFunctionName -v

# Run benchmarks
make bench

# Race condition testing
make test-race
```

# Development Workflow

1. Make changes to main.go or main_test.go
2. Run `make fmt` to format code
3. Run `make test` for quick validation
4. Run `make check` for comprehensive quality checks before committing
5. Use `make example` to test functionality with sample data

# Code Style Notes

- All functionality is in main.go - avoid creating separate files unless absolutely necessary
- Follow Go naming conventions and use `gofmt` formatting
- Tests are comprehensive with high coverage requirements
- Use the existing error handling patterns and logging approach
- Performance is important - maintain concurrent processing patterns
