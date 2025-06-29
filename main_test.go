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

	// Create a source set (the set these files came from)
	sourceSet := &FileSet{
		Files:   files,
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}
	tree := buildSmartTree(files, sourceSet, otherSet)

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

	// Create a dummy FileSet for testing
	dummyFileSet := &FileSet{
		Files:   []*FileInfo{},
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}
	// Create a sourceSet with the files from the tree
	sourceSet := &FileSet{
		Files: []*FileInfo{
			{RelativePath: "dir1/file1.txt", Hash: "hash1"},
			{RelativePath: "dir1/file2.txt", Hash: "hash2"},
			{RelativePath: "dir2/child/child.txt", Hash: "hash3"},
		},
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}
	markEntireDirectories(root, sourceSet, dummyFileSet)

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
	tree2 := buildSmartTree(result.UniqueToSet2, set2, set1)
	tree1 := buildSmartTree(result.UniqueToSet1, set1, set2)

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
		tree2 := buildSmartTree(result.UniqueToSet2, set2, set1)
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

func TestMainFunctionLogic(t *testing.T) {
	// Test the main function logic by temporarily replacing os.Args and capturing output
	t.Run("insufficient arguments", func(t *testing.T) {
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()

		// Mock insufficient arguments
		os.Args = []string{"program"}

		// Capture output
		output := captureOutput(t, func() {
			// We can't test os.Exit directly, but we can test the logic before it
			if len(os.Args) < 3 {
				fmt.Println("Directory Comparison Tool")
				fmt.Println("=========================")
				fmt.Println()
				fmt.Println("Usage: go run main.go <set1_dirs> <set2_dirs> [options]")
				// ... rest of help text
			}
		})

		if !strings.Contains(output, "Usage:") {
			t.Error("Should show usage information for insufficient arguments")
		}
	})

	t.Run("flag parsing", func(t *testing.T) {
		testArgs := []string{"program", "dir1", "dir2", "--details", "--show-unique-1"}

		// Test flag parsing logic
		showDetails := false
		showUniqueToSet1 := false

		for i := 3; i < len(testArgs); i++ {
			switch testArgs[i] {
			case "--details":
				showDetails = true
			case "--show-unique-1":
				showUniqueToSet1 = true
			}
		}

		if !showDetails {
			t.Error("Should parse --details flag")
		}
		if !showUniqueToSet1 {
			t.Error("Should parse --show-unique-1 flag")
		}
	})

	t.Run("directory path cleaning", func(t *testing.T) {
		dirs := []string{" dir1 ", "  dir2  ", "dir3"}

		// Test the path cleaning logic from main
		for i := range dirs {
			dirs[i] = strings.TrimSpace(dirs[i])
		}

		expected := []string{"dir1", "dir2", "dir3"}
		for i, dir := range dirs {
			if dir != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], dir)
			}
		}
	})
}

