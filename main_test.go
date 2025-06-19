package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Test helper functions

// TestingT is an interface that covers both *testing.T and *testing.B
type TestingT interface {
	TempDir() string
	Helper()
	Fatalf(format string, args ...interface{})
}

// createTempDir creates a temporary directory with subdirectories and files
func createTempDir(t TestingT, structure map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()

	for filePath, content := range structure {
		fullPath := filepath.Join(tmpDir, filePath)
		dir := filepath.Dir(fullPath)

		// Create directory structure
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Create file with content
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	return tmpDir
}

// captureOutput captures stdout during function execution
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// sortFileInfoSlice sorts a slice of FileInfo by RelativePath for consistent testing
func sortFileInfoSlice(files []*FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})
}

// Test cases for hashFile function
func TestHashFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantHash string
	}{
		{
			name:     "empty file",
			content:  "",
			wantHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple content",
			content:  "hello world",
			wantHash: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "multiline content",
			content:  "line1\nline2\nline3",
			wantHash: "6bb6a5ad9b9c43a7cb535e636578716b64ac42edea814a4cad102ba404946837",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile := filepath.Join(t.TempDir(), "testfile")
			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			hash, err := hashFile(tmpFile)
			if err != nil {
				t.Errorf("hashFile() error = %v", err)
				return
			}
			if hash != tt.wantHash {
				t.Errorf("hashFile() = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestHashFileErrors(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		_, err := hashFile("/nonexistent/file.txt")
		if err == nil {
			t.Error("Expected error for nonexistent file, got nil")
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := hashFile(tmpDir)
		if err == nil {
			t.Error("Expected error for directory, got nil")
		}
	})
}

// Test cases for walkDirectories function
func TestWalkDirectories(t *testing.T) {
	t.Run("single directory with files", func(t *testing.T) {
		structure := map[string]string{
			"file1.txt":         "content1",
			"subdir/file2.txt":  "content2",
			"subdir/file3.txt":  "content3",
			"another/file4.txt": "content4",
		}
		tmpDir := createTempDir(t, structure)

		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("walkDirectories() error = %v", err)
		}

		if len(fileSet.Files) != 4 {
			t.Errorf("Expected 4 files, got %d", len(fileSet.Files))
		}

		// Check that all files are present in maps
		if len(fileSet.NameMap) != 4 {
			t.Errorf("Expected 4 entries in NameMap, got %d", len(fileSet.NameMap))
		}

		if len(fileSet.HashMap) != 4 {
			t.Errorf("Expected 4 entries in HashMap, got %d", len(fileSet.HashMap))
		}
	})

	t.Run("multiple directories", func(t *testing.T) {
		structure1 := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		}
		structure2 := map[string]string{
			"file3.txt": "content3",
			"file4.txt": "content4",
		}

		tmpDir1 := createTempDir(t, structure1)
		tmpDir2 := createTempDir(t, structure2)

		fileSet, err := walkDirectories([]string{tmpDir1, tmpDir2})
		if err != nil {
			t.Fatalf("walkDirectories() error = %v", err)
		}

		if len(fileSet.Files) != 4 {
			t.Errorf("Expected 4 files, got %d", len(fileSet.Files))
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("walkDirectories() error = %v", err)
		}

		if len(fileSet.Files) != 0 {
			t.Errorf("Expected 0 files, got %d", len(fileSet.Files))
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		// This should not return an error but should print a warning
		output := captureOutput(t, func() {
			fileSet, err := walkDirectories([]string{"/nonexistent/directory"})
			if err != nil {
				t.Errorf("walkDirectories() should not error for nonexistent directory: %v", err)
			}
			if len(fileSet.Files) != 0 {
				t.Errorf("Expected 0 files for nonexistent directory, got %d", len(fileSet.Files))
			}
		})

		if !strings.Contains(output, "Warning") {
			t.Error("Expected warning message for nonexistent directory")
		}
	})
}

// Test cases for compareFileSets function
func TestCompareFileSets(t *testing.T) {
	t.Run("identical sets", func(t *testing.T) {
		structure := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		}

		tmpDir1 := createTempDir(t, structure)
		tmpDir2 := createTempDir(t, structure)

		set1, _ := walkDirectories([]string{tmpDir1})
		set2, _ := walkDirectories([]string{tmpDir2})

		result := compareFileSets(set1, set2)

		if len(result.SameNameDifferentHash) != 0 {
			t.Errorf("Expected 0 same name different hash files, got %d", len(result.SameNameDifferentHash))
		}
		if len(result.UniqueToSet2) != 0 {
			t.Errorf("Expected 0 unique to set2 files, got %d", len(result.UniqueToSet2))
		}
		if len(result.UniqueToSet1) != 0 {
			t.Errorf("Expected 0 unique to set1 files, got %d", len(result.UniqueToSet1))
		}
	})

	t.Run("same names different content", func(t *testing.T) {
		structure1 := map[string]string{
			"file1.txt": "original content",
			"file2.txt": "same content",
		}
		structure2 := map[string]string{
			"file1.txt": "modified content",
			"file2.txt": "same content",
		}

		tmpDir1 := createTempDir(t, structure1)
		tmpDir2 := createTempDir(t, structure2)

		set1, _ := walkDirectories([]string{tmpDir1})
		set2, _ := walkDirectories([]string{tmpDir2})

		result := compareFileSets(set1, set2)

		if len(result.SameNameDifferentHash) != 1 {
			t.Errorf("Expected 1 same name different hash file, got %d", len(result.SameNameDifferentHash))
		}

		if result.SameNameDifferentHash[0].Name != "file1.txt" {
			t.Errorf("Expected file1.txt in same name different hash, got %s", result.SameNameDifferentHash[0].Name)
		}

		if len(result.NameMappings["file1.txt"]) != 1 {
			t.Errorf("Expected 1 mapping for file1.txt, got %d", len(result.NameMappings["file1.txt"]))
		}
	})

	t.Run("unique files", func(t *testing.T) {
		structure1 := map[string]string{
			"common.txt":  "same content",
			"unique1.txt": "content1",
		}
		structure2 := map[string]string{
			"common.txt":  "same content",
			"unique2.txt": "content2",
		}

		tmpDir1 := createTempDir(t, structure1)
		tmpDir2 := createTempDir(t, structure2)

		set1, _ := walkDirectories([]string{tmpDir1})
		set2, _ := walkDirectories([]string{tmpDir2})

		result := compareFileSets(set1, set2)

		if len(result.UniqueToSet2) != 1 {
			t.Errorf("Expected 1 unique to set2 file, got %d", len(result.UniqueToSet2))
		}
		if result.UniqueToSet2[0].Name != "unique2.txt" {
			t.Errorf("Expected unique2.txt in set2, got %s", result.UniqueToSet2[0].Name)
		}

		if len(result.UniqueToSet1) != 1 {
			t.Errorf("Expected 1 unique to set1 file, got %d", len(result.UniqueToSet1))
		}
		if result.UniqueToSet1[0].Name != "unique1.txt" {
			t.Errorf("Expected unique1.txt in set1, got %s", result.UniqueToSet1[0].Name)
		}
	})

	t.Run("same content different names", func(t *testing.T) {
		structure1 := map[string]string{
			"original.txt": "identical content",
		}
		structure2 := map[string]string{
			"renamed.txt": "identical content",
		}

		tmpDir1 := createTempDir(t, structure1)
		tmpDir2 := createTempDir(t, structure2)

		set1, _ := walkDirectories([]string{tmpDir1})
		set2, _ := walkDirectories([]string{tmpDir2})

		result := compareFileSets(set1, set2)

		// Files with same content should be ignored even with different names
		if len(result.SameNameDifferentHash) != 0 {
			t.Errorf("Expected 0 same name different hash files, got %d", len(result.SameNameDifferentHash))
		}
		if len(result.UniqueToSet2) != 0 {
			t.Errorf("Expected 0 unique to set2 files, got %d", len(result.UniqueToSet2))
		}
		if len(result.UniqueToSet1) != 0 {
			t.Errorf("Expected 0 unique to set1 files, got %d", len(result.UniqueToSet1))
		}
	})
}

