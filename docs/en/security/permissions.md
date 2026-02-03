# Permission System

This guide provides a deep dive into AgentFense's permission system: the four permission levels, pattern matching rules, priority algorithms, and practical examples for building secure access policies.

## Permission Levels

### none - Complete Invisibility

**Semantics:** The file or directory does not exist from the agent's perspective.

**What the agent experiences:**

| Operation | Result |
|-----------|--------|
| `ls /workspace/` | File not listed |
| `cat /workspace/secret.txt` | `No such file or directory` |
| `stat /workspace/secret.txt` | `No such file or directory` |
| `[ -f /workspace/secret.txt ]` | Returns false (file test fails) |
| `find /workspace -name secret.txt` | No results |
| `grep pattern /workspace/**` | File skipped (not found) |

**Implementation:** FUSE layer returns `ENOENT` (error code 2) for all operations on paths with `none` permission.

**Use cases:**
- **Secrets and credentials**: `.env`, `.env.*`, `credentials.json`, `config/secrets.yaml`
- **Private keys**: `*.key`, `*.pem`, `*.p12`, `id_rsa`, `id_ed25519`
- **Internal tools**: `/scripts/admin/**`, `/tools/internal/**`
- **Sensitive data**: `/customer_data/**`, `/pii/**`, `/financial/**`

**Example:**

```python
permissions = [
    {"pattern": "**/.env*", "permission": "none"},
    {"pattern": "**/secrets/**", "permission": "none"},
    {"pattern": "**/*.key", "permission": "none"},
    {"pattern": "**/*.pem", "permission": "none"},
    {"pattern": "**/credentials*", "permission": "none"},
]
```

### view - Metadata Only

**Semantics:** The file appears in listings, but content is inaccessible.

**What the agent experiences:**

| Operation | Result |
|-----------|--------|
| `ls -l /workspace/` | File listed with size, permissions, timestamps |
| `cat /workspace/data.json` | `Permission denied` |
| `stat /workspace/data.json` | Succeeds (returns metadata) |
| `file /workspace/data.json` | Shows file type (e.g., "JSON data") |
| `wc -l /workspace/data.json` | `Permission denied` |

**Implementation:** FUSE allows `stat()` and directory listing but rejects `open()` for reading.

**Use cases:**
- **Large datasets**: Show file catalog without exposing content
- **Metadata exploration**: Let agent see structure (file names, sizes) for planning
- **Documentation indexes**: List docs without revealing content

**Example:**

```python
permissions = [
    {"pattern": "/data/catalog/**", "permission": "view"},
    {"pattern": "/metadata/**", "permission": "view"},
]

# Agent can explore structure
$ ls -lh /workspace/data/catalog/
# Output: users.csv (1.2MB), transactions.csv (5.4MB), products.csv (340KB)

# But cannot read content
$ head /workspace/data/catalog/users.csv
# Output: head: cannot open '/workspace/data/catalog/users.csv' for reading: Permission denied
```

**Known limitations:**
- **macOS + Docker Desktop**: VirtioFS doesn't correctly handle metadata-only access. Files appear as "No such file" even with `view` permission. **Workaround:** Use `read` permission or bwrap runtime on Linux.

### read - Read-Only Access

**Semantics:** The file is fully readable but cannot be modified.

**What the agent experiences:**

| Operation | Result |
|-----------|--------|
| `cat /workspace/app.py` | ✅ Succeeds (shows content) |
| `grep pattern /workspace/**` | ✅ Searches file content |
| `diff /workspace/a.txt /workspace/b.txt` | ✅ Works |
| `echo "new" > /workspace/app.py` | ❌ Permission denied |
| `sed -i 's/old/new/' /workspace/app.py` | ❌ Permission denied |
| `rm /workspace/app.py` | ❌ Permission denied |

**Implementation:** FUSE allows `open(O_RDONLY)` but rejects `open(O_WRONLY)`, `open(O_RDWR)`, `unlink()`, and `rename()`.

**Use cases:**
- **Source code**: Application code, libraries, modules
- **Configuration files**: Read configs without allowing changes
- **Documentation**: README, guides, API docs
- **Static assets**: Images, CSS, JavaScript (for web apps)

