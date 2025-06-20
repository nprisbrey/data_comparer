# Directory Comparison Tool

A fast, intelligent directory comparison tool written in Go that identifies differences between file sets using content-based comparison (SHA256 hashing).

## Features

- **Content-based comparison**: Uses SHA256 hashing to detect actual file differences, not just timestamps
- **Smart tree visualization**: Displays results in an intuitive tree format with directory structure
- **Multiple comparison modes**: 
  - Files with same names but different content
  - Files unique to each directory set
  - Optional reverse comparison (files unique to first set)
- **Flexible input**: Compare single directories or multiple directory sets
- **Detailed reporting**: Optional file size display and comprehensive summaries
- **Performance optimized**: Concurrent processing with efficient data structures

## Usage

### Basic Usage

```bash
# Compare two directories
./dir-compare /path/to/set1 /path/to/set2

# Compare multiple directories in each set
./dir-compare /path/to/set1a,/path/to/set1b /path/to/set2a,/path/to/set2b
```

### Options

```bash
# Show file sizes and additional details
./dir-compare /path/to/set1 /path/to/set2 --details

# Show files unique to the first set as well
./dir-compare /path/to/set1 /path/to/set2 --show-unique-1

# Combine options
./dir-compare /path/to/set1 /path/to/set2 --details --show-unique-1
```

### Example

```bash
./dir-compare ./documents ./backup_documents --details
```

## Installation

### Prerequisites

- Go 1.24.3 or later

### Building from Source

```bash
# Clone the repository
git clone https://github.com/nprisbrey/data_comparer.git
cd data_comparer

# Build for current platform
make build

# Or build manually
go build -o dir-compare .
```

## Cross-Platform Compilation

### Build for All Platforms

```bash
# Create release builds for Linux, macOS, and Windows
make release
```

This creates:
- `dir-compare-linux-amd64` (Linux 64-bit)
- `dir-compare-darwin-amd64` (macOS 64-bit)  
- `dir-compare-windows-amd64.exe` (Windows 64-bit)

### Build for Specific Platforms

#### Linux
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o dir-compare-linux .
```

#### Windows
```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o dir-compare-windows.exe .
```

#### macOS
```bash
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o dir-compare-macos .
```

## Running on Different Platforms

### Linux
```bash
chmod +x dir-compare-linux
./dir-compare-linux /path/to/set1 /path/to/set2
```

### Windows
```cmd
dir-compare-windows.exe C:\path\to\set1 C:\path\to\set2
```

### macOS
```bash
chmod +x dir-compare-macos
./dir-compare-macos /path/to/set1 /path/to/set2
```

## Testing

### Run Basic Tests
```bash
make test
```

### Run Tests with Coverage
```bash
make test-coverage
```

### Run Comprehensive Test Suite
```bash
# Using Make
make test-all

# Or using the test runner script directly
chmod +x test_runner.sh
./test_runner.sh
```

### Run Specific Test Types

```bash
# Race condition detection
make test-race

# Benchmarks
make bench

# Static analysis
make vet

# Code formatting check
make fmt-check

# Run all quality checks
make check
```

## Development

### Available Make Targets

```bash
# Building
make build          # Build the binary
make install        # Install to $GOPATH/bin
make release        # Create release builds

# Testing
make test           # Run unit tests
make test-coverage  # Run tests with coverage
make bench          # Run benchmarks

# Quality
make fmt            # Format code
make vet            # Static analysis
make lint           # Run linter (requires golangci-lint)
make security       # Security check (requires gosec)

# Utilities
make clean          # Clean build artifacts
make example        # Run example comparison
make help           # Show all targets
```

### Dependencies

This project uses only Go standard library - no external dependencies required.

## How It Works

1. **File Discovery**: Recursively walks directory trees to find all files
2. **Content Hashing**: Calculates SHA256 hash for each file's content
3. **Intelligent Comparison**: 
   - Files with identical hashes are considered the same (ignored)
   - Files with same names but different hashes are flagged as modified
   - Files with no name or content match are marked as unique
4. **Smart Tree Building**: Constructs directory trees that show:
   - Individual files where appropriate
   - Entire directories when all contents are unique
5. **Formatted Output**: Displays results with proper tree formatting and optional details

## Output Format

The tool produces three types of output trees:

1. **Same Name, Different Content**: Files that exist in both sets but have different content
2. **Unique to Set 2**: Files that only exist in the second directory set
3. **Unique to Set 1** (optional): Files that only exist in the first directory set

Each tree shows:
- 📁 Directory icons for folders
- 📄 File icons for individual files
- File sizes (with `--details` flag)
- File mappings for same-name conflicts
- "(entire directory)" markers for completely unique directories