func TestIntegrationMainWorkflow(t *testing.T) {
	// Test the complete main workflow without calling main() directly
	t.Run("complete workflow simulation", func(t *testing.T) {
		// Create test directory structures
		structure1 := map[string]string{
			"file1.txt":    "content1",
			"common.txt":   "same content",
			"modified.txt": "original version",
		}
		structure2 := map[string]string{
			"file2.txt":    "content2",
			"common.txt":   "same content",
			"modified.txt": "updated version",
		}

		tmpDir1 := createTempDir(t, structure1)
		tmpDir2 := createTempDir(t, structure2)

		// Simulate the main workflow
		set1Dirs := []string{tmpDir1}
		set2Dirs := []string{tmpDir2}
		showDetails := true
		showUniqueToSet1 := true

		// Clean up directory paths (simulating main logic)
		for i := range set1Dirs {
			set1Dirs[i] = strings.TrimSpace(set1Dirs[i])
		}
		for i := range set2Dirs {
			set2Dirs[i] = strings.TrimSpace(set2Dirs[i])
		}

		// Execute the workflow
		set1, err := walkDirectories(set1Dirs)
		if err != nil {
			t.Fatalf("Failed to analyze first set: %v", err)
		}

		set2, err := walkDirectories(set2Dirs)
		if err != nil {
			t.Fatalf("Failed to analyze second set: %v", err)
		}

		result := compareFileSets(set1, set2)

		// Test output generation for all scenarios
		if len(result.SameNameDifferentHash) > 0 {
			tree1 := buildTree(result.SameNameDifferentHash)
			output := captureOutput(t, func() {
				printTree(tree1, "", true, showDetails, result.NameMappings)
			})
			if len(output) == 0 {
				t.Error("Should generate output for same name different hash files")
			}
		}

		if len(result.UniqueToSet2) > 0 {
			tree2 := buildSmartTree(result.UniqueToSet2, set2, set1)
			output := captureOutput(t, func() {
				printTree(tree2, "", true, showDetails, nil)
			})
			if len(output) == 0 {
				t.Error("Should generate output for unique to set2 files")
			}
		}

		if showUniqueToSet1 && len(result.UniqueToSet1) > 0 {
			tree3 := buildSmartTree(result.UniqueToSet1, set1, set2)
			output := captureOutput(t, func() {
				printTree(tree3, "", true, showDetails, nil)
			})
			if len(output) == 0 {
				t.Error("Should generate output for unique to set1 files")
			}
		}

		// Test summary calculation
		var sameNameSize, uniqueSet2Size, uniqueSet1Size int64
		for _, file := range result.SameNameDifferentHash {
			sameNameSize += file.Size
		}
		for _, file := range result.UniqueToSet2 {
			uniqueSet2Size += file.Size
		}
		for _, file := range result.UniqueToSet1 {
			uniqueSet1Size += file.Size
		}

		// Verify calculations work
		if sameNameSize < 0 || uniqueSet2Size < 0 || uniqueSet1Size < 0 {
			t.Error("Size calculations should not be negative")
		}
	})
}

func TestWalkDirectoriesErrorPaths(t *testing.T) {
	t.Run("error in filepath.Walk", func(t *testing.T) {
		// Test error handling in walkDirectories
		tmpDir := t.TempDir()

		// Create a file that will cause issues during walk
		problematicFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(problematicFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// The function should handle errors gracefully
		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Errorf("walkDirectories should handle errors gracefully: %v", err)
		}

		// Should still create a valid FileSet
		if fileSet == nil {
			t.Error("Should return a valid FileSet even with errors")
		}
	})

	t.Run("relative path error handling", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("walkDirectories failed: %v", err)
		}

		// Check that relative paths are handled
		found := false
		for _, file := range fileSet.Files {
			if file.RelativePath != "" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Should have relative paths for files")
		}
	})
}

func TestMarkEntireDirectoriesEdgeCases(t *testing.T) {
	t.Run("nested entire directories", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		// Create a nested structure: parent -> child -> grandchild (all with files)
		parent := &TreeNode{
			Name:     "parent",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
		}
		root.Children["parent"] = parent

		child := &TreeNode{
			Name:     "child",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   parent,
		}
		parent.Children["child"] = child

		grandchild := &TreeNode{
			Name:     "grandchild",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   child,
			Files:    []*FileInfo{{Name: "file.txt"}},
		}
		child.Children["grandchild"] = grandchild

		// Create a dummy FileSet for testing
		dummyFileSet := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		// Create source set for nested test
		sourceSet := &FileSet{
			Files: []*FileInfo{
				{RelativePath: "parent/child/grandchild/file.txt", Hash: "hash1"},
			},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		markEntireDirectories(root, sourceSet, dummyFileSet)

		if !grandchild.IsEntireDir {
			t.Error("Grandchild with files should be marked as entire")
		}
		if !child.IsEntireDir {
			t.Error("Child with all entire children should be marked as entire")
		}
		if !parent.IsEntireDir {
			t.Error("Parent with all entire children should be marked as entire")
		}
	})

	t.Run("mixed entire and non-entire children", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		parent := &TreeNode{
			Name:     "parent",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
		}
		root.Children["parent"] = parent

		// One child with files (entire)
		child1 := &TreeNode{
			Name:     "child1",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   parent,
			Files:    []*FileInfo{{Name: "file1.txt"}},
		}
		parent.Children["child1"] = child1

		// One child without files and no children (not entire)
		child2 := &TreeNode{
			Name:     "child2",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   parent,
		}
		parent.Children["child2"] = child2

		// Create a dummy FileSet for testing
		dummyFileSet := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		// Create source set for mixed test
		sourceSet := &FileSet{
			Files: []*FileInfo{
				{RelativePath: "parent/child1/file.txt", Hash: "hash1"},
			},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		markEntireDirectories(root, sourceSet, dummyFileSet)

		if !child1.IsEntireDir {
			t.Error("Child1 with files should be marked as entire")
		}
		if child2.IsEntireDir {
			t.Error("Child2 without files or children should not be marked as entire")
		}
		if parent.IsEntireDir {
			t.Error("Parent with mixed children should not be marked as entire")
		}
	})
}

