package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileInfo represents metadata about a file
type FileInfo struct {
	RelativePath string // Path relative to the root directory
	AbsolutePath string // Full path
	Name         string // Just the filename
	Hash         string // SHA256 hash of contents
	Size         int64  // File size
	RootDir      string // Which root directory this file came from
}

// FileSet represents a collection of files with lookup maps
type FileSet struct {
	Files   []*FileInfo
	NameMap map[string][]*FileInfo // filename -> list of FileInfo
	HashMap map[string][]*FileInfo // hash -> list of FileInfo
}

// ComparisonResult holds the results of comparing two file sets
type ComparisonResult struct {
	SameNameDifferentHash []*FileInfo            // Files in set2 with same name but different hash as set1
	NameMappings          map[string][]*FileInfo // For same-name files, maps set2 file name to set1 files with same name
	UniqueToSet2          []*FileInfo            // Files in set2 with no name or hash match in set1
	UniqueToSet1          []*FileInfo            // Files in set1 with no name or hash match in set2
}

// TreeNode represents a node in the directory tree for output
type TreeNode struct {
	Name        string
	IsDir       bool
	Files       []*FileInfo
	Children    map[string]*TreeNode
	Parent      *TreeNode
	IsEntireDir bool // True if this entire directory is missing
}

// hashFile calculates SHA256 hash of a file
func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// walkDirectories recursively walks through directories and builds a FileSet
func walkDirectories(dirs []string) (*FileSet, error) {
	fileSet := &FileSet{
		Files:   make([]*FileInfo, 0),
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}

	for _, dir := range dirs {
		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Printf("Warning: Directory %s does not exist, skipping...\n", dir)
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Printf("Warning: Error accessing %s: %v\n", path, err)
				return nil // Continue walking
			}

			if info.IsDir() {
				return nil
			}

			hash, err := hashFile(path)
			if err != nil {
				fmt.Printf("Warning: Could not hash file %s: %v\n", path, err)
				return nil
			}

			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				relPath = path
			}

			fileInfo := &FileInfo{
				RelativePath: relPath,
				AbsolutePath: path,
				Name:         info.Name(),
				Hash:         hash,
				Size:         info.Size(),
				RootDir:      dir,
			}

			fileSet.Files = append(fileSet.Files, fileInfo)
			fileSet.NameMap[info.Name()] = append(fileSet.NameMap[info.Name()], fileInfo)
			fileSet.HashMap[hash] = append(fileSet.HashMap[hash], fileInfo)

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking directory %s: %v", dir, err)
		}
	}

	return fileSet, nil
}

// compareFileSets performs the sophisticated comparison between two file sets
func compareFileSets(set1, set2 *FileSet) *ComparisonResult {
	result := &ComparisonResult{
		SameNameDifferentHash: make([]*FileInfo, 0),
		NameMappings:          make(map[string][]*FileInfo),
		UniqueToSet2:          make([]*FileInfo, 0),
		UniqueToSet1:          make([]*FileInfo, 0),
	}

	// Process files in set2
	for _, file2 := range set2.Files {
		// Check if same hash exists in set1 (ignore these)
		if _, hashExists := set1.HashMap[file2.Hash]; hashExists {
			continue // Same content exists, skip
		}

		// Check if same name exists in set1
		if files1WithSameName, nameExists := set1.NameMap[file2.Name]; nameExists {
			// Same name exists but different hash
			result.SameNameDifferentHash = append(result.SameNameDifferentHash, file2)
			result.NameMappings[file2.Name] = files1WithSameName
		} else {
			// No name or hash match
			result.UniqueToSet2 = append(result.UniqueToSet2, file2)
		}
	}

	// Process files in set1 (for the optional third tree)
	for _, file1 := range set1.Files {
		// Check if same hash exists in set2
		if _, hashExists := set2.HashMap[file1.Hash]; hashExists {
			continue // Same content exists, skip
		}

		// Check if same name exists in set2
		if _, nameExists := set2.NameMap[file1.Name]; !nameExists {
			// No name or hash match
			result.UniqueToSet1 = append(result.UniqueToSet1, file1)
		}
	}

	return result
}

// removeEmptyDirectories removes directories that have no files and no non-empty children
func removeEmptyDirectories(node *TreeNode) bool {
	if !node.IsDir {
		return true // Keep files
	}

	// First, recursively process children and remove empty ones
	for name, child := range node.Children {
		if !removeEmptyDirectories(child) {
			delete(node.Children, name)
		}
	}

	// A directory should be kept if:
	// 1. It has files, OR
	// 2. It has non-empty children
	return len(node.Files) > 0 || len(node.Children) > 0
}

