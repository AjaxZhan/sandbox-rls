# Permissions Guide

AgentFense provides **path-level access control** for sandbox file operations. This guide covers permission levels, patterns, presets, and best practices for securing your sandboxes.

---

## Table of Contents

- [Permission Levels](#permission-levels)
- [Permission Rules](#permission-rules)
- [Pattern Types](#pattern-types)
- [Built-in Presets](#built-in-presets)
- [Custom Permissions](#custom-permissions)
- [Extending Presets](#extending-presets)
- [Rule Priority](#rule-priority)
- [Common Patterns](#common-patterns)
- [Best Practices](#best-practices)

---

## Permission Levels

AgentFense supports **four permission levels**, from most restrictive to least:

| Level | Symbol | Description | `ls` | `cat` | Write |
|-------|--------|-------------|------|-------|-------|
| **NONE** | `none` | Completely invisible | ❌ | ❌ | ❌ |
| **VIEW** | `view` | Visible in directory listings | ✓ | ❌ | ❌ |
| **READ** | `read` | Can read file contents | ✓ | ✓ | ❌ |
| **WRITE** | `write` | Full read/write access | ✓ | ✓ | ✓ |

### NONE - Completely Invisible

Files with `none` permission **don't appear in directory listings** and return "no such file" errors.

```python
from agentfense import Sandbox, PermissionRule, Permission

with Sandbox.from_local(
    "./project",
    permissions=[
        PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    ]
) as sandbox:
    # ❌ File is invisible
    result = sandbox.run("ls /workspace/secrets/")
    print(result.stderr)  # No such file or directory
    
    result = sandbox.run("cat /workspace/secrets/api.key")
    print(result.stderr)  # No such file or directory
```

**Use when**: Secrets, credentials, private keys must be completely hidden.

---

### VIEW - List Only

Files are **visible in `ls`** but cannot be read.

```python
permissions=[
    PermissionRule(pattern="/config/**", permission=Permission.VIEW),
]

# ✓ Can see files exist
result = sandbox.run("ls /workspace/config/")
print(result.stdout)  # database.yaml  api.yaml

# ❌ Cannot read contents
result = sandbox.run("cat /workspace/config/database.yaml")
print(result.stderr)  # Permission denied
```

**Use when**: You want the agent to know files exist but not read them (e.g., config file templates).

---

### READ - Read Only

Files can be **read but not modified**.

```python
permissions=[
    PermissionRule(pattern="**/*.py", permission=Permission.READ),
]

# ✓ Can read
result = sandbox.run("cat /workspace/main.py")
print(result.stdout)  # (file contents)

# ❌ Cannot write
result = sandbox.run("echo 'new line' >> /workspace/main.py")
print(result.stderr)  # Permission denied
```

**Use when**: Static analysis, code review, inspection without modification risk.

---

### WRITE - Full Access

Files can be **created, read, modified, and deleted**.

```python
permissions=[
    PermissionRule(pattern="/output/**", permission=Permission.WRITE),
]

# ✓ Can create
sandbox.run("echo 'result' > /workspace/output/result.txt")

# ✓ Can read
result = sandbox.run("cat /workspace/output/result.txt")

# ✓ Can modify
sandbox.run("echo 'more' >> /workspace/output/result.txt")

# ✓ Can delete
sandbox.run("rm /workspace/output/result.txt")
```

**Use when**: Output directories, temporary files, build artifacts.

---

## Permission Rules

A **PermissionRule** defines access control for a file pattern.

### PermissionRule Class

```python
from agentfense import PermissionRule, Permission, PatternType

rule = PermissionRule(
    pattern="**/*.py",
    permission=Permission.READ,
    type=PatternType.GLOB,
    priority=0,
)
```

#### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `pattern` | `str` | **Required** | File pattern to match |
| `permission` | `Permission` | `Permission.READ` | Access level |
| `type` | `PatternType` | `PatternType.GLOB` | Pattern matching type |
| `priority` | `int` | `0` | Rule priority (higher wins) |

---

### Dictionary Format

You can also specify rules as dictionaries:

```python
permissions=[
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
]
```

---

## Pattern Types

AgentFense supports three pattern matching types:

### GLOB - Wildcard Patterns (Default)

Uses `**` and `*` wildcards for flexible matching.

```python
PatternType.GLOB  # or omit (default)

# Examples:
"**/*.py"           # All .py files anywhere
"**/.env*"          # All files starting with .env anywhere
"/src/**"           # All files under /src
"*.txt"             # All .txt files in root only
"**/*test*.js"      # All files containing "test" in name
```

**Syntax**:
- `**` - Matches any number of directories
- `*` - Matches any characters within a single path component
- `?` - Matches a single character

---

### DIRECTORY - Prefix Match

Matches all files under a directory (including subdirectories).

```python
PermissionRule(
    pattern="/docs/",
    permission=Permission.WRITE,
    type=PatternType.DIRECTORY,
)

# Matches:
# /workspace/docs/README.md
# /workspace/docs/guides/setup.md
# /workspace/docs/api/reference.md
```

**Important**: Directory patterns should end with `/`.

---

### FILE - Exact Match

Matches a specific file path only.

```python
PermissionRule(
    pattern="/config.yaml",
    permission=Permission.READ,
    type=PatternType.FILE,
)

# Matches:
# /workspace/config.yaml

# Does NOT match:
# /workspace/config.yaml.bak
# /workspace/src/config.yaml
```

---

## Built-in Presets

AgentFense includes **five built-in presets** for common scenarios:

### 1. `view-only` (Default)

**Use case**: Audit, inspection, file enumeration without reading.

```python
from agentfense import PRESET_VIEW_ONLY, get_preset

# Rules:
[
    {"pattern": "**/*", "permission": "view"},
]
```

**What it does**:
- ✓ `ls` works everywhere
- ❌ Cannot read any files
- ❌ Cannot write anywhere

---

### 2. `read-only`

**Use case**: Static analysis, code review, linting.

```python
from agentfense import PRESET_READ_ONLY

# Rules:
[
    {"pattern": "**/*", "permission": "read"},
]
```

**What it does**:
- ✓ Can read all files
- ❌ Cannot write anywhere

---

### 3. `agent-safe` (Recommended for AI Agents)

**Use case**: Running AI agents with secure defaults.

```python
from agentfense import PRESET_AGENT_SAFE

# Rules:
[
    {"pattern": "**/*", "permission": "read", "priority": 0},
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    {"pattern": "/tmp/**", "permission": "write", "priority": 10},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
    {"pattern": "**/credentials*", "permission": "none", "priority": 100},
    {"pattern": "**/.git/**", "permission": "none", "priority": 50},
]
```

**What it does**:
- ✓ Read all files (except secrets)
- ✓ Write to `/output` and `/tmp`
- ❌ Cannot access `.env*`, `secrets/`, `*.key`, `*.pem`, `credentials*`, `.git/`

**Recommended for**: AI agents, untrusted automation, code analysis tools.

---

### 4. `development`

**Use case**: Development automation with secret protection.

```python
from agentfense import PRESET_DEVELOPMENT

# Rules:
[
    {"pattern": "**/*", "permission": "write", "priority": 0},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
]
```

**What it does**:
- ✓ Full write access (except secrets)
- ❌ Cannot access `.env*`, `secrets/`, `*.key`, `*.pem`

**Recommended for**: Build scripts, test automation, CI/CD.

---

### 5. `full-access`

**Use case**: Trusted code with no restrictions.

```python
from agentfense import PRESET_FULL_ACCESS

# Rules:
[
    {"pattern": "**/*", "permission": "write", "priority": 0},
]
```

**What it does**:
- ✓ Full read/write access everywhere

**Warning**: No protection against secret leaks. Use only for fully trusted code.

---

## Custom Permissions

### Creating Custom Rules

```python
from agentfense import Sandbox, PermissionRule, Permission

permissions = [
    # Default: read all
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # Allow write to specific directories
    PermissionRule(pattern="/logs/**", permission=Permission.WRITE),
    PermissionRule(pattern="/cache/**", permission=Permission.WRITE),
    
    # Hide secrets
    PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    
    # View-only for config templates
    PermissionRule(pattern="/templates/**", permission=Permission.VIEW),
]

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    sandbox.run("python main.py")
```

---

### Using Dictionary Format

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
]

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    ...
```

---

## Extending Presets

### Adding Rules to a Preset

Use `extend_preset()` to add custom rules to a built-in preset:

```python
from agentfense import Sandbox, extend_preset

permissions = extend_preset(
    "agent-safe",  # Base preset
    additions=[
        {"pattern": "/workspace/data/**", "permission": "write"},
        {"pattern": "/workspace/logs/**", "permission": "write"},
    ]
)

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    ...
```

**Result**: All `agent-safe` rules + your custom rules.

---

### Overriding Preset Rules

Use `overrides` with higher priority to override preset behavior:

```python
permissions = extend_preset(
    "agent-safe",
    overrides=[
        # Allow reading .env (override agent-safe's block)
        {"pattern": "**/.env*", "permission": "read"},
    ]
)
```

**Important**: Overrides automatically get higher priority than base rules.

---

## Rule Priority

When multiple rules match a file, the rule with the **highest priority** wins.

### Automatic Priority

AgentFense automatically assigns priority based on pattern specificity:

| Pattern Type | Auto Priority | Example |
|--------------|---------------|---------|
| FILE | 3 | `/config.yaml` |
| DIRECTORY | 2 | `/docs/` |
| GLOB | 1 | `**/*.py` |

More specific patterns (exact paths) beat general patterns (globs).

---

### Manual Priority

You can manually set priority to override automatic ordering:

```python
permissions = [
    # Default: read all (priority 0)
    PermissionRule(pattern="**/*", permission=Permission.READ, priority=0),
    
    # Block secrets (priority 100 - wins over default)
    PermissionRule(pattern="/secrets/**", permission=Permission.NONE, priority=100),
    
    # Exception: allow reading this specific secret (priority 200 - wins over block)
    PermissionRule(pattern="/secrets/public.key", permission=Permission.READ, priority=200),
]
```

**Rule**: Higher priority value = higher precedence.

---

## Common Patterns

### Pattern 1: Read-Only with Output Directory

```python
permissions = [
    PermissionRule(pattern="**/*", permission=Permission.READ),
    PermissionRule(pattern="/output/**", permission=Permission.WRITE),
]
```

**Use case**: Static analysis that generates reports.

---

### Pattern 2: Hide All Secrets

```python
secret_patterns = [
    "**/.env*",
    "**/secrets/**",
    "**/*.key",
    "**/*.pem",
    "**/*credentials*",
    "**/*password*",
    "**/token*",
]

permissions = [
    PermissionRule(pattern="**/*", permission=Permission.READ),
] + [
    PermissionRule(pattern=p, permission=Permission.NONE, priority=100)
    for p in secret_patterns
]
```

---

### Pattern 3: Selective Write Access

```python
permissions = [
    # Default: read-only
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # Allow write to specific subdirectories
    PermissionRule(pattern="/docs/**", permission=Permission.WRITE),
    PermissionRule(pattern="/tests/**", permission=Permission.WRITE),
    PermissionRule(pattern="/output/**", permission=Permission.WRITE),
]
```

---

### Pattern 4: Language-Specific Access

```python
permissions = [
    # Python files: read-only
    PermissionRule(pattern="**/*.py", permission=Permission.READ),
    
    # JavaScript files: read-only
    PermissionRule(pattern="**/*.js", permission=Permission.READ),
    
    # Config files: view-only (know they exist, can't read)
    PermissionRule(pattern="**/*.yaml", permission=Permission.VIEW),
    PermissionRule(pattern="**/*.json", permission=Permission.VIEW),
    
    # Everything else: none (invisible)
    PermissionRule(pattern="**/*", permission=Permission.NONE, priority=-1),
]
```

---

### Pattern 5: Test Environment

```python
permissions = [
    # Read source code
    PermissionRule(pattern="/src/**", permission=Permission.READ),
    
    # Full access to tests
    PermissionRule(pattern="/tests/**", permission=Permission.WRITE),
    
    # Full access to test output
    PermissionRule(pattern="/coverage/**", permission=Permission.WRITE),
    PermissionRule(pattern="/.pytest_cache/**", permission=Permission.WRITE),
    
    # Hide production secrets
    PermissionRule(pattern="/secrets/**", permission=Permission.NONE, priority=100),
]
```

---

## Best Practices

### 1. Use Presets as Base

```python
# ✓ Good: Start with preset, extend as needed
permissions = extend_preset(
    "agent-safe",
    additions=[{"pattern": "/data/**", "permission": "write"}]
)

# ✗ Avoid: Writing all rules from scratch (error-prone)
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    # ...forgot to block .env files!
]
```

---

### 2. Block Secrets with High Priority

```python
permissions = [
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # High priority ensures secrets are always blocked
    PermissionRule(pattern="**/.env*", permission=Permission.NONE, priority=1000),
    PermissionRule(pattern="**/secrets/**", permission=Permission.NONE, priority=1000),
]
```

---

### 3. Test Permissions Before Production

```python
# Test: Can the agent read what it should?
with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
    result = sandbox.run("cat /workspace/main.py")
    assert result.exit_code == 0, "Should read main.py"
    
    result = sandbox.run("cat /workspace/.env")
    assert result.exit_code != 0, "Should NOT read .env"
```

---

### 4. Document Custom Rules

```python
permissions = [
    # Allow read for analysis
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # Write access needed for:
    # - Generated documentation (docs/)
    # - Test reports (reports/)
    # - Build artifacts (dist/)
    PermissionRule(pattern="/docs/**", permission=Permission.WRITE),
    PermissionRule(pattern="/reports/**", permission=Permission.WRITE),
    PermissionRule(pattern="/dist/**", permission=Permission.WRITE),
]
```

---

### 5. Use Environment-Specific Presets

```python
def get_permissions(environment: str):
    if environment == "production":
        return get_preset("read-only")  # Maximum safety
    elif environment == "staging":
        return get_preset("agent-safe")  # Balanced
    elif environment == "development":
        return get_preset("development")  # More freedom
    else:
        return get_preset("view-only")  # Default safe
```

---

## Registering Custom Presets

Create reusable presets for your organization:

```python
from agentfense import register_preset

register_preset("data-science", [
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.ipynb", "permission": "read"},
    {"pattern": "**/*.csv", "permission": "read"},
    {"pattern": "**/*.parquet", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "/models/**", "permission": "write"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
])

# Use it like built-in presets
with Sandbox.from_local("./ml-project", preset="data-science") as sandbox:
    sandbox.run("python train.py")
```

---

## See Also

- [High-Level API Guide](./high-level.md) - Using permissions with Sandbox
- [Error Handling Guide](./errors.md) - Permission-related exceptions
- [Best Practices](../best-practices.md) - Security guidelines