**Example:**

```python
permissions = [
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "/docs/**", "permission": "read"},
    {"pattern": "/config/**", "permission": "read"},
]

# Agent can read source
$ cat /workspace/app.py
# Output: (file content)

# But cannot modify
$ echo "malicious_code()" >> /workspace/app.py
# Output: bash: /workspace/app.py: Permission denied
```

### write - Full Access

**Semantics:** Full read, write, modify, and delete permissions.

**What the agent experiences:**

| Operation | Result |
|-----------|--------|
| `cat /workspace/output/log.txt` | ✅ Succeeds |
| `echo "data" > /workspace/output/log.txt` | ✅ Succeeds |
| `echo "more" >> /workspace/output/log.txt` | ✅ Succeeds (append) |
| `rm /workspace/output/log.txt` | ✅ Succeeds |
| `mkdir /workspace/output/subdir` | ✅ Succeeds |

**Implementation:** FUSE allows all file operations: read, write, append, delete, rename, chmod, etc.

**Use cases:**
- **Output directories**: `/output/**`, `/results/**`, `/artifacts/**`, `/reports/**`
- **Temp directories**: `/tmp/**`, `/workspace/tmp/**`, `/.cache/**`
- **Build directories**: `/build/**`, `/dist/**`, `/.next/**` (for frameworks)
- **Test files**: `/tests/fixtures/**` (in testing scenarios)

**Example:**

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},          # Default: read-only
    {"pattern": "/output/**", "permission": "write"},   # Output writable
    {"pattern": "/tmp/**", "permission": "write"},      # Temp writable
    {"pattern": "/build/**", "permission": "write"},    # Build artifacts writable
]

# Agent can write to output
$ echo "Analysis complete" > /workspace/output/report.txt
# Output: (success)

$ mkdir /workspace/output/charts
$ echo "data" > /workspace/output/charts/plot.json
# Output: (success)
```

## Pattern Matching

### Pattern Types

AgentFense supports three types of patterns, each with different matching behavior and priority:

#### 1. Glob Patterns (Most Common)

**Syntax:** Use `**` and `*` wildcards

| Pattern | Matches | Priority (default) |
|---------|---------|-------------------|
| `**/*` | All files recursively | 1 (glob) |
| `**/*.py` | All Python files | 1 (glob) |
| `*.txt` | `.txt` files in root only | 1 (glob) |
| `/secrets/**` | All files under `/secrets/` | 1 (glob) |
| `**/.env*` | `.env`, `.env.local`, etc. anywhere | 1 (glob) |
| `**/node_modules/**` | All `node_modules` dirs | 1 (glob) |

**Examples:**

```python
# Hide all .env files anywhere
{"pattern": "**/.env*", "permission": "none"}

# All Python files read-only
{"pattern": "**/*.py", "permission": "read"}

# All files under /secrets/ hidden
{"pattern": "/secrets/**", "permission": "none"}
```

#### 2. Directory Patterns

**Syntax:** Path ending with `/`

| Pattern | Matches | Priority (default) |
|---------|---------|-------------------|
| `/docs/` | All files in `/docs/` and subdirectories | 2 (directory) |
| `/src/utils/` | All files in `/src/utils/` and subdirectories | 2 (directory) |

**Examples:**

```python
# All files in /docs/ writable
{"pattern": "/docs/", "permission": "write", "type": "directory"}

# All files in /src/ read-only
{"pattern": "/src/", "permission": "read", "type": "directory"}
```

**Note:** Directory patterns require explicit `"type": "directory"` in the rule, or the pattern will be treated as glob.

#### 3. File Patterns (Exact Match)

**Syntax:** Exact file path

| Pattern | Matches | Priority (default) |
|---------|---------|-------------------|
| `/config.yaml` | Only `/config.yaml` | 3 (file) |
| `/secrets/public.key` | Only `/secrets/public.key` | 3 (file) |

**Examples:**

```python
# Specific file hidden
{"pattern": "/secrets/private.key", "permission": "none", "type": "file"}

