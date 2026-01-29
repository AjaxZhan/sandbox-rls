package codebase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// TestStore tests Store interface implementations.
// Both MemoryStore and FileStore should pass all these tests.
func TestStore(t *testing.T) {
	// Test both implementations
	implementations := map[string]func(t *testing.T) Store{
		"MemoryStore": func(t *testing.T) Store {
			return NewMemoryStore()
		},
		"FileStore": func(t *testing.T) Store {
			tempDir := t.TempDir()
			store, err := NewFileStore(tempDir)
			if err != nil {
				t.Fatalf("failed to create FileStore: %v", err)
			}
			return store
		},
	}

	for name, createStore := range implementations {
		t.Run(name, func(t *testing.T) {
			t.Run("Create", func(t *testing.T) { testStoreCreate(t, createStore(t)) })
			t.Run("Get", func(t *testing.T) { testStoreGet(t, createStore(t)) })
			t.Run("List", func(t *testing.T) { testStoreList(t, createStore(t)) })
			t.Run("Update", func(t *testing.T) { testStoreUpdate(t, createStore(t)) })
			t.Run("Delete", func(t *testing.T) { testStoreDelete(t, createStore(t)) })
			t.Run("Exists", func(t *testing.T) { testStoreExists(t, createStore(t)) })
		})
	}
}

func testStoreCreate(t *testing.T, store Store) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		cb := &types.Codebase{
			ID:        "cb_001",
			Name:      "test-project",
			OwnerID:   "user_123",
			Size:      1024,
			FileCount: 10,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := store.Create(ctx, cb)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify it was stored
		got, err := store.Get(ctx, cb.ID)
		if err != nil {
			t.Fatalf("expected to find created codebase, got error: %v", err)
		}
		if got.ID != cb.ID {
			t.Errorf("expected ID %s, got %s", cb.ID, got.ID)
		}
		if got.Name != cb.Name {
			t.Errorf("expected Name %s, got %s", cb.Name, got.Name)
		}
	})

	t.Run("nil codebase", func(t *testing.T) {
		err := store.Create(ctx, nil)
		if err == nil {
			t.Error("expected error for nil codebase, got nil")
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		cb := &types.Codebase{
			ID:   "",
			Name: "no-id-project",
		}
		err := store.Create(ctx, cb)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("duplicate ID", func(t *testing.T) {
		cb := &types.Codebase{
			ID:        "cb_dup",
			Name:      "first",
			CreatedAt: time.Now(),
		}

		err := store.Create(ctx, cb)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}

		cb2 := &types.Codebase{
			ID:        "cb_dup",
			Name:      "second",
			CreatedAt: time.Now(),
		}

		err = store.Create(ctx, cb2)
		if err == nil {
			t.Error("expected error for duplicate ID, got nil")
		}
	})
}

func testStoreGet(t *testing.T, store Store) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		cb := &types.Codebase{
			ID:        "cb_get_001",
			Name:      "get-test",
			OwnerID:   "user_123",
			Size:      2048,
			FileCount: 5,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := store.Create(ctx, cb); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		got, err := store.Get(ctx, cb.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if got.ID != cb.ID {
			t.Errorf("expected ID %s, got %s", cb.ID, got.ID)
		}
		if got.Name != cb.Name {
			t.Errorf("expected Name %s, got %s", cb.Name, got.Name)
		}
		if got.OwnerID != cb.OwnerID {
			t.Errorf("expected OwnerID %s, got %s", cb.OwnerID, got.OwnerID)
		}
		if got.Size != cb.Size {
			t.Errorf("expected Size %d, got %d", cb.Size, got.Size)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.Get(ctx, "nonexistent")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		_, err := store.Get(ctx, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})
}

func testStoreList(t *testing.T, store Store) {
	ctx := context.Background()

	// Create test data
	codebases := []*types.Codebase{
		{ID: "cb_list_001", Name: "project-1", OwnerID: "user_A", CreatedAt: time.Now()},
		{ID: "cb_list_002", Name: "project-2", OwnerID: "user_A", CreatedAt: time.Now()},
		{ID: "cb_list_003", Name: "project-3", OwnerID: "user_B", CreatedAt: time.Now()},
		{ID: "cb_list_004", Name: "project-4", OwnerID: "user_B", CreatedAt: time.Now()},
		{ID: "cb_list_005", Name: "project-5", OwnerID: "user_A", CreatedAt: time.Now()},
	}

	for _, cb := range codebases {
		if err := store.Create(ctx, cb); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}

	t.Run("list all", func(t *testing.T) {
		result, err := store.List(ctx, "", 0, 0)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(result) != 5 {
			t.Errorf("expected 5 codebases, got %d", len(result))
		}
	})

	t.Run("list by owner", func(t *testing.T) {
		result, err := store.List(ctx, "user_A", 0, 0)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("expected 3 codebases for user_A, got %d", len(result))
		}

		for _, cb := range result {
			if cb.OwnerID != "user_A" {
				t.Errorf("expected OwnerID user_A, got %s", cb.OwnerID)
			}
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		result, err := store.List(ctx, "", 2, 0)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 codebases with limit, got %d", len(result))
		}
	})

	t.Run("list with offset", func(t *testing.T) {
		result, err := store.List(ctx, "", 2, 3)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 codebases with offset 3, got %d", len(result))
		}
	})

	t.Run("list with offset beyond data", func(t *testing.T) {
		result, err := store.List(ctx, "", 10, 100)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 codebases with offset beyond data, got %d", len(result))
		}
	})

	t.Run("list nonexistent owner", func(t *testing.T) {
		result, err := store.List(ctx, "nonexistent_user", 0, 0)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 codebases for nonexistent owner, got %d", len(result))
		}
	})
}

