// Package fs provides FUSE filesystem implementation with permission control.
package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WhiteoutPrefix is the prefix for whiteout files (marks deletions in delta).
const WhiteoutPrefix = ".wh."

// DeltaLayer manages the delta storage for a sandbox.
// It implements Copy-On-Write (COW) semantics where:
// - Reads first check delta, then fallback to source
// - Writes always go to delta
// - Deletes create whiteout markers in delta
type DeltaLayer struct {
	deltaDir  string // delta directory path (writable)
	sourceDir string // source directory path (read-only shared storage)
	mu        sync.RWMutex
}

// NewDeltaLayer creates a new delta layer.
func NewDeltaLayer(deltaDir, sourceDir string) (*DeltaLayer, error) {
	if deltaDir == "" {
		return nil, fmt.Errorf("deltaDir is required")
	}
	if sourceDir == "" {
		return nil, fmt.Errorf("sourceDir is required")
	}

	// Ensure delta directory exists
	if err := os.MkdirAll(deltaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create delta dir: %w", err)
	}

	return &DeltaLayer{
		deltaDir:  deltaDir,
		sourceDir: sourceDir,
	}, nil
}

// DeltaDir returns the delta directory path.
func (d *DeltaLayer) DeltaDir() string {
	return d.deltaDir
}

// SourceDir returns the source directory path.
func (d *DeltaLayer) SourceDir() string {
	return d.sourceDir
}

// ResolvePath resolves a relative path to the actual filesystem path.
// Priority: delta > source
// Returns the actual path and whether the file is in delta.
func (d *DeltaLayer) ResolvePath(relPath string) (actualPath string, inDelta bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Clean the path
	relPath = filepath.Clean(relPath)
	if relPath == "." {
		relPath = ""
	}

	// Check for whiteout (deletion marker)
	if d.isWhiteout(relPath) {
		// File was deleted, return empty to indicate not found
		return "", false
	}

	// Check delta first
	deltaPath := filepath.Join(d.deltaDir, relPath)
	if _, err := os.Lstat(deltaPath); err == nil {
		return deltaPath, true
	}

	// Fallback to source
	return filepath.Join(d.sourceDir, relPath), false
}

// HasDelta checks if a file exists in the delta directory.
func (d *DeltaLayer) HasDelta(relPath string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	relPath = filepath.Clean(relPath)
	deltaPath := filepath.Join(d.deltaDir, relPath)
	_, err := os.Lstat(deltaPath)
	return err == nil
}

// IsDeleted checks if a file has been marked as deleted (whiteout exists).
func (d *DeltaLayer) IsDeleted(relPath string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.isWhiteout(relPath)
}

// isWhiteout checks if a whiteout marker exists for the given path (internal, no lock).
func (d *DeltaLayer) isWhiteout(relPath string) bool {
	relPath = filepath.Clean(relPath)
	dir := filepath.Dir(relPath)
	base := filepath.Base(relPath)
	whiteoutPath := filepath.Join(d.deltaDir, dir, WhiteoutPrefix+base)
	_, err := os.Lstat(whiteoutPath)
	return err == nil
}

// EnsureDeltaDir ensures the directory structure exists in delta for the given path.
func (d *DeltaLayer) EnsureDeltaDir(relPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	relPath = filepath.Clean(relPath)
	dir := filepath.Dir(relPath)
	if dir == "." {
		return nil
	}

	deltaDir := filepath.Join(d.deltaDir, dir)
	return os.MkdirAll(deltaDir, 0755)
}

// GetDeltaPath returns the path in the delta directory for a given relative path.
func (d *DeltaLayer) GetDeltaPath(relPath string) string {
	return filepath.Join(d.deltaDir, filepath.Clean(relPath))
}

// GetSourcePath returns the path in the source directory for a given relative path.
func (d *DeltaLayer) GetSourcePath(relPath string) string {
	return filepath.Join(d.sourceDir, filepath.Clean(relPath))
}

// CopyToDelata copies a file from source to delta (COW operation).
// If the file doesn't exist in source, this is a no-op.
func (d *DeltaLayer) CopyToDelta(relPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	relPath = filepath.Clean(relPath)
	sourcePath := filepath.Join(d.sourceDir, relPath)
	deltaPath := filepath.Join(d.deltaDir, relPath)

	// Check if already in delta
	if _, err := os.Lstat(deltaPath); err == nil {
		return nil // Already exists in delta
	}

	// Check if exists in source
	srcInfo, err := os.Lstat(sourcePath)
	if os.IsNotExist(err) {
		return nil // Not in source, nothing to copy
	}
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Ensure parent directory exists in delta
	if err := os.MkdirAll(filepath.Dir(deltaPath), 0755); err != nil {
		return fmt.Errorf("failed to create delta parent dir: %w", err)
	}

	// Handle symlinks
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read symlink: %w", err)
		}
		return os.Symlink(target, deltaPath)
	}

	// Handle directories
	if srcInfo.IsDir() {
		return os.MkdirAll(deltaPath, srcInfo.Mode())
	}

	// Copy regular file
	return copyFile(sourcePath, deltaPath)
}

