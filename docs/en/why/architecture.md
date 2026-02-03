# Architecture Overview

This page explains **how AgentFense works internally**, from a developer's perspective. We focus on **key mechanisms** (permission checks, Delta Layer, FUSE) and **data flow** (what happens when you run a command).

---

## TL;DR: How It Works

**The one-sentence version**: AgentFense mounts your codebase as a FUSE filesystem that intercepts every file operation (`open`, `readdir`, `stat`), checks permissions in real-time, and isolates writes via Copy-On-Write—all inside a lightweight namespace (bwrap) or container (Docker).

**Why this matters**:
- **Permission checks happen at syscall level** → even malicious code cannot bypass (not just "hoping" agent obeys rules)
- **Secrets are literally invisible** → `none` permission means files don't appear in `ls`, `find`, or any syscall
- **Multi-sandbox isolation** → 100 sandboxes share 1 codebase (10GB) but writes don't conflict (Delta Layer COW)

---

## Four-Layer Architecture

AgentFense is designed in four layers, each with clear responsibilities:

```
┌─────────────────────────────────────────────────────────────┐
│                   Layer 1: Client                           │
│      Python SDK  │  Go SDK (future)  │  REST API            │
│                                                              │
│  User code:                                                  │
│    with Sandbox.from_local("./repo") as sb:                 │
│        sb.run("grep TODO *.py")                              │
└──────────────────────────┬──────────────────────────────────┘
                           │ gRPC / REST
┌──────────────────────────▼──────────────────────────────────┐
│                   Layer 2: Service                           │
│   gRPC Server  │  REST Gateway  │  Codebase Manager         │
│                                                              │
│  Responsibilities:                                           │
│  - Manage codebase storage (upload, list, delete)           │
│  - Sandbox lifecycle (create, start, exec, stop)            │
│  - Coordinate runtime + filesystem + permissions            │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   Layer 3: Runtime                           │
│   Runtime Manager  │  Permission Engine  │  Executor         │
│                                                              │
│  Implementations:                                            │
│  - bwrap: Lightweight (Linux namespaces + seccomp)          │
│  - docker: Full isolation (containers + resource limits)    │
│  - mock: Testing (no actual execution)                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   Layer 4: Isolation                         │
│   FUSE Filesystem  │  Delta Layer (COW)  │  Namespaces      │
│                                                              │
│  Mechanisms:                                                 │
│  - FUSE: Intercepts every syscall, enforces permissions     │
│  - Delta: Copy-On-Write for multi-sandbox write isolation   │
│  - bwrap/Docker: Process/network/IPC isolation              │
└─────────────────────────────────────────────────────────────┘
```

### Layer 1: Client (User-Facing SDKs)

**Purpose**: Provide ergonomic APIs for developers to create sandboxes, run commands, manage permissions.

**Components**:
- **Python SDK** (`sdk/python/agentfense/`):
  - High-level: `Sandbox.from_local()` (one-liner)
  - Low-level: `SandboxClient` (full control)
  - Async support: `AsyncSandbox` (high concurrency)
- **REST API** (HTTP gateway via grpc-gateway):
  - For non-Python/Go languages
  - Standard CRUD on codebases, sandboxes, exec

**Example**:
```python
from agentfense import Sandbox

# High-level API abstracts all 4 layers
with Sandbox.from_local("./repo", preset="agent-safe") as sandbox:
    result = sandbox.run("grep TODO *.py")  # Permissions enforced transparently
```

---

### Layer 2: Service (gRPC + Business Logic)

**Purpose**: Manage codebase storage, sandbox lifecycle, coordinate between runtimes and filesystems.

**Components**:
- **gRPC Server** (`internal/server/server.go`):
  - Implements `CodebaseService` (create, upload, list, delete)
  - Implements `SandboxService` (create, start, exec, stop, destroy)
  - Validates permission rules, normalizes paths
- **Codebase Manager** (`internal/codebase/manager.go`):
  - Stores codebases on disk (`storage.codebase_path`)
  - Assigns unique IDs (`cb_<uuid>`)
  - Tracks metadata (name, owner, created_at)
- **REST Gateway** (auto-generated via grpc-gateway):
  - Translates HTTP → gRPC (no extra code needed)
  - Enables `curl` access, web frontend integration