// buildSmartTree creates a tree structure that's smart about showing entire directories
func buildSmartTree(files []*FileInfo, otherSet *FileSet) *TreeNode {
	root := &TreeNode{
		Name:     "",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
	}

	for _, file := range files {
		parts := strings.Split(file.RelativePath, string(filepath.Separator))
		current := root

		// Navigate/create the directory structure
		for i, part := range parts {
			if i == len(parts)-1 {
				// This is the file
				current.Files = append(current.Files, file)
			} else {
				// This is a directory
				if current.Children[part] == nil {
					current.Children[part] = &TreeNode{
						Name:     part,
						IsDir:    true,
						Children: make(map[string]*TreeNode),
						Parent:   current,
					}
				}
				current = current.Children[part]
			}
		}
	}

	// Mark directories that are entirely missing
	markEntireDirectories(root, otherSet)

	// Remove empty directories
	removeEmptyDirectories(root)

	return root
}

// markEntireDirectories marks directories where all contents are missing
func markEntireDirectories(node *TreeNode, otherSet *FileSet) {
	if !node.IsDir {
		return
	}

	// Recursively process children first
	for _, child := range node.Children {
		markEntireDirectories(child, otherSet)
	}

	// Build the full path for this directory
	var pathParts []string
	current := node
	for current != nil && current.Name != "" {
		pathParts = append([]string{current.Name}, pathParts...)
		current = current.Parent
	}
	dirPath := strings.Join(pathParts, string(filepath.Separator))

	// Check if ANY file from this directory path exists in the other set
	// This includes files that might not be in our unique list (e.g., files with same content)
	anyFileFromDirExistsInOtherSet := false
	if dirPath != "" {
		for _, file := range otherSet.Files {
			if strings.HasPrefix(file.RelativePath, dirPath+string(filepath.Separator)) || file.RelativePath == dirPath {
				anyFileFromDirExistsInOtherSet = true
				break
			}
		}
	}

	// A directory is "entire" if:
	// 1. No files from this directory path exist in the other set, AND
	// 2. All children (if any) are also "entire"
	// 3. EXCEPT the root node, which should never be marked as "entire"
	if node.Name == "" {
		// Root node should never be marked as "entire"
		node.IsEntireDir = false
	} else if !anyFileFromDirExistsInOtherSet {
		// No files from this directory path exist in the other set
		if len(node.Children) == 0 {
			// Leaf directory with no matches in other set
			node.IsEntireDir = true
		} else {
			// Directory with children - only mark as entire if ALL children are entire
			allChildrenEntire := true
			for _, child := range node.Children {
				if !child.IsEntireDir {
					allChildrenEntire = false
					break
				}
			}
			node.IsEntireDir = allChildrenEntire
		}
	} else {
		// Some files from this directory path exist in the other set, so not entire
		node.IsEntireDir = false
	}

}

// printTree prints the tree structure with proper formatting
func printTree(node *TreeNode, prefix string, isLast bool, showDetails bool, nameMappings map[string][]*FileInfo) {
	if node.Name != "" {
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		if node.IsDir {
			if node.IsEntireDir {
				fmt.Printf("%s%s📁 %s/ (entire directory)\n", prefix, connector, node.Name)
			} else {
				fmt.Printf("%s%s📁 %s/\n", prefix, connector, node.Name)
			}
		}

		if isLast {
			prefix += "    "
		} else {
			prefix += "│   "
		}
	}

	// If this directory is marked as "entire", don't print its contents
	if node.IsEntireDir {
		return
	}

	// Print files in this directory
	for i, file := range node.Files {
		isLastFile := i == len(node.Files)-1 && len(node.Children) == 0
		connector := "├── "
		if isLastFile {
			connector = "└── "
		}

		fileOutput := fmt.Sprintf("📄 %s", file.Name)
		if showDetails {
			fileOutput += fmt.Sprintf(" (%.2f KB)", float64(file.Size)/1024.0)
		}

		// Add mapping information for same-name files
		if nameMappings != nil {
			if mappedFiles, exists := nameMappings[file.Name]; exists && len(mappedFiles) > 0 {
				fileOutput += fmt.Sprintf(" → %s", mappedFiles[0].RelativePath)
			}
		}

		fmt.Printf("%s%s%s\n", prefix, connector, fileOutput)
	}

	// Print subdirectories
	var childNames []string
	for name := range node.Children {
		childNames = append(childNames, name)
	}
	sort.Strings(childNames)

	for i, name := range childNames {
		isLastChild := i == len(childNames)-1
		printTree(node.Children[name], prefix, isLastChild, showDetails, nameMappings)
	}
}