func testStoreUpdate(t *testing.T, store Store) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		cb := &types.Codebase{
			ID:        "cb_update_001",
			Name:      "original-name",
			OwnerID:   "user_123",
			Size:      1024,
			FileCount: 5,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := store.Create(ctx, cb); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Update
		cb.Name = "updated-name"
		cb.Size = 2048
		cb.FileCount = 10

		if err := store.Update(ctx, cb); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// Verify
		got, err := store.Get(ctx, cb.ID)
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}
		if got.Name != "updated-name" {
			t.Errorf("expected Name 'updated-name', got %s", got.Name)
		}
		if got.Size != 2048 {
			t.Errorf("expected Size 2048, got %d", got.Size)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cb := &types.Codebase{
			ID:   "nonexistent",
			Name: "test",
		}

		err := store.Update(ctx, cb)
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("nil codebase", func(t *testing.T) {
		err := store.Update(ctx, nil)
		if err == nil {
			t.Error("expected error for nil codebase, got nil")
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		cb := &types.Codebase{
			ID:   "",
			Name: "test",
		}
		err := store.Update(ctx, cb)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})
}

func testStoreDelete(t *testing.T, store Store) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		cb := &types.Codebase{
			ID:        "cb_delete_001",
			Name:      "to-delete",
			CreatedAt: time.Now(),
		}

		if err := store.Create(ctx, cb); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Verify exists
		exists, _ := store.Exists(ctx, cb.ID)
		if !exists {
			t.Fatal("codebase should exist before delete")
		}

		// Delete
		if err := store.Delete(ctx, cb.ID); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify deleted
		exists, _ = store.Exists(ctx, cb.ID)
		if exists {
			t.Error("codebase should not exist after delete")
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := store.Delete(ctx, "nonexistent")
		if !errors.Is(err, types.ErrCodebaseNotFound) {
			t.Errorf("expected ErrCodebaseNotFound, got %v", err)
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		err := store.Delete(ctx, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})
}

func testStoreExists(t *testing.T, store Store) {
	ctx := context.Background()

	t.Run("exists", func(t *testing.T) {
		cb := &types.Codebase{
			ID:        "cb_exists_001",
			Name:      "exists-test",
			CreatedAt: time.Now(),
		}

		if err := store.Create(ctx, cb); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		exists, err := store.Exists(ctx, cb.ID)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected codebase to exist")
		}
	})

	t.Run("not exists", func(t *testing.T) {
		exists, err := store.Exists(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected codebase to not exist")
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		_, err := store.Exists(ctx, "")
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})
}

// TestNewFileStore tests FileStore constructor.
func TestNewFileStore(t *testing.T) {
	t.Run("creates metadata directory", func(t *testing.T) {
		tempDir := t.TempDir()
		basePath := filepath.Join(tempDir, "codebases")

		store, err := NewFileStore(basePath)
		if err != nil {
			t.Fatalf("NewFileStore failed: %v", err)
		}
		if store == nil {
			t.Fatal("expected non-nil store")
		}

		metaDir := filepath.Join(basePath, "metadata")
		if _, err := os.Stat(metaDir); os.IsNotExist(err) {
			t.Error("metadata directory should be created")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := NewFileStore("")
		if err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})
}

// TestMemoryStoreConcurrency tests concurrent access to MemoryStore.
func TestMemoryStoreConcurrency(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create initial codebases
	for i := 0; i < 100; i++ {
		cb := &types.Codebase{
			ID:        idForConcurrency(i),
			Name:      "concurrent-test",
			CreatedAt: time.Now(),
		}
		if err := store.Create(ctx, cb); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(worker int) {
			for j := 0; j < 100; j++ {
				idx := (worker*100 + j) % 100
				id := idForConcurrency(idx)

				// Read
				_, _ = store.Get(ctx, id)

				// List
				_, _ = store.List(ctx, "", 10, 0)

				// Check exists
				_, _ = store.Exists(ctx, id)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func idForConcurrency(i int) string {
	return "cb_conc_" + string(rune('0'+i/10)) + string(rune('0'+i%10))
}