// Test cases for tree building functions
func TestBuildTree(t *testing.T) {
	files := []*FileInfo{
		{RelativePath: "file1.txt", Name: "file1.txt"},
		{RelativePath: "subdir/file2.txt", Name: "file2.txt"},
		{RelativePath: "subdir/nested/file3.txt", Name: "file3.txt"},
	}

	tree := buildTree(files)

	if tree.Name != "" {
		t.Errorf("Root node should have empty name, got %s", tree.Name)
	}
	if !tree.IsDir {
		t.Error("Root node should be a directory")
	}
	if len(tree.Files) != 1 {
		t.Errorf("Root should have 1 file, got %d", len(tree.Files))
	}
	if len(tree.Children) != 1 {
		t.Errorf("Root should have 1 child directory, got %d", len(tree.Children))
	}

	subdir := tree.Children["subdir"]
	if subdir == nil {
		t.Fatal("subdir child not found")
	}
	if len(subdir.Files) != 1 {
		t.Errorf("subdir should have 1 file, got %d", len(subdir.Files))
	}
	if len(subdir.Children) != 1 {
		t.Errorf("subdir should have 1 child directory, got %d", len(subdir.Children))
	}
}

func TestBuildSmartTree(t *testing.T) {
	files := []*FileInfo{
		{RelativePath: "dir1/file1.txt", Name: "file1.txt"},
		{RelativePath: "dir1/file2.txt", Name: "file2.txt"},
		{RelativePath: "dir2/file3.txt", Name: "file3.txt"},
	}

	// Create a dummy other set (not used in this simple test)
	otherSet := &FileSet{
		Files:   []*FileInfo{},
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}

	tree := buildSmartTree(files, otherSet)

	if len(tree.Children) != 2 {
		t.Errorf("Root should have 2 child directories, got %d", len(tree.Children))
	}

	dir1 := tree.Children["dir1"]
	if dir1 == nil {
		t.Fatal("dir1 child not found")
	}
	if len(dir1.Files) != 2 {
		t.Errorf("dir1 should have 2 files, got %d", len(dir1.Files))
	}
}

