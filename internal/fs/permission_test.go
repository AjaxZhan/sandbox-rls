package fs

import (
	"testing"

	"github.com/AjaxZhan/AgentFense/pkg/types"
)

func TestPermissionEngine_FilePattern(t *testing.T) {
	rules := []types.PermissionRule{
		{Pattern: "/config.yaml", Type: types.PatternFile, Permission: types.PermRead, Priority: 10},
		{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermNone, Priority: 1},
	}

	pe := NewPermissionEngine(rules)

	tests := []struct {
		path     string
		expected types.Permission
	}{
		{"/config.yaml", types.PermRead},
		{"/other.yaml", types.PermNone},
		{"/some/path/config.yaml", types.PermNone}, // Not exact match
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pe.GetPermission(tt.path)
			if got != tt.expected {
				t.Errorf("GetPermission(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPermissionEngine_DirectoryPattern(t *testing.T) {
	rules := []types.PermissionRule{
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermNone, Priority: 1},
	}

	pe := NewPermissionEngine(rules)

	tests := []struct {
		path     string
		expected types.Permission
	}{
		{"/docs", types.PermWrite},
		{"/docs/", types.PermWrite},
		{"/docs/readme.md", types.PermWrite},
		{"/docs/sub/file.txt", types.PermWrite},
		{"/other/docs/file.txt", types.PermNone},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pe.GetPermission(tt.path)
			if got != tt.expected {
				t.Errorf("GetPermission(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPermissionEngine_GlobPattern(t *testing.T) {
	rules := []types.PermissionRule{
		{Pattern: "*.md", Type: types.PatternGlob, Permission: types.PermRead, Priority: 5},
		{Pattern: "**/*.py", Type: types.PatternGlob, Permission: types.PermView, Priority: 5},
		{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermNone, Priority: 1},
	}

	pe := NewPermissionEngine(rules)

	tests := []struct {
		path     string
		expected types.Permission
	}{
		{"/readme.md", types.PermRead},
		{"/docs/guide.md", types.PermNone}, // *.md doesn't match subdirs
		{"/src/main.py", types.PermView},
		{"/src/lib/utils.py", types.PermView},
		{"/file.txt", types.PermNone},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pe.GetPermission(tt.path)
			if got != tt.expected {
				t.Errorf("GetPermission(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPermissionEngine_Priority(t *testing.T) {
	// File pattern should override directory pattern
	rules := []types.PermissionRule{
		{Pattern: "/docs/secret.txt", Type: types.PatternFile, Permission: types.PermNone, Priority: 100},
		{Pattern: "/docs/", Type: types.PatternDirectory, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/**", Type: types.PatternGlob, Permission: types.PermRead, Priority: 1},
	}

	pe := NewPermissionEngine(rules)

	tests := []struct {
		path     string
		expected types.Permission
	}{
		{"/docs/readme.md", types.PermWrite}, // Directory rule
		{"/docs/secret.txt", types.PermNone}, // File rule overrides
		{"/other/file.txt", types.PermRead},  // Glob fallback
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pe.GetPermission(tt.path)
			if got != tt.expected {
				t.Errorf("GetPermission(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPermissionEngine_CheckPermissions(t *testing.T) {
	rules := []types.PermissionRule{
		{Pattern: "/readable", Type: types.PatternFile, Permission: types.PermRead, Priority: 10},
		{Pattern: "/writable", Type: types.PatternFile, Permission: types.PermWrite, Priority: 10},
		{Pattern: "/viewable", Type: types.PatternFile, Permission: types.PermView, Priority: 10},
		{Pattern: "/hidden", Type: types.PatternFile, Permission: types.PermNone, Priority: 10},
	}

	pe := NewPermissionEngine(rules)

	// CheckView tests
	if err := pe.CheckView("/viewable"); err != nil {
		t.Errorf("CheckView(/viewable) should succeed, got %v", err)
	}
	if err := pe.CheckView("/hidden"); err == nil {
		t.Error("CheckView(/hidden) should fail")
	}

	// CheckRead tests
	if err := pe.CheckRead("/readable"); err != nil {
		t.Errorf("CheckRead(/readable) should succeed, got %v", err)
	}
	if err := pe.CheckRead("/viewable"); err == nil {
		t.Error("CheckRead(/viewable) should fail")
	}

	// CheckWrite tests
	if err := pe.CheckWrite("/writable"); err != nil {
		t.Errorf("CheckWrite(/writable) should succeed, got %v", err)
	}
	if err := pe.CheckWrite("/readable"); err == nil {
		t.Error("CheckWrite(/readable) should fail")
	}
}

func TestPermissionEngine_DefaultPermission(t *testing.T) {
	// Empty rules should default to PermNone
	pe := NewPermissionEngine(nil)

	got := pe.GetPermission("/any/path")
	if got != types.PermNone {
		t.Errorf("GetPermission with empty rules = %q, want %q", got, types.PermNone)
	}
}

func TestPermissionEngine_PathNormalization(t *testing.T) {
	rules := []types.PermissionRule{
		{Pattern: "/docs/readme.md", Type: types.PatternFile, Permission: types.PermRead, Priority: 10},
	}

	pe := NewPermissionEngine(rules)

	// All these should match
	paths := []string{
		"/docs/readme.md",
		"docs/readme.md",    // Without leading slash
		"/docs//readme.md",  // Double slash (will be cleaned)
		"/docs/./readme.md", // With dot
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			got := pe.GetPermission(path)
			if got != types.PermRead {
				t.Errorf("GetPermission(%q) = %q, want %q", path, got, types.PermRead)
			}
		})
	}
}
