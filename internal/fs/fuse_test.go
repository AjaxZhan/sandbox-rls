package fs

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// checkFUSEAvailable checks if FUSE is available on the system.
func checkFUSEAvailable(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "darwin" {
		// Check for macFUSE
		if _, err := os.Stat("/Library/Filesystems/macfuse.fs"); os.IsNotExist(err) {
			t.Skip("skipping test: macFUSE is not installed (install from https://osxfuse.github.io/)")
		}
		// Check if mount_macfuse is available
		if _, err := exec.LookPath("mount_macfuse"); err != nil {
			t.Skip("skipping test: mount_macfuse not found in PATH")
		}
	} else if runtime.GOOS == "linux" {
		// Check for FUSE on Linux
		if _, err := os.Stat("/dev/fuse"); os.IsNotExist(err) {
			t.Skip("skipping test: FUSE is not available (/dev/fuse not found)")
		}
	} else {
		t.Skipf("skipping test: FUSE tests not supported on %s", runtime.GOOS)
	}
}

// ============================================================================
// Unit Tests (no FUSE mount required)
// ============================================================================

func TestSandboxFS_NewSandboxFS_ValidConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandboxfs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SandboxFSConfig{
		SourceDir:  tmpDir,
		MountPoint: "/tmp/test-mount",
		Rules: []types.PermissionRule{
			{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermRead, Priority: 1},
		},
	}

	fs, err := NewSandboxFS(config)
	if err != nil {
		t.Errorf("NewSandboxFS with valid config should succeed, got: %v", err)
	}
	if fs == nil {
		t.Error("expected non-nil SandboxFS")
	}
}

func TestSandboxFS_NewSandboxFS_InvalidSourceDir(t *testing.T) {
	config := &SandboxFSConfig{
		SourceDir:  "/nonexistent/path/that/does/not/exist",
		MountPoint: "/tmp/test-mount",
	}

	_, err := NewSandboxFS(config)
	if err == nil {
		t.Error("NewSandboxFS with invalid source dir should fail")
	}
}

func TestSandboxFS_NewSandboxFS_EmptySourceDir(t *testing.T) {
	config := &SandboxFSConfig{
		SourceDir:  "",
		MountPoint: "/tmp/test-mount",
	}

	_, err := NewSandboxFS(config)
	if err != ErrInvalidSourceDir {
		t.Errorf("expected ErrInvalidSourceDir, got: %v", err)
	}
}

func TestSandboxFS_NewSandboxFS_EmptyMountPoint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandboxfs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SandboxFSConfig{
		SourceDir:  tmpDir,
		MountPoint: "",
	}

	_, err = NewSandboxFS(config)
	if err != ErrInvalidMountPoint {
		t.Errorf("expected ErrInvalidMountPoint, got: %v", err)
	}
}

func TestSandboxFS_IsMounted_BeforeMount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandboxfs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SandboxFSConfig{
		SourceDir:  tmpDir,
		MountPoint: "/tmp/test-mount",
	}

	fs, err := NewSandboxFS(config)
	if err != nil {
		t.Fatalf("NewSandboxFS failed: %v", err)
	}

	if fs.IsMounted() {
		t.Error("IsMounted should return false before mount")
	}
}

func TestSandboxFS_UpdateRules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandboxfs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	initialRules := []types.PermissionRule{
		{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermNone, Priority: 1},
	}

	config := &SandboxFSConfig{
		SourceDir:  tmpDir,
		MountPoint: "/tmp/test-mount",
		Rules:      initialRules,
	}

	fs, err := NewSandboxFS(config)
	if err != nil {
		t.Fatalf("NewSandboxFS failed: %v", err)
	}

	// Initially, /test.txt should have none permission
	perm := fs.getPermission("/test.txt")
	if perm != types.PermNone {
		t.Errorf("initial permission should be none, got: %s", perm)
	}

	// Update rules
	newRules := []types.PermissionRule{
		{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermWrite, Priority: 1},
	}
	fs.UpdateRules(newRules)

	// Now /test.txt should have write permission
	perm = fs.getPermission("/test.txt")
	if perm != types.PermWrite {
		t.Errorf("updated permission should be write, got: %s", perm)
	}
}

