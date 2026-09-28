package local

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalOperations(t *testing.T) {
	tempDir := t.TempDir()

	subDir := filepath.Join(tempDir, "test_folder")
	if err := Mkdir(subDir); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	testFile := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	items, err := ListDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to list directory: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Folder should come first
	if !items[0].IsDir || items[0].Name != "test_folder" {
		t.Errorf("expected folder first, got %+v", items[0])
	}
	if items[1].IsDir || items[1].Name != "sample.txt" {
		t.Errorf("expected file second, got %+v", items[1])
	}

	// Test Rename
	renamedFile := filepath.Join(tempDir, "renamed.txt")
	if err := Rename(testFile, renamedFile); err != nil {
		t.Fatalf("failed to rename: %v", err)
	}

	// Test Delete
	if err := Delete(renamedFile, false); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}
	if err := Delete(subDir, true); err != nil {
		t.Fatalf("failed to delete folder: %v", err)
	}

	remaining, _ := ListDirectory(tempDir)
	if len(remaining) != 0 {
		t.Errorf("expected empty directory, got %d items", len(remaining))
	}
}
