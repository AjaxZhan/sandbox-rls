# Security Model

AgentFense enforces **least-privilege access control at the filesystem level**, preventing untrusted AI agents from accessing sensitive data even if they try. This page explains the security model, threat boundaries, and trust guarantees.

## TL;DR

**AgentFense makes files invisible, not just inaccessible.** An agent with `none` permission on `/secrets/.env` cannot:
- Read the file (permission denied)
- See the file in `ls` output (invisible)
- Stat the file (returns "file not found")
- Infer its existence through any filesystem operation

This is enforced at the **FUSE filesystem layer**, not application logic. The agent cannot bypass it.

## Why Filesystem-Level Control?

Traditional sandboxing approaches rely on **application-level permission checks**:

```python
# Traditional approach (fragile)
if user.can_read(path):
    return open(path).read()
else:
    raise PermissionError
```

**Problems:**
- Every code path must check permissions (easy to miss)
- Agents can probe for files by catching exceptions
- Filesystem metadata (size, existence) often leaks
- Developers must remember to add checks

**AgentFense approach:** Permissions enforced in the **FUSE filesystem**:

```python
# AgentFense: no checks needed
open(path).read()  # FUSE rejects at kernel level if not allowed
```

**Benefits:**
- Zero-trust: agent cannot bypass permissions
- No code changes needed: works with existing tools (`cat`, `grep`, `ls`)
- Information hiding: files with `none` permission are invisible
- Defense-in-depth: isolation at kernel/container level + FUSE enforcement

## Security Architecture

```
┌─────────────────────────────────────────────────┐
│                 Agent Process                   │
│          (untrusted code execution)             │
└─────────────────┬───────────────────────────────┘
                  │ open("/workspace/secret.txt")
                  ▼
┌─────────────────────────────────────────────────┐
│          FUSE Filesystem Layer                  │
│      (permission enforcement + hiding)          │
│   - Check permission for "/secret.txt"          │
│   - If "none": return ENOENT (not found)        │
│   - If "view": allow stat/list, deny read       │
│   - If "read": allow read, deny write           │
│   - If "write": allow all operations            │
└─────────────────┬───────────────────────────────┘
                  │ (if allowed)
                  ▼
┌─────────────────────────────────────────────────┐
│              Delta Layer (COW)                  │
│   - Read: check delta, fallback to source       │
│   - Write: always write to per-sandbox delta    │
│   - Delete: create whiteout marker              │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│            Codebase Storage                     │
│         (immutable source files)                │
└─────────────────────────────────────────────────┘
```

## Four Permission Levels

AgentFense provides **granular control** through four permission levels:

### 1. **none** - Invisible (Maximum Security)

**What it does:**
- File does **not appear** in directory listings (`ls`, `find`, `tree`)
- All filesystem operations return **ENOENT** ("No such file or directory")
- Agent cannot infer file existence through any means

**Use cases:**
- **Secrets**: `.env`, `credentials.json`, `*.key`, `*.pem`
- **Private data**: `/secrets/**`, `/internal/**`, `/private/**`
- **Sensitive configs**: API keys, database passwords, tokens

**Example:**

```python
permissions = [
    {"pattern": "**/.env*", "permission": "none"},
    {"pattern": "/secrets/**", "permission": "none"},
]

# Agent tries to access .env
$ ls -la /workspace/
# Output: (no .env file listed)

$ cat /workspace/.env
# Output: cat: /workspace/.env: No such file or directory

$ [ -f /workspace/.env ] && echo "exists"
# Output: (nothing - file test returns false)
```

### 2. **view** - List-Only (Metadata Access)

**What it does:**
- File **appears in directory listings** (name, size, timestamps)
- **Cannot read content** (open/read operations denied)
- Useful for showing file structure without exposing data

**Use cases:**
- **Large datasets**: Show data catalog without exposing actual data
- **Metadata exploration**: Agent can see available files
- **Documentation structure**: Navigate docs without reading content

**Example:**

```python
permissions = [
    {"pattern": "/metadata/**", "permission": "view"},
]

# Agent can see file structure
$ ls -lh /workspace/metadata/
# Output: schema.json (1.2KB), config.yaml (850B)

# But cannot read content
$ cat /workspace/metadata/schema.json
# Output: cat: /workspace/metadata/schema.json: Permission denied
```

