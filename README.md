# Directory Comparison Tool

A fast, intelligent directory comparison tool written in Go that identifies differences between file sets using content-based comparison (SHA256 hashing).

## Features

- **Content-based comparison**: Uses SHA256 hashing to detect actual file differences, not just timestamps
- **Smart tree visualization**: Displays results in an intuitive tree format with directory structure
- **Interactive mode**: Run without arguments for guided, user-friendly experience
- **Preview mode**: Quick sampling of files before full analysis
- **Multiple comparison modes**:
  - Files with same names but different content (modified files)
  - Files unique to each directory set
  - Optional reverse comparison (files unique to first set)
- **Flexible input**: Compare single directories or multiple directory sets
- **Detailed reporting**: Optional file size display and comprehensive summaries
- **Performance optimized**: CPU-optimized parallel processing with intelligent batching
- **Cross-platform**: Works on Linux, macOS, and Windows
- **Progress tracking**: Real-time progress display with speed calculations

## Usage

### Interactive Mode (Recommended for First-Time Users)

Simply run the tool without arguments for a guided experience:

```bash
./dir-compare
```

The interactive mode will:
- Guide you through selecting directories to compare
- Let you choose which comparisons to perform
- Show a preview before running the full analysis
- Provide helpful examples based on your operating system

### Command Line Usage

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

# Show files with same name but different content
./dir-compare /path/to/set1 /path/to/set2 --show-modified

# Show files unique to set 2
./dir-compare /path/to/set1 /path/to/set2 --show-unique-2

# Show files unique to set 1
./dir-compare /path/to/set1 /path/to/set2 --show-unique-1

# Preview mode - analyze only first N files (default 10)
./dir-compare /path/to/set1 /path/to/set2 --preview
./dir-compare /path/to/set1 /path/to/set2 --preview-count 20

# Combine options
./dir-compare /path/to/set1 /path/to/set2 --details --show-modified --show-unique-2
```

### Examples

```bash
# Compare photos with backup
./dir-compare ~/photos ~/backup/photos --details --show-unique-2

# Check what's different between two project versions
./dir-compare ./project-v1 ./project-v2 --show-modified

# Find files that exist in backup but not in current
./dir-compare ./current ./backup --show-unique-1

# Quick preview with 5 files
./dir-compare ./docs ./archive/docs --preview-count 5
```

## Installation

### Prerequisites

- Go 1.19 or later (uses only standard library)

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
make release        # Create release builds for all platforms

# Testing
make test           # Run unit tests with race detection
make test-coverage  # Run tests with coverage report
make test-all       # Comprehensive test suite
make bench          # Run benchmarks
make test-race      # Race condition detection

# Code Quality
make fmt            # Format code with gofmt
make vet            # Static analysis
make lint           # Run linter (requires golangci-lint)
make security       # Security check (requires gosec)
make check          # Run all quality checks
make check-all      # Quality checks with pre-commit

# Pre-commit Hooks
make pre-commit-install  # Install pre-commit hooks
make pre-commit-run      # Run pre-commit on all files

# Utilities
make clean          # Clean build artifacts
make example        # Run example comparison
make help           # Show all available targets
```

### Dependencies

This project uses only Go standard library - no external dependencies required.

## How It Works

1. **File Discovery**: Recursively walks directory trees to find all files (read-only traversal)
2. **Content Hashing**: Calculates SHA256 hash for each file's content (opens files read-only)
3. **Intelligent Comparison**:
   - Files with identical hashes are considered the same (ignored)
   - Files with same names but different hashes are flagged as modified
   - Files with no name or content match are marked as unique
4. **Performance Optimization**:
   - Automatically uses parallel processing for large file sets (>20 files)
   - CPU-optimized with 75% core utilization
   - Intelligent batching to minimize overhead
   - Sequential processing for small workloads
5. **Smart Tree Building**: Constructs directory trees that show:
   - Individual files where appropriate
   - Entire directories when all contents are unique
   - Clear visual indicators with icons (📁 for directories, 📄 for files)
6. **Formatted Output**: Displays results with proper tree formatting, progress tracking, and optional details

## Output Format

The tool can produce up to three types of output trees (configurable):

1. **Modified Files** (`--show-modified`): Files that exist in both sets with same name but different content
2. **Unique to Set 2** (`--show-unique-2`): Files that only exist in the second directory set
3. **Unique to Set 1** (`--show-unique-1`): Files that only exist in the first directory set

Each tree shows:
- 📁 Directory icons for folders
- 📄 File icons for individual files
- File sizes in KB/MB/GB (with `--details` flag)
- File mappings for same-name conflicts (shows where the original file is located)
- "(entire directory)" markers for completely unique directories
- Progress tracking with real-time speed calculations during analysis

### Sample Output

```
Directory Comparison Tool
=========================

📂 Set 1 directories: /home/user/current
📂 Set 2 directories: /home/user/backup

🔍 Analyzing first set of directories...
   Found 250 files
🔍 Analyzing second set of directories...
   Found 275 files
🔍 Comparing file sets...

⚠️  Files with same name but different content (12 files) - Set 2 (/home/user/backup) → Set 1 (/home/user/current):
===================================================

└── 📁 documents/
    ├── 📄 report.txt (0.02 KB) → documents/report.txt
    └── 📄 summary.docx (1.45 MB) → documents/summary.docx

📋 Files unique to Set 2 (/home/user/backup) - not found in Set 1 (/home/user/current) (25 files):
===================================================

├── 📁 archive/ (entire directory)
└── 📁 documents/
    └── 📄 notes.txt (0.01 KB)

📊 Summary:
   • Files in Set 1: 250
   • Files in Set 2: 275
   • Same name, different content: 12
   • Unique to Set 2: 25
   • Total sizes:
     - Same name, different content: 18.3 MB
     - Unique to Set 2: 45.7 MB
```

Note: For large file sets (>20 files), you'll see a progress bar during analysis:
```
🔍 Analyzing files... Files: 1523/1523 (100%) | Size: 2.34 GB/2.34 GB (100%) | Speed: 45.2 MB/s
```
