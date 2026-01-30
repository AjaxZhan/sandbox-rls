// Package fs provides FUSE filesystem implementation with permission control.
package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Errors for SandboxFS
var (
	ErrInvalidSourceDir  = errors.New("invalid source directory")
	ErrInvalidMountPoint = errors.New("invalid mount point")
)

// SandboxFSConfig holds the configuration for creating a SandboxFS.
type SandboxFSConfig struct {
	SourceDir  string                 // The source directory to expose (shared storage, read-only source)
	DeltaDir   string                 // Delta directory for COW writes (optional, enables delta layer)
	MountPoint string                 // Where to mount the FUSE filesystem
	Rules      []types.PermissionRule // Permission rules
}

// SandboxFS is a FUSE filesystem that enforces permission rules.
type SandboxFS struct {
	config     *SandboxFSConfig
	permEngine PermissionEngine
	server     *fuse.Server
	mounted    atomic.Bool
	mu         sync.RWMutex

	// Delta layer for COW support
	delta        *DeltaLayer
	deltaEnabled bool
}

// NewSandboxFS creates a new SandboxFS instance.
func NewSandboxFS(config *SandboxFSConfig) (*SandboxFS, error) {
	// Validate config
	if config.SourceDir == "" {
		return nil, ErrInvalidSourceDir
	}
	if config.MountPoint == "" {
		return nil, ErrInvalidMountPoint
	}

	// Ensure source directory exists
	info, err := os.Stat(config.SourceDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrInvalidSourceDir
	}

	sfs := &SandboxFS{
		config:     config,
		permEngine: NewPermissionEngine(config.Rules),
	}

	// Initialize delta layer if configured
	if config.DeltaDir != "" {
		delta, err := NewDeltaLayer(config.DeltaDir, config.SourceDir)
		if err != nil {
			return nil, err
		}
		sfs.delta = delta
		sfs.deltaEnabled = true
	}

	return sfs, nil
}

// Mount mounts the FUSE filesystem. It blocks until the context is cancelled.
// If ready channel is provided, it receives nil when mount is ready, or an error if mount failed.
func (sfs *SandboxFS) Mount(ctx context.Context) error {
	return sfs.MountWithReady(ctx, nil)
}

// MountWithReady mounts the FUSE filesystem with a ready signal channel.
// The ready channel receives nil when mount succeeds, or an error if mount failed.
func (sfs *SandboxFS) MountWithReady(ctx context.Context, ready chan<- error) error {
	root := &sandboxRoot{
		sfs:       sfs,
		sourceDir: sfs.config.SourceDir,
	}

	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: true, // Allow bwrap processes to access the mount
			FsName:     "sandboxfs",
			Name:       "sandboxfs",
			Debug:      false, // Disable debug for production
		},
	}

	server, err := fs.Mount(sfs.config.MountPoint, root, opts)
	if err != nil {
		if ready != nil {
			ready <- err
		}
		return err
	}

	sfs.mu.Lock()
	sfs.server = server
	sfs.mounted.Store(true)
	sfs.mu.Unlock()

	// Signal that mount is ready
	if ready != nil {
		ready <- nil
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unmount
	if err := server.Unmount(); err != nil {
		return err
	}
	sfs.mounted.Store(false)

	return ctx.Err()
}

// IsMounted returns true if the filesystem is currently mounted.
func (sfs *SandboxFS) IsMounted() bool {
	return sfs.mounted.Load()
}

// UpdateRules updates the permission rules dynamically.
func (sfs *SandboxFS) UpdateRules(rules []types.PermissionRule) {
	sfs.mu.Lock()
	defer sfs.mu.Unlock()
	sfs.permEngine.UpdateRules(rules)
}

// getPermission returns the permission for the given path.
func (sfs *SandboxFS) getPermission(path string) types.Permission {
	sfs.mu.RLock()
	defer sfs.mu.RUnlock()
	return sfs.permEngine.GetPermission(path)
}

// checkRead checks if reading is allowed.
func (sfs *SandboxFS) checkRead(path string) error {
	sfs.mu.RLock()
	defer sfs.mu.RUnlock()
	return sfs.permEngine.CheckRead(path)
}

// checkWrite checks if writing is allowed.
func (sfs *SandboxFS) checkWrite(path string) error {
	sfs.mu.RLock()
	defer sfs.mu.RUnlock()
	return sfs.permEngine.CheckWrite(path)
}

// checkView checks if viewing is allowed.
func (sfs *SandboxFS) checkView(path string) error {
	sfs.mu.RLock()
	defer sfs.mu.RUnlock()
	return sfs.permEngine.CheckView(path)
}

// DeltaEnabled returns true if the delta layer is enabled.
func (sfs *SandboxFS) DeltaEnabled() bool {
	return sfs.deltaEnabled
}

