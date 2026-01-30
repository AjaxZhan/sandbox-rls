package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDeltaLayer(t *testing.T) {
	// Create temp directories
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Test successful creation
	delta, err := NewDeltaLayer(deltaDir, sourceDir)
	if err != nil {
		t.Fatalf("NewDeltaLayer failed: %v", err)
	}
	if delta == nil {
		t.Fatal("Expected non-nil DeltaLayer")
	}
	if delta.DeltaDir() != deltaDir {
		t.Errorf("DeltaDir mismatch: got %s, want %s", delta.DeltaDir(), deltaDir)
	}
	if delta.SourceDir() != sourceDir {
		t.Errorf("SourceDir mismatch: got %s, want %s", delta.SourceDir(), sourceDir)
	}

	// Test missing deltaDir
	_, err = NewDeltaLayer("", sourceDir)
	if err == nil {
		t.Error("Expected error for empty deltaDir")
	}

	// Test missing sourceDir
	_, err = NewDeltaLayer(deltaDir, "")
	if err == nil {
		t.Error("Expected error for empty sourceDir")
	}
}

func TestDeltaLayer_ResolvePath(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create a file in source
	sourceFile := filepath.Join(sourceDir, "source.txt")
	if err := os.WriteFile(sourceFile, []byte("source content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file in delta
	deltaFile := filepath.Join(deltaDir, "delta.txt")
	if err := os.WriteFile(deltaFile, []byte("delta content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file that exists in both (delta should win)
	bothSourceFile := filepath.Join(sourceDir, "both.txt")
	if err := os.WriteFile(bothSourceFile, []byte("source version"), 0644); err != nil {
		t.Fatal(err)
	}
	bothDeltaFile := filepath.Join(deltaDir, "both.txt")
	if err := os.WriteFile(bothDeltaFile, []byte("delta version"), 0644); err != nil {
		t.Fatal(err)
	}

	delta, err := NewDeltaLayer(deltaDir, sourceDir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		relPath        string
		expectedPath   string
		expectedDelta  bool
	}{
		{
			name:          "file only in source",
			relPath:       "source.txt",
			expectedPath:  sourceFile,
			expectedDelta: false,
		},
		{
			name:          "file only in delta",
			relPath:       "delta.txt",
			expectedPath:  deltaFile,
			expectedDelta: true,
		},
		{
			name:          "file in both - delta wins",
			relPath:       "both.txt",
			expectedPath:  bothDeltaFile,
			expectedDelta: true,
		},
		{
			name:          "non-existent file",
			relPath:       "nonexistent.txt",
			expectedPath:  filepath.Join(sourceDir, "nonexistent.txt"),
			expectedDelta: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualPath, inDelta := delta.ResolvePath(tt.relPath)
			if actualPath != tt.expectedPath {
				t.Errorf("ResolvePath(%s) path = %s, want %s", tt.relPath, actualPath, tt.expectedPath)
			}
			if inDelta != tt.expectedDelta {
				t.Errorf("ResolvePath(%s) inDelta = %v, want %v", tt.relPath, inDelta, tt.expectedDelta)
			}
		})
	}
}

func TestDeltaLayer_HasDelta(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create file in delta
	if err := os.WriteFile(filepath.Join(deltaDir, "delta.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	if !delta.HasDelta("delta.txt") {
		t.Error("HasDelta should return true for file in delta")
	}
	if delta.HasDelta("nonexistent.txt") {
		t.Error("HasDelta should return false for non-existent file")
	}
}

func TestDeltaLayer_CopyToDelta(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create file in source
	content := []byte("original content")
	if err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create nested directory and file in source
	nestedDir := filepath.Join(sourceDir, "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Test COW for simple file
	if err := delta.CopyToDelta("file.txt"); err != nil {
		t.Fatalf("CopyToDelta failed: %v", err)
	}

	// Verify file was copied
	deltaPath := filepath.Join(deltaDir, "file.txt")
	copiedContent, err := os.ReadFile(deltaPath)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(copiedContent) != string(content) {
		t.Errorf("Copied content mismatch: got %s, want %s", copiedContent, content)
	}

	// Test COW for nested file
	if err := delta.CopyToDelta("subdir/nested.txt"); err != nil {
		t.Fatalf("CopyToDelta nested failed: %v", err)
	}

	nestedDeltaPath := filepath.Join(deltaDir, "subdir", "nested.txt")
	if _, err := os.Stat(nestedDeltaPath); os.IsNotExist(err) {
		t.Error("Nested file should have been copied to delta")
	}

	// Test COW for non-existent file (should be no-op)
	if err := delta.CopyToDelta("nonexistent.txt"); err != nil {
		t.Errorf("CopyToDelta for non-existent should not error: %v", err)
	}
}

func TestDeltaLayer_MarkDeleted(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create file in source
	if err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Mark as deleted
	if err := delta.MarkDeleted("file.txt"); err != nil {
		t.Fatalf("MarkDeleted failed: %v", err)
	}

	// Check whiteout exists
	whiteoutPath := filepath.Join(deltaDir, ".wh.file.txt")
	if _, err := os.Stat(whiteoutPath); os.IsNotExist(err) {
		t.Error("Whiteout marker should exist")
	}

	// Check IsDeleted
	if !delta.IsDeleted("file.txt") {
		t.Error("IsDeleted should return true")
	}

	// Test nested deletion
	if err := delta.MarkDeleted("subdir/nested.txt"); err != nil {
		t.Fatalf("MarkDeleted nested failed: %v", err)
	}
	if !delta.IsDeleted("subdir/nested.txt") {
		t.Error("Nested file should be marked as deleted")
	}
}

func TestDeltaLayer_RemoveWhiteout(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Mark as deleted
	delta.MarkDeleted("file.txt")

	// Verify deleted
	if !delta.IsDeleted("file.txt") {
		t.Fatal("File should be marked as deleted")
	}

	// Remove whiteout
	if err := delta.RemoveWhiteout("file.txt"); err != nil {
		t.Fatalf("RemoveWhiteout failed: %v", err)
	}

	// Verify not deleted anymore
	if delta.IsDeleted("file.txt") {
		t.Error("File should not be marked as deleted after RemoveWhiteout")
	}
}

func TestDeltaLayer_Sync(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create initial file in source
	sourceFile := filepath.Join(sourceDir, "existing.txt")
	if err := os.WriteFile(sourceFile, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create file to be deleted in source
	toDeleteFile := filepath.Join(sourceDir, "todelete.txt")
	if err := os.WriteFile(toDeleteFile, []byte("will be deleted"), 0644); err != nil {
		t.Fatal(err)
	}

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Create new file in delta
	newFile := filepath.Join(deltaDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modify existing file in delta (COW)
	modifiedFile := filepath.Join(deltaDir, "existing.txt")
	if err := os.WriteFile(modifiedFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mark file for deletion
	if err := delta.MarkDeleted("todelete.txt"); err != nil {
		t.Fatal(err)
	}

	// Create nested structure in delta
	nestedDir := filepath.Join(deltaDir, "newdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync
	if err := delta.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify new file was synced
	syncedNew, err := os.ReadFile(filepath.Join(sourceDir, "new.txt"))
	if err != nil {
		t.Errorf("New file should exist in source: %v", err)
	} else if string(syncedNew) != "new content" {
		t.Errorf("New file content mismatch: got %s", syncedNew)
	}

	// Verify modified file was synced
	syncedMod, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Errorf("Modified file should exist: %v", err)
	} else if string(syncedMod) != "modified" {
		t.Errorf("Modified file content mismatch: got %s, want 'modified'", syncedMod)
	}

	// Verify deleted file was removed
	if _, err := os.Stat(toDeleteFile); !os.IsNotExist(err) {
		t.Error("Deleted file should not exist in source after sync")
	}

	// Verify nested structure was synced
	nestedContent, err := os.ReadFile(filepath.Join(sourceDir, "newdir", "nested.txt"))
	if err != nil {
		t.Errorf("Nested file should exist: %v", err)
	} else if string(nestedContent) != "nested" {
		t.Errorf("Nested content mismatch: got %s", nestedContent)
	}
}

func TestDeltaLayer_Clear(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Create some files in delta
	os.WriteFile(filepath.Join(deltaDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(deltaDir, "file2.txt"), []byte("2"), 0644)
	os.MkdirAll(filepath.Join(deltaDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(deltaDir, "subdir", "nested.txt"), []byte("n"), 0644)

	// Clear
	if err := delta.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify delta is empty
	entries, err := os.ReadDir(deltaDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Delta should be empty after Clear, found %d entries", len(entries))
	}
}

func TestDeltaLayer_ListChanges(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Create modifications
	os.WriteFile(filepath.Join(deltaDir, "modified.txt"), []byte("m"), 0644)
	os.MkdirAll(filepath.Join(deltaDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(deltaDir, "subdir", "new.txt"), []byte("n"), 0644)
	delta.MarkDeleted("deleted.txt")

	changes, err := delta.ListChanges()
	if err != nil {
		t.Fatalf("ListChanges failed: %v", err)
	}

	// Check we have expected changes
	expectedChanges := map[string]bool{
		"M:modified.txt":     false,
		"M:subdir/new.txt":   false,
		"D:deleted.txt":      false,
	}

	for _, change := range changes {
		if _, ok := expectedChanges[change]; ok {
			expectedChanges[change] = true
		}
	}

	for change, found := range expectedChanges {
		if !found {
			t.Errorf("Expected change %s not found", change)
		}
	}
}

func TestDeltaLayer_MergedReadDir(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create files in source
	os.WriteFile(filepath.Join(sourceDir, "source.txt"), []byte("s"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "both.txt"), []byte("source"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "deleted.txt"), []byte("d"), 0644)

	// Create files in delta
	os.WriteFile(filepath.Join(deltaDir, "delta.txt"), []byte("d"), 0644)
	os.WriteFile(filepath.Join(deltaDir, "both.txt"), []byte("delta"), 0644)

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Mark file as deleted
	delta.MarkDeleted("deleted.txt")

	entries, err := delta.MergedReadDir("")
	if err != nil {
		t.Fatalf("MergedReadDir failed: %v", err)
	}

	// Collect entry names
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}

	// Check expected entries
	expectedEntries := []string{"source.txt", "delta.txt", "both.txt"}
	for _, expected := range expectedEntries {
		if !names[expected] {
			t.Errorf("Expected entry %s not found", expected)
		}
	}

	// Check deleted file is not present
	if names["deleted.txt"] {
		t.Error("Deleted file should not appear in merged listing")
	}

	// Check whiteout marker is not present
	if names[".wh.deleted.txt"] {
		t.Error("Whiteout marker should not appear in merged listing")
	}
}

func TestDeltaLayer_Whiteout(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()

	// Create file in source
	os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644)

	delta, _ := NewDeltaLayer(deltaDir, sourceDir)

	// Initially, file should resolve to source
	path, _ := delta.ResolvePath("file.txt")
	if path != filepath.Join(sourceDir, "file.txt") {
		t.Errorf("Before deletion, should resolve to source: got %s", path)
	}

	// Mark as deleted
	delta.MarkDeleted("file.txt")

	// After deletion, ResolvePath should return empty (file is deleted)
	path, _ = delta.ResolvePath("file.txt")
	if path != "" {
		t.Errorf("After deletion, ResolvePath should return empty: got %s", path)
	}

	// IsDeleted should return true
	if !delta.IsDeleted("file.txt") {
		t.Error("IsDeleted should return true for whiteout file")
	}
}

// TestSandboxFS_DeltaRename tests rename operations in delta mode
func TestSandboxFS_DeltaRename(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()
	mountPoint := t.TempDir()

	// Create files in source
	os.WriteFile(filepath.Join(sourceDir, "source_file.txt"), []byte("source content"), 0644)

	// Create SandboxFS with delta enabled
	config := &SandboxFSConfig{
		SourceDir:  sourceDir,
		DeltaDir:   deltaDir,
		MountPoint: mountPoint,
		Rules:      nil,
	}
	sfs, err := NewSandboxFS(config)
	if err != nil {
		t.Fatalf("NewSandboxFS failed: %v", err)
	}

	if !sfs.DeltaEnabled() {
		t.Fatal("Delta should be enabled")
	}

	// Test deltaRename: rename file from source to new name
	errno := sfs.deltaRename("source_file.txt", "renamed_file.txt")
	if errno != 0 {
		t.Fatalf("deltaRename failed with errno: %v", errno)
	}

	// Verify: old file should have whiteout (since it existed in source)
	if !sfs.delta.IsDeleted("source_file.txt") {
		t.Error("Old file should have whiteout marker after rename")
	}

	// Verify: new file should exist in delta
	newDeltaPath := filepath.Join(deltaDir, "renamed_file.txt")
	content, err := os.ReadFile(newDeltaPath)
	if err != nil {
		t.Errorf("New file should exist in delta: %v", err)
	} else if string(content) != "source content" {
		t.Errorf("Content mismatch: got %s", content)
	}

	// Verify: source file is unchanged
	sourceContent, err := os.ReadFile(filepath.Join(sourceDir, "source_file.txt"))
	if err != nil {
		t.Errorf("Source file should still exist: %v", err)
	} else if string(sourceContent) != "source content" {
		t.Error("Source file should be unchanged")
	}
}

// TestSandboxFS_DeltaRename_InDelta tests rename of a file already in delta
func TestSandboxFS_DeltaRename_InDelta(t *testing.T) {
	sourceDir := t.TempDir()
	deltaDir := t.TempDir()
	mountPoint := t.TempDir()

	// Create file directly in delta
	os.WriteFile(filepath.Join(deltaDir, "delta_file.txt"), []byte("delta content"), 0644)

	config := &SandboxFSConfig{
		SourceDir:  sourceDir,
		DeltaDir:   deltaDir,
		MountPoint: mountPoint,
		Rules:      nil,
	}
	sfs, err := NewSandboxFS(config)
	if err != nil {
		t.Fatalf("NewSandboxFS failed: %v", err)
	}

	// Rename file within delta
	errno := sfs.deltaRename("delta_file.txt", "renamed_delta.txt")
	if errno != 0 {
		t.Fatalf("deltaRename failed with errno: %v", errno)
	}

	// Old file should not exist in delta anymore
	oldPath := filepath.Join(deltaDir, "delta_file.txt")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file should not exist in delta after rename")
	}

	// New file should exist
	newPath := filepath.Join(deltaDir, "renamed_delta.txt")
	content, err := os.ReadFile(newPath)
	if err != nil {
		t.Errorf("New file should exist: %v", err)
	} else if string(content) != "delta content" {
		t.Errorf("Content mismatch: got %s", content)
	}

	// No whiteout should exist (file didn't exist in source)
	if sfs.delta.IsDeleted("delta_file.txt") {
		t.Error("Should not create whiteout for file that only existed in delta")
	}
}

// TestSandboxFS_DeltaEnabled tests delta layer configuration
func TestSandboxFS_DeltaEnabled(t *testing.T) {
	sourceDir := t.TempDir()
	mountPoint := t.TempDir()

	// Without delta dir
	config1 := &SandboxFSConfig{
		SourceDir:  sourceDir,
		MountPoint: mountPoint,
	}
	sfs1, _ := NewSandboxFS(config1)
	if sfs1.DeltaEnabled() {
		t.Error("Delta should be disabled without DeltaDir")
	}

	// With delta dir
	deltaDir := t.TempDir()
	config2 := &SandboxFSConfig{
		SourceDir:  sourceDir,
		DeltaDir:   deltaDir,
		MountPoint: mountPoint,
	}
	sfs2, _ := NewSandboxFS(config2)
	if !sfs2.DeltaEnabled() {
		t.Error("Delta should be enabled with DeltaDir")
	}
}
