// Package fs provides FUSE filesystem implementation with permission control.
package fs

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/AjaxZhan/AgentFense/pkg/types"
)

// PermissionEngine handles permission checking for file operations.
type PermissionEngine interface {
	// GetPermission returns the effective permission for a given path.
	GetPermission(path string) types.Permission

	// CheckRead checks if the path can be read.
	CheckRead(path string) error

	// CheckWrite checks if the path can be written.
	CheckWrite(path string) error

	// CheckView checks if the path can be viewed (listed).
	CheckView(path string) error

	// UpdateRules updates the permission rules.
	UpdateRules(rules []types.PermissionRule)
}

// permissionEngine is the default implementation of PermissionEngine.
type permissionEngine struct {
	rules []types.PermissionRule
}

// NewPermissionEngine creates a new permission engine with the given rules.
func NewPermissionEngine(rules []types.PermissionRule) PermissionEngine {
	pe := &permissionEngine{}
	pe.UpdateRules(rules)
	return pe
}

// UpdateRules updates the permission rules and sorts them by priority.
func (pe *permissionEngine) UpdateRules(rules []types.PermissionRule) {
	// Copy rules to avoid external modification
	pe.rules = make([]types.PermissionRule, len(rules))
	copy(pe.rules, rules)

	// Sort by priority (higher priority first)
	// If priority is equal: file > directory > glob
	// If type is also equal: more specific patterns first (longer prefix, not starting with **)
	sort.Slice(pe.rules, func(i, j int) bool {
		if pe.rules[i].Priority != pe.rules[j].Priority {
			return pe.rules[i].Priority > pe.rules[j].Priority
		}
		typePriorityI := patternTypePriority(pe.rules[i].Type)
		typePriorityJ := patternTypePriority(pe.rules[j].Type)
		if typePriorityI != typePriorityJ {
			return typePriorityI > typePriorityJ
		}
		// For same type, more specific patterns come first
		return patternSpecificity(pe.rules[i].Pattern) > patternSpecificity(pe.rules[j].Pattern)
	})
}

// patternSpecificity calculates how specific a pattern is.
// Higher values mean more specific (should match first).
func patternSpecificity(pattern string) int {
	specificity := 0

	// Patterns starting with "/" are more specific than "**" patterns
	if strings.HasPrefix(pattern, "/") {
		specificity += 100
	}

	// Patterns NOT starting with "**" are more specific
	if !strings.HasPrefix(pattern, "**") {
		specificity += 50
	}

	// Longer prefix (before **) means more specific
	if idx := strings.Index(pattern, "**"); idx > 0 {
		specificity += idx
	}

	// Exact paths (no wildcards) are most specific
	if !strings.Contains(pattern, "*") {
		specificity += 200
	}

	return specificity
}

// patternTypePriority returns the priority of a pattern type.
func patternTypePriority(t types.PatternType) int {
	switch t {
	case types.PatternFile:
		return 3
	case types.PatternDirectory:
		return 2
	case types.PatternGlob:
		return 1
	default:
		return 0
	}
}

// GetPermission returns the effective permission for a given path.
func (pe *permissionEngine) GetPermission(path string) types.Permission {
	// Normalize path
	path = normalizePath(path)

	// Find matching rule with highest priority
	for _, rule := range pe.rules {
		if pe.matchRule(rule, path) {
			return rule.Permission
		}
	}

	// Default: no permission (invisible)
	return types.PermNone
}

// matchRule checks if a path matches a permission rule.
func (pe *permissionEngine) matchRule(rule types.PermissionRule, path string) bool {
	pattern := normalizePath(rule.Pattern)

	switch rule.Type {
	case types.PatternFile:
		// Exact file match
		return path == pattern

	case types.PatternDirectory:
		// Directory match: path is in or under the directory
		if !strings.HasSuffix(pattern, "/") {
			pattern += "/"
		}
		return path == strings.TrimSuffix(pattern, "/") || strings.HasPrefix(path+"/", pattern) || strings.HasPrefix(path, pattern)

	case types.PatternGlob:
		// Glob pattern match
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
		// Try matching with double star expansion
		if strings.Contains(pattern, "**") {
			return matchDoubleGlob(pattern, path)
		}
		return false

	default:
		return false
	}
}

// matchDoubleGlob handles ** glob patterns.
func matchDoubleGlob(pattern, path string) bool {
	// Handle **/ prefix (match any directory path)
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		// Check if the basename matches the suffix pattern
		basename := filepath.Base(path)
		matched, _ := filepath.Match(suffix, basename)
		return matched
	}

	// Handle general ** patterns like "/secrets/**" or "/docs/**"
	parts := strings.Split(pattern, "**")
	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]

		// Remove trailing slash from prefix for comparison
		// e.g., "/secrets/" -> "/secrets"
		prefixClean := strings.TrimSuffix(prefix, "/")

		// Check if path matches the prefix exactly (directory itself)
		// or if path starts with the prefix (files/subdirs inside)
		if prefixClean != "" {
			// Match the directory itself: /secrets matches /secrets/**
			if path == prefixClean {
				return true
			}
			// Match contents: /secrets/file.txt matches /secrets/**
			if !strings.HasPrefix(path, prefixClean+"/") && !strings.HasPrefix(path, prefix) {
				return false
			}
		}

		// For suffix, we need to check if it matches the end of the path
		if suffix == "" {
			return true
		}

		// If suffix starts with /, it should match exactly from some point
		if strings.HasPrefix(suffix, "/") {
			// suffix like "/*.py" - match any path ending with .py in any subdir
			suffixPattern := strings.TrimPrefix(suffix, "/")
			basename := filepath.Base(path)
			matched, _ := filepath.Match(suffixPattern, basename)
			return matched
		}

		hasSuffix := strings.HasSuffix(path, suffix)
		return hasSuffix
	}
	return false
}

// normalizePath normalizes a file path for comparison.
func normalizePath(path string) string {
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Clean the path
	path = filepath.Clean(path)
	return path
}

// CheckView checks if the path can be viewed (listed).
func (pe *permissionEngine) CheckView(path string) error {
	perm := pe.GetPermission(path)
	if perm.Level() < types.PermView.Level() {
		return &types.PermissionError{
			Path:       path,
			Operation:  "view",
			Permission: perm,
			Required:   types.PermView,
		}
	}
	return nil
}

// CheckRead checks if the path can be read.
func (pe *permissionEngine) CheckRead(path string) error {
	perm := pe.GetPermission(path)
	if perm.Level() < types.PermRead.Level() {
		return &types.PermissionError{
			Path:       path,
			Operation:  "read",
			Permission: perm,
			Required:   types.PermRead,
		}
	}
	return nil
}

// CheckWrite checks if the path can be written.
func (pe *permissionEngine) CheckWrite(path string) error {
	perm := pe.GetPermission(path)
	if perm.Level() < types.PermWrite.Level() {
		return &types.PermissionError{
			Path:       path,
			Operation:  "write",
			Permission: perm,
			Required:   types.PermWrite,
		}
	}
	return nil
}