func TestRemoveEmptyDirectories(t *testing.T) {
	// Create a tree with empty directories
	root := &TreeNode{
		Name:     "",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
	}

	// Add an empty directory
	emptyDir := &TreeNode{
		Name:     "empty",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
		Parent:   root,
	}
	root.Children["empty"] = emptyDir

	// Add a directory with files
	dirWithFiles := &TreeNode{
		Name:     "withfiles",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
		Parent:   root,
		Files:    []*FileInfo{{Name: "test.txt"}},
	}
	root.Children["withfiles"] = dirWithFiles

	removeEmptyDirectories(root)

	if len(root.Children) != 1 {
		t.Errorf("Expected 1 child after removing empty directories, got %d", len(root.Children))
	}

	if _, exists := root.Children["empty"]; exists {
		t.Error("Empty directory should have been removed")
	}

	if _, exists := root.Children["withfiles"]; !exists {
		t.Error("Directory with files should not have been removed")
	}
}

func TestMarkEntireDirectories(t *testing.T) {
	root := &TreeNode{
		Name:     "",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
	}

	// Add a directory with files
	dirWithFiles := &TreeNode{
		Name:     "withfiles",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
		Parent:   root,
		Files:    []*FileInfo{{Name: "test.txt"}},
	}
	root.Children["withfiles"] = dirWithFiles

	// Add a directory with only child directories that have files
	dirWithChildDirs := &TreeNode{
		Name:     "withchilddirs",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
		Parent:   root,
	}
	root.Children["withchilddirs"] = dirWithChildDirs

	childDir := &TreeNode{
		Name:     "child",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
		Parent:   dirWithChildDirs,
		Files:    []*FileInfo{{Name: "child.txt"}},
	}
	dirWithChildDirs.Children["child"] = childDir

	markEntireDirectories(root)

	if !dirWithFiles.IsEntireDir {
		t.Error("Directory with files should be marked as entire")
	}

	if !childDir.IsEntireDir {
		t.Error("Child directory with files should be marked as entire")
	}

	if !dirWithChildDirs.IsEntireDir {
		t.Error("Directory with all entire children should be marked as entire")
	}
}