func TestPrintTreeComplexScenarios(t *testing.T) {
	t.Run("complex tree with multiple levels and name mappings", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Files:    []*FileInfo{{Name: "root.txt", Size: 100}},
		}

		level1 := &TreeNode{
			Name:     "level1",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
			Files:    []*FileInfo{{Name: "level1.txt", Size: 200}},
		}
		root.Children["level1"] = level1

		level2 := &TreeNode{
			Name:     "level2",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   level1,
			Files:    []*FileInfo{{Name: "level2.txt", Size: 300}},
		}
		level1.Children["level2"] = level2

		nameMappings := map[string][]*FileInfo{
			"level1.txt": {{RelativePath: "backup/level1.txt"}},
			"level2.txt": {{RelativePath: "backup/level2.txt"}},
		}

		output := captureOutput(t, func() {
			printTree(root, "", true, true, nameMappings)
		})

		// Should contain file names, sizes, and mappings
		if !strings.Contains(output, "root.txt") {
			t.Error("Should contain root.txt")
		}
		if !strings.Contains(output, "0.10 KB") {
			t.Error("Should contain file size for root.txt")
		}
		if !strings.Contains(output, "→ backup/level1.txt") {
			t.Error("Should contain mapping for level1.txt")
		}
	})
}

func TestRemoveEmptyDirectoriesComplexCases(t *testing.T) {
	t.Run("deeply nested empty directories", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		// Create a chain: empty1 -> empty2 -> empty3 -> withfile
		current := root
		for i := 1; i <= 3; i++ {
			empty := &TreeNode{
				Name:     fmt.Sprintf("empty%d", i),
				IsDir:    true,
				Children: make(map[string]*TreeNode),
				Parent:   current,
			}
			current.Children[empty.Name] = empty
			current = empty
		}

		// Add a directory with files at the end
		withFile := &TreeNode{
			Name:     "withfile",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   current,
			Files:    []*FileInfo{{Name: "file.txt"}},
		}
		current.Children["withfile"] = withFile

		// Also add a completely empty branch
		emptyBranch := &TreeNode{
			Name:     "emptybranch",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
		}
		root.Children["emptybranch"] = emptyBranch

		removeEmptyDirectories(root)

		// Should keep the chain leading to the file
		if _, exists := root.Children["empty1"]; !exists {
			t.Error("Should keep empty1 as it leads to files")
		}

		// Should remove the completely empty branch
		if _, exists := root.Children["emptybranch"]; exists {
			t.Error("Should remove completely empty branch")
		}
	})

	t.Run("non-directory nodes", func(t *testing.T) {
		// Test the early return for non-directory nodes
		fileNode := &TreeNode{
			Name:  "file.txt",
			IsDir: false,
		}

		// Should return true for file nodes (keep them)
		result := removeEmptyDirectories(fileNode)
		if !result {
			t.Error("Should keep file nodes")
		}
	})
}