func TestSandboxFS_CheckPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandboxfs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rules := []types.PermissionRule{
		{Pattern: "/readable", Type: types.PatternFile, Permission: types.PermRead, Priority: 10},
		{Pattern: "/writable", Type: types.PatternFile, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/viewable", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
	}

	config := &SandboxFSConfig{
		SourceDir:  tmpDir,
		MountPoint: "/tmp/test-mount",
		Rules:      rules,
	}

	fs, err := NewSandboxFS(config)
	if err != nil {
		t.Fatalf("NewSandboxFS failed: %v", err)
	}

	// Test checkRead
	if err := fs.checkRead("/readable"); err != nil {
		t.Errorf("checkRead(/readable) should succeed: %v", err)
	}
	if err := fs.checkRead("/viewable"); err == nil {
		t.Error("checkRead(/viewable) should fail")
	}

	// Test checkWrite
	if err := fs.checkWrite("/writable"); err != nil {
		t.Errorf("checkWrite(/writable) should succeed: %v", err)
	}
	if err := fs.checkWrite("/readable"); err == nil {
		t.Error("checkWrite(/readable) should fail")
	}

	// Test checkView
	if err := fs.checkView("/viewable"); err != nil {
		t.Errorf("checkView(/viewable) should succeed: %v", err)
	}
}

// ============================================================================
// Integration Tests (require FUSE mount)
// ============================================================================

// setupTestDir creates a temporary directory with test files for FUSE testing.
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "sandboxfs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create test directory structure
	// /
	// ├── docs/
	// │   ├── readme.md
	// │   └── secret.txt
	// ├── src/
	// │   ├── main.py
	// │   └── utils.py
	// └── config.yaml

	dirs := []string{
		filepath.Join(tmpDir, "docs"),
		filepath.Join(tmpDir, "src"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	files := map[string]string{
		filepath.Join(tmpDir, "docs", "readme.md"):  "# README\nThis is a readme file.",
		filepath.Join(tmpDir, "docs", "secret.txt"): "This is a secret file.",
		filepath.Join(tmpDir, "src", "main.py"):     "print('hello world')",
		filepath.Join(tmpDir, "src", "utils.py"):    "def helper(): pass",
		filepath.Join(tmpDir, "config.yaml"):        "key: value\n",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	return tmpDir
}

// setupTestMount creates a SandboxFS and mounts it.
// Returns the mount point and a cleanup function.
func setupTestMount(t *testing.T, sourceDir string, rules []types.PermissionRule) (string, func()) {
	t.Helper()

	// Check FUSE availability first
	checkFUSEAvailable(t)

	mountPoint, err := os.MkdirTemp("", "sandboxfs-mount-*")
	if err != nil {
		t.Fatalf("failed to create mount point: %v", err)
	}

	config := &SandboxFSConfig{
		SourceDir:  sourceDir,
		MountPoint: mountPoint,
		Rules:      rules,
	}

	sfs, err := NewSandboxFS(config)
	if err != nil {
		os.RemoveAll(mountPoint)
		t.Fatalf("failed to create SandboxFS: %v", err)
	}

	// Start mount in background
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- sfs.Mount(ctx)
	}()

	// Wait for mount to be ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if sfs.IsMounted() {
			break
		}
		select {
		case err := <-errCh:
			cancel()
			os.RemoveAll(mountPoint)
			t.Fatalf("mount failed: %v", err)
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if !sfs.IsMounted() {
		cancel()
		os.RemoveAll(mountPoint)
		t.Skip("skipping test: FUSE mount timed out (FUSE may not be properly configured)")
	}

	cleanup := func() {
		cancel()
		// Wait for unmount
		select {
		case <-errCh:
		case <-time.After(5 * time.Second):
			t.Log("warning: unmount timed out")
		}
		os.RemoveAll(mountPoint)
	}

	return mountPoint, cleanup
}

func TestSandboxFS_ReadDir_FilterByPermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root (FUSE behaves differently)")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	// Rules:
	// - /docs/ is writable
	// - /docs/secret.txt is hidden (none)
	// - /src/ is readable
	// - config.yaml is view only
	rules := []types.PermissionRule{
		{Pattern: "/docs/secret.txt", Type: types.PatternFile, Permission: types.PermNone, Priority: 100},
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
		{Pattern: "/config.yaml", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test reading root directory
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		t.Fatalf("ReadDir(/) failed: %v", err)
	}

	// Should see: docs, src, config.yaml (but not hidden files)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}

	if !names["docs"] {
		t.Error("expected 'docs' directory to be visible")
	}
	if !names["src"] {
		t.Error("expected 'src' directory to be visible")
	}
	if !names["config.yaml"] {
		t.Error("expected 'config.yaml' to be visible")
	}

	// Test reading /docs - should not include secret.txt (permission: none)
	docsEntries, err := os.ReadDir(filepath.Join(mountPoint, "docs"))
	if err != nil {
		t.Fatalf("ReadDir(/docs) failed: %v", err)
	}

	docsNames := make(map[string]bool)
	for _, e := range docsEntries {
		docsNames[e.Name()] = true
	}

	if !docsNames["readme.md"] {
		t.Error("expected 'readme.md' to be visible in /docs")
	}
	if docsNames["secret.txt"] {
		t.Error("'secret.txt' should be hidden (permission: none)")
	}
}

func TestSandboxFS_Read_RequiresReadPermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/readme.md", Type: types.PatternFile, Permission: types.PermRead, Priority: 10},
		{Pattern: "/config.yaml", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test reading a file with read permission - should succeed
	content, err := os.ReadFile(filepath.Join(mountPoint, "docs", "readme.md"))
	if err != nil {
		t.Errorf("ReadFile(/docs/readme.md) should succeed, got: %v", err)
	}
	if string(content) != "# README\nThis is a readme file." {
		t.Errorf("ReadFile content mismatch, got: %q", string(content))
	}

	// Test reading a file with view permission only - should fail
	_, err = os.ReadFile(filepath.Join(mountPoint, "config.yaml"))
	if err == nil {
		t.Error("ReadFile(/config.yaml) should fail with view permission")
	}
	// Should be a permission error
	if !os.IsPermission(err) {
		t.Errorf("expected permission error, got: %v", err)
	}
}

func TestSandboxFS_Write_RequiresWritePermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test writing to a file with write permission - should succeed
	newContent := "# Updated README\nThis is updated."
	err := os.WriteFile(filepath.Join(mountPoint, "docs", "readme.md"), []byte(newContent), 0644)
	if err != nil {
		t.Errorf("WriteFile(/docs/readme.md) should succeed, got: %v", err)
	}

	// Verify the write
	content, err := os.ReadFile(filepath.Join(mountPoint, "docs", "readme.md"))
	if err != nil {
		t.Errorf("ReadFile after write failed: %v", err)
	}
	if string(content) != newContent {
		t.Errorf("content mismatch after write, got: %q", string(content))
	}

	// Test writing to a file with read permission only - should fail
	err = os.WriteFile(filepath.Join(mountPoint, "src", "main.py"), []byte("new content"), 0644)
	if err == nil {
		t.Error("WriteFile(/src/main.py) should fail with read permission")
	}
	if !os.IsPermission(err) {
		t.Errorf("expected permission error, got: %v", err)
	}
}