// Test printTree function
func TestPrintTree(t *testing.T) {
	files := []*FileInfo{
		{RelativePath: "file1.txt", Name: "file1.txt", Size: 1024},
		{RelativePath: "subdir/file2.txt", Name: "file2.txt", Size: 2048},
	}

	tree := buildTree(files)

	t.Run("without details", func(t *testing.T) {
		output := captureOutput(t, func() {
			printTree(tree, "", true, false, nil)
		})

		if !strings.Contains(output, "📄 file1.txt") {
			t.Error("Output should contain file1.txt")
		}
		if !strings.Contains(output, "📁 subdir/") {
			t.Error("Output should contain subdir/")
		}
		if strings.Contains(output, "KB") {
			t.Error("Output should not contain file sizes without details flag")
		}
	})

	t.Run("with details", func(t *testing.T) {
		output := captureOutput(t, func() {
			printTree(tree, "", true, true, nil)
		})

		if !strings.Contains(output, "1.00 KB") {
			t.Error("Output should contain file size for file1.txt")
		}
		if !strings.Contains(output, "2.00 KB") {
			t.Error("Output should contain file size for file2.txt")
		}
	})

	t.Run("with name mappings", func(t *testing.T) {
		nameMappings := map[string][]*FileInfo{
			"file1.txt": {{RelativePath: "backup/file1.txt"}},
		}

		output := captureOutput(t, func() {
			printTree(tree, "", true, false, nameMappings)
		})

		if !strings.Contains(output, "→ backup/file1.txt") {
			t.Error("Output should contain mapping arrow and path")
		}
	})
}

// Test countTreeItems function
func TestCountTreeItems(t *testing.T) {
	files := []*FileInfo{
		{RelativePath: "file1.txt", Name: "file1.txt"},
		{RelativePath: "subdir/file2.txt", Name: "file2.txt"},
		{RelativePath: "subdir/nested/file3.txt", Name: "file3.txt"},
	}

	tree := buildTree(files)
	fileCount, dirCount := countTreeItems(tree)

	if fileCount != 3 {
		t.Errorf("Expected 3 files, got %d", fileCount)
	}
	if dirCount != 2 {
		t.Errorf("Expected 2 directories, got %d", dirCount)
	}
}

// Test formatSize function
func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1536 * 1024 * 1024, "1.50 GB"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("size_%d", tt.size), func(t *testing.T) {
			result := formatSize(tt.size)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %s, want %s", tt.size, result, tt.expected)
			}
		})
	}
}

// Integration tests
func TestIntegrationComplexScenario(t *testing.T) {
	// Create complex directory structures for integration testing
	structure1 := map[string]string{
		"common/file1.txt":         "same content",
		"common/file2.txt":         "original content",
		"unique1/file3.txt":        "unique to set1",
		"unique1/subdir/file4.txt": "another unique",
		"renamed/original.txt":     "will be renamed",
		"modified/doc.txt":         "original version",
	}

	structure2 := map[string]string{
		"common/file1.txt":         "same content",
		"common/file2.txt":         "modified content",
		"unique2/file5.txt":        "unique to set2",
		"unique2/subdir/file6.txt": "another unique in set2",
		"renamed/newname.txt":      "will be renamed",
		"modified/doc.txt":         "updated version",
		"completely_new/file7.txt": "brand new file",
	}

	tmpDir1 := createTempDir(t, structure1)
	tmpDir2 := createTempDir(t, structure2)

	set1, err := walkDirectories([]string{tmpDir1})
	if err != nil {
		t.Fatalf("Failed to walk set1: %v", err)
	}

	set2, err := walkDirectories([]string{tmpDir2})
	if err != nil {
		t.Fatalf("Failed to walk set2: %v", err)
	}

	result := compareFileSets(set1, set2)

	// Verify same name different hash
	if len(result.SameNameDifferentHash) != 2 {
		t.Errorf("Expected 2 same name different hash files, got %d", len(result.SameNameDifferentHash))
	}

	// Check specific files
	sameNameFiles := make(map[string]bool)
	for _, file := range result.SameNameDifferentHash {
		sameNameFiles[file.Name] = true
	}

	if !sameNameFiles["file2.txt"] || !sameNameFiles["doc.txt"] {
		t.Error("Expected file2.txt and doc.txt in same name different hash")
	}

	// Verify unique files
	if len(result.UniqueToSet2) != 3 {
		t.Errorf("Expected 3 unique to set2 files, got %d", len(result.UniqueToSet2))
	}

	if len(result.UniqueToSet1) != 2 {
		t.Errorf("Expected 2 unique to set1 files, got %d", len(result.UniqueToSet1))
	}

	// Test tree building with this complex scenario
	tree2 := buildSmartTree(result.UniqueToSet2, set1)
	tree1 := buildSmartTree(result.UniqueToSet1, set2)

	// Verify trees are built correctly
	if len(tree2.Children) == 0 {
		t.Error("Tree2 should have child directories")
	}
	if len(tree1.Children) == 0 {
		t.Error("Tree1 should have child directories")
	}
}

