# Permission Presets

Permission presets are pre-configured rule sets for common use cases. Instead of manually defining permission rules, you can start with a preset and customize as needed. This guide covers the built-in presets, how to extend them, and how to create custom presets.

## Built-In Presets

AgentFense includes five built-in presets optimized for different scenarios:

### agent-safe (Default for AI Agents)

**Use case:** AI agents exploring codebases with controlled output and hidden secrets.

**Philosophy:** Allow agents to read and explore the codebase freely, write to designated output directories, but hide all secrets and sensitive data.

**Rules:**

| Pattern | Permission | Priority | Why |
|---------|------------|----------|-----|
| `**/*` | `read` | 0 | Default: read all files |
| `/output/**` | `write` | 10 | Allow writing results |
| `/tmp/**` | `write` | 10 | Allow temporary files |
| `**/.env*` | `none` | 100 | Hide environment files |
| `**/secrets/**` | `none` | 100 | Hide secrets directory |
| `**/*.key` | `none` | 100 | Hide private keys |
| `**/*.pem` | `none` | 100 | Hide PEM certificates |
| `**/credentials*` | `none` | 100 | Hide credential files |
| `**/.git/**` | `none` | 50 | Hide git history |

**Example:**

```python
from agentfense import Sandbox

# Use agent-safe preset (default)
with Sandbox.from_local("./my-project") as sandbox:
    # Agent can read source code
    result = sandbox.run("cat /workspace/app.py")
    print(result.stdout)
    
    # Agent can write to /output
    result = sandbox.run("echo 'results' > /workspace/output/report.txt")
    assert result.exit_code == 0
    
    # Agent cannot see secrets
    result = sandbox.run("ls /workspace/.env")
    assert result.exit_code != 0  # File not found
```

**When to use:**
- Running AI agents (Claude, GPT) on your codebase
- Automated code analysis tools
- Documentation generators
- Testing assistants

### read-only (Safe Browsing)

**Use case:** Explore codebases without any write access.

**Philosophy:** Grant full read access to all files, deny all writes.

**Rules:**

| Pattern | Permission | Priority |
|---------|------------|----------|
| `**/*` | `read` | 0 |

**Example:**

```python
from agentfense import Sandbox

# Read-only mode
with Sandbox.from_local("./my-project", preset="read-only") as sandbox:
    # All reads work
    result = sandbox.run("cat /workspace/config.yaml")
    print(result.stdout)
    
    # All writes fail
    result = sandbox.run("echo test > /workspace/output.txt")
    assert result.exit_code != 0  # Permission denied
```

**When to use:**
- Code review by AI agents
- Static analysis tools
- Documentation search
- Safe exploration of unknown codebases

### full-access (Trusted Automation)

**Use case:** Trusted automation with full read/write access.

**Philosophy:** Grant complete access to all files (use with caution).

**Rules:**

| Pattern | Permission | Priority |
|---------|------------|----------|
| `**/*` | `write` | 0 |

**Example:**

```python
from agentfense import Sandbox

# Full access (use with caution!)
with Sandbox.from_local("./my-project", preset="full-access") as sandbox:
    # Can read and modify any file
    result = sandbox.run("sed -i 's/v1/v2/' /workspace/version.txt")
    assert result.exit_code == 0
```

**When to use:**
- Trusted automation scripts (e.g., build systems)
- Testing in isolated environments
- Development/debugging (never in production with untrusted code)

**⚠️ Warning:** This preset grants full write access. Only use with trusted code or in fully isolated environments.

### development (Developer-Friendly)

**Use case:** Local development with safety guardrails.

**Philosophy:** Full access except for secrets (prevents accidental exposure).

**Rules:**

| Pattern | Permission | Priority |
|---------|------------|----------|
| `**/*` | `write` | 0 |
| `**/.env*` | `none` | 100 |
| `**/secrets/**` | `none` | 100 |
| `**/*.key` | `none` | 100 |
| `**/*.pem` | `none` | 100 |

**Example:**

```python
from agentfense import Sandbox

# Development mode
with Sandbox.from_local("./my-project", preset="development") as sandbox:
    # Can modify source code
    result = sandbox.run("echo 'print(1)' >> /workspace/test.py")
    assert result.exit_code == 0
    
    # But secrets are hidden
    result = sandbox.run("cat /workspace/.env")
    assert result.exit_code != 0  # File not found
```