func TestSandboxFS_CreateFile_RequiresWritePermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test creating a new file in writable directory
	newFile := filepath.Join(mountPoint, "docs", "newfile.txt")
	err := os.WriteFile(newFile, []byte("new file content"), 0644)
	if err != nil {
		t.Errorf("creating new file in /docs should succeed, got: %v", err)
	}

	// Test creating a new file in read-only directory - should fail
	readOnlyFile := filepath.Join(mountPoint, "src", "newfile.py")
	err = os.WriteFile(readOnlyFile, []byte("new content"), 0644)
	if err == nil {
		t.Error("creating new file in /src should fail with read permission")
	}
}

func TestSandboxFS_Stat_ViewPermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/config.yaml", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
		{Pattern: "/docs/secret.txt", Type: types.PatternFile, Permission: types.PermNone, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test stat on view permission - should succeed
	info, err := os.Stat(filepath.Join(mountPoint, "config.yaml"))
	if err != nil {
		t.Errorf("Stat(/config.yaml) should succeed with view permission, got: %v", err)
	}
	if info == nil {
		t.Error("expected non-nil FileInfo")
	}

	// Test stat on none permission - should fail (file not visible)
	_, err = os.Stat(filepath.Join(mountPoint, "docs", "secret.txt"))
	if err == nil {
		t.Error("Stat(/docs/secret.txt) should fail with none permission")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected 'not exist' error for hidden file, got: %v", err)
	}
}

func TestSandboxFS_DeleteFile_RequiresWritePermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test deleting a file with write permission - should succeed
	err := os.Remove(filepath.Join(mountPoint, "docs", "readme.md"))
	if err != nil {
		t.Errorf("Remove(/docs/readme.md) should succeed, got: %v", err)
	}

	// Verify the file is gone
	_, err = os.Stat(filepath.Join(mountPoint, "docs", "readme.md"))
	if !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}

	// Test deleting a file with read permission only - should fail
	err = os.Remove(filepath.Join(mountPoint, "src", "main.py"))
	if err == nil {
		t.Error("Remove(/src/main.py) should fail with read permission")
	}
	if !os.IsPermission(err) {
		t.Errorf("expected permission error, got: %v", err)
	}
}