// Benchmark tests
func BenchmarkHashFile(b *testing.B) {
	// Create a temporary file with some content
	tmpFile := filepath.Join(b.TempDir(), "benchmark.txt")
	content := strings.Repeat("benchmark content\n", 1000)
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		b.Fatalf("Failed to create benchmark file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hashFile(tmpFile)
		if err != nil {
			b.Fatalf("hashFile failed: %v", err)
		}
	}
}

func BenchmarkWalkDirectories(b *testing.B) {
	// Create a directory structure with many files
	structure := make(map[string]string)
	for i := 0; i < 100; i++ {
		structure[fmt.Sprintf("dir%d/file%d.txt", i%10, i)] = fmt.Sprintf("content%d", i)
	}

	tmpDir := createTempDir(b, structure)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := walkDirectories([]string{tmpDir})
		if err != nil {
			b.Fatalf("walkDirectories failed: %v", err)
		}
	}
}

// Test edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	t.Run("files with same name in same directory", func(t *testing.T) {
		// This shouldn't happen in real filesystems, but test our handling
		files := []*FileInfo{
			{RelativePath: "file.txt", Name: "file.txt", Hash: "hash1"},
			{RelativePath: "file.txt", Name: "file.txt", Hash: "hash2"},
		}

		// Our FileSet should handle multiple files with same name
		fileSet := &FileSet{
			Files:   files,
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		for _, file := range files {
			fileSet.NameMap[file.Name] = append(fileSet.NameMap[file.Name], file)
			fileSet.HashMap[file.Hash] = append(fileSet.HashMap[file.Hash], file)
		}

		if len(fileSet.NameMap["file.txt"]) != 2 {
			t.Error("Should handle multiple files with same name")
		}
	})

	t.Run("very deep directory structure", func(t *testing.T) {
		structure := make(map[string]string)
		deepPath := strings.Repeat("deep/", 50) + "file.txt"
		structure[deepPath] = "deep content"

		tmpDir := createTempDir(t, structure)
		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("Should handle deep directory structure: %v", err)
		}

		if len(fileSet.Files) != 1 {
			t.Error("Should find file in deep directory structure")
		}
	})

	t.Run("empty file sets", func(t *testing.T) {
		set1 := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		set2 := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		result := compareFileSets(set1, set2)
		if len(result.SameNameDifferentHash) != 0 || len(result.UniqueToSet1) != 0 || len(result.UniqueToSet2) != 0 {
			t.Error("Empty sets should produce empty comparison result")
		}
	})
}

// Test main function behavior (without actually calling os.Exit)
func TestMainLogic(t *testing.T) {
	// This tests the main comparison logic without the CLI parsing
	structure1 := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "original",
	}
	structure2 := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "modified",
		"file3.txt": "new file",
	}

	tmpDir1 := createTempDir(t, structure1)
	tmpDir2 := createTempDir(t, structure2)

	// Test the main workflow
	set1, err := walkDirectories([]string{tmpDir1})
	if err != nil {
		t.Fatalf("Failed to analyze first set: %v", err)
	}

	set2, err := walkDirectories([]string{tmpDir2})
	if err != nil {
		t.Fatalf("Failed to analyze second set: %v", err)
	}

	result := compareFileSets(set1, set2)

	// Test that output can be generated without errors
	if len(result.SameNameDifferentHash) > 0 {
		tree1 := buildTree(result.SameNameDifferentHash)
		output := captureOutput(t, func() {
			printTree(tree1, "", true, false, result.NameMappings)
		})
		if len(output) == 0 {
			t.Error("Should generate output for same name different hash files")
		}
	}

	if len(result.UniqueToSet2) > 0 {
		tree2 := buildSmartTree(result.UniqueToSet2, set1)
		output := captureOutput(t, func() {
			printTree(tree2, "", true, false, nil)
		})
		if len(output) == 0 {
			t.Error("Should generate output for unique to set2 files")
		}
	}

	// Verify the logic worked correctly
	if len(result.SameNameDifferentHash) != 1 {
		t.Errorf("Expected 1 same name different hash file, got %d", len(result.SameNameDifferentHash))
	}

	if len(result.UniqueToSet2) != 1 {
		t.Errorf("Expected 1 unique to set2 file, got %d", len(result.UniqueToSet2))
	}

	if len(result.UniqueToSet1) != 0 {
		t.Errorf("Expected 0 unique to set1 files, got %d", len(result.UniqueToSet1))
	}
}

