package codebase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// FileInfo represents information about a file or directory.
type FileInfo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

// Manager handles codebase operations including file management.
type Manager struct {
	store     Store
	basePath  string
	filesPath string
}

// NewManager creates a new codebase manager.
func NewManager(basePath string) (*Manager, error) {
	if basePath == "" {
		return nil, errors.New("base path cannot be empty")
	}

	filesPath := filepath.Join(basePath, "files")

	// Create directories
	if err := os.MkdirAll(filesPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create files directory: %w", err)
	}

	store, err := NewFileStore(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	return &Manager{
		store:     store,
		basePath:  basePath,
		filesPath: filesPath,
	}, nil
}

// NewManagerWithStore creates a manager with a custom store (useful for testing).
func NewManagerWithStore(basePath string, store Store) (*Manager, error) {
	if basePath == "" {
		return nil, errors.New("base path cannot be empty")
	}

	filesPath := filepath.Join(basePath, "files")

	if err := os.MkdirAll(filesPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create files directory: %w", err)
	}

	return &Manager{
		store:     store,
		basePath:  basePath,
		filesPath: filesPath,
	}, nil
}

// generateID generates a unique codebase ID.
func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "cb_" + hex.EncodeToString(bytes)
}

// CreateCodebase creates a new codebase.
func (m *Manager) CreateCodebase(ctx context.Context, req *types.CreateCodebaseRequest) (*types.Codebase, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}
	if req.Name == "" {
		return nil, errors.New("name cannot be empty")
	}
	if req.OwnerID == "" {
		return nil, errors.New("owner ID cannot be empty")
	}

	id := generateID()
	now := time.Now()

	// Create codebase directory
	cbPath := filepath.Join(m.filesPath, id)
	if err := os.MkdirAll(cbPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create codebase directory: %w", err)
	}

	codebase := &types.Codebase{
		ID:        id,
		Name:      req.Name,
		Path:      cbPath,
		OwnerID:   req.OwnerID,
		Size:      0,
		FileCount: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := m.store.Create(ctx, codebase); err != nil {
		// Cleanup directory on failure
		os.RemoveAll(cbPath)
		return nil, fmt.Errorf("failed to store codebase metadata: %w", err)
	}

	return codebase, nil
}

// GetCodebase retrieves a codebase by ID.
func (m *Manager) GetCodebase(ctx context.Context, id string) (*types.Codebase, error) {
	if id == "" {
		return nil, errors.New("codebase ID cannot be empty")
	}
	return m.store.Get(ctx, id)
}

// ListCodebases lists codebases for an owner.
func (m *Manager) ListCodebases(ctx context.Context, ownerID string, limit, offset int) ([]*types.Codebase, error) {
	return m.store.List(ctx, ownerID, limit, offset)
}

// DeleteCodebase deletes a codebase and all its files.
func (m *Manager) DeleteCodebase(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("codebase ID cannot be empty")
	}

	// Get codebase to find its path
	cb, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}

	// Delete from store first
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}

	// Then delete files
	cbPath := filepath.Join(m.filesPath, id)
	if cb.Path != "" {
		cbPath = cb.Path
	}
	if err := os.RemoveAll(cbPath); err != nil {
		// Log but don't fail - metadata is already deleted
		// In production, we'd want to handle this more gracefully
	}

	return nil
}

// GetCodebasePath returns the file system path for a codebase.
func (m *Manager) GetCodebasePath(ctx context.Context, id string) (string, error) {
	cb, err := m.store.Get(ctx, id)
	if err != nil {
		return "", err
	}

	// Use stored path or construct from ID
	path := cb.Path
	if path == "" {
		path = filepath.Join(m.filesPath, id)
	}

	return path, nil
}

// validatePath checks if the path is safe (no path traversal).
func (m *Manager) validatePath(codebasePath, filePath string) (string, error) {
	if filePath == "" {
		return "", errors.New("file path cannot be empty")
	}

	// Clean the path
	cleanPath := filepath.Clean(filePath)

	// Check for path traversal
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "../") {
		return "", errors.New("path traversal not allowed")
	}

	// Construct full path
	fullPath := filepath.Join(codebasePath, cleanPath)

	// Verify the path is still within the codebase directory
	if !strings.HasPrefix(fullPath, codebasePath) {
		return "", errors.New("path traversal not allowed")
	}

	return fullPath, nil
}

// WriteFile writes content to a file in the codebase.
func (m *Manager) WriteFile(ctx context.Context, codebaseID, filePath string, content io.Reader) error {
	cbPath, err := m.GetCodebasePath(ctx, codebaseID)
	if err != nil {
		return err
	}

	fullPath, err := m.validatePath(cbPath, filePath)
	if err != nil {
		return err
	}

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create/overwrite file
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update codebase stats
	if err := m.updateCodebaseStats(ctx, codebaseID); err != nil {
		// Log but don't fail the write
	}

	return nil
}

// ReadFile opens a file for reading from the codebase.
func (m *Manager) ReadFile(ctx context.Context, codebaseID, filePath string) (io.ReadCloser, error) {
	cbPath, err := m.GetCodebasePath(ctx, codebaseID)
	if err != nil {
		return nil, err
	}

	fullPath, err := m.validatePath(cbPath, filePath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

// DeleteFile removes a file from the codebase.
func (m *Manager) DeleteFile(ctx context.Context, codebaseID, filePath string) error {
	cbPath, err := m.GetCodebasePath(ctx, codebaseID)
	if err != nil {
		return err
	}

	fullPath, err := m.validatePath(cbPath, filePath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Update codebase stats
	if err := m.updateCodebaseStats(ctx, codebaseID); err != nil {
		// Log but don't fail the delete
	}

	return nil
}

// ListFiles lists files in a codebase directory.
func (m *Manager) ListFiles(ctx context.Context, codebaseID, path string, recursive bool) ([]FileInfo, error) {
	cbPath, err := m.GetCodebasePath(ctx, codebaseID)
	if err != nil {
		return nil, err
	}

	var targetPath string
	if path == "" {
		targetPath = cbPath
	} else {
		targetPath, err = m.validatePath(cbPath, path)
		if err != nil {
			return nil, err
		}
	}

	var files []FileInfo

	if recursive {
		err = filepath.Walk(targetPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if p == targetPath {
				return nil // Skip root
			}

			relPath, _ := filepath.Rel(cbPath, p)
			files = append(files, FileInfo{
				Path:       relPath,
				Name:       info.Name(),
				IsDir:      info.IsDir(),
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
			return nil
		})
	} else {
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return []FileInfo{}, nil
			}
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			relPath := entry.Name()
			if path != "" {
				relPath = filepath.Join(path, entry.Name())
			}

			files = append(files, FileInfo{
				Path:       relPath,
				Name:       entry.Name(),
				IsDir:      entry.IsDir(),
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// updateCodebaseStats recalculates and updates codebase size and file count.
func (m *Manager) updateCodebaseStats(ctx context.Context, codebaseID string) error {
	cb, err := m.store.Get(ctx, codebaseID)
	if err != nil {
		return err
	}

	cbPath, _ := m.GetCodebasePath(ctx, codebaseID)

	var totalSize int64
	var fileCount int

	filepath.Walk(cbPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	cb.Size = totalSize
	cb.FileCount = fileCount
	cb.UpdatedAt = time.Now()

	return m.store.Update(ctx, cb)
}

// RefreshStats recalculates stats for a codebase (public API).
func (m *Manager) RefreshStats(ctx context.Context, codebaseID string) error {
	return m.updateCodebaseStats(ctx, codebaseID)
}