func TestSandboxFS_Mkdir_RequiresWritePermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test creating directory with write permission - should succeed
	err := os.Mkdir(filepath.Join(mountPoint, "docs", "subdir"), 0755)
	if err != nil {
		t.Errorf("Mkdir(/docs/subdir) should succeed, got: %v", err)
	}

	// Test creating directory with read permission only - should fail
	err = os.Mkdir(filepath.Join(mountPoint, "src", "subdir"), 0755)
	if err == nil {
		t.Error("Mkdir(/src/subdir) should fail with read permission")
	}
}

func TestSandboxFS_OpenFile_Modes(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/readme.md", Type: types.PatternFile, Permission: types.PermRead, Priority: 10},
		{Pattern: "/config.yaml", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test opening for read with read permission - should succeed
	f, err := os.OpenFile(filepath.Join(mountPoint, "docs", "readme.md"), os.O_RDONLY, 0)
	if err != nil {
		t.Errorf("OpenFile(O_RDONLY) should succeed with read permission, got: %v", err)
	} else {
		f.Close()
	}

	// Test opening for write with read permission - should fail
	f, err = os.OpenFile(filepath.Join(mountPoint, "docs", "readme.md"), os.O_WRONLY, 0)
	if err == nil {
		f.Close()
		t.Error("OpenFile(O_WRONLY) should fail with read permission")
	}

	// Test opening for read with view permission - should fail
	f, err = os.OpenFile(filepath.Join(mountPoint, "config.yaml"), os.O_RDONLY, 0)
	if err == nil {
		f.Close()
		t.Error("OpenFile(O_RDONLY) should fail with view permission")
	}
}

func TestSandboxFS_Truncate(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test truncate with write permission - should succeed
	err := os.Truncate(filepath.Join(mountPoint, "docs", "readme.md"), 5)
	if err != nil {
		t.Errorf("Truncate(/docs/readme.md) should succeed, got: %v", err)
	}

	// Verify truncation
	content, _ := os.ReadFile(filepath.Join(mountPoint, "docs", "readme.md"))
	if len(content) != 5 {
		t.Errorf("expected truncated size 5, got %d", len(content))
	}

	// Test truncate with read permission - should fail
	err = os.Truncate(filepath.Join(mountPoint, "src", "main.py"), 5)
	if err == nil {
		t.Error("Truncate(/src/main.py) should fail with read permission")
	}
}

func TestSandboxFS_Rename_RequiresWritePermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/src/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test rename within writable directory - should succeed
	oldPath := filepath.Join(mountPoint, "docs", "readme.md")
	newPath := filepath.Join(mountPoint, "docs", "README.md")
	err := os.Rename(oldPath, newPath)
	if err != nil {
		t.Errorf("Rename within /docs should succeed, got: %v", err)
	}

	// Verify rename
	_, err = os.Stat(newPath)
	if err != nil {
		t.Error("renamed file should exist")
	}

	// Test rename in read-only directory - should fail
	err = os.Rename(
		filepath.Join(mountPoint, "src", "main.py"),
		filepath.Join(mountPoint, "src", "MAIN.py"),
	)
	if err == nil {
		t.Error("Rename in /src should fail with read permission")
	}
}

func TestSandboxFS_Symlink_Handling(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	// Create a symlink in the source directory
	linkPath := filepath.Join(sourceDir, "docs", "link.md")
	targetPath := filepath.Join(sourceDir, "docs", "readme.md")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test reading symlink content (should follow link and check target permissions)
	_, err := os.ReadFile(filepath.Join(mountPoint, "docs", "link.md"))
	if err != nil {
		t.Errorf("reading symlink should succeed with read permission, got: %v", err)
	}

	// Test Lstat (should see symlink)
	info, err := os.Lstat(filepath.Join(mountPoint, "docs", "link.md"))
	if err != nil {
		t.Errorf("Lstat on symlink failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink mode")
	}
}

func TestSandboxFS_Chmod_Ignored(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Chmod should succeed but may be ignored (depends on implementation)
	// At minimum, it should not crash
	err := os.Chmod(filepath.Join(mountPoint, "docs", "readme.md"), 0755)
	if err != nil && !os.IsPermission(err) {
		t.Errorf("Chmod should either succeed or return permission error, got: %v", err)
	}
}