**Note:** `view` permission has known limitations on macOS with Docker Desktop (VirtioFS issue). Use `read` as fallback.

### 3. **read** - Read-Only (Default for Code)

**What it does:**
- File appears in listings
- Can read content (`cat`, `grep`, `diff`)
- **Cannot modify** (write/append/delete denied)

**Use cases:**
- **Source code**: Read-only access to application code
- **Documentation**: Browse docs without modification
- **Configuration files**: Read configs without changing them
- **Logs**: View logs without tampering

**Example:**

```python
permissions = [
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "/docs/**", "permission": "read"},
]

# Agent can read files
$ cat /workspace/app.py
# Output: (file content)

# But cannot modify
$ echo "malicious" >> /workspace/app.py
# Output: bash: /workspace/app.py: Permission denied
```

### 4. **write** - Full Access (Use Sparingly)

**What it does:**
- Full read/write access
- Can create, modify, delete files
- Should only be granted to **controlled output directories**

**Use cases:**
- **Output directories**: `/output/**`, `/results/**`, `/artifacts/**`
- **Temp directories**: `/tmp/**`, `/workspace/tmp/**`
- **Build directories**: `/build/**`, `/dist/**`
- **Test files** (in testing scenarios)

**Example:**

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},          # Default: read-only
    {"pattern": "/output/**", "permission": "write"},   # Allow writes to output
    {"pattern": "/tmp/**", "permission": "write"},      # Allow temp files
]

# Agent can write to output
$ echo "analysis complete" > /workspace/output/result.txt
# Output: (success)

# But cannot write to other paths
$ echo "malicious" > /workspace/config.yaml
# Output: bash: /workspace/config.yaml: Permission denied
```

## Isolation Boundaries

AgentFense provides **three layers of isolation**:

### Layer 1: Process Isolation

| Runtime | Mechanism | Isolation Strength | Use Case |
|---------|-----------|-------------------|----------|
| **bwrap** | Linux namespaces (PID, mount, network, IPC, UTS) | Medium | Development, fast iteration |
| **Docker** | Full container with cgroups + namespaces | Strong | Production, strict isolation |

**What it prevents:**
- Accessing host filesystem outside `/workspace`
- Network access (configurable)
- Killing host processes
- Escaping to host environment

### Layer 2: Filesystem Isolation (FUSE)

**FUSE mounts** provide:
- Permission enforcement on every file operation
- Filesystem-level hiding (`none` permission)
- Read-only enforcement (`view`, `read` permissions)

**What it prevents:**
- Reading files outside permission rules
- Discovering hidden files through filesystem probing
- Modifying read-only files
- Bypassing application-level permission checks

### Layer 3: Write Isolation (Delta Layer)

**Copy-On-Write** provides:
- Per-sandbox write isolation
- Protection of shared codebases from corruption
- Rollback on failure (ephemeral writes)

**What it prevents:**
- One sandbox corrupting another's data
- Permanent damage to source codebase
- Write conflicts across concurrent sandboxes

## Threat Model

### What AgentFense Protects Against

| Threat | Protection | How |
|--------|-----------|-----|
| **Secret exposure** | ✅ Strong | Files with `none` permission are invisible at kernel level |
| **Source code tampering** | ✅ Strong | `read` permission prevents writes; delta layer isolates allowed writes |
| **Privilege escalation** | ✅ Medium | bwrap/Docker isolation prevents escaping to host |
| **Data exfiltration via writes** | ✅ Strong | Only designated output directories are writable |
| **Filesystem probing** | ✅ Strong | `none` permission hides existence; no metadata leakage |
| **Resource exhaustion** | ⚠️ Medium | Resource limits (CPU, memory, PIDs) enforce caps |
| **Network attacks** | ⚠️ Configurable | Network disabled by default; enable if needed |

### What AgentFense Does NOT Protect Against

| Threat | Status | Mitigation |
|--------|--------|-----------|
| **Side-channel timing attacks** | ❌ Not protected | Not a primary concern for AI agents; use constant-time crypto if critical |
| **Inference from allowed files** | ⚠️ Limited | Agent can infer from **allowed** file content; minimize readable files |
| **Network data exfiltration** | ❌ If networking enabled | Disable networking or use egress filtering |
| **CPU/GPU side channels** | ❌ Not protected | Use physical isolation for high-security scenarios |
| **Kernel exploits** | ⚠️ Limited | Docker provides stronger isolation than bwrap; keep kernel updated |

## Trust Boundaries

AgentFense establishes these trust boundaries:

```
┌──────────────────────────────────────────────┐
│              UNTRUSTED ZONE                  │
│                                              │
│  ┌────────────────────────────────────┐     │
│  │       Agent Process                │     │
│  │   (untrusted code execution)       │     │
│  └────────────────────────────────────┘     │
│                                              │
│  - Can only access files per permissions    │
│  - Cannot escape isolation                   │
│  - Cannot see/modify hidden files            │
└──────────────────┬───────────────────────────┘
                   │ FUSE + Isolation
