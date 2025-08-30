package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// FileJob represents a batch of files to be hashed
type FileJob struct {
	Files []FileTask
}

// FileTask represents a single file to be hashed
type FileTask struct {
	Path    string
	Info    os.FileInfo
	RootDir string
	RelPath string
}

// FileResult represents the result of hashing a batch of files
type FileResult struct {
	FileInfos []*FileInfo
	Errors    []error
}

// hashWorker processes batches of files from the job channel
func hashWorker(jobs <-chan FileJob, results chan<- FileResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		batch := FileResult{
			FileInfos: make([]*FileInfo, 0, len(job.Files)),
			Errors:    make([]error, 0),
		}

		for _, task := range job.Files {
			hash, err := hashFile(task.Path)
			if err != nil {
				batch.Errors = append(batch.Errors,
					fmt.Errorf("could not hash file %s: %v", task.Path, err))
				continue
			}

			fileInfo := &FileInfo{
				RelativePath: task.RelPath,
				AbsolutePath: task.Path,
				Name:         task.Info.Name(),
				Hash:         hash,
				Size:         task.Info.Size(),
				RootDir:      task.RootDir,
			}

			batch.FileInfos = append(batch.FileInfos, fileInfo)
		}

		results <- batch
	}
}

// walkDirectories recursively walks through directories and builds a FileSet
func walkDirectories(dirs []string) (*FileSet, error) {
	return walkDirectoriesWithLimit(dirs, -1)
}

// walkDirectoriesWithLimit recursively walks through directories and builds a FileSet with optional file limit
func walkDirectoriesWithLimit(dirs []string, limit int) (*FileSet, error) {
	// First, collect all files to determine if parallelization is worthwhile
	var allTasks []FileTask
	taskCount := 0

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

			// Check limit before adding to tasks
			if limit > 0 && taskCount >= limit {
				return filepath.SkipAll
			}
			taskCount++

			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				relPath = path
			}

			task := FileTask{
				Path:    path,
				Info:    info,
				RootDir: dir,
				RelPath: relPath,
			}

			allTasks = append(allTasks, task)
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking directory %s: %v", dir, err)
		}
	}

	// Determine if we should use parallel processing
	// Only parallelize if we have enough work to justify the overhead
	const minFilesForParallelization = 20
	if len(allTasks) < minFilesForParallelization {
		// Process sequentially for small workloads
		return processFilesSequentially(allTasks)
	}

	return processFilesInParallel(allTasks)
}

// processFilesSequentially handles small workloads without goroutine overhead
func processFilesSequentially(tasks []FileTask) (*FileSet, error) {
	fileSet := &FileSet{
		Files:   make([]*FileInfo, 0, len(tasks)),
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}

	for _, task := range tasks {
		hash, err := hashFile(task.Path)
		if err != nil {
			fmt.Printf("Warning: Could not hash file %s: %v\n", task.Path, err)
			continue
		}

		fileInfo := &FileInfo{
			RelativePath: task.RelPath,
			AbsolutePath: task.Path,
			Name:         task.Info.Name(),
			Hash:         hash,
			Size:         task.Info.Size(),
			RootDir:      task.RootDir,
		}

		fileSet.Files = append(fileSet.Files, fileInfo)
		fileSet.NameMap[fileInfo.Name] = append(fileSet.NameMap[fileInfo.Name], fileInfo)
		fileSet.HashMap[fileInfo.Hash] = append(fileSet.HashMap[fileInfo.Hash], fileInfo)
	}

	return fileSet, nil
}