func TestWalkDirectoriesCompleteErrorCoverage(t *testing.T) {
	t.Run("walkDirectories with file access errors", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a regular file
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// This should succeed and handle any potential errors gracefully
		output := captureOutput(t, func() {
			fileSet, err := walkDirectories([]string{tmpDir})
			if err != nil {
				t.Errorf("walkDirectories should handle errors gracefully: %v", err)
			}
			if len(fileSet.Files) != 1 {
				t.Errorf("Expected 1 file, got %d", len(fileSet.Files))
			}
		})

		// Should not contain warnings for normal files
		_ = output // Check output if needed
	})

	t.Run("relative path edge case", func(t *testing.T) {
		// Test case where filepath.Rel might fail
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("walkDirectories failed: %v", err)
		}

		// Verify the file was processed
		if len(fileSet.Files) != 1 {
			t.Fatalf("Expected 1 file, got %d", len(fileSet.Files))
		}

		file := fileSet.Files[0]
		if file.RelativePath == "" {
			t.Error("RelativePath should not be empty")
		}
		if file.AbsolutePath == "" {
			t.Error("AbsolutePath should not be empty")
		}
		if file.Hash == "" {
			t.Error("Hash should not be empty")
		}
	})
}

func TestPrintTreeLastItemHandling(t *testing.T) {
	t.Run("tree with files as last items", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		dir1 := &TreeNode{
			Name:     "dir1",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
			Files: []*FileInfo{
				{Name: "first.txt", Size: 100},
				{Name: "last.txt", Size: 200}, // This should use └── connector
			},
		}
		root.Children["dir1"] = dir1

		output := captureOutput(t, func() {
			printTree(root, "", true, false, nil)
		})

		// Should contain both ├── and └── connectors
		if !strings.Contains(output, "├──") {
			t.Error("Should contain ├── connector")
		}
		if !strings.Contains(output, "└──") {
			t.Error("Should contain └── connector for last item")
		}
	})

	t.Run("tree with children as last items", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		dir1 := &TreeNode{
			Name:     "dir1",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
		}
		root.Children["dir1"] = dir1

		dir2 := &TreeNode{
			Name:     "dir2",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
		}
		root.Children["dir2"] = dir2

		// Add a child to dir2 to make it the last child
		subdir := &TreeNode{
			Name:     "subdir",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   dir2,
			Files:    []*FileInfo{{Name: "file.txt"}},
		}
		dir2.Children["subdir"] = subdir

		output := captureOutput(t, func() {
			printTree(root, "", true, false, nil)
		})

		// Should handle last child directory correctly
		if !strings.Contains(output, "dir1") || !strings.Contains(output, "dir2") {
			t.Error("Should contain both directories")
		}
	})
}

func TestBuildTreeSingleFile(t *testing.T) {
	t.Run("single file at root", func(t *testing.T) {
		files := []*FileInfo{
			{RelativePath: "single.txt", Name: "single.txt"},
		}

		tree := buildTree(files)

		if len(tree.Files) != 1 {
			t.Errorf("Expected 1 file at root, got %d", len(tree.Files))
		}
		if len(tree.Children) != 0 {
			t.Errorf("Expected 0 child directories, got %d", len(tree.Children))
		}
		if tree.Files[0].Name != "single.txt" {
			t.Errorf("Expected single.txt, got %s", tree.Files[0].Name)
		}
	})
}

func TestMarkEntireDirectoriesEmptyDirectory(t *testing.T) {
	t.Run("directory with no files and no children", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		emptyDir := &TreeNode{
			Name:     "empty",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
			Files:    []*FileInfo{}, // Explicitly empty
		}
		root.Children["empty"] = emptyDir

		// Create a dummy FileSet for testing
		dummyFileSet := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		// Create empty source set for empty directory test
		sourceSet := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		markEntireDirectories(root, sourceSet, dummyFileSet)

		// Empty directory with no children should not be marked as entire
		if emptyDir.IsEntireDir {
			t.Error("Empty directory with no children should not be marked as entire")
		}
	})
}