# Specific file writable (exception)
{"pattern": "/output/current.txt", "permission": "write", "type": "file"}
```

### Pattern Precedence

When multiple patterns match a path, the **highest priority wins**. Priority is determined by:

1. **Explicit `priority` field** (if set) - highest wins
2. **Pattern type** - `file` (3) > `directory` (2) > `glob` (1)
3. **Pattern specificity** - more specific patterns win

#### Example: Priority in Action

```python
rules = [
    {"pattern": "**/*", "permission": "read"},                  # Priority: 0, type: glob (1)
    {"pattern": "/secrets/**", "permission": "none"},           # Priority: 0, type: glob (1)
    {"pattern": "/secrets/public.key", "permission": "read"},   # Priority: 0, type: file (3)
]

# Evaluation:
# /app/main.py           → "read" (matched by **/*) 
# /secrets/private.key   → "none" (matched by /secrets/** - more specific than **/*) 
# /secrets/public.key    → "read" (type: file overrides glob /secrets/**)
```

**Why this works:**
- `**/*` matches everything with priority 1 (glob)
- `/secrets/**` also has priority 1 (glob), but is more specific for paths under `/secrets/`
- `/secrets/public.key` has priority 3 (file), which **overrides** the glob `/secrets/**`

#### Example: Explicit Priority

```python
rules = [
    {"pattern": "**/*", "permission": "read", "priority": 0},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},  # High priority
    {"pattern": "/config/.env.public", "permission": "read", "priority": 200},  # Even higher
]

# Evaluation:
# /app/main.py                → "read" (priority 0)
# /config/.env                → "none" (priority 100 overrides 0)
# /config/.env.local          → "none" (priority 100 overrides 0)
# /config/.env.public         → "read" (priority 200 overrides 100)
```

### Pattern Matching Algorithm

Pseudo-code for permission evaluation:

```python
def evaluate_permission(path: str, rules: list) -> Permission:
    # Step 1: Find all matching rules
    matching_rules = [r for r in rules if r.pattern.matches(path)]
    
    # Step 2: Sort by priority (descending)
    matching_rules.sort(key=lambda r: (
        r.priority,           # Explicit priority (higher wins)
        r.type.value,         # Pattern type: file=3, directory=2, glob=1
        r.pattern_specificity # Specificity score (more specific wins)
    ), reverse=True)
    
    # Step 3: Return permission from highest-priority rule
    if matching_rules:
        return matching_rules[0].permission
    else:
        return Permission.NONE  # Default: deny (invisible)
```

## Building Permission Policies

### Strategy 1: Default Deny + Explicit Allow

**Recommended for security-critical scenarios.**

```python
permissions = [
    # Step 1: Default to maximum restriction (everything invisible)
    {"pattern": "**/*", "permission": "none", "priority": 0},
    
    # Step 2: Explicitly allow specific paths
    {"pattern": "/src/**", "permission": "read", "priority": 10},
    {"pattern": "/docs/**", "permission": "read", "priority": 10},
    {"pattern": "/output/**", "permission": "write", "priority": 10},
]