// Sync synchronizes delta changes to the source directory.
// This should be called after exec() completes to persist changes.
func (sfs *SandboxFS) Sync() error {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return nil
	}
	return sfs.delta.Sync()
}

// ClearDelta clears the delta directory without syncing.
// Use this to discard changes (e.g., on exec failure).
func (sfs *SandboxFS) ClearDelta() error {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return nil
	}
	return sfs.delta.Clear()
}

// resolvePath resolves a virtual path to an actual filesystem path.
// If delta is enabled, it checks delta first, then falls back to source.
func (sfs *SandboxFS) resolvePath(relPath string) string {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return filepath.Join(sfs.config.SourceDir, relPath)
	}
	actualPath, _ := sfs.delta.ResolvePath(relPath)
	return actualPath
}

// getDeltaPath returns the path in delta directory for writing.
func (sfs *SandboxFS) getDeltaPath(relPath string) string {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return filepath.Join(sfs.config.SourceDir, relPath)
	}
	return sfs.delta.GetDeltaPath(relPath)
}

// ensureDeltaDir ensures parent directories exist in delta for a path.
func (sfs *SandboxFS) ensureDeltaDir(relPath string) error {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return nil
	}
	return sfs.delta.EnsureDeltaDir(relPath)
}

// copyToDelta performs COW copy from source to delta.
func (sfs *SandboxFS) copyToDelta(relPath string) error {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return nil
	}
	return sfs.delta.CopyToDelta(relPath)
}

// markDeleted marks a file as deleted in delta.
func (sfs *SandboxFS) markDeleted(relPath string) error {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return nil
	}
	return sfs.delta.MarkDeleted(relPath)
}

// removeWhiteout removes deletion marker when file is recreated.
func (sfs *SandboxFS) removeWhiteout(relPath string) error {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return nil
	}
	return sfs.delta.RemoveWhiteout(relPath)
}

// isDeleted checks if a file has been deleted in delta.
func (sfs *SandboxFS) isDeleted(relPath string) bool {
	if !sfs.deltaEnabled || sfs.delta == nil {
		return false
	}
	return sfs.delta.IsDeleted(relPath)
}

// deltaRename handles rename operations in delta mode.
// It copies the file to delta if needed, performs the rename within delta,
// and creates a whiteout for the old location if it existed in source.
func (sfs *SandboxFS) deltaRename(oldRelPath, newRelPath string) syscall.Errno {
	if sfs.delta == nil {
		return syscall.EIO
	}

	// Resolve old path - get actual location
	oldActualPath, oldInDelta := sfs.delta.ResolvePath(oldRelPath)
	if oldActualPath == "" {
		return syscall.ENOENT
	}

	// Ensure new path's parent directory exists in delta
	if err := sfs.delta.EnsureDeltaDir(newRelPath); err != nil {
		return syscall.EIO
	}

	// Remove any whiteout at the new location
	sfs.delta.RemoveWhiteout(newRelPath)

	// Get new delta path
	newDeltaPath := sfs.delta.GetDeltaPath(newRelPath)

	// Check if old file exists in source (need to create whiteout after move)
	oldSourcePath := sfs.delta.GetSourcePath(oldRelPath)
	oldExistsInSource := false
	if _, err := os.Lstat(oldSourcePath); err == nil {
		oldExistsInSource = true
	}

	if oldInDelta {
		// File is already in delta, just rename within delta
		oldDeltaPath := sfs.delta.GetDeltaPath(oldRelPath)
		if err := os.Rename(oldDeltaPath, newDeltaPath); err != nil {
			return toErrno(err)
		}
	} else {
		// File is in source only, copy to delta at new location
		if err := copyFile(oldActualPath, newDeltaPath); err != nil {
			return toErrno(err)
		}
	}

	// If old file existed in source, create whiteout
	if oldExistsInSource {
		if err := sfs.delta.MarkDeleted(oldRelPath); err != nil {
			return syscall.EIO
		}
	}

	return fs.OK
}

// sandboxRoot is the root node of the FUSE filesystem.
type sandboxRoot struct {
	fs.Inode
	sfs       *SandboxFS
	sourceDir string // base source directory (may be overridden by delta)
}

var _ = (fs.NodeLookuper)((*sandboxRoot)(nil))
var _ = (fs.NodeReaddirer)((*sandboxRoot)(nil))
var _ = (fs.NodeGetattrer)((*sandboxRoot)(nil))
var _ = (fs.NodeMkdirer)((*sandboxRoot)(nil))
var _ = (fs.NodeUnlinker)((*sandboxRoot)(nil))
var _ = (fs.NodeRmdirer)((*sandboxRoot)(nil))
var _ = (fs.NodeRenamer)((*sandboxRoot)(nil))
var _ = (fs.NodeCreater)((*sandboxRoot)(nil))
var _ = (fs.NodeSymlinker)((*sandboxRoot)(nil))
var _ = (fs.NodeReadlinker)((*sandboxRoot)(nil))

