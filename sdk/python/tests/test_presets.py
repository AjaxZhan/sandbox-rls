"""Tests for the presets module."""

import pytest
from sandbox_sdk.presets import (
    PRESETS,
    get_preset,
    get_preset_dicts,
    extend_preset,
    list_presets,
    register_preset,
)
from sandbox_sdk.types import Permission, PatternType, PermissionRule


class TestGetPreset:
    """Tests for get_preset function."""
    
    def test_get_agent_safe_preset(self):
        """Test getting the agent-safe preset."""
        rules = get_preset("agent-safe")
        
        assert len(rules) > 0
        assert all(isinstance(r, PermissionRule) for r in rules)
        
        # Check that it has read-all rule
        patterns = [r.pattern for r in rules]
        assert "**/*" in patterns
        
        # Check that secrets are hidden
        permissions_by_pattern = {r.pattern: r.permission for r in rules}
        assert permissions_by_pattern.get("**/.env*") == Permission.NONE
        assert permissions_by_pattern.get("**/secrets/**") == Permission.NONE
    
    def test_get_read_only_preset(self):
        """Test getting the read-only preset."""
        rules = get_preset("read-only")
        
        assert len(rules) == 1
        assert rules[0].pattern == "**/*"
        assert rules[0].permission == Permission.READ
    
    def test_get_full_access_preset(self):
        """Test getting the full-access preset."""
        rules = get_preset("full-access")
        
        assert len(rules) == 1
        assert rules[0].pattern == "**/*"
        assert rules[0].permission == Permission.WRITE
    
    def test_get_unknown_preset_raises(self):
        """Test that unknown preset raises ValueError."""
        with pytest.raises(ValueError) as exc_info:
            get_preset("unknown-preset")
        
        assert "Unknown preset" in str(exc_info.value)
        assert "unknown-preset" in str(exc_info.value)


class TestGetPresetDicts:
    """Tests for get_preset_dicts function."""
    
    def test_get_preset_dicts_returns_list(self):
        """Test that get_preset_dicts returns a list of dicts."""
        rules = get_preset_dicts("read-only")
        
        assert isinstance(rules, list)
        assert len(rules) > 0
        assert all(isinstance(r, dict) for r in rules)
    
    def test_get_preset_dicts_has_required_keys(self):
        """Test that each rule has required keys."""
        rules = get_preset_dicts("agent-safe")
        
        for rule in rules:
            assert "pattern" in rule
            assert "permission" in rule
    
    def test_get_unknown_preset_dicts_raises(self):
        """Test that unknown preset raises ValueError."""
        with pytest.raises(ValueError):
            get_preset_dicts("nonexistent")


class TestExtendPreset:
    """Tests for extend_preset function."""
    
    def test_extend_with_additions(self):
        """Test extending a preset with additional rules."""
        rules = extend_preset(
            "read-only",
            additions=[{"pattern": "/output/**", "permission": "write"}]
        )
        
        # Should have original rule + addition
        assert len(rules) == 2
        patterns = [r.pattern for r in rules]
        assert "**/*" in patterns
        assert "/output/**" in patterns
    
    def test_extend_with_overrides(self):
        """Test extending a preset with override rules."""
        rules = extend_preset(
            "read-only",
            overrides=[{"pattern": "/src/**", "permission": "write", "priority": "50"}]
        )
        
        # Override should have higher priority
        override_rule = next(r for r in rules if r.pattern == "/src/**")
        base_rule = next(r for r in rules if r.pattern == "**/*")
        assert override_rule.priority > base_rule.priority
    
    def test_extend_with_permission_rule_objects(self):
        """Test extending with PermissionRule objects."""
        addition = PermissionRule(
            pattern="/custom/**",
            permission=Permission.WRITE,
        )
        rules = extend_preset("read-only", additions=[addition])
        
        assert len(rules) == 2
        custom_rule = next(r for r in rules if r.pattern == "/custom/**")
        assert custom_rule.permission == Permission.WRITE


class TestListPresets:
    """Tests for list_presets function."""
    
    def test_list_presets_returns_sorted_list(self):
        """Test that list_presets returns a sorted list."""
        presets = list_presets()
        
        assert isinstance(presets, list)
        assert len(presets) > 0
        assert presets == sorted(presets)
    
    def test_list_presets_contains_known_presets(self):
        """Test that list includes known presets."""
        presets = list_presets()
        
        assert "agent-safe" in presets
        assert "read-only" in presets
        assert "full-access" in presets


class TestRegisterPreset:
    """Tests for register_preset function."""
    
    def test_register_new_preset(self):
        """Test registering a new preset."""
        # Use a unique name to avoid conflicts with other tests
        preset_name = "test-custom-unique-12345"
        rules = [
            {"pattern": "**/*", "permission": "read"},
            {"pattern": "/custom/**", "permission": "write"},
        ]
        
        try:
            register_preset(preset_name, rules)
            
            # Should be able to get it
            retrieved = get_preset_dicts(preset_name)
            assert len(retrieved) == 2
            assert preset_name in list_presets()
        finally:
            # Clean up
            if preset_name in PRESETS:
                del PRESETS[preset_name]
    
    def test_register_existing_preset_raises(self):
        """Test that registering an existing preset raises ValueError."""
        with pytest.raises(ValueError) as exc_info:
            register_preset("read-only", [{"pattern": "**/*", "permission": "write"}])
        
        assert "already exists" in str(exc_info.value)


class TestPresetsPriority:
    """Tests for preset priority handling."""
    
    def test_agent_safe_priorities(self):
        """Test that agent-safe preset has correct priority ordering."""
        rules = get_preset("agent-safe")
        
        # Find specific rules
        base_rule = next(r for r in rules if r.pattern == "**/*")
        secret_rule = next(r for r in rules if r.pattern == "**/.env*")
        
        # Secret rules should have higher priority than base
        assert secret_rule.priority > base_rule.priority
    
    def test_preset_rules_have_valid_types(self):
        """Test that all preset rules have valid pattern types."""
        for preset_name in list_presets():
            rules = get_preset(preset_name)
            for rule in rules:
                assert isinstance(rule.type, PatternType)
                assert isinstance(rule.permission, Permission)
