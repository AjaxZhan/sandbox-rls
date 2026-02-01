"""Preset permission templates for common use cases.

This module provides pre-defined permission configurations that cover
common scenarios when working with AI agents and sandboxed environments.
"""

from typing import Dict, List, Optional, Union

from .types import Permission, PatternType, PermissionRule


# Pre-defined permission templates
PRESETS: Dict[str, List[Dict[str, str]]] = {
    # Safe preset for AI agents: read most files, write to output, hide secrets
    "agent-safe": [
        {"pattern": "**/*", "permission": "read", "priority": "0"},
        {"pattern": "/output/**", "permission": "write", "priority": "10"},
        {"pattern": "/tmp/**", "permission": "write", "priority": "10"},
        {"pattern": "**/.env*", "permission": "none", "priority": "100"},
        {"pattern": "**/secrets/**", "permission": "none", "priority": "100"},
        {"pattern": "**/*.key", "permission": "none", "priority": "100"},
        {"pattern": "**/*.pem", "permission": "none", "priority": "100"},
        {"pattern": "**/credentials*", "permission": "none", "priority": "100"},
        {"pattern": "**/.git/**", "permission": "none", "priority": "50"},
    ],
    # Read-only access to all files
    "read-only": [
        {"pattern": "**/*", "permission": "read", "priority": "0"},
    ],
    # Full write access to all files
    "full-access": [
        {"pattern": "**/*", "permission": "write", "priority": "0"},
    ],
    # Development preset: full access except secrets
    "development": [
        {"pattern": "**/*", "permission": "write", "priority": "0"},
        {"pattern": "**/.env*", "permission": "none", "priority": "100"},
        {"pattern": "**/secrets/**", "permission": "none", "priority": "100"},
        {"pattern": "**/*.key", "permission": "none", "priority": "100"},
        {"pattern": "**/*.pem", "permission": "none", "priority": "100"},
    ],
    # View-only: can see file names but not read content
    "view-only": [
        {"pattern": "**/*", "permission": "view", "priority": "0"},
    ],
}


def get_preset(name: str) -> List[PermissionRule]:
    """Get a preset permission configuration by name.
    
    Args:
        name: The name of the preset (e.g., "agent-safe", "read-only").
        
    Returns:
        List of PermissionRule objects for the preset.
        
    Raises:
        ValueError: If the preset name is not found.
        
    Example:
        >>> rules = get_preset("agent-safe")
        >>> for rule in rules:
        ...     print(f"{rule.pattern}: {rule.permission.value}")
    """
    if name not in PRESETS:
        available = ", ".join(sorted(PRESETS.keys()))
        raise ValueError(f"Unknown preset '{name}'. Available presets: {available}")
    
    return [
        PermissionRule(
            pattern=rule["pattern"],
            permission=Permission(rule["permission"]),
            type=PatternType(rule.get("type", "glob")),
            priority=int(rule.get("priority", 0)),
        )
        for rule in PRESETS[name]
    ]


def get_preset_dicts(name: str) -> List[Dict[str, str]]:
    """Get a preset as a list of dictionaries (for direct API use).
    
    Args:
        name: The name of the preset.
        
    Returns:
        List of permission rule dictionaries.
        
    Raises:
        ValueError: If the preset name is not found.
    """
    if name not in PRESETS:
        available = ", ".join(sorted(PRESETS.keys()))
        raise ValueError(f"Unknown preset '{name}'. Available presets: {available}")
    
    return PRESETS[name].copy()


def extend_preset(
    base: str,
    additions: Optional[List[Union[PermissionRule, Dict]]] = None,
    overrides: Optional[List[Union[PermissionRule, Dict]]] = None,
) -> List[PermissionRule]:
    """Extend a preset with additional or overriding rules.
    
    Args:
        base: The name of the base preset to extend.
        additions: Additional rules to append (lower priority than base).
        overrides: Rules to add with higher priority (override base rules).
        
    Returns:
        Combined list of PermissionRule objects.
        
    Example:
        >>> # Allow write access to /workspace in addition to agent-safe rules
        >>> rules = extend_preset(
        ...     "agent-safe",
        ...     additions=[{"pattern": "/workspace/**", "permission": "write"}]
        ... )
    """
    rules = get_preset(base)
    
    def to_rule(item: Union[PermissionRule, Dict]) -> PermissionRule:
        if isinstance(item, PermissionRule):
            return item
        return PermissionRule(
            pattern=item["pattern"],
            permission=Permission(item["permission"]),
            type=PatternType(item.get("type", "glob")),
            priority=int(item.get("priority", 0)),
        )
    
    if additions:
        for item in additions:
            rules.append(to_rule(item))
    
    if overrides:
        # Add overrides with boosted priority
        max_priority = max(r.priority for r in rules) if rules else 0
        for item in overrides:
            rule = to_rule(item)
            # Ensure override has higher priority
            if rule.priority <= max_priority:
                rule = PermissionRule(
                    pattern=rule.pattern,
                    permission=rule.permission,
                    type=rule.type,
                    priority=max_priority + 100,
                )
            rules.append(rule)
    
    return rules


def list_presets() -> List[str]:
    """List all available preset names.
    
    Returns:
        Sorted list of preset names.
    """
    return sorted(PRESETS.keys())


def register_preset(name: str, rules: List[Dict[str, str]]) -> None:
    """Register a custom preset.
    
    Args:
        name: The name for the new preset.
        rules: List of permission rule dictionaries.
        
    Raises:
        ValueError: If a preset with the name already exists.
        
    Example:
        >>> register_preset("my-custom", [
        ...     {"pattern": "**/*", "permission": "read"},
        ...     {"pattern": "/src/**", "permission": "write"},
        ... ])
    """
    if name in PRESETS:
        raise ValueError(f"Preset '{name}' already exists. Use a different name.")
    
    PRESETS[name] = rules