// sourcePath returns the full source path for a given relative path.
// If delta is enabled, it resolves through the delta layer.
func (r *sandboxRoot) sourcePath(relPath string) string {
	if r.sfs.deltaEnabled && r.sfs.delta != nil {
		actualPath, _ := r.sfs.delta.ResolvePath(relPath)
		return actualPath
	}
	return filepath.Join(r.sourceDir, relPath)
}

// deltaPath returns the path in delta directory for writes.
func (r *sandboxRoot) deltaPath(relPath string) string {
	if r.sfs.deltaEnabled && r.sfs.delta != nil {
		return r.sfs.delta.GetDeltaPath(relPath)
	}
	return filepath.Join(r.sourceDir, relPath)
}

// baseSourcePath returns the path in base source directory (ignoring delta).
func (r *sandboxRoot) baseSourcePath(relPath string) string {
	return filepath.Join(r.sourceDir, relPath)
}

// virtualPath returns the virtual path (as seen by the user) for a node.
func (r *sandboxRoot) virtualPath(name string) string {
	path := r.Path(nil)
	if path == "" {
		return "/" + name
	}
	return "/" + path + "/" + name
}

// Getattr implements fs.NodeGetattrer.
func (r *sandboxRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// Root always has view permission
	st := syscall.Stat_t{}
	if err := syscall.Stat(r.sourceDir, &st); err != nil {
		return syscall.EIO
	}
	out.FromStat(&st)
	return fs.OK
}

// Lookup implements fs.NodeLookuper.
func (r *sandboxRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := r.virtualPath(name)

	// Check permission (at least view required)
	perm := r.sfs.getPermission(virtualPath)
	if perm == types.PermNone {
		return nil, syscall.ENOENT // Hidden file appears as non-existent
	}

	// Check if file was deleted in delta
	if r.sfs.isDeleted(name) {
		return nil, syscall.ENOENT
	}

	// Resolve the actual path (delta-aware)
	sourcePath := r.sourcePath(name)

	// Stat the source file
	var st syscall.Stat_t
	if err := syscall.Lstat(sourcePath, &st); err != nil {
		return nil, syscall.ENOENT
	}

	out.Attr.FromStat(&st)

	// Create appropriate node based on file type
	// Use S_IFMT mask to correctly extract the file type
	fileType := st.Mode & syscall.S_IFMT
	var child fs.InodeEmbedder
	var stableAttr fs.StableAttr

	// For directories, we need to use the base source path for sourceDir
	// because sandboxDir.sourceDir is used as a prefix for child paths
	baseSourcePath := r.baseSourcePath(name)

	if fileType == syscall.S_IFDIR {
		// Directory
		child = &sandboxDir{
			sandboxRoot: sandboxRoot{
				sfs:       r.sfs,
				sourceDir: baseSourcePath,
			},
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFDIR}
	} else if fileType == syscall.S_IFLNK {
		// Symlink
		child = &sandboxSymlink{
			sfs:         r.sfs,
			sourcePath:  sourcePath,
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFLNK}
	} else {
		// Regular file
		child = &sandboxFile{
			sfs:         r.sfs,
			sourcePath:  sourcePath,
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFREG}
	}

	return r.NewInode(ctx, child, stableAttr), fs.OK
}

// Readdir implements fs.NodeReaddirer.
func (r *sandboxRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []os.DirEntry
	var err error

	// Use merged directory listing if delta is enabled
	if r.sfs.deltaEnabled && r.sfs.delta != nil {
		entries, err = r.sfs.delta.MergedReadDir("")
	} else {
		entries, err = os.ReadDir(r.sourceDir)
	}
	if err != nil {
		return nil, syscall.EIO
	}

	var result []fuse.DirEntry
	for _, entry := range entries {
		virtualPath := r.virtualPath(entry.Name())

		// Check permission (at least view required to see the entry)
		perm := r.sfs.getPermission(virtualPath)
		if perm == types.PermNone {
			continue // Skip hidden files
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		var mode uint32
		if entry.IsDir() {
			mode = fuse.S_IFDIR
		} else if info.Mode()&os.ModeSymlink != 0 {
			mode = fuse.S_IFLNK
		} else {
			mode = fuse.S_IFREG
		}

		result = append(result, fuse.DirEntry{
			Name: entry.Name(),
			Mode: mode,
		})
	}

	return fs.NewListDirStream(result), fs.OK
}

// Mkdir implements fs.NodeMkdirer.
func (r *sandboxRoot) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := r.virtualPath(name)

	// Check write permission
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return nil, syscall.EACCES
	}

	// Remove whiteout if exists (file was previously deleted)
	if r.sfs.deltaEnabled {
		r.sfs.removeWhiteout(name)
	}

	// Write to delta if enabled, otherwise to source
	targetPath := r.deltaPath(name)
	if r.sfs.deltaEnabled {
		// Ensure parent delta directory exists
		if err := r.sfs.ensureDeltaDir(name); err != nil {
			return nil, syscall.EIO
		}
	}

	if err := os.Mkdir(targetPath, os.FileMode(mode)); err != nil {
		return nil, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Stat(targetPath, &st); err != nil {
		return nil, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxDir{
		sandboxRoot: sandboxRoot{
			sfs:       r.sfs,
			sourceDir: r.baseSourcePath(name),
		},
		virtualPath: virtualPath,
	}

	return r.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), fs.OK
}

