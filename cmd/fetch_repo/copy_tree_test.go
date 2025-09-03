package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTree(t *testing.T) {
	// Create a temporary source directory
	srcDir := t.TempDir()

	// Create a temporary destination directory
	destDir := t.TempDir()

	// Setup source directory structure
	setupTestTree(t, srcDir)

	// Test copying the tree
	err := copyTree(destDir, srcDir)
	if err != nil {
		t.Fatalf("copyTree failed: %v", err)
	}

	// Verify the copied tree
	verifyTestTree(t, srcDir, destDir)
}

func TestCopyTreeRegularFiles(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create test files with different content
	testFiles := map[string]string{
		"file1.txt":  "Hello, World!",
		"file2.go":   "package main\n\nfunc main() {}\n",
		"empty.txt":  "",
		"binary.bin": "\x00\x01\x02\x03\xFF\xFE",
	}

	for filename, content := range testFiles {
		err := os.WriteFile(filepath.Join(srcDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Copy the files
	err := copyTree(destDir, srcDir)
	if err != nil {
		t.Fatalf("copyTree failed: %v", err)
	}

	// Verify files were copied correctly
	for filename, expectedContent := range testFiles {
		destPath := filepath.Join(destDir, filename)
		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Errorf("Failed to read copied file %s: %v", filename, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content mismatch. Expected %q, got %q", filename, expectedContent, string(content))
		}
	}
}

func TestCopyTreeDirectories(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create nested directory structure
	dirs := []string{
		"dir1",
		"dir1/subdir1",
		"dir1/subdir2",
		"dir2",
		"dir2/nested/deep/path",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(srcDir, dir), 0755)
		if err != nil {
			t.Fatal(err)
		}
		// Add a file in each directory
		err = os.WriteFile(filepath.Join(srcDir, dir, "test.txt"), []byte("test content"), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Copy the tree
	err := copyTree(destDir, srcDir)
	if err != nil {
		t.Fatalf("copyTree failed: %v", err)
	}

	// Verify directories were copied
	for _, dir := range dirs {
		destPath := filepath.Join(destDir, dir)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			t.Errorf("Directory %s was not copied", dir)
		}
		// Verify file in directory
		filePath := filepath.Join(destPath, "test.txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File in directory %s was not copied", dir)
		}
	}
}

func TestCopyTreeSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create a regular file to link to
	targetFile := filepath.Join(srcDir, "target.txt")
	err := os.WriteFile(targetFile, []byte("target content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create absolute symlink
	absoluteLink := filepath.Join(srcDir, "absolute_link")
	err = os.Symlink(targetFile, absoluteLink)
	if err != nil {
		t.Fatal(err)
	}

	// Create relative symlink
	relativeLink := filepath.Join(srcDir, "relative_link")
	err = os.Symlink("target.txt", relativeLink)
	if err != nil {
		t.Fatal(err)
	}

	// Create directory and symlink from subdirectory
	subDir := filepath.Join(srcDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	relativeLinkFromSub := filepath.Join(subDir, "link_to_parent")
	err = os.Symlink("../target.txt", relativeLinkFromSub)
	if err != nil {
		t.Fatal(err)
	}

	// Copy the tree
	err = copyTree(destDir, srcDir)
	if err != nil {
		t.Fatalf("copyTree failed: %v", err)
	}

	// Verify symlinks were copied
	tests := []struct {
		name     string
		linkPath string
	}{
		{"absolute_link", filepath.Join(destDir, "absolute_link")},
		{"relative_link", filepath.Join(destDir, "relative_link")},
		{"relative_from_sub", filepath.Join(destDir, "subdir", "link_to_parent")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Check if symlink exists
			info, err := os.Lstat(test.linkPath)
			if err != nil {
				t.Fatalf("Symlink %s was not copied: %v", test.name, err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s is not a symlink", test.name)
			}

			// Check target - symlinks are copied as-is
			target, err := os.Readlink(test.linkPath)
			if err != nil {
				t.Fatalf("Failed to read symlink %s: %v", test.name, err)
			}

			// Verify the symlink target matches the expected pattern
			var expectedTarget string
			switch test.name {
			case "absolute_link":
				expectedTarget = targetFile // Should remain absolute to original source
			case "relative_link":
				expectedTarget = "target.txt" // Should remain relative
			case "relative_from_sub":
				expectedTarget = "../target.txt" // Should remain relative
			}

			if target != expectedTarget {
				t.Errorf("Symlink %s target mismatch. Expected %q, got %q", test.name, expectedTarget, target)
			}
		})
	}
}

func TestCopyTreeNonExistentSource(t *testing.T) {
	destDir := t.TempDir()

	err := copyTree(destDir, "/non/existent/path")
	if err == nil {
		t.Error("Expected error when copying from non-existent source")
	}
}

func TestCopyTreeEmptyDirectory(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := copyTree(destDir, srcDir)
	if err != nil {
		t.Fatalf("copyTree failed on empty directory: %v", err)
	}

	// Verify destination exists and is empty
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty destination directory, but found %d entries", len(entries))
	}
}

// Helper functions

func setupTestTree(t *testing.T, rootDir string) {
	// Create files
	files := map[string]string{
		"root.txt":             "root content",
		"subdir/sub.txt":       "sub content",
		"subdir/deep/file.txt": "deep content",
	}

	for path, content := range files {
		fullPath := filepath.Join(rootDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create symlinks if the system supports them
	target := filepath.Join(rootDir, "root.txt")
	symlink := filepath.Join(rootDir, "root_link.txt")
	if err := os.Symlink(target, symlink); err != nil {
		t.Logf("Skipping symlink creation (not supported): %v", err)
	}
}

func verifyTestTree(t *testing.T, srcDir, destDir string) {
	// Walk the source directory and verify each item exists in destination
	err := filepath.Walk(srcDir, func(srcPath string, srcInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		destPath := filepath.Join(destDir, rel)
		destInfo, err := os.Lstat(destPath)
		if err != nil {
			t.Errorf("File/directory %s was not copied: %v", rel, err)
			return nil
		}

		// Check type matches
		if srcInfo.Mode().IsDir() != destInfo.Mode().IsDir() {
			t.Errorf("Type mismatch for %s: src is dir=%v, dest is dir=%v",
				rel, srcInfo.Mode().IsDir(), destInfo.Mode().IsDir())
		}

		if srcInfo.Mode()&os.ModeSymlink != 0 {
			if destInfo.Mode()&os.ModeSymlink == 0 {
				t.Errorf("Symlink %s was not copied as symlink", rel)
			}
		} else if srcInfo.Mode().IsRegular() {
			// Verify file content
			srcContent, err := os.ReadFile(srcPath)
			if err != nil {
				t.Errorf("Failed to read source file %s: %v", rel, err)
				return nil
			}
			destContent, err := os.ReadFile(destPath)
			if err != nil {
				t.Errorf("Failed to read dest file %s: %v", rel, err)
				return nil
			}
			if string(srcContent) != string(destContent) {
				t.Errorf("Content mismatch for file %s", rel)
			}
		}

		return nil
	})

	if err != nil {
		t.Errorf("Error verifying test tree: %v", err)
	}
}
