// Package codebase provides codebase management functionality.
package codebase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// Store defines the interface for codebase metadata storage.
type Store interface {
	// Create stores a new codebase metadata.
	Create(ctx context.Context, codebase *types.Codebase) error

	// Get retrieves a codebase by ID.
	Get(ctx context.Context, id string) (*types.Codebase, error)

	// List retrieves all codebases for an owner.
	// If ownerID is empty, returns all codebases.
	List(ctx context.Context, ownerID string, limit, offset int) ([]*types.Codebase, error)

	// Update updates an existing codebase metadata.
	Update(ctx context.Context, codebase *types.Codebase) error

	// Delete removes a codebase metadata by ID.
	Delete(ctx context.Context, id string) error

	// Exists checks if a codebase with the given ID exists.
	Exists(ctx context.Context, id string) (bool, error)
}

// MemoryStore implements Store interface using in-memory storage.
// Useful for testing and development.
type MemoryStore struct {
	mu        sync.RWMutex
	codebases map[string]*types.Codebase
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		codebases: make(map[string]*types.Codebase),
	}
}

// Create stores a new codebase metadata.
func (s *MemoryStore) Create(ctx context.Context, codebase *types.Codebase) error {
	if codebase == nil {
		return errors.New("codebase cannot be nil")
	}
	if codebase.ID == "" {
		return errors.New("codebase ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.codebases[codebase.ID]; exists {
		return fmt.Errorf("codebase with ID %s already exists", codebase.ID)
	}

	// Make a copy to avoid external mutations
	cb := *codebase
	s.codebases[codebase.ID] = &cb
	return nil
}

// Get retrieves a codebase by ID.
func (s *MemoryStore) Get(ctx context.Context, id string) (*types.Codebase, error) {
	if id == "" {
		return nil, errors.New("codebase ID cannot be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	cb, exists := s.codebases[id]
	if !exists {
		return nil, types.ErrCodebaseNotFound
	}

	// Return a copy
	result := *cb
	return &result, nil
}

// List retrieves all codebases for an owner.
func (s *MemoryStore) List(ctx context.Context, ownerID string, limit, offset int) ([]*types.Codebase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Codebase
	for _, cb := range s.codebases {
		if ownerID == "" || cb.OwnerID == ownerID {
			// Make a copy
			copy := *cb
			result = append(result, &copy)
		}
	}

	// Apply pagination
	if offset >= len(result) {
		return []*types.Codebase{}, nil
	}

	end := offset + limit
	if limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

// Update updates an existing codebase metadata.
func (s *MemoryStore) Update(ctx context.Context, codebase *types.Codebase) error {
	if codebase == nil {
		return errors.New("codebase cannot be nil")
	}
	if codebase.ID == "" {
		return errors.New("codebase ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.codebases[codebase.ID]; !exists {
		return types.ErrCodebaseNotFound
	}

	// Make a copy
	cb := *codebase
	s.codebases[codebase.ID] = &cb
	return nil
}

// Delete removes a codebase metadata by ID.
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("codebase ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.codebases[id]; !exists {
		return types.ErrCodebaseNotFound
	}

	delete(s.codebases, id)
	return nil
}

// Exists checks if a codebase with the given ID exists.
func (s *MemoryStore) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, errors.New("codebase ID cannot be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.codebases[id]
	return exists, nil
}

// FileStore implements Store interface using file-based JSON storage.
// Each codebase metadata is stored as a JSON file.
type FileStore struct {
	mu       sync.RWMutex
	basePath string
}

// NewFileStore creates a new file-based store.
func NewFileStore(basePath string) (*FileStore, error) {
	if basePath == "" {
		return nil, errors.New("base path cannot be empty")
	}

	metaPath := filepath.Join(basePath, "metadata")
	if err := os.MkdirAll(metaPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	return &FileStore{
		basePath: basePath,
	}, nil
}

func (s *FileStore) metadataPath(id string) string {
	return filepath.Join(s.basePath, "metadata", id+".json")
}

// Create stores a new codebase metadata.
func (s *FileStore) Create(ctx context.Context, codebase *types.Codebase) error {
	if codebase == nil {
		return errors.New("codebase cannot be nil")
	}
	if codebase.ID == "" {
		return errors.New("codebase ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaPath := s.metadataPath(codebase.ID)

	// Check if already exists
	if _, err := os.Stat(metaPath); err == nil {
		return fmt.Errorf("codebase with ID %s already exists", codebase.ID)
	}

	data, err := json.MarshalIndent(codebase, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal codebase: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Get retrieves a codebase by ID.
func (s *FileStore) Get(ctx context.Context, id string) (*types.Codebase, error) {
	if id == "" {
		return nil, errors.New("codebase ID cannot be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	metaPath := s.metadataPath(id)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, types.ErrCodebaseNotFound
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var codebase types.Codebase
	if err := json.Unmarshal(data, &codebase); err != nil {
		return nil, fmt.Errorf("failed to unmarshal codebase: %w", err)
	}

	return &codebase, nil
}

// List retrieves all codebases for an owner.
func (s *FileStore) List(ctx context.Context, ownerID string, limit, offset int) ([]*types.Codebase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metaDir := filepath.Join(s.basePath, "metadata")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.Codebase{}, nil
		}
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	var result []*types.Codebase
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(metaDir, entry.Name()))
		if err != nil {
			continue
		}

		var codebase types.Codebase
		if err := json.Unmarshal(data, &codebase); err != nil {
			continue
		}

		if ownerID == "" || codebase.OwnerID == ownerID {
			result = append(result, &codebase)
		}
	}

	// Apply pagination
	if offset >= len(result) {
		return []*types.Codebase{}, nil
	}

	end := offset + limit
	if limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

// Update updates an existing codebase metadata.
func (s *FileStore) Update(ctx context.Context, codebase *types.Codebase) error {
	if codebase == nil {
		return errors.New("codebase cannot be nil")
	}
	if codebase.ID == "" {
		return errors.New("codebase ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaPath := s.metadataPath(codebase.ID)

	// Check if exists
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return types.ErrCodebaseNotFound
	}

	// Update timestamp
	codebase.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(codebase, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal codebase: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Delete removes a codebase metadata by ID.
func (s *FileStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("codebase ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaPath := s.metadataPath(id)

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return types.ErrCodebaseNotFound
	}

	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("failed to delete metadata file: %w", err)
	}

	return nil
}

// Exists checks if a codebase with the given ID exists.
func (s *FileStore) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, errors.New("codebase ID cannot be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	metaPath := s.metadataPath(id)

	if _, err := os.Stat(metaPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check metadata file: %w", err)
	}

	return true, nil
}

// Ensure implementations satisfy the interface
var (
	_ Store = (*MemoryStore)(nil)
	_ Store = (*FileStore)(nil)
)
