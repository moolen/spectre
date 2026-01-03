package fileio

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
)

func TestFileReader(t *testing.T) {
	logger := logging.GetLogger("test")
	reader := NewFileReader(logger)

	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.json")
	testContent := `{"test": "content"}`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("reads existing file", func(t *testing.T) {
		rc, err := reader.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		defer rc.Close()

		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}

		if string(content) != testContent {
			t.Errorf("Expected content %q, got %q", testContent, string(content))
		}
	})

	t.Run("fails on empty path", func(t *testing.T) {
		_, err := reader.ReadFile("")
		if err == nil {
			t.Error("Expected error for empty path")
		}
		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("Expected 'cannot be empty' error, got: %v", err)
		}
	})

	t.Run("fails on non-existent file", func(t *testing.T) {
		_, err := reader.ReadFile(filepath.Join(tmpDir, "nonexistent.json"))
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Expected 'does not exist' error, got: %v", err)
		}
	})

	t.Run("fails on directory path", func(t *testing.T) {
		_, err := reader.ReadFile(tmpDir)
		if err == nil {
			t.Error("Expected error for directory path")
		}
		if !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("Expected 'is a directory' error, got: %v", err)
		}
	})
}

func TestDirectoryWalker(t *testing.T) {
	logger := logging.GetLogger("test")
	walker := NewDirectoryWalker(logger)

	tmpDir := t.TempDir()

	// Create test directory structure
	subDir1 := filepath.Join(tmpDir, "dir1")
	subDir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create test files
	files := map[string]string{
		filepath.Join(tmpDir, "file1.json"):   `{"event": 1}`,
		filepath.Join(subDir1, "file2.json"):  `{"event": 2}`,
		filepath.Join(subDir2, "file3.json"):  `{"event": 3}`,
		filepath.Join(tmpDir, "readme.txt"):   "should be ignored",
		filepath.Join(subDir1, "data.JSON"):   `{"event": 4}`, // uppercase extension
		filepath.Join(subDir1, "ignore.xml"):  "<xml></xml>",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	t.Run("finds all JSON files recursively", func(t *testing.T) {
		results, err := walker.WalkJSON(tmpDir)
		if err != nil {
			t.Fatalf("WalkJSON failed: %v", err)
		}

		// Should find 4 JSON files (file1, file2, file3, data.JSON)
		// Should ignore readme.txt and ignore.xml
		if len(results) != 4 {
			t.Errorf("Expected 4 JSON files, got %d", len(results))
		}

		// Verify results contain expected files
		foundPaths := make(map[string]bool)
		for _, result := range results {
			foundPaths[result.FilePath] = true
			if result.Size <= 0 {
				t.Errorf("Expected positive file size for %s, got %d", result.FilePath, result.Size)
			}
		}

		expectedFiles := []string{
			filepath.Join(tmpDir, "file1.json"),
			filepath.Join(subDir1, "file2.json"),
			filepath.Join(subDir2, "file3.json"),
			filepath.Join(subDir1, "data.JSON"),
		}

		for _, expectedPath := range expectedFiles {
			if !foundPaths[expectedPath] {
				t.Errorf("Expected to find %s, but it was not in results", expectedPath)
			}
		}
	})

	t.Run("fails on empty directory", func(t *testing.T) {
		emptyDir := filepath.Join(tmpDir, "empty")
		if err := os.MkdirAll(emptyDir, 0755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}

		_, err := walker.WalkJSON(emptyDir)
		if err == nil {
			t.Error("Expected error for empty directory")
		}
		if !strings.Contains(err.Error(), "no JSON files found") {
			t.Errorf("Expected 'no JSON files found' error, got: %v", err)
		}
	})

	t.Run("fails on empty path", func(t *testing.T) {
		_, err := walker.WalkJSON("")
		if err == nil {
			t.Error("Expected error for empty path")
		}
		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("Expected 'cannot be empty' error, got: %v", err)
		}
	})

	t.Run("fails on non-existent directory", func(t *testing.T) {
		_, err := walker.WalkJSON(filepath.Join(tmpDir, "nonexistent"))
		if err == nil {
			t.Error("Expected error for non-existent directory")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Expected 'does not exist' error, got: %v", err)
		}
	})

	t.Run("fails on file path instead of directory", func(t *testing.T) {
		_, err := walker.WalkJSON(filepath.Join(tmpDir, "file1.json"))
		if err == nil {
			t.Error("Expected error for file path")
		}
		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("Expected 'not a directory' error, got: %v", err)
		}
	})
}

func TestDetectPathType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		expectedType PathType
		expectError  bool
	}{
		{
			name:         "detects file",
			path:         testFile,
			expectedType: PathTypeFile,
			expectError:  false,
		},
		{
			name:         "detects directory",
			path:         subDir,
			expectedType: PathTypeDirectory,
			expectError:  false,
		},
		{
			name:         "fails on empty path",
			path:         "",
			expectedType: PathTypeUnknown,
			expectError:  true,
		},
		{
			name:         "fails on non-existent path",
			path:         filepath.Join(tmpDir, "nonexistent"),
			expectedType: PathTypeUnknown,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathType, err := DetectPathType(tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if pathType != tt.expectedType {
					t.Errorf("Expected path type %v, got %v", tt.expectedType, pathType)
				}
			}
		})
	}
}

func TestIsJSONFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.json", true},
		{"test.JSON", true},
		{"test.Json", true},
		{"path/to/file.json", true},
		{"test.txt", false},
		{"test.xml", false},
		{"test", false},
		{"", false},
		{".json", true},
		{"file.json.bak", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isJSONFile(tt.path)
			if result != tt.expected {
				t.Errorf("isJSONFile(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}
