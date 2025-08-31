# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Commands

### Building
```bash
make build              # Build the binary (creates dir-compare)
go build -o dir-compare # Alternative manual build
```

### Testing
```bash
make test               # Run tests with race detection and coverage
make test-coverage      # Run tests and generate HTML coverage report
make test-all           # Run comprehensive test suite using test_runner.sh
make test-race          # Run with race detector
make bench              # Run benchmarks
```

### Code Quality
```bash
make fmt                # Format code
make fmt-check          # Check if code is properly formatted
make vet                # Run static analysis
make check              # Run all quality checks (fmt-check, vet, lint, security, test-race)
```

### Other Commands
```bash
make clean              # Remove build artifacts and coverage files
make release            # Create cross-platform release builds (Linux, macOS, Windows)
make example            # Run example comparison demo
```

## Architecture Overview

This is a directory comparison tool that identifies differences between file sets using content-based comparison (SHA256 hashing). The tool is written in pure Go with no external dependencies.

### Core Components

1. **FileInfo Structure** (main.go:14-21): Represents metadata about a file including path, hash, size, and source directory.

2. **FileSet Structure** (main.go:24-28): Collection of files with lookup maps for efficient searching by name and hash.

3. **Comparison Logic** (main.go:125-166):
   - Ignores files with identical content (same hash)
   - Identifies files with same name but different content
   - Identifies files unique to each set

4. **Tree Building** (main.go:188-291):
   - Smart tree structure that marks entire directories as missing when appropriate
   - Removes empty directories from output
   - Provides formatted tree visualization

### Key Design Decisions

- **Content-based comparison**: Uses SHA256 hashing instead of timestamps for accurate file comparison
- **Multiple directory support**: Can compare sets of directories, not just single directories
- **Smart output**: Tree visualization intelligently groups entire missing directories
- **No external dependencies**: Uses only Go standard library for maximum portability

### Testing Approach

The project includes comprehensive unit tests in `main_test.go`. When adding new features:
- Run `make test` to ensure all tests pass
- Run `make test-coverage` to check coverage (target is >90%)
- Use `make test-race` to detect race conditions
- The `test_runner.sh` script provides a comprehensive test suite with colored output