┌──────────────────▼───────────────────────────┐
│             TRUSTED ZONE                     │
│                                              │
│  - AgentFense server                        │
│  - Permission engine                         │
│  - Codebase storage                          │
│  - Host filesystem                           │
└──────────────────────────────────────────────┘
```

**Assumptions:**
1. **Server is trusted**: The AgentFense server process runs with elevated privileges (for FUSE mounts)
2. **Kernel is trusted**: Depends on Linux kernel for namespace isolation
3. **Configuration is trusted**: Permission rules are set by trusted administrators

**Key Insight:** Even if agent code is compromised or malicious, it **cannot bypass filesystem permissions** because enforcement happens outside its control.

## Enforcement Mechanisms

### 1. FUSE Permission Checks

Every filesystem operation triggers permission evaluation:

```go
// Pseudo-code of FUSE permission check
func (fs *FuseFS) Open(path string, flags int) (File, error) {
    perm := fs.permEngine.Evaluate(path)
    
    switch perm {
    case PermNone:
        return nil, syscall.ENOENT  // File not found (invisible)
    case PermView:
        if flags & O_RDONLY != 0 {
            return nil, syscall.EACCES  // Read denied
        }
        return fs.statOnly(path)  // Metadata only
    case PermRead:
        if flags & O_WRONLY != 0 || flags & O_RDWR != 0 {
            return nil, syscall.EACCES  // Write denied
        }
        return fs.openRead(path)
    case PermWrite:
        return fs.openWrite(path)  // Full access
    }
}
```

### 2. Priority-Based Rule Matching

When multiple rules match a path, the **highest priority wins**:

1. Explicit `priority` field (if set)
2. Pattern type: `file` (3) > `directory` (2) > `glob` (1)
3. Pattern specificity (longer/more specific wins)

**Example:**

```python
rules = [
    {"pattern": "**/*", "permission": "read"},                  # Priority: 0, type: glob
    {"pattern": "/secrets/**", "permission": "none"},           # Priority: 0, type: glob
    {"pattern": "/secrets/public.key", "permission": "read"},   # Priority: 0, type: file
]

# Results:
# /app/main.py           → read (matched by **/*) 
# /secrets/private.key   → none (matched by /secrets/**)
# /secrets/public.key    → read (file pattern overrides glob)
```

**Why this matters for security:**
- **Deny-by-default** with exceptions: Set restrictive base rules, add specific exceptions
- **Least privilege**: Default to minimal access, grant more only where needed
- **No ambiguity**: Deterministic priority prevents accidental exposure

### 3. Delta Layer Sync

Writes are isolated to per-sandbox delta directories:

```
Source (immutable):      /var/lib/agentfense/codebases/cb_123/app.py
Sandbox A delta:         /var/lib/agentfense/mounts/sb_A/delta/app.py
Sandbox B delta:         /var/lib/agentfense/mounts/sb_B/delta/app.py
```

**On exec completion:**
- Delta syncs to source (Last-Writer-Wins if conflicts)
- Prevents concurrent write corruption
- Ephemeral writes (lost if sandbox crashes)

## Security Best Practices

### 1. Default Deny, Explicit Allow

```python
# Good: Start with minimal permissions, add what's needed
permissions = [
    {"pattern": "**/*", "permission": "none"},      # Default: invisible
    {"pattern": "/src/**", "permission": "read"},   # Explicitly allow reading source
    {"pattern": "/output/**", "permission": "write"}, # Explicitly allow writing output
]