**When to use:**
- Local development with AI assistants
- Prototyping and experimentation
- Debugging agent workflows

### view-only (Metadata Exploration)

**Use case:** See file structure without exposing content.

**Philosophy:** Allow agents to see file names and metadata, but deny reading content.

**Rules:**

| Pattern | Permission | Priority |
|---------|------------|----------|
| `**/*` | `view` | 0 |

**Example:**

```python
from agentfense import Sandbox

# View-only mode
with Sandbox.from_local("./my-project", preset="view-only") as sandbox:
    # Can list files
    result = sandbox.run("ls -lh /workspace/")
    print(result.stdout)  # Shows file names, sizes, timestamps
    
    # Cannot read content
    result = sandbox.run("cat /workspace/app.py")
    assert result.exit_code != 0  # Permission denied
```

**When to use:**
- File structure exploration
- Metadata indexing
- Planning tools (agent sees what's available before requesting access)

**Note:** `view` permission has limitations on macOS with Docker Desktop. Use `read` instead if you encounter issues.

## Using Presets

### Method 1: High-Level API

```python
from agentfense import Sandbox

# Use a preset by name
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("python main.py")
```

### Method 2: Low-Level API

```python
from agentfense import SandboxClient, get_preset_dicts

client = SandboxClient()
codebase = client.create_codebase(name="my-project", owner_id="user_001")

# Get preset as dicts for API
permissions = get_preset_dicts("agent-safe")

sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=permissions,
)
```

### Listing Available Presets

```python
from agentfense import list_presets

# Get all preset names
presets = list_presets()
print(presets)  # ['agent-safe', 'development', 'full-access', 'read-only', 'view-only']
```

## Extending Presets

Presets are starting points. Use `extend_preset()` to customize them:

### Adding Rules

```python
from agentfense import extend_preset, Sandbox

# Start with agent-safe, add write access to /logs
rules = extend_preset(
    base="agent-safe",
    additions=[
        {"pattern": "/logs/**", "permission": "write"},
    ],
)

with Sandbox.from_local("./project", permissions=rules) as sandbox:
    # Now can write to /logs (in addition to /output and /tmp)
    result = sandbox.run("echo 'log entry' > /workspace/logs/app.log")
    assert result.exit_code == 0
```

### Overriding Rules

```python
from agentfense import extend_preset

# Start with agent-safe, make .git readable (override hidden)
rules = extend_preset(
    base="agent-safe",
    overrides=[
        {"pattern": "**/.git/**", "permission": "read"},  # Override: unhide .git
    ],
)

# .git is now readable (overrides the "none" from agent-safe)
```

**How `overrides` works:**
- Automatically assigns higher priority than all base rules
- Ensures your override takes precedence
- Useful for exceptions to preset rules

### Combining Additions and Overrides

```python
from agentfense import extend_preset

rules = extend_preset(
    base="read-only",
    additions=[
        {"pattern": "/output/**", "permission": "write"},  # Add write to output
    ],
    overrides=[
        {"pattern": "**/.env*", "permission": "none"},  # Hide .env files
    ],
)

# Result:
# - All files readable (base: read-only)
# - /output/** writable (addition)
# - .env files hidden (override)
```

## Creating Custom Presets

Register your own presets for reuse across projects:

### Registering a Preset

```python
from agentfense import register_preset

# Define a custom preset for CI/CD
register_preset("ci-pipeline", [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/build/**", "permission": "write"},
    {"pattern": "/dist/**", "permission": "write"},
    {"pattern": "/.cache/**", "permission": "write"},
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
])

# Use it like a built-in preset
from agentfense import Sandbox

with Sandbox.from_local("./app", preset="ci-pipeline") as sandbox:
    sandbox.run("npm run build")
```

### Best Practices for Custom Presets

1. **Name descriptively**: Use names that explain the use case (`ci-pipeline`, `data-science`, `code-review`)
2. **Document the preset**: Add comments explaining the rules
3. **Set explicit priorities**: For security-critical rules (e.g., hiding secrets), use high priority
4. **Test thoroughly**: Verify the preset behaves as expected before production use

### Example: Data Science Preset

```python
from agentfense import register_preset

register_preset("data-science", [
    # Read code and configs
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.ipynb", "permission": "read"},
    {"pattern": "**/*.yaml", "permission": "read"},
    {"pattern": "**/*.json", "permission": "read"},
    
    # View data files (no content access)
    {"pattern": "/data/**", "permission": "view"},
    
    # Write to output and cache
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "/models/**", "permission": "write"},
    {"pattern": "/.cache/**", "permission": "write"},
    
    # Hide secrets
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/credentials/**", "permission": "none", "priority": 100},
])
```

### Example: Documentation Generator Preset

```python
from agentfense import register_preset

register_preset("doc-generator", [
    # Read source files
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "**/*.ts", "permission": "read"},
    
    # Read existing docs
    {"pattern": "**/*.md", "permission": "read"},
    {"pattern": "/docs/**", "permission": "read"},
    
    # Write generated docs
    {"pattern": "/docs/generated/**", "permission": "write"},
    {"pattern": "/docs/api/**", "permission": "write"},
    
    # Hide private/internal code
    {"pattern": "**/internal/**", "permission": "none"},
    {"pattern": "**/_private/**", "permission": "none"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
])
```

## Preset Comparison

| Preset | Read Code | Write Code | Write Output | Secrets Visible | Use Case |
|--------|-----------|------------|--------------|-----------------|----------|
| **agent-safe** | ✅ | ❌ | ✅ (/output, /tmp) | ❌ Hidden | AI agents, automation |
| **read-only** | ✅ | ❌ | ❌ | ✅ Visible | Code review, analysis |
| **full-access** | ✅ | ✅ | ✅ | ✅ Visible | Trusted automation |
| **development** | ✅ | ✅ | ✅ | ❌ Hidden | Local development |
| **view-only** | 📋 Metadata | ❌ | ❌ | 📋 Metadata | Structure exploration |

## Migrating Between Presets

### From read-only to agent-safe

```python
# Before: read-only
with Sandbox.from_local("./project", preset="read-only") as sandbox:
    # Cannot write anywhere
    pass

# After: agent-safe
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    # Can write to /output and /tmp
    sandbox.run("echo result > /workspace/output/log.txt")
```

**Why migrate:** Enable agents to save results while keeping code read-only.

### From agent-safe to development

```python
# Before: agent-safe (code is read-only)
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("echo test >> /workspace/app.py")
    assert result.exit_code != 0  # Fails

# After: development (code is writable, secrets hidden)
with Sandbox.from_local("./project", preset="development") as sandbox:
    result = sandbox.run("echo test >> /workspace/app.py")
    assert result.exit_code == 0  # Succeeds
```

**Why migrate:** Allow agents to modify code (e.g., refactoring, testing) while keeping secrets hidden.

### From full-access to agent-safe

```python
# Before: full-access (dangerous)
with Sandbox.from_local("./project", preset="full-access") as sandbox:
    # Can read secrets and modify code
    pass

# After: agent-safe (safer)
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    # Secrets hidden, code read-only, output writable
    pass
```

**Why migrate:** Reduce risk when transitioning from trusted to untrusted automation.

## Troubleshooting

### Preset Not Found

**Error:** `ValueError: Unknown preset 'my-preset'`

**Solution:** Check preset name spelling. List available presets with `list_presets()`.

### Secrets Still Visible

**Problem:** Files like `.env` appear in `ls` output

**Diagnosis:**
1. Check if you're using the right preset: `agent-safe`, `development`, or custom with secret rules
2. Verify pattern matches: `**/.env*` matches `.env`, `.env.local`, etc.
3. Check priority: Secret rules should have higher priority than base rules

**Solution:**

```python
from agentfense import extend_preset

# Ensure secrets have high priority
rules = extend_preset(
    base="read-only",
    overrides=[
        {"pattern": "**/.env*", "permission": "none", "priority": 100},
    ],
)
```

### Cannot Write to Output Directory

**Problem:** `Permission denied` when writing to `/output`

**Diagnosis:** Check if your preset allows writing to `/output`. `read-only` and `view-only` do not.

**Solution:** Use `agent-safe`, `full-access`, or extend your preset:

```python
from agentfense import extend_preset

rules = extend_preset(
    base="read-only",
    additions=[
        {"pattern": "/output/**", "permission": "write"},
    ],
)
```

## Next Steps

- **[Permission System](permissions.md)** - Deep dive into permission rules
- **[Best Practices](best-practices.md)** - Production deployment patterns
- **[Security Model](index.md)** - Overall security architecture