// Unlink implements fs.NodeUnlinker.
func (r *sandboxRoot) Unlink(ctx context.Context, name string) syscall.Errno {
	virtualPath := r.virtualPath(name)

	// Check write permission
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

	if r.sfs.deltaEnabled {
		// Check if file exists in delta
		deltaPath := r.deltaPath(name)
		if _, err := os.Lstat(deltaPath); err == nil {
			// File exists in delta, remove it
			if err := os.Remove(deltaPath); err != nil {
				return toErrno(err)
			}
		}
		// Also check if file exists in source - if so, create whiteout
		baseSourcePath := r.baseSourcePath(name)
		if _, err := os.Lstat(baseSourcePath); err == nil {
			if err := r.sfs.markDeleted(name); err != nil {
				return syscall.EIO
			}
		}
		return fs.OK
	}

	// No delta - direct removal from source
	sourcePath := r.sourcePath(name)
	if err := os.Remove(sourcePath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Rmdir implements fs.NodeRmdirer.
func (r *sandboxRoot) Rmdir(ctx context.Context, name string) syscall.Errno {
	virtualPath := r.virtualPath(name)

	// Check write permission
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

	if r.sfs.deltaEnabled {
		// Check if directory exists in delta
		deltaPath := r.deltaPath(name)
		if _, err := os.Lstat(deltaPath); err == nil {
			// Directory exists in delta, remove it
			if err := os.Remove(deltaPath); err != nil {
				return toErrno(err)
			}
		}
		// Also check if directory exists in source - if so, create whiteout
		baseSourcePath := r.baseSourcePath(name)
		if _, err := os.Lstat(baseSourcePath); err == nil {
			if err := r.sfs.markDeleted(name); err != nil {
				return syscall.EIO
			}
		}
		return fs.OK
	}

	// No delta - direct removal from source
	sourcePath := r.sourcePath(name)
	if err := os.Remove(sourcePath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Rename implements fs.NodeRenamer.
func (r *sandboxRoot) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	virtualPath := r.virtualPath(name)
	oldRelPath := strings.TrimPrefix(virtualPath, "/")

	// Check write permission for both old and new paths
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

	newVirtualPath := "/" + newName
	if np, ok := newParent.(*sandboxDir); ok {
		newVirtualPath = np.virtualPath + "/" + newName
	}
	newRelPath := strings.TrimPrefix(newVirtualPath, "/")

	if err := r.sfs.checkWrite(newVirtualPath); err != nil {
		return syscall.EACCES
	}

	// Handle delta mode
	if r.sfs.deltaEnabled && r.sfs.delta != nil {
		return r.sfs.deltaRename(oldRelPath, newRelPath)
	}

	// No delta - direct rename in source
	// Get new parent's source path
	var newSourceDir string
	switch p := newParent.(type) {
	case *sandboxRoot:
		newSourceDir = p.sourceDir
	case *sandboxDir:
		newSourceDir = p.sourceDir
	default:
		return syscall.EINVAL
	}

	oldPath := r.sourcePath(name)
	newPath := filepath.Join(newSourceDir, newName)

	if err := os.Rename(oldPath, newPath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Create implements fs.NodeCreater.
func (r *sandboxRoot) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	virtualPath := r.virtualPath(name)

	// Check write permission
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return nil, nil, 0, syscall.EACCES
	}

	// Remove whiteout if exists (file was previously deleted)
	if r.sfs.deltaEnabled {
		r.sfs.removeWhiteout(name)
	}

	// Write to delta if enabled, otherwise to source
	targetPath := r.deltaPath(name)
	if r.sfs.deltaEnabled {
		// Ensure parent delta directory exists
		if err := r.sfs.ensureDeltaDir(name); err != nil {
			return nil, nil, 0, syscall.EIO
		}
	}

	// Strip FUSE-specific flags and use syscall.Open
	flags = flags &^ FMODE_EXEC
	fd, err := syscall.Open(targetPath, int(flags)|os.O_CREATE, mode)
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		syscall.Close(fd)
		return nil, nil, 0, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxFile{
		sfs:         r.sfs,
		sourcePath:  targetPath,
		virtualPath: virtualPath,
	}

	return r.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), &sandboxFileHandle{fd: fd}, 0, fs.OK
}

// Symlink implements fs.NodeSymlinker.
func (r *sandboxRoot) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := r.virtualPath(name)

	// Check write permission
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return nil, syscall.EACCES
	}

	// Remove whiteout if exists
	if r.sfs.deltaEnabled {
		r.sfs.removeWhiteout(name)
	}

	// Write to delta if enabled, otherwise to source
	targetPath := r.deltaPath(name)
	if r.sfs.deltaEnabled {
		// Ensure parent delta directory exists
		if err := r.sfs.ensureDeltaDir(name); err != nil {
			return nil, syscall.EIO
		}
	}

	if err := os.Symlink(target, targetPath); err != nil {
		return nil, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Lstat(targetPath, &st); err != nil {
		return nil, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxSymlink{
		sfs:         r.sfs,
		sourcePath:  targetPath,
		virtualPath: virtualPath,
	}

	return r.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFLNK}), fs.OK
}