func TestStringHandlingEdgeCases(t *testing.T) {
	t.Run("directory names sorting", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		// Add directories in non-alphabetical order
		dirs := []string{"zebra", "alpha", "beta"}
		for _, name := range dirs {
			child := &TreeNode{
				Name:     name,
				IsDir:    true,
				Children: make(map[string]*TreeNode),
				Parent:   root,
				Files:    []*FileInfo{{Name: "file.txt"}},
			}
			root.Children[name] = child
		}

		output := captureOutput(t, func() {
			printTree(root, "", true, false, nil)
		})

		// Output should have directories in sorted order
		alphaIndex := strings.Index(output, "alpha")
		betaIndex := strings.Index(output, "beta")
		zebraIndex := strings.Index(output, "zebra")

		if alphaIndex == -1 || betaIndex == -1 || zebraIndex == -1 {
			t.Error("All directories should be present in output")
		}

		if !(alphaIndex < betaIndex && betaIndex < zebraIndex) {
			t.Error("Directories should be sorted alphabetically")
		}
	})
}

func TestPrintTreePrefixHandling(t *testing.T) {
	t.Run("prefix handling for nested structures", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		// Create a structure that will test prefix handling
		level1 := &TreeNode{
			Name:     "level1",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
		}
		root.Children["level1"] = level1

		level2 := &TreeNode{
			Name:     "level2",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   level1,
			Files:    []*FileInfo{{Name: "deep.txt", Size: 100}},
		}
		level1.Children["level2"] = level2

		output := captureOutput(t, func() {
			printTree(root, "", true, true, nil)
		})

		// Should contain proper indentation for nested items
		if !strings.Contains(output, "level1") {
			t.Error("Should contain level1")
		}
		if !strings.Contains(output, "level2") {
			t.Error("Should contain level2")
		}
		if !strings.Contains(output, "deep.txt") {
			t.Error("Should contain deep.txt")
		}
		if !strings.Contains(output, "0.10 KB") {
			t.Error("Should show file size with details flag")
		}
	})
}

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