// countTreeItems counts total files and directories in the tree
func countTreeItems(node *TreeNode) (files int, dirs int) {
	files += len(node.Files)

	for _, child := range node.Children {
		if child.IsDir {
			dirs++
			childFiles, childDirs := countTreeItems(child)
			files += childFiles
			dirs += childDirs
		}
	}

	return files, dirs
}

// buildTree creates a simple tree structure from the list of files
func buildTree(files []*FileInfo) *TreeNode {
	root := &TreeNode{
		Name:     "",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
	}

	for _, file := range files {
		parts := strings.Split(file.RelativePath, string(filepath.Separator))
		current := root

		// Navigate/create the directory structure
		for i, part := range parts {
			if i == len(parts)-1 {
				// This is the file
				current.Files = append(current.Files, file)
			} else {
				// This is a directory
				if current.Children[part] == nil {
					current.Children[part] = &TreeNode{
						Name:     part,
						IsDir:    true,
						Children: make(map[string]*TreeNode),
						Parent:   current,
					}
				}
				current = current.Children[part]
			}
		}
	}

	return root
}

func main() {
	if len(os.Args) < 3 {
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
		os.Exit(1)
	}

	set1Dirs := strings.Split(os.Args[1], ",")
	set2Dirs := strings.Split(os.Args[2], ",")

	showDetails := false
	showUniqueToSet1 := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--details":
			showDetails = true
		case "--show-unique-1":
			showUniqueToSet1 = true
		}
	}

	// Clean up directory paths
	for i := range set1Dirs {
		set1Dirs[i] = strings.TrimSpace(set1Dirs[i])
	}
	for i := range set2Dirs {
		set2Dirs[i] = strings.TrimSpace(set2Dirs[i])
	}

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
		os.Exit(1)
	}
	fmt.Printf("   Found %d files\n", len(set1.Files))

	fmt.Println("🔍 Analyzing second set of directories...")
	set2, err := walkDirectories(set2Dirs)
	if err != nil {
		fmt.Printf("❌ Error analyzing second set: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Found %d files\n", len(set2.Files))

	fmt.Println("🔍 Comparing file sets...")
	result := compareFileSets(set1, set2)

	fmt.Println()

	// First tree: Files with same name but different content
	if len(result.SameNameDifferentHash) > 0 {
		fmt.Printf("⚠️  Files with same name but different content (%d files):\n", len(result.SameNameDifferentHash))
		fmt.Println("=" + strings.Repeat("=", 50))
		fmt.Println()

		tree1 := buildTree(result.SameNameDifferentHash)
		printTree(tree1, "", true, showDetails, result.NameMappings)
		fmt.Println()
	} else {
		fmt.Println("✅ No files found with same name but different content.")
		fmt.Println()
	}

	// Second tree: Files unique to set 2
	if len(result.UniqueToSet2) > 0 {
		fmt.Printf("📋 Files in Set 2 with no match in Set 1 (%d files):\n", len(result.UniqueToSet2))
		fmt.Println("=" + strings.Repeat("=", 50))
		fmt.Println()

		tree2 := buildSmartTree(result.UniqueToSet2, set1)
		printTree(tree2, "", true, showDetails, nil)
		fmt.Println()
	} else {
		fmt.Println("✅ No unique files found in Set 2.")
		fmt.Println()
	}

	// Third tree: Files unique to set 1 (optional)
	if showUniqueToSet1 {
		if len(result.UniqueToSet1) > 0 {
			fmt.Printf("📋 Files in Set 1 with no match in Set 2 (%d files):\n", len(result.UniqueToSet1))
			fmt.Println("=" + strings.Repeat("=", 50))
			fmt.Println()

			tree3 := buildSmartTree(result.UniqueToSet1, set2)
			printTree(tree3, "", true, showDetails, nil)
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
	if showUniqueToSet1 {
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

	if sameNameSize > 0 || uniqueSet2Size > 0 || (showUniqueToSet1 && uniqueSet1Size > 0) {
		fmt.Println("   • Total sizes:")
		if sameNameSize > 0 {
			fmt.Printf("     - Same name, different content: %s\n", formatSize(sameNameSize))
		}
		if uniqueSet2Size > 0 {
			fmt.Printf("     - Unique to Set 2: %s\n", formatSize(uniqueSet2Size))
		}
		if showUniqueToSet1 && uniqueSet1Size > 0 {
			fmt.Printf("     - Unique to Set 1: %s\n", formatSize(uniqueSet1Size))
		}
	}
}

// formatSize formats file sizes in human-readable format
func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024.0)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/(1024.0*1024.0))
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/(1024.0*1024.0*1024.0))
	}
}