func TestSandboxFS_LargeFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	// Create a large file (1MB)
	largeFilePath := filepath.Join(sourceDir, "docs", "large.bin")
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(largeFilePath, largeContent, 0644); err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test reading large file
	readContent, err := os.ReadFile(filepath.Join(mountPoint, "docs", "large.bin"))
	if err != nil {
		t.Fatalf("ReadFile(large.bin) failed: %v", err)
	}

	if len(readContent) != len(largeContent) {
		t.Errorf("large file size mismatch: got %d, want %d", len(readContent), len(largeContent))
	}

	// Verify content
	for i := 0; i < 100; i++ { // Check first 100 bytes
		if readContent[i] != largeContent[i] {
			t.Errorf("content mismatch at byte %d", i)
			break
		}
	}
}

func TestSandboxFS_ConcurrentAccess(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Concurrent reads and writes
	done := make(chan error, 10)

	for i := 0; i < 5; i++ {
		go func(n int) {
			_, err := os.ReadFile(filepath.Join(mountPoint, "docs", "readme.md"))
			done <- err
		}(i)
	}

	for i := 0; i < 5; i++ {
		go func(n int) {
			content := []byte("concurrent write " + string(rune('0'+n)))
			err := os.WriteFile(filepath.Join(mountPoint, "docs", "readme.md"), content, 0644)
			done <- err
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent operation failed: %v", err)
		}
	}
}

func TestSandboxFS_SeekAndRead(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test seek and read
	f, err := os.Open(filepath.Join(mountPoint, "docs", "readme.md"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Seek to position 2 (skip "# ")
	_, err = f.Seek(2, io.SeekStart)
	if err != nil {
		t.Errorf("Seek failed: %v", err)
	}

	// Read 6 bytes ("README")
	buf := make([]byte, 6)
	n, err := f.Read(buf)
	if err != nil {
		t.Errorf("Read after seek failed: %v", err)
	}
	if n != 6 || string(buf) != "README" {
		t.Errorf("expected 'README', got %q (n=%d)", string(buf[:n]), n)
	}
}

func TestSandboxFS_UpdateRules_WithMount(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}
	checkFUSEAvailable(t)

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	// Start with read-only rule
	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermRead, Priority: 10},
	}

	mountPoint, err := os.MkdirTemp("", "sandboxfs-mount-*")
	if err != nil {
		t.Fatalf("failed to create mount point: %v", err)
	}
	defer os.RemoveAll(mountPoint)

	config := &SandboxFSConfig{
		SourceDir:  sourceDir,
		MountPoint: mountPoint,
		Rules:      rules,
	}

	sfs, err := NewSandboxFS(config)
	if err != nil {
		t.Fatalf("failed to create SandboxFS: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sfs.Mount(ctx)
	}()

	// Wait for mount
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !sfs.IsMounted() {
		time.Sleep(50 * time.Millisecond)
	}
	if !sfs.IsMounted() {
		t.Skip("skipping test: FUSE mount timed out")
	}

	// Initially, write should fail
	err = os.WriteFile(filepath.Join(mountPoint, "docs", "readme.md"), []byte("new"), 0644)
	if err == nil {
		t.Error("write should fail with read permission")
	}

	// Update rules to allow write
	newRules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
	}
	sfs.UpdateRules(newRules)

	// Now write should succeed
	err = os.WriteFile(filepath.Join(mountPoint, "docs", "readme.md"), []byte("new"), 0644)
	if err != nil {
		t.Errorf("write should succeed after rule update, got: %v", err)
	}
}

func TestSandboxFS_ErrorCodes(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test as root")
	}

	sourceDir := setupTestDir(t)
	defer os.RemoveAll(sourceDir)

	rules := []types.PermissionRule{
		{Pattern: "/docs/secret.txt", Type: types.PatternFile, Permission: types.PermNone, Priority: 100},
		{Pattern: "/config.yaml", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
	}

	mountPoint, cleanup := setupTestMount(t, sourceDir, rules)
	defer cleanup()

	// Test ENOENT for hidden file
	_, err := os.Stat(filepath.Join(mountPoint, "docs", "secret.txt"))
	if !os.IsNotExist(err) {
		t.Errorf("expected ENOENT for hidden file, got: %v", err)
	}

	// Test EACCES for permission denied
	_, err = os.ReadFile(filepath.Join(mountPoint, "config.yaml"))
	if err == nil {
		t.Error("expected error for view-only file")
	}
	if !os.IsPermission(err) {
		// Check if it's wrapped in PathError with EACCES
		if pe, ok := err.(*os.PathError); ok {
			if pe.Err != syscall.EACCES {
				t.Errorf("expected EACCES, got: %v", pe.Err)
			}
		} else {
			t.Errorf("expected permission error, got: %v", err)
		}
	}
}