// Readlink implements fs.NodeReadlinker.
func (r *sandboxRoot) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	// Root is not a symlink
	return nil, syscall.EINVAL
}

// sandboxDir represents a directory in the sandbox filesystem.
type sandboxDir struct {
	sandboxRoot
	virtualPath string
}

var _ = (fs.NodeLookuper)((*sandboxDir)(nil))
var _ = (fs.NodeReaddirer)((*sandboxDir)(nil))
var _ = (fs.NodeGetattrer)((*sandboxDir)(nil))
var _ = (fs.NodeMkdirer)((*sandboxDir)(nil))
var _ = (fs.NodeUnlinker)((*sandboxDir)(nil))
var _ = (fs.NodeRmdirer)((*sandboxDir)(nil))
var _ = (fs.NodeRenamer)((*sandboxDir)(nil))
var _ = (fs.NodeCreater)((*sandboxDir)(nil))

// virtualPath returns the virtual path for a child.
func (d *sandboxDir) virtualPathFor(name string) string {
	return d.virtualPath + "/" + name
}

// Getattr implements fs.NodeGetattrer for directories.
func (d *sandboxDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// Check view permission
	if err := d.sfs.checkView(d.virtualPath); err != nil {
		return syscall.EACCES
	}

	// Resolve actual path through delta layer
	relPath := strings.TrimPrefix(d.virtualPath, "/")
	var actualPath string
	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		actualPath, _ = d.sfs.delta.ResolvePath(relPath)
		if actualPath == "" {
			// Directory was deleted
			return syscall.ENOENT
		}
	} else {
		actualPath = d.sourceDir
	}

	var st syscall.Stat_t
	if err := syscall.Stat(actualPath, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

// Lookup implements fs.NodeLookuper for directories.
func (d *sandboxDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := d.virtualPathFor(name)

	// Check permission
	perm := d.sfs.getPermission(virtualPath)
	if perm == types.PermNone {
		return nil, syscall.ENOENT
	}

	// Get relative path from root for delta operations
	relPath := strings.TrimPrefix(virtualPath, "/")

	// Check if file was deleted in delta
	if d.sfs.isDeleted(relPath) {
		return nil, syscall.ENOENT
	}

	// Resolve actual path through delta layer
	var sourcePath string
	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		sourcePath, _ = d.sfs.delta.ResolvePath(relPath)
	} else {
		sourcePath = filepath.Join(d.sourceDir, name)
	}

	var st syscall.Stat_t
	if err := syscall.Lstat(sourcePath, &st); err != nil {
		return nil, syscall.ENOENT
	}
	out.Attr.FromStat(&st)

	// Use S_IFMT mask to correctly extract the file type
	fileType := st.Mode & syscall.S_IFMT
	var child fs.InodeEmbedder
	var stableAttr fs.StableAttr

	// Base source path for directory (used as prefix for child paths)
	baseSourcePath := filepath.Join(d.sourceDir, name)

	if fileType == syscall.S_IFDIR {
		child = &sandboxDir{
			sandboxRoot: sandboxRoot{
				sfs:       d.sfs,
				sourceDir: baseSourcePath,
			},
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFDIR}
	} else if fileType == syscall.S_IFLNK {
		child = &sandboxSymlink{
			sfs:         d.sfs,
			sourcePath:  sourcePath,
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFLNK}
	} else {
		child = &sandboxFile{
			sfs:         d.sfs,
			sourcePath:  sourcePath,
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFREG}
	}

	return d.NewInode(ctx, child, stableAttr), fs.OK
}

