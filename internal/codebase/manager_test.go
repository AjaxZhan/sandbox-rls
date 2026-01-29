package codebase

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// TestManager tests the Manager implementation.
func TestManager(t *testing.T) {
	t.Run("CreateCodebase", testManagerCreateCodebase)
	t.Run("GetCodebase", testManagerGetCodebase)
	t.Run("ListCodebases", testManagerListCodebases)
	t.Run("DeleteCodebase", testManagerDeleteCodebase)
	t.Run("WriteFile", testManagerWriteFile)
	t.Run("ReadFile", testManagerReadFile)
	t.Run("DeleteFile", testManagerDeleteFile)
	t.Run("ListFiles", testManagerListFiles)
	t.Run("GetCodebasePath", testManagerGetCodebasePath)
}

func createTestManager(t *testing.T) *Manager {
	tempDir := t.TempDir()
	mgr, err := NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	return mgr
}

func testManagerCreateCodebase(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	t.Run("success", func(t *testing.T) {
		req := &types.CreateCodebaseRequest{
			Name:    "test-project",
			OwnerID: "user_123",
		}

		cb, err := mgr.CreateCodebase(ctx, req)
		if err != nil {
			t.Fatalf("CreateCodebase failed: %v", err)
		}

		if cb.ID == "" {
			t.Error("expected non-empty ID")
		}
		if cb.Name != req.Name {
			t.Errorf("expected Name %s, got %s", req.Name, cb.Name)
		}
		if cb.OwnerID != req.OwnerID {
			t.Errorf("expected OwnerID %s, got %s", req.OwnerID, cb.OwnerID)
		}
		if cb.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}

		// Verify codebase directory was created
		path, err := mgr.GetCodebasePath(ctx, cb.ID)
		if err != nil {
			t.Fatalf("GetCodebasePath failed: %v", err)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("codebase directory should exist")
		}
	})

	t.Run("nil request", func(t *testing.T) {
		_, err := mgr.CreateCodebase(ctx, nil)
		if err == nil {
			t.Error("expected error for nil request, got nil")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		req := &types.CreateCodebaseRequest{
			Name:    "",
			OwnerID: "user_123",
		}
		_, err := mgr.CreateCodebase(ctx, req)
		if err == nil {
			t.Error("expected error for empty name, got nil")
		}
	})

	t.Run("empty owner ID", func(t *testing.T) {
		req := &types.CreateCodebaseRequest{
			Name:    "test-project",
			OwnerID: "",
		}
		_, err := mgr.CreateCodebase(ctx, req)
		if err == nil {
			t.Error("expected error for empty owner ID, got nil")
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 10; i++ {
			req := &types.CreateCodebaseRequest{
				Name:    "test-project",
				OwnerID: "user_123",
			}
			cb, err := mgr.CreateCodebase(ctx, req)
			if err != nil {
				t.Fatalf("CreateCodebase failed: %v", err)
			}
			if ids[cb.ID] {
				t.Errorf("duplicate ID generated: %s", cb.ID)
			}
			ids[cb.ID] = true
		}
	})
}

func testManagerGetCodebase(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	t.Run("success", func(t *testing.T) {
		req := &types.CreateCodebaseRequest{
			Name:    "get-test",
			OwnerID: "user_123",
		}
		created, err := mgr.CreateCodebase(ctx, req)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		got, err := mgr.GetCodebase(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetCodebase failed: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("expected ID %s, got %s", created.ID, got.ID)
		}
		if got.Name != created.Name {
			t.Errorf("expected Name %s, got %s", created.Name, got.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := mgr.GetCodebase(ctx, "nonexistent")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		_, err := mgr.GetCodebase(ctx, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})
}

func testManagerListCodebases(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	// Create test data
	for i := 0; i < 5; i++ {
		owner := "user_A"
		if i >= 3 {
			owner = "user_B"
		}
		req := &types.CreateCodebaseRequest{
			Name:    "project-" + string(rune('1'+i)),
			OwnerID: owner,
		}
		if _, err := mgr.CreateCodebase(ctx, req); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}

	t.Run("list by owner", func(t *testing.T) {
		result, err := mgr.ListCodebases(ctx, "user_A", 10, 0)
		if err != nil {
			t.Fatalf("ListCodebases failed: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("expected 3 codebases for user_A, got %d", len(result))
		}
	})

	t.Run("list all", func(t *testing.T) {
		result, err := mgr.ListCodebases(ctx, "", 10, 0)
		if err != nil {
			t.Fatalf("ListCodebases failed: %v", err)
		}
		if len(result) != 5 {
			t.Errorf("expected 5 codebases, got %d", len(result))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		result, err := mgr.ListCodebases(ctx, "", 2, 2)
		if err != nil {
			t.Fatalf("ListCodebases failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 codebases with pagination, got %d", len(result))
		}
	})
}

func testManagerDeleteCodebase(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	t.Run("success", func(t *testing.T) {
		req := &types.CreateCodebaseRequest{
			Name:    "to-delete",
			OwnerID: "user_123",
		}
		created, err := mgr.CreateCodebase(ctx, req)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Get path before delete
		path, _ := mgr.GetCodebasePath(ctx, created.ID)

		// Add a file
		if err := mgr.WriteFile(ctx, created.ID, "test.txt", strings.NewReader("hello")); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Delete
		if err := mgr.DeleteCodebase(ctx, created.ID); err != nil {
			t.Fatalf("DeleteCodebase failed: %v", err)
		}

		// Verify metadata deleted
		_, err = mgr.GetCodebase(ctx, created.ID)
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Error("codebase should not exist after delete")
		}

		// Verify directory deleted
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("codebase directory should be deleted")
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := mgr.DeleteCodebase(ctx, "nonexistent")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})
}

func testManagerWriteFile(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	cb, err := mgr.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    "write-test",
		OwnerID: "user_123",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("success simple file", func(t *testing.T) {
		content := "Hello, World!"
		err := mgr.WriteFile(ctx, cb.ID, "hello.txt", strings.NewReader(content))
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file was written
		path, _ := mgr.GetCodebasePath(ctx, cb.ID)
		data, err := os.ReadFile(filepath.Join(path, "hello.txt"))
		if err != nil {
			t.Fatalf("file should exist: %v", err)
		}
		if string(data) != content {
			t.Errorf("expected content %q, got %q", content, string(data))
		}
	})

	t.Run("success nested path", func(t *testing.T) {
		content := "nested content"
		err := mgr.WriteFile(ctx, cb.ID, "dir1/dir2/nested.txt", strings.NewReader(content))
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		path, _ := mgr.GetCodebasePath(ctx, cb.ID)
		data, err := os.ReadFile(filepath.Join(path, "dir1/dir2/nested.txt"))
		if err != nil {
			t.Fatalf("nested file should exist: %v", err)
		}
		if string(data) != content {
			t.Errorf("expected content %q, got %q", content, string(data))
		}
	})

	t.Run("overwrite existing", func(t *testing.T) {
		// Write initial
		err := mgr.WriteFile(ctx, cb.ID, "overwrite.txt", strings.NewReader("old"))
		if err != nil {
			t.Fatalf("initial write failed: %v", err)
		}

		// Overwrite
		err = mgr.WriteFile(ctx, cb.ID, "overwrite.txt", strings.NewReader("new"))
		if err != nil {
			t.Fatalf("overwrite failed: %v", err)
		}

		path, _ := mgr.GetCodebasePath(ctx, cb.ID)
		data, _ := os.ReadFile(filepath.Join(path, "overwrite.txt"))
		if string(data) != "new" {
			t.Errorf("expected 'new', got %q", string(data))
		}
	})

	t.Run("updates size and file count", func(t *testing.T) {
		mgr2 := createTestManager(t)
		cb2, _ := mgr2.CreateCodebase(ctx, &types.CreateCodebaseRequest{
			Name: "size-test", OwnerID: "user_123",
		})

		// Initial state
		got, _ := mgr2.GetCodebase(ctx, cb2.ID)
		if got.Size != 0 || got.FileCount != 0 {
			t.Errorf("expected size=0 and fileCount=0, got size=%d fileCount=%d", got.Size, got.FileCount)
		}

		// Write a file
		mgr2.WriteFile(ctx, cb2.ID, "test.txt", strings.NewReader("12345"))

		// Check updated stats (need to wait for async update or call refresh)
		got, _ = mgr2.GetCodebase(ctx, cb2.ID)
		if got.Size != 5 {
			t.Errorf("expected size=5, got %d", got.Size)
		}
		if got.FileCount != 1 {
			t.Errorf("expected fileCount=1, got %d", got.FileCount)
		}
	})

	t.Run("codebase not found", func(t *testing.T) {
		err := mgr.WriteFile(ctx, "nonexistent", "test.txt", strings.NewReader("data"))
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		err := mgr.WriteFile(ctx, cb.ID, "../escape.txt", strings.NewReader("evil"))
		if err == nil {
			t.Error("expected error for path traversal, got nil")
		}

		err = mgr.WriteFile(ctx, cb.ID, "dir/../../escape.txt", strings.NewReader("evil"))
		if err == nil {
			t.Error("expected error for path traversal, got nil")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		err := mgr.WriteFile(ctx, cb.ID, "", strings.NewReader("data"))
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})
}

func testManagerReadFile(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	cb, err := mgr.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    "read-test",
		OwnerID: "user_123",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Write test files
	mgr.WriteFile(ctx, cb.ID, "test.txt", strings.NewReader("test content"))
	mgr.WriteFile(ctx, cb.ID, "dir/nested.txt", strings.NewReader("nested content"))

	t.Run("success", func(t *testing.T) {
		reader, err := mgr.ReadFile(ctx, cb.ID, "test.txt")
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if string(data) != "test content" {
			t.Errorf("expected 'test content', got %q", string(data))
		}
	})

	t.Run("nested file", func(t *testing.T) {
		reader, err := mgr.ReadFile(ctx, cb.ID, "dir/nested.txt")
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		defer reader.Close()

		data, _ := io.ReadAll(reader)
		if string(data) != "nested content" {
			t.Errorf("expected 'nested content', got %q", string(data))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := mgr.ReadFile(ctx, cb.ID, "nonexistent.txt")
		if err == nil {
			t.Error("expected error for nonexistent file, got nil")
		}
	})

	t.Run("codebase not found", func(t *testing.T) {
		_, err := mgr.ReadFile(ctx, "nonexistent", "test.txt")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		_, err := mgr.ReadFile(ctx, cb.ID, "../escape.txt")
		if err == nil {
			t.Error("expected error for path traversal, got nil")
		}
	})
}

func testManagerDeleteFile(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	cb, err := mgr.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    "delete-file-test",
		OwnerID: "user_123",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		mgr.WriteFile(ctx, cb.ID, "to-delete.txt", strings.NewReader("data"))

		err := mgr.DeleteFile(ctx, cb.ID, "to-delete.txt")
		if err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}

		// Verify deleted
		_, err = mgr.ReadFile(ctx, cb.ID, "to-delete.txt")
		if err == nil {
			t.Error("file should be deleted")
		}
	})

	t.Run("delete nested", func(t *testing.T) {
		mgr.WriteFile(ctx, cb.ID, "dir/nested.txt", strings.NewReader("data"))

		err := mgr.DeleteFile(ctx, cb.ID, "dir/nested.txt")
		if err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		err := mgr.DeleteFile(ctx, cb.ID, "nonexistent.txt")
		if err == nil {
			t.Error("expected error for nonexistent file, got nil")
		}
	})

	t.Run("codebase not found", func(t *testing.T) {
		err := mgr.DeleteFile(ctx, "nonexistent", "test.txt")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})
}

func testManagerListFiles(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	cb, err := mgr.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    "list-files-test",
		OwnerID: "user_123",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Create file structure:
	// - file1.txt
	// - file2.txt
	// - dir1/
	//   - nested1.txt
	//   - subdir/
	//     - deep.txt
	mgr.WriteFile(ctx, cb.ID, "file1.txt", strings.NewReader("content1"))
	mgr.WriteFile(ctx, cb.ID, "file2.txt", strings.NewReader("content2"))
	mgr.WriteFile(ctx, cb.ID, "dir1/nested1.txt", strings.NewReader("nested1"))
	mgr.WriteFile(ctx, cb.ID, "dir1/subdir/deep.txt", strings.NewReader("deep"))

	t.Run("list root non-recursive", func(t *testing.T) {
		files, err := mgr.ListFiles(ctx, cb.ID, "", false)
		if err != nil {
			t.Fatalf("ListFiles failed: %v", err)
		}

		// Should have 3 entries: file1.txt, file2.txt, dir1/
		if len(files) != 3 {
			t.Errorf("expected 3 entries, got %d", len(files))
		}

		// Check we have expected entries
		names := make(map[string]bool)
		for _, f := range files {
			names[f.Name] = true
		}
		if !names["file1.txt"] || !names["file2.txt"] || !names["dir1"] {
			t.Errorf("missing expected entries: %v", files)
		}
	})

	t.Run("list root recursive", func(t *testing.T) {
		files, err := mgr.ListFiles(ctx, cb.ID, "", true)
		if err != nil {
			t.Fatalf("ListFiles failed: %v", err)
		}

		// Should have all files (excluding directories): 4 files
		var fileCount int
		for _, f := range files {
			if !f.IsDir {
				fileCount++
			}
		}
		if fileCount != 4 {
			t.Errorf("expected 4 files recursively, got %d", fileCount)
		}
	})

	t.Run("list subdirectory", func(t *testing.T) {
		files, err := mgr.ListFiles(ctx, cb.ID, "dir1", false)
		if err != nil {
			t.Fatalf("ListFiles failed: %v", err)
		}

		// Should have: nested1.txt, subdir/
		if len(files) != 2 {
			t.Errorf("expected 2 entries, got %d", len(files))
		}
	})

	t.Run("codebase not found", func(t *testing.T) {
		_, err := mgr.ListFiles(ctx, "nonexistent", "", false)
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		_, err := mgr.ListFiles(ctx, cb.ID, "../", false)
		if err == nil {
			t.Error("expected error for path traversal, got nil")
		}
	})
}

func testManagerGetCodebasePath(t *testing.T) {
	ctx := context.Background()
	mgr := createTestManager(t)

	cb, err := mgr.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    "path-test",
		OwnerID: "user_123",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		path, err := mgr.GetCodebasePath(ctx, cb.ID)
		if err != nil {
			t.Fatalf("GetCodebasePath failed: %v", err)
		}
		if path == "" {
			t.Error("expected non-empty path")
		}

		// Verify path exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("path should exist")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := mgr.GetCodebasePath(ctx, "nonexistent")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})
}

// TestNewManager tests Manager constructor.
func TestNewManager(t *testing.T) {
	t.Run("creates directories", func(t *testing.T) {
		tempDir := t.TempDir()
		basePath := filepath.Join(tempDir, "new-base")

		mgr, err := NewManager(basePath)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}

		// Verify directories created
		filesDir := filepath.Join(basePath, "files")
		if _, err := os.Stat(filesDir); os.IsNotExist(err) {
			t.Error("files directory should be created")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := NewManager("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})
}

// TestManagerConcurrency tests concurrent operations.
func TestManagerConcurrency(t *testing.T) {
	mgr := createTestManager(t)
	ctx := context.Background()

	// Create a codebase for concurrent access
	cb, err := mgr.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    "concurrent-test",
		OwnerID: "user_123",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				filename := filepath.Join("worker", string(rune('0'+id)), string(rune('0'+j))+".txt")
				content := strings.Repeat("data", 100)
				mgr.WriteFile(ctx, cb.ID, filename, strings.NewReader(content))
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				mgr.GetCodebase(ctx, cb.ID)
				mgr.ListFiles(ctx, cb.ID, "", false)
				time.Sleep(time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}
}
