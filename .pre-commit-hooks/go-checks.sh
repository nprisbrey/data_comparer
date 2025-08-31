#!/bin/bash

# Custom Go checks for pre-commit
# This script provides additional checks beyond the standard hooks

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[PRE-COMMIT]${NC} $1"
}

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

# Check if we're in a Go project
if [[ ! -f "go.mod" ]]; then
    print_error "Not in a Go module directory"
    exit 1
fi

print_status "Running custom Go checks..."

# 1. Check for inefficient string concatenation in loops
print_status "Checking for inefficient string concatenation..."
if find . -name "*.go" -not -path "./vendor/*" -exec grep -l "for.*+=" {} \; | head -1 | grep -q "."; then
    print_warning "Found potential inefficient string concatenation in loops. Consider using strings.Builder"
    find . -name "*.go" -not -path "./vendor/*" -exec grep -Hn "for.*+=" {} \;
fi

# 2. Check for missing error handling
print_status "Checking for potential missing error handling..."
if find . -name "*.go" -not -path "./vendor/*" -exec grep -l "_, err :=.*\n.*if err != nil" {} \; | head -1 | grep -q "."; then
    print_status "Error handling patterns found (good practice)"
else
    print_warning "Consider reviewing error handling patterns"
fi

# 3. Check for goroutine leaks (basic check)
print_status "Checking for potential goroutine leaks..."
if find . -name "*.go" -not -path "./vendor/*" -exec grep -l "go func" {} \; | head -1 | grep -q "."; then
    print_warning "Found goroutines. Ensure proper cleanup and context cancellation"
    find . -name "*.go" -not -path "./vendor/*" -exec grep -Hn "go func" {} \;
fi

# 4. Check for hardcoded credentials or sensitive data
print_status "Checking for potential sensitive data..."
SENSITIVE_PATTERNS="password|secret|key|token|credential"
if find . -name "*.go" -not -path "./vendor/*" -exec grep -iE "$SENSITIVE_PATTERNS" {} \; | head -1 | grep -q "."; then
    print_warning "Found potential sensitive data. Review carefully:"
    find . -name "*.go" -not -path "./vendor/*" -exec grep -iHnE "$SENSITIVE_PATTERNS" {} \;
fi

# 5. Check for TODO/FIXME comments (informational)
print_status "Checking for TODO/FIXME comments..."
TODO_COUNT=$(find . -name "*.go" -not -path "./vendor/*" -exec grep -c "TODO\|FIXME" {} \; 2>/dev/null | paste -sd+ | bc 2>/dev/null || echo 0)
if [[ $TODO_COUNT -gt 0 ]]; then
    print_warning "Found $TODO_COUNT TODO/FIXME comments"
    find . -name "*.go" -not -path "./vendor/*" -exec grep -Hn "TODO\|FIXME" {} \;
fi

# 6. Check test coverage expectation
print_status "Checking test files..."
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -name "*_test.go" | wc -l)
TEST_FILES=$(find . -name "*_test.go" -not -path "./vendor/*" | wc -l)

if [[ $GO_FILES -gt 0 && $TEST_FILES -eq 0 ]]; then
    print_warning "No test files found. Consider adding tests for better code quality"
elif [[ $GO_FILES -gt 0 ]]; then
    TEST_RATIO=$(echo "scale=2; $TEST_FILES / $GO_FILES * 100" | bc -l 2>/dev/null || echo "0")
    print_status "Test ratio: $TEST_FILES test files for $GO_FILES source files (${TEST_RATIO}% ratio)"
fi

# 7. Check for proper package documentation
print_status "Checking package documentation..."
PACKAGES=$(find . -name "*.go" -not -path "./vendor/*" -not -name "*_test.go" -exec dirname {} \; | sort -u)
for pkg in $PACKAGES; do
    if [[ ! $(find "$pkg" -name "*.go" -not -name "*_test.go" -exec head -20 {} \; | grep -E "^// Package|^//.*package") ]]; then
        print_warning "Package $pkg may be missing documentation"
    fi
done

# 8. Check go.mod and go.sum consistency
print_status "Checking go.mod/go.sum consistency..."
if [[ -f "go.sum" ]]; then
    if ! go mod verify >/dev/null 2>&1; then
        print_error "go.sum verification failed. Run 'go mod tidy' to fix"
        exit 1
    fi
else
    print_warning "go.sum not found. Run 'go mod download' to generate it"
fi

# 9. Run additional static checks
print_status "Running additional static analysis..."

# Check for unreachable code
if command -v deadcode >/dev/null 2>&1; then
    print_status "Running deadcode analysis..."
    if deadcode ./... 2>/dev/null | head -1 | grep -q "."; then
        print_warning "Dead code found:"
        deadcode ./...
    fi
fi

# Check for unused variables/imports
if go vet ./... >/dev/null 2>&1; then
    print_success "go vet passed"
else
    print_error "go vet found issues"
    go vet ./...
    exit 1
fi

print_success "Custom Go checks completed successfully!"