// Readdir implements fs.NodeReaddirer for directories.
func (d *sandboxDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var entries []os.DirEntry
	var err error

	// Get relative path from root for delta operations
	relPath := strings.TrimPrefix(d.virtualPath, "/")

	// Use merged directory listing if delta is enabled
	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		entries, err = d.sfs.delta.MergedReadDir(relPath)
	} else {
		entries, err = os.ReadDir(d.sourceDir)
	}
	if err != nil {
		return nil, syscall.EIO
	}

	var result []fuse.DirEntry
	for _, entry := range entries {
		virtualPath := d.virtualPathFor(entry.Name())

		perm := d.sfs.getPermission(virtualPath)
		if perm == types.PermNone {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		var mode uint32
		if entry.IsDir() {
			mode = fuse.S_IFDIR
		} else if info.Mode()&os.ModeSymlink != 0 {
			mode = fuse.S_IFLNK
		} else {
			mode = fuse.S_IFREG
		}

		result = append(result, fuse.DirEntry{
			Name: entry.Name(),
			Mode: mode,
		})
	}

	return fs.NewListDirStream(result), fs.OK
}

// Mkdir implements fs.NodeMkdirer for directories.
func (d *sandboxDir) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := d.virtualPathFor(name)
	relPath := strings.TrimPrefix(virtualPath, "/")

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return nil, syscall.EACCES
	}

	// Remove whiteout if exists
	if d.sfs.deltaEnabled {
		d.sfs.removeWhiteout(relPath)
	}

	// Write to delta if enabled
	var targetPath string
	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		if err := d.sfs.ensureDeltaDir(relPath); err != nil {
			return nil, syscall.EIO
		}
		targetPath = d.sfs.delta.GetDeltaPath(relPath)
	} else {
		targetPath = filepath.Join(d.sourceDir, name)
	}

	if err := os.Mkdir(targetPath, os.FileMode(mode)); err != nil {
		return nil, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Stat(targetPath, &st); err != nil {
		return nil, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxDir{
		sandboxRoot: sandboxRoot{
			sfs:       d.sfs,
			sourceDir: filepath.Join(d.sourceDir, name),
		},
		virtualPath: virtualPath,
	}

	return d.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), fs.OK
}

// Unlink implements fs.NodeUnlinker for directories.
func (d *sandboxDir) Unlink(ctx context.Context, name string) syscall.Errno {
	virtualPath := d.virtualPathFor(name)
	relPath := strings.TrimPrefix(virtualPath, "/")

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		// Check if file exists in delta
		deltaPath := d.sfs.delta.GetDeltaPath(relPath)
		if _, err := os.Lstat(deltaPath); err == nil {
			if err := os.Remove(deltaPath); err != nil {
				return toErrno(err)
			}
		}
		// Check if file exists in source - if so, create whiteout
		sourcePath := d.sfs.delta.GetSourcePath(relPath)
		if _, err := os.Lstat(sourcePath); err == nil {
			if err := d.sfs.markDeleted(relPath); err != nil {
				return syscall.EIO
			}
		}
		return fs.OK
	}

	sourcePath := filepath.Join(d.sourceDir, name)
	if err := os.Remove(sourcePath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Rmdir implements fs.NodeRmdirer for directories.
func (d *sandboxDir) Rmdir(ctx context.Context, name string) syscall.Errno {
	virtualPath := d.virtualPathFor(name)
	relPath := strings.TrimPrefix(virtualPath, "/")

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		// Check if directory exists in delta
		deltaPath := d.sfs.delta.GetDeltaPath(relPath)
		if _, err := os.Lstat(deltaPath); err == nil {
			if err := os.Remove(deltaPath); err != nil {
				return toErrno(err)
			}
		}
		// Check if directory exists in source - if so, create whiteout
		sourcePath := d.sfs.delta.GetSourcePath(relPath)
		if _, err := os.Lstat(sourcePath); err == nil {
			if err := d.sfs.markDeleted(relPath); err != nil {
				return syscall.EIO
			}
		}
		return fs.OK
	}

	sourcePath := filepath.Join(d.sourceDir, name)
	if err := os.Remove(sourcePath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Rename implements fs.NodeRenamer for directories.
func (d *sandboxDir) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	virtualPath := d.virtualPathFor(name)
	oldRelPath := strings.TrimPrefix(virtualPath, "/")

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

	var newSourceDir string
	var newVirtualPath string

	switch p := newParent.(type) {
	case *sandboxRoot:
		newSourceDir = p.sourceDir
		newVirtualPath = "/" + newName
	case *sandboxDir:
		newSourceDir = p.sourceDir
		newVirtualPath = p.virtualPath + "/" + newName
	default:
		return syscall.EINVAL
	}
	newRelPath := strings.TrimPrefix(newVirtualPath, "/")

	if err := d.sfs.checkWrite(newVirtualPath); err != nil {
		return syscall.EACCES
	}

	// Handle delta mode
	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		return d.sfs.deltaRename(oldRelPath, newRelPath)
	}

	// No delta - direct rename in source
	oldPath := filepath.Join(d.sourceDir, name)
	newPath := filepath.Join(newSourceDir, newName)

	if err := os.Rename(oldPath, newPath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Create implements fs.NodeCreater for directories.
func (d *sandboxDir) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	virtualPath := d.virtualPathFor(name)
	relPath := strings.TrimPrefix(virtualPath, "/")

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return nil, nil, 0, syscall.EACCES
	}

	// Remove whiteout if exists
	if d.sfs.deltaEnabled {
		d.sfs.removeWhiteout(relPath)
	}

	// Write to delta if enabled
	var targetPath string
	if d.sfs.deltaEnabled && d.sfs.delta != nil {
		if err := d.sfs.ensureDeltaDir(relPath); err != nil {
			return nil, nil, 0, syscall.EIO
		}
		targetPath = d.sfs.delta.GetDeltaPath(relPath)
	} else {
		targetPath = filepath.Join(d.sourceDir, name)
	}

	// Strip FUSE-specific flags and use syscall.Open
	flags = flags &^ FMODE_EXEC
	fd, err := syscall.Open(targetPath, int(flags)|os.O_CREATE, mode)
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		syscall.Close(fd)
		return nil, nil, 0, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxFile{
		sfs:         d.sfs,
		sourcePath:  targetPath,
		virtualPath: virtualPath,
	}

	return d.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), &sandboxFileHandle{fd: fd}, 0, fs.OK
}