**Data flow on sandbox creation**:
1. Client: `create_sandbox(codebase_id, permissions)`
2. Server validates permission rules (patterns, types)
3. Server allocates sandbox ID (`sb_<uuid>`)
4. Server creates delta directory (`<mount_path>/delta/<sandbox_id>`)
5. Server signals runtime layer: "prepare sandbox"

---

### Layer 3: Runtime (Isolation + Execution)

**Purpose**: Execute commands inside isolated environments (namespaces / containers), enforce resource limits.

**Components**:
- **Runtime Interface** (`internal/runtime/runtime.go`):
  - `Create()`: Prepare sandbox environment
  - `Start()`: Activate isolation (mount FUSE, start container)
  - `Exec()`: Run command, return stdout/stderr/exit_code
  - `Stop()`, `Destroy()`: Cleanup

**Implementations**:

#### bwrap Runtime (`internal/runtime/bwrap/`)
- **Mechanism**: Linux namespaces (mount, PID, network, IPC) + bubblewrap
- **Advantages**: Lightweight (<1s startup), low overhead (~5MB/sandbox)
- **Limitations**: Linux-only, requires `bwrap` binary, weaker isolation than VMs
- **Use case**: High concurrency (200+ sandboxes), trusted-ish code

**How it works**:
```bash
# AgentFense runs this under the hood
bwrap \
  --ro-bind /usr /usr \
  --bind /path/to/fuse-mount /workspace \  # FUSE mount with permissions
  --unshare-all \
  --die-with-parent \
  bash -c "grep TODO *.py"
```

#### Docker Runtime (`internal/runtime/docker/`)
- **Mechanism**: Docker containers with bind mounts
- **Advantages**: Full isolation (process/network/filesystem), custom images, resource limits (CPU/memory/PIDs)
- **Limitations**: Slower startup (~5-10s), higher overhead (~512MB/container)
- **Use case**: Untrusted code, compliance requirements, need custom environments

**How it works**:
```bash
# AgentFense runs this under the hood
docker run --rm \
  -v /path/to/fuse-mount:/workspace \  # FUSE mount (permissions enforced)
  --memory=512m --cpus=1 --pids-limit=100 \
  --network=none \  # Optional: disable network
  python:3.11-slim bash -c "grep TODO *.py"
```

#### Mock Runtime (`internal/runtime/mock/`)
- **Purpose**: Testing only (no actual execution, returns empty results)
- **Use case**: Server/API logic tests without isolation overhead

**Session Support** (Stateful Shell):
- **Problem**: Each `exec()` call starts fresh shell → loses `cd`, env vars
- **Solution**: `SessionManager` maintains persistent shell process
- **Implementation**: Multiplexed stdin/stdout over persistent bash process

```python
with sandbox.session() as session:
    session.exec("cd /workspace/src")  # State persists
    result = session.exec("pwd")       # → /workspace/src
    session.exec("export VAR=value")
    result = session.exec("echo $VAR") # → value
```

---

### Layer 4: Isolation (FUSE + Delta + Namespaces)

**Purpose**: Enforce permissions at kernel level, isolate writes across sandboxes, prevent process escape.

**Components**:

#### FUSE Filesystem (`internal/fs/fuse.go`)

**What is FUSE?**
- **Filesystem in Userspace**: Lets you implement a custom filesystem as a regular program (no kernel module needed)
- **How it works**: Kernel forwards syscalls (`open`, `readdir`, `stat`) to your program → your program decides what to return
- **Production usage**: Google (GCS Fuse), Dropbox (file sync), S3 mounts, encrypted filesystems