// MarkDeleted creates a whiteout marker for a deleted file.
func (d *DeltaLayer) MarkDeleted(relPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	relPath = filepath.Clean(relPath)
	dir := filepath.Dir(relPath)
	base := filepath.Base(relPath)

	// Ensure parent directory exists
	deltaDir := filepath.Join(d.deltaDir, dir)
	if err := os.MkdirAll(deltaDir, 0755); err != nil {
		return fmt.Errorf("failed to create delta dir: %w", err)
	}

	// Remove any existing delta file
	deltaPath := filepath.Join(d.deltaDir, relPath)
	os.RemoveAll(deltaPath)

	// Create whiteout marker
	whiteoutPath := filepath.Join(deltaDir, WhiteoutPrefix+base)
	f, err := os.Create(whiteoutPath)
	if err != nil {
		return fmt.Errorf("failed to create whiteout: %w", err)
	}
	return f.Close()
}

// RemoveWhiteout removes a whiteout marker (used when a file is recreated).
func (d *DeltaLayer) RemoveWhiteout(relPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	relPath = filepath.Clean(relPath)
	dir := filepath.Dir(relPath)
	base := filepath.Base(relPath)
	whiteoutPath := filepath.Join(d.deltaDir, dir, WhiteoutPrefix+base)
	
	if err := os.Remove(whiteoutPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove whiteout: %w", err)
	}
	return nil
}

// Sync synchronizes delta changes to source using Last-Writer-Wins (LWW) strategy.
func (d *DeltaLayer) Sync() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return filepath.Walk(d.deltaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the delta root directory itself
		if path == d.deltaDir {
			return nil
		}

		relPath, err := filepath.Rel(d.deltaDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(d.sourceDir, relPath)
		baseName := filepath.Base(path)

		// Handle whiteout files (deletion markers)
		if strings.HasPrefix(baseName, WhiteoutPrefix) {
			originalName := strings.TrimPrefix(baseName, WhiteoutPrefix)
			originalPath := filepath.Join(filepath.Dir(destPath), originalName)
			if err := os.RemoveAll(originalPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete %s: %w", originalPath, err)
			}
			return nil
		}

		// Handle directories
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
			// Remove existing file/symlink if it exists
			os.Remove(destPath)
			return os.Symlink(target, destPath)
		}

		// Copy regular file (LWW: directly overwrite)
		return copyFile(path, destPath)
	})
}

// Clear removes all content from the delta directory.
func (d *DeltaLayer) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	entries, err := os.ReadDir(d.deltaDir)
	if err != nil {
		return fmt.Errorf("failed to read delta dir: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(d.deltaDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	return nil
}

// ListChanges returns a list of all modified files in the delta.
// Returns relative paths.
func (d *DeltaLayer) ListChanges() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var changes []string

	err := filepath.Walk(d.deltaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the delta root directory itself
		if path == d.deltaDir {
			return nil
		}

		// Skip directories (we only report files)
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(d.deltaDir, path)
		if err != nil {
			return err
		}

		// For whiteout files, report the original file name with a delete indicator
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, WhiteoutPrefix) {
			originalName := strings.TrimPrefix(baseName, WhiteoutPrefix)
			originalPath := filepath.Join(filepath.Dir(relPath), originalName)
			changes = append(changes, "D:"+originalPath) // D: indicates deletion
		} else {
			changes = append(changes, "M:"+relPath) // M: indicates modification/addition
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return changes, nil
}

// MergedReadDir reads a directory, merging delta and source contents.
// Returns entries visible to the user (respecting whiteouts).
func (d *DeltaLayer) MergedReadDir(relPath string) ([]os.DirEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	relPath = filepath.Clean(relPath)
	if relPath == "." {
		relPath = ""
	}

	deltaPath := filepath.Join(d.deltaDir, relPath)
	sourcePath := filepath.Join(d.sourceDir, relPath)

	// Collect all entries and whiteouts
	entries := make(map[string]os.DirEntry)
	whiteouts := make(map[string]bool)

	// Read source directory first
	if srcEntries, err := os.ReadDir(sourcePath); err == nil {
		for _, entry := range srcEntries {
			entries[entry.Name()] = entry
		}
	}

	// Read delta directory (overrides source)
	if deltaEntries, err := os.ReadDir(deltaPath); err == nil {
		for _, entry := range deltaEntries {
			name := entry.Name()
			if strings.HasPrefix(name, WhiteoutPrefix) {
				// Mark as deleted
				originalName := strings.TrimPrefix(name, WhiteoutPrefix)
				whiteouts[originalName] = true
				delete(entries, originalName)
			} else {
				// Override with delta version
				entries[name] = entry
			}
		}
	}

	// Remove whiteout entries and whiteout marker files from result
	result := make([]os.DirEntry, 0, len(entries))
	for name, entry := range entries {
		if !whiteouts[name] && !strings.HasPrefix(name, WhiteoutPrefix) {
			result = append(result, entry)
		}
	}

	return result, nil
}

// copyFile copies a file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