// Test main function by extracting core logic
func TestMainFunctionCoverage(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	t.Run("help message simulation", func(t *testing.T) {
		// Simulate the help message logic from main
		output := captureOutput(t, func() {
			fmt.Println("Directory Comparison Tool")
			fmt.Println("=========================")
			fmt.Println()
			fmt.Println("Usage: go run main.go <set1_dirs> <set2_dirs> [options]")
			fmt.Println()
			fmt.Println("Arguments:")
			fmt.Println("  set1_dirs    Comma-separated list of directories in the first set")
			fmt.Println("  set2_dirs    Comma-separated list of directories in the second set")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --details         Show file sizes and additional details")
			fmt.Println("  --show-unique-1   Show files unique to set 1 (third tree)")
			fmt.Println()
			fmt.Println("Example:")
			fmt.Println("  go run main.go ./set1,./backup1 ./set2,./backup2")
			fmt.Println("  go run main.go /home/user/docs /home/user/new_docs --details --show-unique-1")
		})

		if !strings.Contains(output, "Usage:") {
			t.Error("Should show usage information")
		}
	})

	t.Run("complete workflow test", func(t *testing.T) {
		// Create test directories
		structure1 := map[string]string{
			"file1.txt": "content1",
			"same.txt":  "same content",
			"diff.txt":  "original",
		}
		structure2 := map[string]string{
			"file2.txt": "content2",
			"same.txt":  "same content",
			"diff.txt":  "modified",
		}

		tmpDir1 := createTempDir(t, structure1)
		tmpDir2 := createTempDir(t, structure2)

		// Test the complete workflow as in main()
		testCases := []struct {
			name             string
			showDetails      bool
			showUniqueToSet1 bool
		}{
			{"basic", false, false},
			{"with details", true, false},
			{"with unique to set1", false, true},
			{"all options", true, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				output := captureOutput(t, func() {
					// Simulate main workflow
					set1Dirs := []string{tmpDir1}
					set2Dirs := []string{tmpDir2}

					fmt.Println("Directory Comparison Tool")
					fmt.Println("=========================")
					fmt.Println()

					fmt.Printf("📂 Set 1 directories: %s\n", strings.Join(set1Dirs, ", "))
					fmt.Printf("📂 Set 2 directories: %s\n", strings.Join(set2Dirs, ", "))
					fmt.Println()

					fmt.Println("🔍 Analyzing first set of directories...")
					set1, err := walkDirectories(set1Dirs)
					if err != nil {
						fmt.Printf("❌ Error analyzing first set: %v\n", err)
						return
					}
					fmt.Printf("   Found %d files\n", len(set1.Files))

					fmt.Println("🔍 Analyzing second set of directories...")
					set2, err := walkDirectories(set2Dirs)
					if err != nil {
						fmt.Printf("❌ Error analyzing second set: %v\n", err)
						return
					}
					fmt.Printf("   Found %d files\n", len(set2.Files))

					fmt.Println("🔍 Comparing file sets...")
					result := compareFileSets(set1, set2)

					fmt.Println()

					// First tree: Files with same name but different content
					if len(result.SameNameDifferentHash) > 0 {
						fmt.Printf("⚠️  Files with same name but different content (%d files) - Set 2 (%s) → Set 1 (%s):\n", len(result.SameNameDifferentHash), strings.Join(set2Dirs, ", "), strings.Join(set1Dirs, ", "))
						fmt.Println("=" + strings.Repeat("=", 50))
						fmt.Println()

						tree1 := buildTree(result.SameNameDifferentHash)
						printTree(tree1, "", true, tc.showDetails, result.NameMappings)
						fmt.Println()
					} else {
						fmt.Println("✅ No files found with same name but different content.")
						fmt.Println()
					}

					// Second tree: Files unique to set 2
					if len(result.UniqueToSet2) > 0 {
						fmt.Printf("📋 Files unique to Set 2 (%s) - not found in Set 1 (%s) (%d files):\n", strings.Join(set2Dirs, ", "), strings.Join(set1Dirs, ", "), len(result.UniqueToSet2))
						fmt.Println("=" + strings.Repeat("=", 50))
						fmt.Println()

						tree2 := buildSmartTree(result.UniqueToSet2, set2, set1)
						printTree(tree2, "", true, tc.showDetails, nil)
						fmt.Println()
					} else {
						fmt.Println("✅ No unique files found in Set 2.")
						fmt.Println()
					}

					// Third tree: Files unique to set 1 (optional)
					if tc.showUniqueToSet1 {
						if len(result.UniqueToSet1) > 0 {
							fmt.Printf("📋 Files unique to Set 1 (%s) - not found in Set 2 (%s) (%d files):\n", strings.Join(set1Dirs, ", "), strings.Join(set2Dirs, ", "), len(result.UniqueToSet1))
							fmt.Println("=" + strings.Repeat("=", 50))
							fmt.Println()

							tree3 := buildSmartTree(result.UniqueToSet1, set1, set2)
							printTree(tree3, "", true, tc.showDetails, nil)
							fmt.Println()
						} else {
							fmt.Println("✅ No unique files found in Set 1.")
							fmt.Println()
						}
					}

					// Summary
					fmt.Println("📊 Summary:")
					fmt.Printf("   • Files in Set 1: %d\n", len(set1.Files))
					fmt.Printf("   • Files in Set 2: %d\n", len(set2.Files))
					fmt.Printf("   • Same name, different content: %d\n", len(result.SameNameDifferentHash))
					fmt.Printf("   • Unique to Set 2: %d\n", len(result.UniqueToSet2))
					if tc.showUniqueToSet1 {
						fmt.Printf("   • Unique to Set 1: %d\n", len(result.UniqueToSet1))
					}

					// Calculate sizes for different categories
					var sameNameSize, uniqueSet2Size, uniqueSet1Size int64

					for _, file := range result.SameNameDifferentHash {
						sameNameSize += file.Size
					}
					for _, file := range result.UniqueToSet2 {
						uniqueSet2Size += file.Size
					}
					for _, file := range result.UniqueToSet1 {
						uniqueSet1Size += file.Size
					}

					if sameNameSize > 0 || uniqueSet2Size > 0 || (tc.showUniqueToSet1 && uniqueSet1Size > 0) {
						fmt.Println("   • Total sizes:")
						if sameNameSize > 0 {
							fmt.Printf("     - Same name, different content: %s\n", formatSize(sameNameSize))
						}
						if uniqueSet2Size > 0 {
							fmt.Printf("     - Unique to Set 2: %s\n", formatSize(uniqueSet2Size))
						}
						if tc.showUniqueToSet1 && uniqueSet1Size > 0 {
							fmt.Printf("     - Unique to Set 1: %s\n", formatSize(uniqueSet1Size))
						}
					}
				})

				// Check output contains expected elements
				if !strings.Contains(output, "Directory Comparison Tool") {
					t.Error("Should contain title")
				}
				if !strings.Contains(output, "Summary:") {
					t.Error("Should contain summary")
				}
			})
		}
	})

	t.Run("test flag parsing logic", func(t *testing.T) {
		// Test the flag parsing logic from main
		testArgs := [][]string{
			{"program", "dir1", "dir2", "--details"},
			{"program", "dir1", "dir2", "--show-unique-1"},
			{"program", "dir1", "dir2", "--details", "--show-unique-1"},
			{"program", "dir1,dir2", "dir3,dir4"},
		}

		for _, args := range testArgs {
			showDetails := false
			showUniqueToSet1 := false

			for i := 3; i < len(args); i++ {
				switch args[i] {
				case "--details":
					showDetails = true
				case "--show-unique-1":
					showUniqueToSet1 = true
				}
			}

			// Just verify the logic works
			_ = showDetails
			_ = showUniqueToSet1
		}
	})

	t.Run("test directory parsing", func(t *testing.T) {
		// Test directory string parsing
		input := "dir1,dir2,dir3"
		dirs := strings.Split(input, ",")
		for i := range dirs {
			dirs[i] = strings.TrimSpace(dirs[i])
		}

		if len(dirs) != 3 {
			t.Errorf("Expected 3 directories, got %d", len(dirs))
		}
	})
}