# Result:
# /src/app.py        → read (explicitly allowed)
# /data/secret.json  → none (default deny)
# /output/log.txt    → write (explicitly allowed)
```

### Strategy 2: Default Allow + Explicit Deny

**Recommended for agent exploration scenarios.**

```python
permissions = [
    # Step 1: Default to read access (agent can explore)
    {"pattern": "**/*", "permission": "read", "priority": 0},
    
    # Step 2: Explicitly hide sensitive paths
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/.git/**", "permission": "none", "priority": 50},
    
    # Step 3: Allow writes to specific directories
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    {"pattern": "/tmp/**", "permission": "write", "priority": 10},
]

# Result:
# /src/app.py        → read (default allow)
# /.env              → none (explicitly hidden)
# /secrets/api.key   → none (explicitly hidden)
# /output/log.txt    → write (explicitly allowed)
```

### Strategy 3: Layered Permissions

**Recommended for complex projects with multiple permission zones.**

```python
permissions = [
    # Layer 1: Base rule (read all)
    {"pattern": "**/*", "permission": "read", "priority": 0},
    
    # Layer 2: Secrets (hidden)
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    
    # Layer 3: Metadata (view only)
    {"pattern": "/metadata/**", "permission": "view", "priority": 20},
    {"pattern": "/schemas/**", "permission": "view", "priority": 20},
    
    # Layer 4: Writable zones
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    {"pattern": "/logs/**", "permission": "write", "priority": 10},
    {"pattern": "/tmp/**", "permission": "write", "priority": 10},
    
    # Layer 5: Exceptions (specific files)
    {"pattern": "/secrets/public.key", "permission": "read", "priority": 200},
    {"pattern": "/config/readonly.yaml", "permission": "view", "priority": 200},
]
```

## Common Patterns

### Hide All Secrets

```python
secret_patterns = [
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
    {"pattern": "**/*.p12", "permission": "none", "priority": 100},
    {"pattern": "**/credentials*", "permission": "none", "priority": 100},
    {"pattern": "**/*_secret*", "permission": "none", "priority": 100},
    {"pattern": "**/*_token*", "permission": "none", "priority": 100},
    {"pattern": "**/id_rsa*", "permission": "none", "priority": 100},
    {"pattern": "**/id_ed25519*", "permission": "none", "priority": 100},
]
```

### Code Review Mode (Read-Only)

```python
code_review = [
    # All code readable
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "**/*.ts", "permission": "read"},
    {"pattern": "**/*.go", "permission": "read"},
    {"pattern": "**/*.java", "permission": "read"},
    
    # Docs readable
    {"pattern": "**/README*", "permission": "read"},
    {"pattern": "**/*.md", "permission": "read"},
    
    # Hide secrets
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    
    # Hide build artifacts and dependencies
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/__pycache__/**", "permission": "none"},
    {"pattern": "**/dist/**", "permission": "none"},
    {"pattern": "**/build/**", "permission": "none"},
]
```

### CI/CD Pipeline Mode

```python
ci_cd = [
    # Read source code
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "/package.json", "permission": "read"},
    {"pattern": "/requirements.txt", "permission": "read"},
    
    # Write build outputs
    {"pattern": "/build/**", "permission": "write"},
    {"pattern": "/dist/**", "permission": "write"},
    {"pattern": "/.cache/**", "permission": "write"},
    
    # Hide secrets
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    
    # Hide unnecessary files
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/.git/**", "permission": "none"},
]
```

## Debugging Permission Rules

### Test Permission Evaluation

Use the SDK to test which permission applies to a path:

```python
from agentfense import Sandbox

permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/secrets/**", "permission": "none"},
    {"pattern": "/secrets/public.key", "permission": "read"},
]

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    # Test /secrets/private.key
    result = sandbox.run("cat /workspace/secrets/private.key")
    if result.exit_code != 0:
        print(f"Access denied: {result.stderr}")  # Expected: No such file or directory
    
    # Test /secrets/public.key
    result = sandbox.run("cat /workspace/secrets/public.key")
    if result.exit_code == 0:
        print(f"Access granted: {result.stdout}")
```

### Common Debugging Steps

1. **Check pattern syntax**: Ensure `**` for recursive, `*` for single level
2. **Verify priority**: Higher priority should override lower
3. **Test specificity**: More specific patterns should win (file > directory > glob)
4. **Check path prefix**: Patterns like `/secrets/**` require leading `/`

## Performance Considerations

### Rule Count Impact

- **Small rulesets (<10 rules)**: Negligible overhead (~1μs per file operation)
- **Medium rulesets (10-50 rules)**: Low overhead (~5-10μs per file operation)
- **Large rulesets (>100 rules)**: Moderate overhead (~50μs per file operation)

**Recommendation:** Keep rulesets under 50 rules for best performance. Use glob patterns to collapse multiple similar rules.

### Pattern Optimization

```python
# Less efficient: Many specific patterns
rules = [
    {"pattern": "/secrets/api.key", "permission": "none"},
    {"pattern": "/secrets/db.key", "permission": "none"},
    {"pattern": "/secrets/jwt.key", "permission": "none"},
    # ... 20 more similar rules
]

# More efficient: Single glob pattern
rules = [
    {"pattern": "/secrets/**", "permission": "none"},
]
```

## Next Steps

- **[Presets Guide](presets.md)** - Pre-configured permission templates
- **[Best Practices](best-practices.md)** - Production deployment security patterns
- **[Security Model](index.md)** - Overall security architecture