// processFilesInParallel handles large workloads with optimal parallelization
func processFilesInParallel(tasks []FileTask) (*FileSet, error) {
	// Use 75% of CPU cores as requested
	numWorkers := int(float64(runtime.NumCPU()) * 0.75)
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Calculate optimal batch size based on total work and number of workers
	// Aim for at least 10 files per batch to justify goroutine overhead
	const minBatchSize = 10
	batchSize := len(tasks) / (numWorkers * 2) // Aim for 2 batches per worker
	if batchSize < minBatchSize {
		batchSize = minBatchSize
	}

	// Create work batches
	var jobs []FileJob
	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		jobs = append(jobs, FileJob{Files: tasks[i:end]})
	}

	// Create channels with appropriate buffer sizes
	jobChannel := make(chan FileJob, len(jobs))
	resultChannel := make(chan FileResult, len(jobs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go hashWorker(jobChannel, resultChannel, &wg)
	}

	// Send jobs to workers
	go func() {
		for _, job := range jobs {
			jobChannel <- job
		}
		close(jobChannel)
	}()

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChannel)
	}()

	// Collect results
	fileSet := &FileSet{
		Files:   make([]*FileInfo, 0, len(tasks)),
		NameMap: make(map[string][]*FileInfo),
		HashMap: make(map[string][]*FileInfo),
	}

	for result := range resultChannel {
		// Handle errors
		for _, err := range result.Errors {
			fmt.Printf("Warning: %v\n", err)
		}

		// Add successful results
		for _, fileInfo := range result.FileInfos {
			fileSet.Files = append(fileSet.Files, fileInfo)
			fileSet.NameMap[fileInfo.Name] = append(fileSet.NameMap[fileInfo.Name], fileInfo)
			fileSet.HashMap[fileInfo.Hash] = append(fileSet.HashMap[fileInfo.Hash], fileInfo)
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
func buildSmartTree(files []*FileInfo, sourceSet *FileSet, otherSet *FileSet) *TreeNode {
	root := &TreeNode{
		Name:     "",
		IsDir:    true,
		Children: make(map[string]*TreeNode),
	}

	// Build a map of directory paths to check which directories exist in sourceSet
	directoriesInSourceSet := make(map[string]bool)
	for _, file := range sourceSet.Files {
		dir := filepath.Dir(file.RelativePath)
		for dir != "." && dir != "" {
			directoriesInSourceSet[dir] = true
			dir = filepath.Dir(dir)
		}
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
	markEntireDirectoriesNew(root, sourceSet, otherSet, directoriesInSourceSet)

	// Remove empty directories
	removeEmptyDirectories(root)

	return root
}

// collectAllFilesUnderNode collects all files under a given tree node (including subdirectories)
func collectAllFilesUnderNode(node *TreeNode) []*FileInfo {
	var files []*FileInfo

	// Add files from this node
	files = append(files, node.Files...)

	// Recursively add files from all children
	for _, child := range node.Children {
		files = append(files, collectAllFilesUnderNode(child)...)
	}

	return files
}

// markEntireDirectoriesNew is the new implementation that properly handles partial matches
func markEntireDirectoriesNew(node *TreeNode, sourceSet *FileSet, otherSet *FileSet, directoriesInSourceSet map[string]bool) {
	if !node.IsDir {
		return
	}

	// Recursively process children first
	for _, child := range node.Children {
		markEntireDirectoriesNew(child, sourceSet, otherSet, directoriesInSourceSet)
	}

	// Skip root node
	if node.Name == "" {
		node.IsEntireDir = false
		return
	}

	// Build the full path for this directory
	var pathParts []string
	current := node
	for current != nil && current.Name != "" {
		pathParts = append([]string{current.Name}, pathParts...)
		current = current.Parent
	}
	dirPath := strings.Join(pathParts, string(filepath.Separator))

	// Check if this exact directory exists in the source set
	if !directoriesInSourceSet[dirPath] {
		// This directory doesn't exist in source set at all, so it can't be "entire"
		node.IsEntireDir = false
		return
	}

	// Count how many files from this directory in sourceSet have no match in otherSet
	filesInDirCount := 0
	filesWithoutMatchCount := 0

	for _, sourceFile := range sourceSet.Files {
		// Check if this file is directly in our directory (not in subdirectories)
		if filepath.Dir(sourceFile.RelativePath) == dirPath {
			filesInDirCount++
			// Check if its content exists in the other set
			if _, hashExists := otherSet.HashMap[sourceFile.Hash]; !hashExists {
				filesWithoutMatchCount++
			}
		}
	}

	// A directory can be marked as "entire" only if:
	// 1. ALL files directly in this directory (not subdirs) have no match in otherSet (or there are no direct files)
	// 2. ALL child directories are marked as entire (or there are no child directories)
	// 3. There is at least SOME content (files or subdirs) in this directory
	allDirectFilesUnmatched := filesInDirCount == 0 || (filesInDirCount > 0 && filesInDirCount == filesWithoutMatchCount)

	allChildrenAreEntire := true
	hasChildDirs := false
	for _, child := range node.Children {
		if child.IsDir {
			hasChildDirs = true
			if !child.IsEntireDir {
				allChildrenAreEntire = false
				break
			}
		}
	}

	// Directory must have some content (either files or subdirectories)
	hasContent := filesInDirCount > 0 || hasChildDirs

	if hasContent && allDirectFilesUnmatched && (!hasChildDirs || allChildrenAreEntire) {
		node.IsEntireDir = true
	} else {
		node.IsEntireDir = false
	}
}

// markEntireDirectories marks directories where all contents are missing
func markEntireDirectories(node *TreeNode, sourceSet *FileSet, otherSet *FileSet) {
	if !node.IsDir {
		return
	}

	// Recursively process children first
	for _, child := range node.Children {
		markEntireDirectories(child, sourceSet, otherSet)
	}

	// Skip root node
	if node.Name == "" {
		node.IsEntireDir = false
		return
	}

	// A directory can be marked as "entire" only if:
	// 1. It has no child directories, OR all child directories are marked as "entire"
	// 2. It has files (either directly or in subdirectories)
	// 3. This is a directory that's actually being shown in our tree (not just a parent of shown files)

	// Check if all children (if any) are marked as entire
	allChildrenAreEntire := true
	hasChildren := len(node.Children) > 0

	for _, child := range node.Children {
		if child.IsDir && !child.IsEntireDir {
			allChildrenAreEntire = false
			break
		}
	}

	// A leaf directory (no subdirectories) with files
	if !hasChildren && len(node.Files) > 0 {
		node.IsEntireDir = true
	} else if hasChildren && allChildrenAreEntire {
		// A directory where ALL subdirectories are marked as entire
		node.IsEntireDir = true
	} else {
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

// getOSSpecificExamples returns example paths and descriptions based on the current OS
func getOSSpecificExamples() (string, string, string, string) {
	if runtime.GOOS == "windows" {
		return "C:\\Photos\\2023", "D:\\Backup\\Photos",
			"C:\\Photos\\2023,C:\\Photos\\2024", "D:\\Backup\\Photos"
	}
	return "/home/user/photos/2023", "/home/user/backup/photos",
		"/home/user/photos/2023,/home/user/photos/2024", "/home/user/backup/photos"
}

// readUserInput reads a line of input from the user with a prompt
func readUserInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

// readYesNo reads a yes/no response from the user
func readYesNo(prompt string) bool {
	for {
		response := strings.ToLower(readUserInput(prompt))
		if response == "y" || response == "yes" {
			return true
		}
		if response == "n" || response == "no" {
			return false
		}
		fmt.Println("Please enter 'y' or 'n'")
	}
}

// runInteractiveMode runs the tool in interactive mode when no arguments are provided
func runInteractiveMode(execName string) ([]string, []string, bool, bool, bool, bool) {
	fmt.Println("Directory Comparison Tool - Interactive Mode")
	fmt.Println("=============================================")
	fmt.Println()
	fmt.Println("No arguments provided. Starting interactive mode...")
	fmt.Println()

	example1, example2, multiExample1, _ := getOSSpecificExamples()

	fmt.Printf("Example: If you want to compare your photos folder with your backup:\n")
	fmt.Printf("- Set 1 could be: %s\n", example1)
	fmt.Printf("- Set 2 could be: %s\n", example2)
	fmt.Println()

	set1Input := readUserInput(fmt.Sprintf("Enter Set 1 directories (comma-separated if multiple):\nExample: %s or %s\n> ", example1, multiExample1))
	for set1Input == "" {
		fmt.Println("Set 1 directories cannot be empty.")
		set1Input = readUserInput("> ")
	}

	set2Input := readUserInput(fmt.Sprintf("Enter Set 2 directories (comma-separated if multiple):\nExample: %s\n> ", example2))
	for set2Input == "" {
		fmt.Println("Set 2 directories cannot be empty.")
		set2Input = readUserInput("> ")
	}

	fmt.Println()
	showModified := readYesNo("Show files that were modified (same name, different content)? (y/n): ")
	showUniqueToSet2 := readYesNo("Show files unique to Set 2 (files in Set 2 not in Set 1)? (y/n): ")
	showUniqueToSet1 := readYesNo("Show files unique to Set 1 (files in Set 1 not in Set 2)? (y/n): ")
	showDetails := readYesNo("Show file size details? (y/n): ")

	set1Dirs := strings.Split(set1Input, ",")
	set2Dirs := strings.Split(set2Input, ",")

	// Clean up directory paths
	for i := range set1Dirs {
		set1Dirs[i] = strings.TrimSpace(set1Dirs[i])
	}
	for i := range set2Dirs {
		set2Dirs[i] = strings.TrimSpace(set2Dirs[i])
	}

	// Show preview
	fmt.Println()
	fmt.Println("📋 Let's show you a quick preview with the first 10 files...")
	fmt.Println()
	runPreview(set1Dirs, set2Dirs, 10, showDetails, showModified, showUniqueToSet1, showUniqueToSet2)

	fmt.Println()
	if !readYesNo("Continue with full scan? (y/n): ") {
		fmt.Println("Exiting...")
		os.Exit(0)
	}

	fmt.Println()
	return set1Dirs, set2Dirs, showModified, showUniqueToSet2, showUniqueToSet1, showDetails
}

func main() {
	execName := filepath.Base(os.Args[0])

	var set1Dirs, set2Dirs []string
	var showDetails, showUniqueToSet1, showModified, showUniqueToSet2 bool

	if len(os.Args) < 3 {
		// Interactive mode or show help
		if len(os.Args) == 1 {
			// No arguments - start interactive mode
			set1Dirs, set2Dirs, showModified, showUniqueToSet2, showUniqueToSet1, showDetails = runInteractiveMode(execName)
		} else {
			// Not enough arguments - show help
			example1, example2, multiExample1, multiExample2 := getOSSpecificExamples()

			fmt.Println("Directory Comparison Tool")
			fmt.Println("=========================")
			fmt.Println()
			fmt.Printf("Usage: %s <set1_dirs> <set2_dirs> [options]\n", execName)
			fmt.Println()
			fmt.Println("Arguments:")
			fmt.Println("  set1_dirs    Comma-separated list of directories in the first set")
			fmt.Println("  set2_dirs    Comma-separated list of directories in the second set")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --details         Show file sizes and additional details")
			fmt.Println("  --show-modified   Show files with same name but different content")
			fmt.Println("  --show-unique-2   Show files unique to set 2")
			fmt.Println("  --show-unique-1   Show files unique to set 1")
			fmt.Println("  --preview         Show preview with first 10 files")
			fmt.Println("  --preview-count N Set number of files to process in preview mode")
			fmt.Println()
			fmt.Println("Example:")
			fmt.Printf("  %s %s %s\n", execName, multiExample1, multiExample2)
			fmt.Printf("  %s %s %s --details --show-unique-1\n", execName, example1, example2)
			fmt.Println()
			fmt.Println("Or run without arguments for interactive mode:")
			fmt.Printf("  %s\n", execName)
			os.Exit(1)
		}
	} else {
		// Command line mode
		set1Dirs = strings.Split(os.Args[1], ",")
		set2Dirs = strings.Split(os.Args[2], ",")

		// Parse flags
		var isPreview bool
		var previewCount int = 10 // default preview count
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--details":
				showDetails = true
			case "--show-unique-1":
				showUniqueToSet1 = true
			case "--show-unique-2":
				showUniqueToSet2 = true
			case "--show-modified":
				showModified = true
			case "--preview":
				isPreview = true
			case "--preview-count":
				if i+1 < len(os.Args) {
					if count, err := strconv.Atoi(os.Args[i+1]); err != nil || count < 1 {
						fmt.Printf("Invalid preview count: %s. Using default of 10.\n", os.Args[i+1])
						previewCount = 10
					} else {
						previewCount = count
					}
					i++ // skip next argument
				}
				isPreview = true
			}
		}

		// If preview mode, run preview and exit
		if isPreview {
			runPreview(set1Dirs, set2Dirs, previewCount, showDetails, showModified, showUniqueToSet1, showUniqueToSet2)
			return
		}

		// Clean up directory paths
		for i := range set1Dirs {
			set1Dirs[i] = strings.TrimSpace(set1Dirs[i])
		}
		for i := range set2Dirs {
			set2Dirs[i] = strings.TrimSpace(set2Dirs[i])
		}
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

	// First tree: Files with same name but different content (optional)
	if showModified {
		if len(result.SameNameDifferentHash) > 0 {
			fmt.Printf("⚠️  Files with same name but different content (%d files) - Set 2 (%s) → Set 1 (%s):\n", len(result.SameNameDifferentHash), strings.Join(set2Dirs, ", "), strings.Join(set1Dirs, ", "))
			fmt.Println("=" + strings.Repeat("=", 50))
			fmt.Println()

			tree1 := buildTree(result.SameNameDifferentHash)
			printTree(tree1, "", true, showDetails, result.NameMappings)
			fmt.Println()
		} else {
			fmt.Println("✅ No files found with same name but different content.")
			fmt.Println()
		}
	}

	// Second tree: Files unique to set 2 (optional)
	if showUniqueToSet2 {
		if len(result.UniqueToSet2) > 0 {
			fmt.Printf("📋 Files unique to Set 2 (%s) - not found in Set 1 (%s) (%d files):\n", strings.Join(set2Dirs, ", "), strings.Join(set1Dirs, ", "), len(result.UniqueToSet2))
			fmt.Println("=" + strings.Repeat("=", 50))
			fmt.Println()

			tree2 := buildSmartTree(result.UniqueToSet2, set2, set1)
			printTree(tree2, "", true, showDetails, nil)
			fmt.Println()
		} else {
			fmt.Println("✅ No unique files found in Set 2.")
			fmt.Println()
		}
	}

	// Third tree: Files unique to set 1 (optional)
	if showUniqueToSet1 {
		if len(result.UniqueToSet1) > 0 {
			fmt.Printf("📋 Files unique to Set 1 (%s) - not found in Set 2 (%s) (%d files):\n", strings.Join(set1Dirs, ", "), strings.Join(set2Dirs, ", "), len(result.UniqueToSet1))
			fmt.Println("=" + strings.Repeat("=", 50))
			fmt.Println()

			tree3 := buildSmartTree(result.UniqueToSet1, set1, set2)
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
	if showModified {
		fmt.Printf("   • Same name, different content: %d\n", len(result.SameNameDifferentHash))
	}
	if showUniqueToSet2 {
		fmt.Printf("   • Unique to Set 2: %d\n", len(result.UniqueToSet2))
	}
	if showUniqueToSet1 {
		fmt.Printf("   • Unique to Set 1: %d\n", len(result.UniqueToSet1))
	}

	// Calculate sizes for different categories
	var sameNameSize, uniqueSet2Size, uniqueSet1Size int64

	if showModified {
		for _, file := range result.SameNameDifferentHash {
			sameNameSize += file.Size
		}
	}
	if showUniqueToSet2 {
		for _, file := range result.UniqueToSet2 {
			uniqueSet2Size += file.Size
		}
	}
	if showUniqueToSet1 {
		for _, file := range result.UniqueToSet1 {
			uniqueSet1Size += file.Size
		}
	}

	if (showModified && sameNameSize > 0) || (showUniqueToSet2 && uniqueSet2Size > 0) || (showUniqueToSet1 && uniqueSet1Size > 0) {
		fmt.Println("   • Total sizes:")
		if showModified && sameNameSize > 0 {
			fmt.Printf("     - Same name, different content: %s\n", formatSize(sameNameSize))
		}
		if showUniqueToSet2 && uniqueSet2Size > 0 {
			fmt.Printf("     - Unique to Set 2: %s\n", formatSize(uniqueSet2Size))
		}
		if showUniqueToSet1 && uniqueSet1Size > 0 {
			fmt.Printf("     - Unique to Set 1: %s\n", formatSize(uniqueSet1Size))
		}
	}

	// On Windows, wait for user input before closing
	if runtime.GOOS == "windows" {
		fmt.Println()
		fmt.Print("Press Enter to exit...")
		bufio.NewScanner(os.Stdin).Scan()
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

// runPreview runs the tool in preview mode with limited file processing
func runPreview(set1Dirs, set2Dirs []string, previewCount int, showDetails, showModified, showUniqueToSet1, showUniqueToSet2 bool) {
	fmt.Println("⚡ Directory Comparison Tool - PREVIEW MODE")
	fmt.Println("=" + strings.Repeat("=", 45))
	fmt.Printf("📋 Processing first %d files as sample\n", previewCount)
	fmt.Println()

	fmt.Printf("📂 Set 1 directories: %s\n", strings.Join(set1Dirs, ", "))
	fmt.Printf("📂 Set 2 directories: %s\n", strings.Join(set2Dirs, ", "))
	fmt.Println()

	fmt.Println("🔍 Analyzing first files in set 1...")
	set1, err := walkDirectoriesWithLimit(set1Dirs, previewCount)
	if err != nil {
		fmt.Printf("❌ Error analyzing first set: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Processed %d files\n", len(set1.Files))

	fmt.Println("🔍 Analyzing first files in set 2...")
	set2, err := walkDirectoriesWithLimit(set2Dirs, previewCount)
	if err != nil {
		fmt.Printf("❌ Error analyzing second set: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Processed %d files\n", len(set2.Files))

	fmt.Println("🔍 Comparing file sets...")
	result := compareFileSets(set1, set2)

	fmt.Println()
	fmt.Println("━━━ PREVIEW RESULTS ━━━")

	// Show results for enabled categories
	if showModified {
		if len(result.SameNameDifferentHash) > 0 {
			fmt.Printf("⚠️  Modified files found (%d in sample):\n", len(result.SameNameDifferentHash))
			fmt.Println("─" + strings.Repeat("─", 30))
			tree1 := buildTree(result.SameNameDifferentHash)
			printTree(tree1, "", true, showDetails, result.NameMappings)
			fmt.Println()
		} else {
			fmt.Println("✅ No modified files found in this sample.")
			fmt.Println()
		}
	}

	if showUniqueToSet2 {
		if len(result.UniqueToSet2) > 0 {
			fmt.Printf("📋 Files unique to Set 2 (%d in sample):\n", len(result.UniqueToSet2))
			fmt.Println("─" + strings.Repeat("─", 30))
			tree2 := buildTree(result.UniqueToSet2)
			printTree(tree2, "", true, showDetails, nil)
			fmt.Println()
		} else {
			fmt.Println("✅ No files unique to Set 2 found in this sample.")
			fmt.Println()
		}
	}

	if showUniqueToSet1 {
		if len(result.UniqueToSet1) > 0 {
			fmt.Printf("📋 Files unique to Set 1 (%d in sample):\n", len(result.UniqueToSet1))
			fmt.Println("─" + strings.Repeat("─", 30))
			tree3 := buildTree(result.UniqueToSet1)
			printTree(tree3, "", true, showDetails, nil)
			fmt.Println()
		} else {
			fmt.Println("✅ No files unique to Set 1 found in this sample.")
			fmt.Println()
		}
	}

	// Summary and next steps
	fmt.Println("📊 Preview Summary:")
	fmt.Printf("   • Sample size: %d files from each directory set\n", previewCount)
	fmt.Printf("   • Files processed from Set 1: %d\n", len(set1.Files))
	fmt.Printf("   • Files processed from Set 2: %d\n", len(set2.Files))
	if showModified {
		fmt.Printf("   • Modified files in sample: %d\n", len(result.SameNameDifferentHash))
	}
	if showUniqueToSet2 {
		fmt.Printf("   • Unique to Set 2 in sample: %d\n", len(result.UniqueToSet2))
	}
	if showUniqueToSet1 {
		fmt.Printf("   • Unique to Set 1 in sample: %d\n", len(result.UniqueToSet1))
	}
	fmt.Println()
	fmt.Println("💡 To see complete results, run the same command without --preview")
}