# Bad: Over-permissive default
permissions = [
    {"pattern": "**/*", "permission": "write"},     # Everything writable (dangerous)
]
```

### 2. Hide Secrets with High Priority

```python
# Ensure secrets are hidden even if other rules match
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},  # High priority
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
]
```

### 3. Use Presets as Starting Point

```python
from agentfense import extend_preset

# Start with agent-safe, customize
rules = extend_preset(
    base="agent-safe",  # Hides secrets, allows read-all, write to /output & /tmp
    additions=[
        {"pattern": "/logs/**", "permission": "write"},  # Also allow log writes
    ],
)
```

### 4. Disable Networking Unless Required

```yaml
# config.yaml
runtime:
  docker:
    network_mode: "none"         # No network access
    enable_networking: false     # Enforce no networking
```

### 5. Set Resource Limits

```python
from agentfense import Sandbox, ResourceLimits

with Sandbox.from_local(
    "./project",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB max
        pids_limit=100,                  # Max 100 processes
        cpu_quota=50000,                 # 50% CPU
    ),
) as sandbox:
    sandbox.run("untrusted_command")
```

## Auditing and Monitoring

Track sandbox activity for security analysis:

### Server Logs

AgentFense logs all sandbox operations:

```json
{
  "level": "info",
  "time": "2026-02-03T10:15:30Z",
  "msg": "sandbox_created",
  "sandbox_id": "sb_abc123",
  "codebase_id": "cb_xyz789",
  "user_id": "user_001",
  "permissions": [...]
}

{
  "level": "info",
  "time": "2026-02-03T10:15:35Z",
  "msg": "command_executed",
  "sandbox_id": "sb_abc123",
  "command": "cat /workspace/app.py",
  "exit_code": 0,
  "duration_ms": 45
}

{
  "level": "warn",
  "time": "2026-02-03T10:15:40Z",
  "msg": "permission_denied",
  "sandbox_id": "sb_abc123",
  "path": "/workspace/.env",
  "operation": "open",
  "permission": "none"
}
```

### What to Monitor

| Event | Why Monitor | Alert Threshold |
|-------|------------|----------------|
| `permission_denied` | Agent trying to access restricted files | >10/min per sandbox |
| `command_executed` | Audit trail of all commands | Log all for forensics |
| `sandbox_created` | Track sandbox creation rate | >100/hour per user |
| `resource_limit_exceeded` | Resource exhaustion attempts | Any occurrence |
| `session_duration` | Long-running sessions may indicate issues | >1 hour |

## Common Pitfalls

### 1. Forgetting to Hide `.git` Directories

```python
# Problem: .git may contain sensitive commit history
permissions = [
    {"pattern": "**/*", "permission": "read"},
]

# Solution: Explicitly hide .git
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "**/.git/**", "permission": "none"},  # Hide git history
]
```

### 2. Pattern Typos Exposing Secrets

```python
# WRONG: Typo in pattern (secrets still visible)
{"pattern": "/secret/**", "permission": "none"},  # Typo: "secret" not "secrets"

# CORRECT:
{"pattern": "/secrets/**", "permission": "none"},
```

**Mitigation:** Test permission rules before production. Use integration tests to verify secrets are hidden.

### 3. Over-Broad Write Permissions

```python
# DANGEROUS: Agent can modify source code
{"pattern": "**/*", "permission": "write"}

# SAFE: Only specific output directories are writable
{"pattern": "/output/**", "permission": "write"}
{"pattern": "/tmp/**", "permission": "write"}
```

## Next Steps

- **[Permission System](permissions.md)** - Deep dive into permission rules and patterns
- **[Presets Guide](presets.md)** - Using and extending permission presets
- **[Best Practices](best-practices.md)** - Production deployment security patterns
