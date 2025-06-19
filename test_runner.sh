#!/bin/bash

# Test Runner Script for Directory Comparison Tool
# This script runs comprehensive tests and generates coverage reports

set -e

echo "🧪 Directory Comparison Tool - Test Suite"
echo "=========================================="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_status "Go version: $(go version)"
echo

# Clean up previous test artifacts
print_status "Cleaning up previous test artifacts..."
rm -f coverage.out coverage.html
echo

# Run tests with coverage
print_status "Running unit tests with coverage..."
if go test -v -race -coverprofile=coverage.out -covermode=atomic .; then
    print_success "All tests passed!"
else
    print_error "Some tests failed!"
    exit 1
fi
echo

# Generate coverage report
print_status "Generating coverage report..."
go tool cover -html=coverage.out -o coverage.html

# Show coverage statistics
print_status "Coverage Statistics:"
go tool cover -func=coverage.out | tail -1
echo

# Check coverage threshold (aim for >90%)
COVERAGE=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
THRESHOLD=90

if (( $(echo "$COVERAGE >= $THRESHOLD" | bc -l) )); then
    print_success "Coverage target met: ${COVERAGE}% >= ${THRESHOLD}%"
else
    print_warning "Coverage below threshold: ${COVERAGE}% < ${THRESHOLD}%"
fi
echo

# Run benchmarks
print_status "Running benchmark tests..."
go test -bench=. -benchmem -run=^$ .
echo

# Test with race detector
print_status "Running race condition tests..."
if go test -race .; then
    print_success "No race conditions detected!"
else
    print_error "Race conditions detected!"
    exit 1
fi
echo

# Run static analysis with go vet
print_status "Running static analysis (go vet)..."
if go vet .; then
    print_success "Static analysis passed!"
else
    print_error "Static analysis failed!"
    exit 1
fi
echo

# Run gofmt check
print_status "Checking code formatting..."
if [ -z "$(gofmt -l .)" ]; then
    print_success "Code formatting is correct!"
else
    print_warning "Code formatting issues found:"
    gofmt -l .
fi
echo

# Generate detailed coverage breakdown
print_status "Detailed coverage breakdown:"
go tool cover -func=coverage.out | head -20
echo

print_success "Test suite completed successfully!"
print_status "Coverage report generated: coverage.html"
print_status "Open coverage.html in your browser to view detailed coverage information"

# Optionally open coverage report in browser (macOS)
if command -v open &> /dev/null; then
    read -p "Would you like to open the coverage report in your browser? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        open coverage.html
    fi
fi