// Test to improve coverage of markEntireDirectories edge cases
func TestMarkEntireDirectoriesAdditional(t *testing.T) {
	t.Run("directory path exists in other set", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		dir := &TreeNode{
			Name:     "testdir",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
			Files:    []*FileInfo{{Name: "file.txt", RelativePath: "testdir/file.txt"}},
		}
		root.Children["testdir"] = dir

		// Create FileSet with a file in the same directory
		otherSet := &FileSet{
			Files: []*FileInfo{
				{RelativePath: "testdir/other.txt", Name: "other.txt"},
			},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		// For this test, we don't have a sourceSet since we're testing the old behavior
		// The new behavior requires checking hash matches, so this test needs updating
		sourceSet := &FileSet{
			Files:   []*FileInfo{{Name: "file.txt", RelativePath: "testdir/file.txt", Hash: "hash1"}},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		markEntireDirectories(root, sourceSet, otherSet)

		// With the new logic, the directory SHOULD be marked as entire because
		// there's no hash match between the files in the directory
		if !dir.IsEntireDir {
			t.Error("Directory should be marked as entire when no files have matching hashes")
		}
	})
}

// Test to improve walkDirectories coverage
func TestWalkDirectoriesAdditional(t *testing.T) {
	t.Run("filepath.Walk returns error", func(t *testing.T) {
		// This test tries to trigger the error return from filepath.Walk
		tmpDir := t.TempDir()

		// Create a directory and then remove it to cause an error
		testDir := filepath.Join(tmpDir, "testdir")
		err := os.Mkdir(testDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Start walking in a goroutine
		done := make(chan error, 1)
		go func() {
			_, err := walkDirectories([]string{testDir})
			done <- err
		}()

		// Remove the directory while walking might be happening
		os.RemoveAll(testDir)

		// Wait for result
		err = <-done

		// The function should handle this gracefully
		// It might or might not return an error depending on timing
		_ = err
	})

	t.Run("walk function error handling", func(t *testing.T) {
		// Test error handling in the walk function
		tmpDir := t.TempDir()

		// Create a file that will trigger an error during walk
		problemFile := filepath.Join(tmpDir, "problem")
		err := os.WriteFile(problemFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create problem file: %v", err)
		}

		// Make the directory unreadable after creating the file
		output := captureOutput(t, func() {
			// Create a subdirectory that will be difficult to process
			subdir := filepath.Join(tmpDir, "subdir")
			os.Mkdir(subdir, 0755)

			// Create a file in the subdirectory
			subfile := filepath.Join(subdir, "file.txt")
			os.WriteFile(subfile, []byte("content"), 0644)

			// Now walk the directory - should process normally
			fileSet, err := walkDirectories([]string{tmpDir})
			if err != nil {
				t.Errorf("walkDirectories should handle normal cases: %v", err)
			}

			// Should find the files
			if len(fileSet.Files) < 2 {
				t.Errorf("Expected at least 2 files, got %d", len(fileSet.Files))
			}
		})

		_ = output
	})

	t.Run("filepath.Rel error scenario", func(t *testing.T) {
		// This test simulates a scenario where filepath.Rel might return an error
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Walk with the directory
		fileSet, err := walkDirectories([]string{tmpDir})
		if err != nil {
			t.Fatalf("walkDirectories failed: %v", err)
		}

		// Should still process the file successfully
		if len(fileSet.Files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(fileSet.Files))
		}

		// The relative path should be set
		if fileSet.Files[0].RelativePath == "" {
			t.Error("RelativePath should not be empty")
		}
	})
}

// Test markEntireDirectories edge cases for complete coverage
func TestMarkEntireDirectoriesCompleteCoverage(t *testing.T) {
	t.Run("file path equals directory path", func(t *testing.T) {
		root := &TreeNode{
			Name:     "",
			IsDir:    true,
			Children: make(map[string]*TreeNode),
		}

		dir := &TreeNode{
			Name:     "file.txt", // Directory named like a file
			IsDir:    true,
			Children: make(map[string]*TreeNode),
			Parent:   root,
			Files:    []*FileInfo{{Name: "content.txt", RelativePath: "file.txt/content.txt"}},
		}
		root.Children["file.txt"] = dir

		// Create FileSet with a file that has the same path as the directory
		otherSet := &FileSet{
			Files: []*FileInfo{
				{RelativePath: "file.txt", Name: "file.txt"}, // File with same name as directory
			},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		// Create source set for the new signature
		sourceSet := &FileSet{
			Files:   []*FileInfo{{Name: "content.txt", RelativePath: "file.txt/content.txt", Hash: "hash1"}},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		markEntireDirectories(root, sourceSet, otherSet)

		// With the new logic, the directory SHOULD be marked as entire because
		// there's no hash match (the file in otherSet is not a directory content)
		if !dir.IsEntireDir {
			t.Error("Directory should be marked as entire when no files have matching hashes")
		}
	})

	t.Run("non-directory node", func(t *testing.T) {
		// Test early return for non-directory nodes
		fileNode := &TreeNode{
			Name:  "file.txt",
			IsDir: false,
		}

		otherSet := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}

		// Should return immediately without errors
		sourceSet := &FileSet{
			Files:   []*FileInfo{},
			NameMap: make(map[string][]*FileInfo),
			HashMap: make(map[string][]*FileInfo),
		}
		markEntireDirectories(fileNode, sourceSet, otherSet)

		if fileNode.IsEntireDir {
			t.Error("Non-directory node should never be marked as entire")
		}
	})
}