// sandboxFile represents a regular file in the sandbox filesystem.
type sandboxFile struct {
	fs.Inode
	sfs         *SandboxFS
	sourcePath  string
	virtualPath string
}

var _ = (fs.NodeGetattrer)((*sandboxFile)(nil))
var _ = (fs.NodeSetattrer)((*sandboxFile)(nil))
var _ = (fs.NodeOpener)((*sandboxFile)(nil))

// Getattr implements fs.NodeGetattrer for files.
func (f *sandboxFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// Check view permission
	if err := f.sfs.checkView(f.virtualPath); err != nil {
		return syscall.EACCES
	}

	var st syscall.Stat_t
	if err := syscall.Stat(f.sourcePath, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

// Setattr implements fs.NodeSetattrer for files (handles truncate, chmod, etc.).
func (f *sandboxFile) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	// Most setattr operations require write permission
	if in.Valid&(fuse.FATTR_SIZE|fuse.FATTR_ATIME|fuse.FATTR_MTIME|fuse.FATTR_MODE) != 0 {
		if err := f.sfs.checkWrite(f.virtualPath); err != nil {
			return syscall.EACCES
		}
	}

	// For write operations with delta enabled, perform COW if needed
	relPath := strings.TrimPrefix(f.virtualPath, "/")
	targetPath := f.sourcePath

	if f.sfs.deltaEnabled && f.sfs.delta != nil {
		// Check if file exists in delta
		deltaPath := f.sfs.delta.GetDeltaPath(relPath)
		if _, err := os.Lstat(deltaPath); os.IsNotExist(err) {
			// File not in delta, need to copy from source (COW)
			if err := f.sfs.copyToDelta(relPath); err != nil {
				return syscall.EIO
			}
		}
		targetPath = deltaPath
		f.sourcePath = targetPath
	}

	// Handle truncate
	if in.Valid&fuse.FATTR_SIZE != 0 {
		if err := os.Truncate(targetPath, int64(in.Size)); err != nil {
			return toErrno(err)
		}
	}

	// Handle chmod
	if in.Valid&fuse.FATTR_MODE != 0 {
		if err := os.Chmod(targetPath, os.FileMode(in.Mode)); err != nil {
			return toErrno(err)
		}
	}

	// Update output attributes
	var st syscall.Stat_t
	if err := syscall.Stat(targetPath, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

// FMODE_EXEC is a FUSE-specific flag that should be stripped before passing to OS
const FMODE_EXEC = 0x20

// Open implements fs.NodeOpener for files.
func (f *sandboxFile) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// Check permissions based on open flags
	accMode := flags & syscall.O_ACCMODE

	switch accMode {
	case syscall.O_RDONLY:
		if err := f.sfs.checkRead(f.virtualPath); err != nil {
			return nil, 0, syscall.EACCES
		}
	case syscall.O_WRONLY, syscall.O_RDWR:
		if err := f.sfs.checkWrite(f.virtualPath); err != nil {
			return nil, 0, syscall.EACCES
		}
	}

	// Determine the actual file path
	actualPath := f.sourcePath
	relPath := strings.TrimPrefix(f.virtualPath, "/")

	// For write access with delta enabled, perform COW if needed
	if f.sfs.deltaEnabled && f.sfs.delta != nil && accMode != syscall.O_RDONLY {
		// Check if file exists in delta
		deltaPath := f.sfs.delta.GetDeltaPath(relPath)
		if _, err := os.Lstat(deltaPath); os.IsNotExist(err) {
			// File not in delta, need to copy from source (COW)
			if err := f.sfs.copyToDelta(relPath); err != nil {
				return nil, 0, syscall.EIO
			}
		}
		actualPath = deltaPath
		// Update sourcePath for subsequent operations
		f.sourcePath = actualPath
	}

	// Strip FUSE-specific flags that shouldn't be passed to the OS
	flags = flags &^ (syscall.O_APPEND | FMODE_EXEC)

	// Use syscall.Open directly
	fd, err := syscall.Open(actualPath, int(flags), 0)
	if err != nil {
		return nil, 0, toErrno(err)
	}

	return &sandboxFileHandle{fd: fd}, 0, fs.OK
}

// sandboxFileHandle is a simple file handle that uses standard read/write operations
// instead of splice-based operations (which may not work in all configurations).
type sandboxFileHandle struct {
	fd int
}

var _ = (fs.FileReader)((*sandboxFileHandle)(nil))
var _ = (fs.FileWriter)((*sandboxFileHandle)(nil))
var _ = (fs.FileReleaser)((*sandboxFileHandle)(nil))
var _ = (fs.FileFlusher)((*sandboxFileHandle)(nil))
var _ = (fs.FileGetattrer)((*sandboxFileHandle)(nil))
var _ = (fs.FileLseeker)((*sandboxFileHandle)(nil))

// Read implements fs.FileReader using standard pread instead of splice.
func (fh *sandboxFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := syscall.Pread(fh.fd, dest, off)
	if err != nil && err != syscall.Errno(0) {
		return nil, toErrno(err)
	}
	return fuse.ReadResultData(dest[:n]), fs.OK
}

// Write implements fs.FileWriter.
func (fh *sandboxFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	n, err := syscall.Pwrite(fh.fd, data, off)
	if err != nil {
		return 0, toErrno(err)
	}
	return uint32(n), fs.OK
}

// Release implements fs.FileReleaser.
func (fh *sandboxFileHandle) Release(ctx context.Context) syscall.Errno {
	if fh.fd != -1 {
		err := syscall.Close(fh.fd)
		fh.fd = -1
		return toErrno(err)
	}
	return syscall.EBADF
}

// Flush implements fs.FileFlusher.
func (fh *sandboxFileHandle) Flush(ctx context.Context) syscall.Errno {
	// Dup and close to flush without closing the original fd
	newFd, err := syscall.Dup(fh.fd)
	if err != nil {
		return toErrno(err)
	}
	return toErrno(syscall.Close(newFd))
}

// Getattr implements fs.FileGetattrer.
func (fh *sandboxFileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	var st syscall.Stat_t
	if err := syscall.Fstat(fh.fd, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

// Lseek implements fs.FileLseeker.
func (fh *sandboxFileHandle) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	n, err := syscall.Seek(fh.fd, int64(off), int(whence))
	if err != nil {
		return 0, toErrno(err)
	}
	return uint64(n), fs.OK
}

// sandboxSymlink represents a symbolic link in the sandbox filesystem.
type sandboxSymlink struct {
	fs.Inode
	sfs         *SandboxFS
	sourcePath  string
	virtualPath string
}

var _ = (fs.NodeGetattrer)((*sandboxSymlink)(nil))
var _ = (fs.NodeReadlinker)((*sandboxSymlink)(nil))

// Getattr implements fs.NodeGetattrer for symlinks.
func (s *sandboxSymlink) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	if err := s.sfs.checkView(s.virtualPath); err != nil {
		return syscall.EACCES
	}

	var st syscall.Stat_t
	if err := syscall.Lstat(s.sourcePath, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

// Readlink implements fs.NodeReadlinker for symlinks.
func (s *sandboxSymlink) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	if err := s.sfs.checkView(s.virtualPath); err != nil {
		return nil, syscall.EACCES
	}

	target, err := os.Readlink(s.sourcePath)
	if err != nil {
		return nil, toErrno(err)
	}
	return []byte(target), fs.OK
}

// toErrno converts a Go error to a syscall.Errno.
func toErrno(err error) syscall.Errno {
	if err == nil {
		return fs.OK
	}

	// Check for permission errors from our permission engine
	if _, ok := err.(*types.PermissionError); ok {
		return syscall.EACCES
	}

	// Check for os.PathError
	if pe, ok := err.(*os.PathError); ok {
		if errno, ok := pe.Err.(syscall.Errno); ok {
			return errno
		}
	}

	// Check for direct syscall.Errno
	if errno, ok := err.(syscall.Errno); ok {
		return errno
	}

	// Map common errors
	if os.IsNotExist(err) {
		return syscall.ENOENT
	}
	if os.IsPermission(err) {
		return syscall.EACCES
	}
	if os.IsExist(err) {
		return syscall.EEXIST
	}

	// Check error message for common patterns
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "permission denied") {
		return syscall.EACCES
	}
	if strings.Contains(errStr, "no such file") || strings.Contains(errStr, "not found") {
		return syscall.ENOENT
	}
	if strings.Contains(errStr, "file exists") {
		return syscall.EEXIST
	}

	return syscall.EIO
}