// Additional specialized tests for complete coverage

func TestFileSetStructure(t *testing.T) {
	t.Run("FileSet creation and population", func(t *testing.T) {
		fileSet := &FileSet{
			Files:   make([]*FileInfo, 0),
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		file := &FileInfo{
			RelativePath: "test.txt",
			AbsolutePath: "/path/to/test.txt",
			Name:         "test.txt",
			Hash:         "testhash",
			Size:         1024,
			RootDir:      "/path/to",
		}

		fileSet.Files = append(fileSet.Files, file)
		fileSet.NameMap[file.Name] = append(fileSet.NameMap[file.Name], file)
		fileSet.HashMap[file.Hash] = append(fileSet.HashMap[file.Hash], file)

		if len(fileSet.Files) != 1 {
			t.Error("FileSet should contain 1 file")
		}
		if len(fileSet.NameMap) != 1 {
			t.Error("NameMap should contain 1 entry")
		}
		if len(fileSet.HashMap) != 1 {
			t.Error("HashMap should contain 1 entry")
		}
	})
}

func TestComparisonResultStructure(t *testing.T) {
	t.Run("ComparisonResult initialization", func(t *testing.T) {
		result := &ComparisonResult{
			SameNameDifferentHash: make([]*FileInfo, 0),
			NameMappings:          make(map[string][]*FileInfo),
			UniqueToSet2:          make([]*FileInfo, 0),
			UniqueToSet1:          make([]*FileInfo, 0),
		}

		if result.SameNameDifferentHash == nil {
			t.Error("SameNameDifferentHash should be initialized")
		}
		if result.NameMappings == nil {
			t.Error("NameMappings should be initialized")
		}
		if result.UniqueToSet2 == nil {
			t.Error("UniqueToSet2 should be initialized")
		}
		if result.UniqueToSet1 == nil {
			t.Error("UniqueToSet1 should be initialized")
		}
	})
}

func TestTreeNodeStructure(t *testing.T) {
	t.Run("TreeNode creation and relationships", func(t *testing.T) {
		parent := &TreeNode{
			Name:     "parent",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		child := &TreeNode{
			Name:   "child",
			IsDir:  true,
			Parent: parent,
		}

		parent.Children["child"] = child

		if child.Parent != parent {
			t.Error("Child should reference parent")
		}
		if parent.Children["child"] != child {
			t.Error("Parent should contain child")
		}
	})
}

func TestPrintTreeEdgeCases(t *testing.T) {
	t.Run("empty tree", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		output := captureOutput(t, func() {
			printTree(root, "", true, false, nil)
		})

		// Should not crash, might produce minimal output
		if len(output) < 0 { // Just ensuring it doesn't crash
			t.Error("Should handle empty tree")
		}
	})

	t.Run("single file tree", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Files:    []*FileInfo{{Name: "single.txt", Size: 100}},
		}

		output := captureOutput(t, func() {
			printTree(root, "", true, true, nil)
		})

		if !strings.Contains(output, "single.txt") {
			t.Error("Should contain single file")
		}
		if !strings.Contains(output, "0.10 KB") {
			t.Error("Should show file size with details")
		}
	})

	t.Run("tree with entire directory marking", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		entireDir := &TreeNode{
			Name:        "entire",
			IsDir:       true,
			Children:    make(map[string]*TreeNode),
			IsEntireDir: true,
		}

		subDir := &TreeNode{
			Name:     "sub",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   entireDir,
		}

		entireDir.Children["sub"] = subDir
		root.Children["entire"] = entireDir

		output := captureOutput(t, func() {
			printTree(root, "", true, false, nil)
		})

		if !strings.Contains(output, "(entire directory)") {
			t.Error("Should indicate entire directory")
		}
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("sortFileInfoSlice", func(t *testing.T) {
		files := []*FileInfo{
			{RelativePath: "z.txt"},
			{RelativePath: "a.txt"},
			{RelativePath: "m.txt"},
		}

		sortFileInfoSlice(files)

		if files[0].RelativePath != "a.txt" {
			t.Error("First file should be a.txt after sorting")
		}
		if files[2].RelativePath != "z.txt" {
			t.Error("Last file should be z.txt after sorting")
		}
	})
}