**AgentFense's FUSE implementation**:
1. **Mount codebase**: Maps `/path/to/source` → `/path/to/fuse-mount`
2. **Intercept syscalls**: Every `open()`, `readdir()`, `stat()` goes through FUSE
3. **Check permissions**: For each operation, check permission rules → allow or deny
4. **Return results**: If allowed, proxy to source; if denied, return `EACCES`; if `none`, return `ENOENT` (file doesn't exist)

**Permission enforcement examples**:

| Syscall | Path | Permission | Result |
|---------|------|------------|--------|
| `open("/workspace/src/main.py", O_RDONLY)` | `/src/main.py` | `read` | ✅ Return file descriptor |
| `open("/workspace/secrets/.env", O_RDONLY)` | `/secrets/.env` | `none` | ❌ Return `ENOENT` (no such file) |
| `open("/workspace/docs/README.md", O_WRONLY)` | `/docs/README.md` | `read` (not write) | ❌ Return `EACCES` (permission denied) |
| `readdir("/workspace")` | `/` | - | Return list WITHOUT `/secrets` (invisible) |

**Why FUSE, not LD_PRELOAD or syscall hooks?**
- **LD_PRELOAD**: Only intercepts libc calls, trivial to bypass (direct syscalls)
- **Syscall hooks** (ptrace): High overhead, fragile (breaks debuggers)
- **FUSE**: Kernel-enforced, cannot bypass, standard Linux feature

#### Permission Engine (`internal/fs/permission.go`)

**Permission model**:

```
Pattern Types:
  - file: Exact match (/config.yaml)
  - directory: Prefix match (/docs/ matches /docs/README.md)
  - glob: Wildcard (**/*.py, /secrets/**)

Permission Levels:
  - none: Invisible (doesn't appear in ls, behaves like doesn't exist)
  - view: List-only (can see name in ls, cannot read content)
  - read: Can read file content
  - write: Can read + modify/create files
```

**Priority algorithm** (deterministic rule resolution):
1. **Explicit Priority field** (if set by user)
2. **Pattern type**: file (3) > directory (2) > glob (1)
3. **Pattern specificity**: `/secrets/public.key` > `/secrets/**` > `**/*`

**Example resolution**:
```python
rules = [
    {"pattern": "**/*", "permission": "read"},           # Priority: 1 (glob)
    {"pattern": "/secrets/**", "permission": "none"},    # Priority: 1 (glob, but more specific)
    {"pattern": "/secrets/public.key", "permission": "read"},  # Priority: 3 (file)
]

# Query: /secrets/public.key
# Match: All 3 rules match
# Winner: /secrets/public.key (highest priority: file > glob)
# Result: read (public key is readable exception)
```

#### Delta Layer (Copy-On-Write) (`internal/fs/delta.go`)

**Problem**: Multiple sandboxes share the same codebase → if one writes to `/output/report.txt`, others shouldn't see it (isolation), but copying entire codebase per sandbox is expensive (1TB for 100 users).

**Solution**: Copy-On-Write (COW) at file level.

**How it works**:

```
Directory structure:
/storage/
  codebases/
    cb_001/          ← Source codebase (10GB, shared)
      src/
      docs/
      tests/
  mounts/
    delta/
      sb_user_a/     ← User A's changes (5MB)
        output/
          report.txt
      sb_user_b/     ← User B's changes (5MB)
        output/
          results.json
    mount_sb_user_a/  ← User A's FUSE mount
    mount_sb_user_b/  ← User B's FUSE mount
```

**Read logic** (FUSE intercepts `open` for read):
1. Check delta directory: `/storage/mounts/delta/sb_user_a/path/to/file`
2. If exists → return delta version (user's changes)
3. If not → fallback to source: `/storage/codebases/cb_001/path/to/file`

**Write logic** (FUSE intercepts `open` for write):
1. Write goes to delta: `/storage/mounts/delta/sb_user_a/path/to/file`
2. Create parent directories if needed
3. Source remains untouched (other sandboxes see original)

**Delete logic** (whiteout markers):
1. Create `.wh.<filename>` in delta (e.g., `.wh.old.txt`)
2. On read: If `.wh.old.txt` exists → return `ENOENT` (file deleted)
3. Source file still exists (other sandboxes unaffected)

**Sync logic** (on `exec()` completion):
- Option 1: **No sync** (default for ephemeral sandboxes)
- Option 2: **Sync to source** (Last-Writer-Wins for persistent changes)
  - Copy delta files → source
  - Remove whiteout markers (apply deletes)
  - Conflict resolution: newer timestamp wins

**Resource usage**:
- 100 sandboxes, 10GB codebase, each writes 5MB:
  - Traditional: 10GB × 100 = 1TB
  - Delta Layer: 10GB + 5MB × 100 = 10.5GB (98% reduction)

---

## Data Flow: What Happens When You Run a Command

Let's trace a complete request from client to syscall enforcement.

### Step-by-Step Example

**User code**:
```python
from agentfense import Sandbox

with Sandbox.from_local(
    "./repo",
    permissions=[
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "/secrets/**", "permission": "none"},
    ],
    runtime="bwrap",
) as sandbox:
    result = sandbox.run("cat /workspace/src/main.py")
```

**Timeline** (what happens under the hood):

#### Phase 1: Sandbox Creation (setup)

1. **Client → gRPC**: `CreateCodebase(name="repo")`
   - Server assigns ID: `cb_12345`
   - Creates directory: `/storage/codebases/cb_12345/`

2. **Client → gRPC**: `UploadFiles(codebase_id="cb_12345", files=...)`
   - Copies `./repo/*` → `/storage/codebases/cb_12345/`

3. **Client → gRPC**: `CreateSandbox(codebase_id="cb_12345", permissions=[...])`
   - Server validates rules
   - Assigns ID: `sb_67890`
   - Creates delta: `/storage/mounts/delta/sb_67890/`

4. **Client → gRPC**: `StartSandbox(sandbox_id="sb_67890")`
   - Server signals runtime: "start sandbox"
   - Runtime starts FUSE mount:
     - Source: `/storage/codebases/cb_12345/`
     - Delta: `/storage/mounts/delta/sb_67890/`
     - Mount point: `/storage/mounts/mount_sb_67890/`
     - Permission rules: loaded into FUSE server
   - Runtime prepares bwrap command (but doesn't execute yet)

#### Phase 2: Command Execution

5. **Client → gRPC**: `Exec(sandbox_id="sb_67890", command="cat /workspace/src/main.py")`
   - Server asks runtime: "execute command"
   - Runtime runs:
     ```bash
     bwrap \
       --bind /storage/mounts/mount_sb_67890 /workspace \
       bash -c "cat /workspace/src/main.py"
     ```

6. **Inside bwrap namespace**:
   - Process starts: `bash -c "cat /workspace/src/main.py"`
   - `cat` calls `open("/workspace/src/main.py", O_RDONLY)`
   - **Kernel → FUSE**: "open request for /workspace/src/main.py"

7. **FUSE server** (AgentFense):
   - Normalize path: `/workspace/src/main.py` → `/src/main.py`
   - Check permission: match rules, find `**/*` → `read` permission
   - Check delta: `/storage/mounts/delta/sb_67890/src/main.py` → doesn't exist
   - Fallback to source: `/storage/codebases/cb_12345/src/main.py` → exists
   - Log: `[INFO] Read allowed: /src/main.py`
   - Return file descriptor to kernel

8. **Kernel → cat**: File descriptor returned
   - `cat` reads file content
   - Prints to stdout

9. **Runtime → Server**: Command finished (stdout, stderr, exit_code=0)

10. **Server → Client**: Return `ExecuteResult(stdout="...", exit_code=0)`

#### Phase 3: Permission Denial (what if agent tries to read secrets?)

**User runs**: `sandbox.run("cat /workspace/secrets/.env")`

6. **Inside bwrap**: `cat` calls `open("/workspace/secrets/.env", O_RDONLY)`

7. **FUSE server**:
   - Normalize: `/workspace/secrets/.env` → `/secrets/.env`
   - Check permission: match `/secrets/**` → `none` permission
   - Log: `[WARN] Path invisible: /secrets/.env (permission: none)`
   - Return `ENOENT` (no such file or directory)

8. **Kernel → cat**: Error `ENOENT`
   - `cat` prints: `cat: /workspace/secrets/.env: No such file or directory`
   - Exit code: 1

**Key insight**: Agent cannot tell if `.env` exists or never existed—it's truly invisible.

---

## Key Mechanisms Explained

### 1. Why Secrets Are Invisible (Not Just Blocked)

**Traditional blocklist** (returns "Permission denied"):
```
$ ls /workspace
src/  docs/  .env  ← Agent sees .env exists
$ cat .env
cat: .env: Permission denied  ← Agent knows file is sensitive
```

**AgentFense `none` permission** (returns "No such file"):
```
$ ls /workspace
src/  docs/  ← .env doesn't appear
$ cat .env
cat: .env: No such file or directory  ← Agent has zero knowledge
```

**Implementation**:
- `readdir()` (list directory): Filter out `none` paths before returning
- `stat()` (check file exists): Return `ENOENT` for `none` paths
- `open()`: Return `ENOENT` for `none` paths

### 2. The `view` Permission (List-Only)

**Use case**: Agent needs to see "what configs exist" but cannot read values.

```python
{"pattern": "/configs/**", "permission": "view"}
```

**Behavior**:
- `readdir("/workspace/configs")` → Returns `["api.yaml", "db.yaml"]` ✅
- `open("/workspace/configs/api.yaml", O_RDONLY)` → `EACCES` (permission denied) ❌

**Implementation**:
- `readdir()`: Allow (show filenames)
- `open()` for read: Deny (`EACCES`)
- Useful for documentation generation, schema discovery

### 3. Multi-Sandbox Isolation (Delta Layer)

**Scenario**: 2 users run agents on same codebase, both write to `/output/report.txt`.

**Without Delta Layer** (traditional Docker):
- User A's container: Writes `/output/report.txt` (content: "A's data")
- User B's container: Writes `/output/report.txt` (content: "B's data")
- **Problem**: Last write wins, A's data lost (or need separate volumes → 1TB storage)

**With Delta Layer**:
- User A's sandbox:
  - Writes to: `/storage/mounts/delta/sb_user_a/output/report.txt` (content: "A's data")
  - Reads from: Delta first, fallback to source
  - Sees: `/output/report.txt` = "A's data"

- User B's sandbox:
  - Writes to: `/storage/mounts/delta/sb_user_b/output/report.txt` (content: "B's data")
  - Reads from: Delta first, fallback to source
  - Sees: `/output/report.txt` = "B's data"

- Source: `/storage/codebases/cb_001/output/` (empty, untouched)

**Result**: Both users see their own data, zero conflicts, 10GB shared base + 5MB per user.

---

## Performance Characteristics

### Benchmarks (1 sandbox, bwrap runtime)

| Operation | Overhead vs. Native |
|-----------|---------------------|
| `open()` (permission check) | +50μs (0.05ms) |
| `readdir()` (filter `none` paths) | +100μs for 1000 files |
| `grep -r "TODO" src/` (10,000 files) | +3% (FUSE overhead) |
| Command startup | +200ms (FUSE mount) |

### Scalability (100 concurrent sandboxes)

| Runtime | Storage | Memory | CPU | Max Concurrency |
|---------|---------|--------|-----|-----------------|
| bwrap | 10.5GB | 800MB | 10% (idle) | 200+ |
| Docker | 10.5GB | 1.3GB | 15% (idle) | 150+ |

**Bottlenecks**:
- Memory: Each FUSE mount + delta ~8MB
- File descriptors: Each sandbox opens ~50 FDs
- CPU: FUSE syscall handling (minimal)

---

## Failure Modes & Recovery

### 1. FUSE Mount Fails

**Symptom**: `fusermount: mount failed: Transport endpoint not connected`

**Cause**: FUSE process crashed, stale mount

**Recovery**:
- Unmount: `fusermount -u /path/to/mount`
- Check logs: FUSE server stderr
- Restart sandbox

### 2. Delta Sync Conflict (LWW)

**Symptom**: User A and B both edited `/src/file.py`, sync uses Last-Writer-Wins

**Result**: B's changes overwrite A's (if B finishes last)

**Mitigation**:
- Use ephemeral sandboxes (don't sync delta → source)
- Implement 3-way merge (future)
- Lock files during multi-sandbox editing (future)

### 3. Permission Denial Debugging

**Symptom**: Agent gets "Permission denied" unexpectedly

**Debug**:
1. Check FUSE logs: `[WARN] Write denied: /src/main.py (permission: read)`
2. Verify permission rules: Does pattern match path?
3. Check priority: Is a more specific rule overriding?

---

## Next Steps

- **Try it hands-on**: [Quick Start](../quickstart.md)
- **Real-world scenarios**: [Use Cases](./use-cases.md)
- **Compare alternatives**: [vs. Docker/E2B](./comparison.md)
- **Contribute**: [GitHub Repository](https://github.com/AjaxZhan/AgentFense)
