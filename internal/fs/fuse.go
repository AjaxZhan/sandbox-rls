// Package fs provides FUSE filesystem implementation with permission control.
package fs

import (
	"context"
	"errors"
	"io"
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
	SourceDir  string                 // The source directory to expose
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

	return &SandboxFS{
		config:     config,
		permEngine: NewPermissionEngine(config.Rules),
	}, nil
}

// Mount mounts the FUSE filesystem. It blocks until the context is cancelled.
func (sfs *SandboxFS) Mount(ctx context.Context) error {
	root := &sandboxRoot{
		sfs:       sfs,
		sourceDir: sfs.config.SourceDir,
	}

	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: true, // Allow bwrap processes to access the mount
			FsName:     "sandboxfs",
			Name:       "sandboxfs",
			Debug:      false,
		},
	}

	server, err := fs.Mount(sfs.config.MountPoint, root, opts)
	if err != nil {
		return err
	}

	sfs.mu.Lock()
	sfs.server = server
	sfs.mounted.Store(true)
	sfs.mu.Unlock()

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

// sandboxRoot is the root node of the FUSE filesystem.
type sandboxRoot struct {
	fs.Inode
	sfs       *SandboxFS
	sourceDir string
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
func (r *sandboxRoot) sourcePath(relPath string) string {
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
	sourcePath := r.sourcePath(name)

	// Check permission (at least view required)
	perm := r.sfs.getPermission(virtualPath)
	if perm == types.PermNone {
		return nil, syscall.ENOENT // Hidden file appears as non-existent
	}

	// Stat the source file
	var st syscall.Stat_t
	if err := syscall.Lstat(sourcePath, &st); err != nil {
		return nil, syscall.ENOENT
	}

	out.Attr.FromStat(&st)

	// Create appropriate node based on file type
	mode := st.Mode
	var child fs.InodeEmbedder
	var stableAttr fs.StableAttr

	if mode&syscall.S_IFDIR != 0 {
		// Directory
		child = &sandboxDir{
			sandboxRoot: sandboxRoot{
				sfs:       r.sfs,
				sourceDir: sourcePath,
			},
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFDIR}
	} else if mode&syscall.S_IFLNK != 0 {
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
	entries, err := os.ReadDir(r.sourceDir)
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

	sourcePath := r.sourcePath(name)
	if err := os.Mkdir(sourcePath, os.FileMode(mode)); err != nil {
		return nil, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Stat(sourcePath, &st); err != nil {
		return nil, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxDir{
		sandboxRoot: sandboxRoot{
			sfs:       r.sfs,
			sourceDir: sourcePath,
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

	sourcePath := r.sourcePath(name)
	if err := os.Remove(sourcePath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Rename implements fs.NodeRenamer.
func (r *sandboxRoot) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	virtualPath := r.virtualPath(name)

	// Check write permission for both old and new paths
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
	}

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

	newVirtualPath := "/" + newName
	if np, ok := newParent.(*sandboxDir); ok {
		newVirtualPath = np.virtualPath + "/" + newName
	}

	if err := r.sfs.checkWrite(newVirtualPath); err != nil {
		return syscall.EACCES
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

	sourcePath := r.sourcePath(name)
	f, err := os.OpenFile(sourcePath, int(flags)|os.O_CREATE, os.FileMode(mode))
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		f.Close()
		return nil, nil, 0, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxFile{
		sfs:         r.sfs,
		sourcePath:  sourcePath,
		virtualPath: virtualPath,
	}

	handle := &sandboxFileHandle{
		file:        f,
		virtualPath: virtualPath,
		sfs:         r.sfs,
	}

	return r.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), handle, 0, fs.OK
}

// Symlink implements fs.NodeSymlinker.
func (r *sandboxRoot) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := r.virtualPath(name)

	// Check write permission
	if err := r.sfs.checkWrite(virtualPath); err != nil {
		return nil, syscall.EACCES
	}

	sourcePath := r.sourcePath(name)
	if err := os.Symlink(target, sourcePath); err != nil {
		return nil, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Lstat(sourcePath, &st); err != nil {
		return nil, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxSymlink{
		sfs:         r.sfs,
		sourcePath:  sourcePath,
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

	var st syscall.Stat_t
	if err := syscall.Stat(d.sourceDir, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

// Lookup implements fs.NodeLookuper for directories.
func (d *sandboxDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	virtualPath := d.virtualPathFor(name)
	sourcePath := filepath.Join(d.sourceDir, name)

	// Check permission
	perm := d.sfs.getPermission(virtualPath)
	if perm == types.PermNone {
		return nil, syscall.ENOENT
	}

	var st syscall.Stat_t
	if err := syscall.Lstat(sourcePath, &st); err != nil {
		return nil, syscall.ENOENT
	}
	out.Attr.FromStat(&st)

	mode := st.Mode
	var child fs.InodeEmbedder
	var stableAttr fs.StableAttr

	if mode&syscall.S_IFDIR != 0 {
		child = &sandboxDir{
			sandboxRoot: sandboxRoot{
				sfs:       d.sfs,
				sourceDir: sourcePath,
			},
			virtualPath: virtualPath,
		}
		stableAttr = fs.StableAttr{Mode: fuse.S_IFDIR}
	} else if mode&syscall.S_IFLNK != 0 {
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
	entries, err := os.ReadDir(d.sourceDir)
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

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return nil, syscall.EACCES
	}

	sourcePath := filepath.Join(d.sourceDir, name)
	if err := os.Mkdir(sourcePath, os.FileMode(mode)); err != nil {
		return nil, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Stat(sourcePath, &st); err != nil {
		return nil, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxDir{
		sandboxRoot: sandboxRoot{
			sfs:       d.sfs,
			sourceDir: sourcePath,
		},
		virtualPath: virtualPath,
	}

	return d.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFDIR}), fs.OK
}

// Unlink implements fs.NodeUnlinker for directories.
func (d *sandboxDir) Unlink(ctx context.Context, name string) syscall.Errno {
	virtualPath := d.virtualPathFor(name)

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
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

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return syscall.EACCES
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

	if err := d.sfs.checkWrite(newVirtualPath); err != nil {
		return syscall.EACCES
	}

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

	if err := d.sfs.checkWrite(virtualPath); err != nil {
		return nil, nil, 0, syscall.EACCES
	}

	sourcePath := filepath.Join(d.sourceDir, name)
	f, err := os.OpenFile(sourcePath, int(flags)|os.O_CREATE, os.FileMode(mode))
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}

	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		f.Close()
		return nil, nil, 0, syscall.EIO
	}
	out.Attr.FromStat(&st)

	child := &sandboxFile{
		sfs:         d.sfs,
		sourcePath:  sourcePath,
		virtualPath: virtualPath,
	}

	handle := &sandboxFileHandle{
		file:        f,
		virtualPath: virtualPath,
		sfs:         d.sfs,
	}

	return d.NewInode(ctx, child, fs.StableAttr{Mode: fuse.S_IFREG}), handle, 0, fs.OK
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
	if in.Valid&(fuse.FATTR_SIZE|fuse.FATTR_ATIME|fuse.FATTR_MTIME) != 0 {
		if err := f.sfs.checkWrite(f.virtualPath); err != nil {
			return syscall.EACCES
		}
	}

	// Handle truncate
	if in.Valid&fuse.FATTR_SIZE != 0 {
		if err := os.Truncate(f.sourcePath, int64(in.Size)); err != nil {
			return toErrno(err)
		}
	}

	// Handle chmod (optional, may be ignored)
	if in.Valid&fuse.FATTR_MODE != 0 {
		if err := f.sfs.checkWrite(f.virtualPath); err != nil {
			return syscall.EACCES
		}
		if err := os.Chmod(f.sourcePath, os.FileMode(in.Mode)); err != nil {
			return toErrno(err)
		}
	}

	// Update output attributes
	var st syscall.Stat_t
	if err := syscall.Stat(f.sourcePath, &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
}

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

	// Filter flags to only include valid bits for os.OpenFile
	// FUSE may pass additional flags that don't translate to OS flags
	osFlags := int(accMode)
	if flags&syscall.O_APPEND != 0 {
		osFlags |= syscall.O_APPEND
	}
	if flags&syscall.O_TRUNC != 0 {
		osFlags |= syscall.O_TRUNC
	}

	file, err := os.OpenFile(f.sourcePath, osFlags, 0)
	if err != nil {
		return nil, 0, toErrno(err)
	}

	return &sandboxFileHandle{
		file:        file,
		virtualPath: f.virtualPath,
		sfs:         f.sfs,
	}, 0, fs.OK
}

// sandboxFileHandle represents an open file handle.
type sandboxFileHandle struct {
	file        *os.File
	virtualPath string
	sfs         *SandboxFS
}

var _ = (fs.FileReader)((*sandboxFileHandle)(nil))
var _ = (fs.FileWriter)((*sandboxFileHandle)(nil))
var _ = (fs.FileFlusher)((*sandboxFileHandle)(nil))
var _ = (fs.FileReleaser)((*sandboxFileHandle)(nil))
var _ = (fs.FileLseeker)((*sandboxFileHandle)(nil))
var _ = (fs.FileGetattrer)((*sandboxFileHandle)(nil))

// Read implements fs.FileReader.
func (fh *sandboxFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := fh.file.ReadAt(dest, off)
	// ReadAt returns io.EOF when reading at or past end of file
	// This is normal behavior, not an error for FUSE
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, toErrno(err)
	}
	return fuse.ReadResultData(dest[:n]), fs.OK
}

// Write implements fs.FileWriter.
func (fh *sandboxFileHandle) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	n, err := fh.file.WriteAt(data, off)
	if err != nil {
		return 0, toErrno(err)
	}
	return uint32(n), fs.OK
}

// Flush implements fs.FileFlusher.
func (fh *sandboxFileHandle) Flush(ctx context.Context) syscall.Errno {
	if err := fh.file.Sync(); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Release implements fs.FileReleaser.
func (fh *sandboxFileHandle) Release(ctx context.Context) syscall.Errno {
	if err := fh.file.Close(); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Lseek implements fs.FileLseeker.
func (fh *sandboxFileHandle) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	newOff, err := fh.file.Seek(int64(off), int(whence))
	if err != nil {
		return 0, toErrno(err)
	}
	return uint64(newOff), fs.OK
}

// Getattr implements fs.FileGetattrer.
func (fh *sandboxFileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	var st syscall.Stat_t
	if err := syscall.Fstat(int(fh.file.Fd()), &st); err != nil {
		return toErrno(err)
	}
	out.Attr.FromStat(&st)
	return fs.OK
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