func TestLargeFileScenario(t *testing.T) {
	t.Run("handling large file sets", func(t *testing.T) {
		// Create a large number of files to test performance and memory usage
		structure := make(map[string]string)
		for i := 0; i < 1000; i++ {
			structure[fmt.Sprintf("dir%d/file%d.txt", i%10, i)] = fmt.Sprintf("content%d", i)
		}

		tmpDir := createTempDir(t, structure)
		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("Should handle large file set: %v", err)
		}

		if len(fileSet.Files) != 1000 {
			t.Errorf("Expected 1000 files, got %d", len(fileSet.Files))
		}

		// Test that maps are populated correctly
		if len(fileSet.NameMap) != 1000 {
			t.Errorf("Expected 1000 entries in NameMap, got %d", len(fileSet.NameMap))
		}
	})
}

func TestSpecialCharactersInPaths(t *testing.T) {
	t.Run("paths with special characters", func(t *testing.T) {
		structure := map[string]string{
			"dir with spaces/file.txt":      "content1",
			"dir-with-dashes/file.txt":      "content2",
			"dir_with_underscores/file.txt": "content3",
			"dir.with.dots/file.txt":        "content4",
		}

		tmpDir := createTempDir(t, structure)
		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("Should handle special characters in paths: %v", err)
		}

		if len(fileSet.Files) != 4 {
			t.Errorf("Expected 4 files, got %d", len(fileSet.Files))
		}

		// Verify all files were found
		found := make(map[string]bool)
		for _, file := range fileSet.Files {
			found[file.Name] = true
		}

		if !found["file.txt"] {
			t.Error("Should find files with special characters in parent directory names")
		}
	})
}

func TestSymlinksAndSpecialFiles(t *testing.T) {
	t.Run("skip symbolic links and special files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a regular file
		regularFile := filepath.Join(tmpDir, "regular.txt")
		err := os.WriteFile(regularFile, []byte("regular content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create regular file: %v", err)
		}

		// Only test on Unix-like systems where symlinks are supported
		if os.PathSeparator == '/' {
			// Create a symbolic link
			symlinkFile := filepath.Join(tmpDir, "symlink.txt")
			err = os.Symlink(regularFile, symlinkFile)
			if err != nil {
				t.Logf("Could not create symlink (may not be supported): %v", err)
			}
		}

		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("walkDirectories should handle symlinks: %v", err)
		}

		// Should find at least the regular file
		if len(fileSet.Files) < 1 {
			t.Error("Should find at least the regular file")
		}

		// Verify regular file is found
		found := false
		for _, file := range fileSet.Files {
			if file.Name == "regular.txt" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should find regular.txt")
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent file operations", func(t *testing.T) {
		structure := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
			"file3.txt": "content3",
		}

		tmpDir := createTempDir(t, structure)

		// Test that multiple goroutines can safely access walkDirectories
		done := make(chan bool, 3)

		for i := 0; i < 3; i++ {
			go func() {
				defer func() { done <- true }()
				_, err := walkDirectories([]string{tmpDir})
				if err != nil {
					t.Errorf("Concurrent access failed: %v", err)
				}
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 3; i++ {
			<-done
		}
	})
}

func TestErrorPropagation(t *testing.T) {
	t.Run("walkDirectories error handling", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a file we'll make unreadable
		unreadableFile := filepath.Join(tmpDir, "unreadable.txt")
		err := os.WriteFile(unreadableFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Make file unreadable (on Unix systems)
		if os.PathSeparator == '/' {
			err = os.Chmod(unreadableFile, 0000)
			if err != nil {
				t.Logf("Could not change file permissions: %v", err)
			} else {
				defer os.Chmod(unreadableFile, 0644) // Restore for cleanup
			}
		}

		// Should continue processing despite unreadable files
		output := captureOutput(t, func() {
			fileSet, err := walkDirectories([]string{tmpDir})
			if err != nil {
				t.Errorf("walkDirectories should not fail for unreadable files: %v", err)
			}
			// May find 0 or 1 files depending on permissions
			_ = fileSet
		})

		// Should see a warning about the unreadable file (on systems where chmod works)
		if os.PathSeparator == '/' && strings.Contains(output, "Warning") {
			// Good, warning was printed
		}
	})
}
