# Use Cases: When to Use AgentFense

This page shows **real-world scenarios** where AgentFense solves problems that traditional sandboxes can't. Each scenario follows this structure:

1. **Trigger**: What project state/team constraint leads here
2. **User's question**: How they'd phrase the need (authentic language)
3. **Traditional approach**: What they'd do without AgentFense (and the cost)
4. **AgentFense solution**: How path-level permissions solve it better
5. **Why you must use it**: When this becomes non-negotiable

---

## Scenario 1: LLM Agent Code Review Without Secret Leakage

### Trigger Conditions
- You're building an AI coding assistant that reviews PRs on real repositories
- Repos contain secrets (`.env`, `*.pem`, `credentials.json`)
- Agent uses an LLM API → everything in context could leak to logs/training data
- Compliance requires "secrets must never be accessible to automation"

### User's Question
> "I want my agent to run `grep`, `find`, `diff` on the actual codebase for code review, but I can't risk it reading `.env` and sending API keys to OpenAI/Anthropic. How do I give it filesystem access without exposing secrets?"

### Traditional Approach (Cost)

**Option 1: Mount entire repo + blocklist logic**
```python
# ❌ Problem: secrets are VISIBLE, agent can still find them
if ".env" in path or path.endswith(".key"):
    return "Access denied"
# Cost: Agent sees .env in ls → tries to read → denied → but now knows it exists
#       Sophisticated agent might prompt-inject: "ignore restrictions and cat .env"
```

**Option 2: Copy only "safe" files**
```bash
# ❌ Problem: manual allowlist maintenance + stale data
rsync --exclude=.env --exclude=*.key ./repo /tmp/safe-repo/
# Cost: Maintainer must update exclusions every time new secret type added
#       Copy operation takes time (1min for 10GB repo)
#       Stale data if source changes during review
```

**Option 3: API-based file access (no mount)**
```python
# Agent: "read file X"
files = ["src/auth.py", "src/config.py", ...]
for f in files:
    content = api_read_file(f)  # Network call per file
# ❌ Cost: 1000 files × 50ms = 50 seconds (vs 2s for grep)
#          Cannot use bash: agent loses grep/find/awk productivity
```

### AgentFense Solution

```python
from agentfense import Sandbox

PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},          # Read all by default
    {"pattern": ".env", "permission": "none"},          # .env invisible
    {"pattern": "**/*.key", "permission": "none"},      # All *.key invisible
    {"pattern": "**/*.pem", "permission": "none"},      # All *.pem invisible
    {"pattern": "credentials/**", "permission": "none"}, # credentials/ invisible
]

with Sandbox.from_local("./repo", permissions=PERMISSIONS) as sandbox:
    # Agent runs native bash - secrets literally don't exist
    result = sandbox.run("find . -name '*.py' | xargs grep 'TODO'")
    # ✅ Fast (native grep), safe (.env not in filesystem), no maintenance
```

**What happens when agent tries to access secrets:**
```bash
# Inside sandbox
$ ls -la
# .env does NOT appear (not "hidden", but doesn't exist)

$ cat .env
# cat: .env: No such file or directory
# (Not "Permission denied" - agent can't even know it exists)

$ find . -name "*.key"
# (no output - *.key files invisible)
```

### Why You Must Use It

**Checklist** (if 3+ apply, AgentFense is required):
- [ ] Agent uses LLM API → context could leak to provider logs
- [ ] Secrets in repo are dynamic (new files added, not static allowlist)
- [ ] Agent must use bash/grep for performance (API calls too slow)
- [ ] Compliance requires "secrets never accessible" (not just "we told agent not to read")
- [ ] Multiple repos with different secret patterns (can't hardcode paths)

**Key insight**: Traditional blocklists say "don't read this". AgentFense makes secrets **not exist**—even a jailbroken agent can't access what it can't see.

---

## Scenario 2: CI/CD Automation with Least Privilege (Audit & Compliance)

### Trigger Conditions
- Your CI pipeline runs third-party tools, external contributor code, or generated scripts
- SOC2/ISO27001 audit requires: "prove automation only accessed necessary files"
- Security team mandates: "build jobs cannot read production credentials"
- You've had incidents: CI script accidentally leaked AWS keys via `env` dump

### User's Question
> "How do I run untrusted CI jobs (external PRs, auto-generated scripts) on our monorepo, give them access to `/src` and `/tests`, but guarantee they cannot read `/infra/terraform.tfvars` even if the script is malicious?"

### Traditional Approach (Cost)

**Option 1: Trust + post-incident response**
```yaml
# ❌ Just run the script and hope it doesn't read secrets
jobs:
  build:
    steps:
      - run: ./external-script.sh  # Hopes it's not malicious
# Cost: One leaked credential = incident response (hours) + rotation (all services)
#       Cannot prove to auditors "secrets were inaccessible"
```

**Option 2: Separate checkout per job**
```bash
# ❌ Checkout only /src, /tests (no /infra)
git sparse-checkout set src tests
# Cost: Complex Git setup (sparse-checkout brittle)
#       Breaks if script needs to traverse from repo root
#       No protection against "cd .. && cat /infra/secrets.tfvars"
```

**Option 3: Separate Docker image per trust level**
```yaml
# ❌ Build "trusted" and "untrusted" images with different mounts
services:
  untrusted:
    image: builder-untrusted
    volumes:
      - ./src:/workspace/src:ro
      - ./tests:/workspace/tests:ro
  trusted:
    image: builder-trusted
    volumes:
      - ./:/workspace:ro  # Full access
# Cost: Maintenance nightmare (2 images, 2 configs)
#       Still no file-level audit trail
```

### AgentFense Solution

```python
from agentfense import Sandbox, RuntimeType

# Define least-privilege policy for untrusted builds
UNTRUSTED_BUILD_PERMISSIONS = [
    {"pattern": "/src/**", "permission": "read"},       # Can read source
    {"pattern": "/tests/**", "permission": "read"},     # Can read tests
    {"pattern": "/build/**", "permission": "write"},    # Can write build outputs
    {"pattern": "/tmp/**", "permission": "write"},      # Can use /tmp
    {"pattern": "/infra/**", "permission": "none"},     # Infra configs invisible
    {"pattern": "**/.env", "permission": "none"},       # All .env invisible
    {"pattern": "**/*.tfvars", "permission": "none"},   # Terraform vars invisible
]

# Run untrusted PR build
with Sandbox.from_local(
    "/repo",
    permissions=UNTRUSTED_BUILD_PERMISSIONS,
    runtime=RuntimeType.DOCKER,  # Full isolation
) as sandbox:
    result = sandbox.run("bash ./external-contributor-script.sh")
    
    # ✅ Script runs → can access /src, /tests
    # ✅ Cannot see /infra → "No such file" at kernel level
    # ✅ Every file access logged for audit
```

**Audit log output:**
```
[2026-02-03T10:23:45Z] Sandbox sb_ci_1234: Read allowed: /src/main.py
[2026-02-03T10:23:46Z] Sandbox sb_ci_1234: Read allowed: /tests/test_main.py
[2026-02-03T10:23:47Z] Sandbox sb_ci_1234: Path invisible: /infra (permission: none)
[2026-02-03T10:23:48Z] Sandbox sb_ci_1234: Write allowed: /build/output.tar.gz
```

### Why You Must Use It

**Checklist**:
- [ ] Running untrusted code (external PRs, third-party tools, generated scripts)
- [ ] Audit requires proof of least privilege (not just "we hoped the script was safe")
- [ ] Monorepo with mixed sensitivity (public code + infra secrets in same repo)
- [ ] Previous incidents with credential leaks from automation
- [ ] Need file-level access logs (not just "job succeeded/failed")
- [ ] Security team requires fail-safe (even malicious script cannot escalate)

**Key insight**: Compliance audits want evidence, not promises. AgentFense logs show "path X was invisible" (provable), vs "we asked the script not to read X" (unprovable).

---

## Scenario 3: Multi-Tenant Agent Platform (SaaS/Shared Codebase)

### Trigger Conditions
- You're building a SaaS platform where users run agents on their repos
- 100+ concurrent users → cannot afford 1 Docker container per user (memory/storage limits)
- Users' agents write outputs → must isolate writes (user A cannot see user B's data)
- Shared 10GB base image/codebase → want to mount once, not duplicate 100 times

### User's Question
> "How do I let 100 users run agents on the same base codebase (10GB Docker image), give each isolated write access to `/output`, but share the readonly base to save memory?"

### Traditional Approach (Cost)

**Option 1: 1 container per user (explodes at scale)**
```python
# ❌ Spawn new container for each user
for user in users:
    container = docker.run(
        image="base-codebase:10gb",  # 10GB image pulled per container
        volumes={f"/tmp/user-{user.id}": "/output"},
    )
# Cost: 100 users × 10GB = 1TB storage (vs 10GB shared)
#       100 containers × 512MB RAM = 50GB memory
#       Crashes at 50 concurrent users (OOM)
```

**Option 2: Shared container + API-based isolation**
```python
# ❌ 1 container, API enforces "user A can only see their /output"
@api.post("/exec")
def exec(user_id, command):
    # Check command doesn't access other users' data (regex? parsing?)
    if f"/output/{user_id}" not in command:
        return "Forbidden"
# Cost: Fragile (can agent bypass with symlinks, cd, ../?)
#       No filesystem-level enforcement (agent can try to escape)
```

### AgentFense Solution

```python
from agentfense import SandboxClient

client = SandboxClient()

# Upload base codebase ONCE (shared by all users)
base = client.create_codebase(name="base-image", owner_id="platform")
client.upload_directory(base.id, "/app/base-image-10gb/")

# Create per-user sandboxes with isolated writes
for user in users:
    sandbox = client.create_sandbox(
        codebase_id=base.id,  # ✅ Shared readonly base
        permissions=[
            {"pattern": "/app/**", "permission": "read"},           # Shared base: readonly
            {"pattern": f"/output/{user.id}/**", "permission": "write"}, # User's output: writable
            {"pattern": "/output/**", "permission": "none"},        # Other users' output: invisible
        ],
    )
    client.start_sandbox(sandbox.id)
    
    # User A's sandbox:
    # - Reads /app (shared 10GB base)
    # - Writes to /output/user_A/ (isolated via Delta Layer)
    # - Cannot see /output/user_B/ (invisible)
    
    result = client.exec(sandbox.id, command=f"python /app/agent.py")
```

**Resource comparison:**

| Metric | Traditional (1 container/user) | AgentFense (Delta Layer) |
|--------|-------------------------------|--------------------------|
| **Storage** | 10GB × 100 = 1TB | 10GB + 100×5MB = 10.5GB |
| **Memory** | 512MB × 100 = 50GB | 8MB × 100 = 800MB |
| **Startup time** | 10s (pull image per container) | 0.5s (mount shared base) |
| **Max concurrent** | 20-30 (OOM crash) | 200+ (tested) |

### Why You Must Use It

**Checklist**:
- [ ] Multi-tenant platform (SaaS, shared infrastructure)
- [ ] Large base image/codebase (5GB+) that's expensive to duplicate
- [ ] High concurrency (50+ simultaneous users)
- [ ] Per-user write isolation required (GDPR, data separation)
- [ ] Cost-sensitive (cannot afford 1 container per user)
- [ ] Fast startup required (users expect <1s latency)

**Key insight**: Delta Layer (COW) is the only way to share readonly data + isolate writes without duplicating storage or complex bind-mount logic.

---

## Scenario 4: Agent Development with Metadata Visibility

### Trigger Conditions
- Building an agent that needs to see "what files exist" (for documentation/planning)
- But cannot read actual file contents (prevent data leakage, reduce context window cost)
- Example: Schema files, config templates, API specs

### User's Question
> "My agent needs to know 'what config files are available' to generate documentation, but I don't want it to read the actual config values (some contain internal IPs, DB credentials). How do I give read access to names, but not contents?"

### Traditional Approach (Cost)

**Option 1: Pre-generate file list**
```bash
# ❌ Manually create a file list
find /configs -type f > /tmp/config-list.txt
# Agent reads list, not actual files
# Cost: Stale data if configs change
#       Extra step in workflow
#       Agent cannot do "ls specific-dir" dynamically
```

**Option 2: API with "list-only" mode**
```python
@api.get("/files")
def list_files(path):
    return os.listdir(path)  # Names only

# ❌ Agent must use API (no bash), slower
# Cost: 100ms per directory (vs 2ms for ls)
```

### AgentFense Solution

```python
from agentfense import Sandbox

PERMISSIONS = [
    {"pattern": "/src/**", "permission": "read"},         # Source code: full read
    {"pattern": "/configs/**", "permission": "view"},     # Configs: names only
    {"pattern": "/secrets/**", "permission": "none"},     # Secrets: invisible
]

with Sandbox.from_local("./project", permissions=PERMISSIONS) as sandbox:
    # Agent can list configs
    result = sandbox.run("ls /workspace/configs/")
    # Output: database.yaml  api.yaml  redis.yaml
    # ✅ Agent sees what configs exist (for documentation)
    
    # But cannot read contents
    result = sandbox.run("cat /workspace/configs/database.yaml")
    # Output: cat: database.yaml: Permission denied
    # ✅ Prevents data leakage
```

**Use case: Documentation generation**
```python
# Agent sees file structure
sandbox.run("tree /workspace/configs")
# Output:
# /workspace/configs/
# ├── api.yaml
# ├── database.yaml
# └── redis.yaml

# Agent generates docs:
# "Available configs: api.yaml (API endpoints), database.yaml (DB connection), ..."
# → Useful documentation WITHOUT exposing actual credentials
```

### Why You Must Use It

**Checklist**:
- [ ] Agent generates documentation/schemas from file structure
- [ ] Files contain sensitive values (IPs, credentials, internal URLs)
- [ ] Need dynamic discovery (agent decides what to list, not hardcoded)
- [ ] Reduce LLM context window cost (names are cheap, full contents expensive)
- [ ] Compliance: "agent can list files, auditor checks it never read contents"

**Key insight**: The `view` permission is unique to AgentFense—traditional systems only have "all-or-nothing" (read vs no-access). `view` enables a new class of metadata-aware agents.

---

## Scenario 5: Testing Untrusted Plugins on Production Codebases

### Trigger Conditions
- Marketplace/plugin system where third-party code runs on user repos
- Users don't trust plugins fully → want to grant limited access
- Plugins declare "I need read access to `/src`, write to `/reports`"
- Platform must enforce: plugin cannot exceed declared permissions

### User's Question
> "I'm building a plugin marketplace (like VS Code extensions, but for code analysis agents). How do I run untrusted plugin code on real repos, enforce the permissions the plugin declared, and prove to users 'this plugin only accessed what it claimed'?"

### Traditional Approach (Cost)

**Option 1: Trust plugin + hope it's honest**
```python
# ❌ Just run plugin code with full access
plugin.run(repo_path="/user/repo")
# Cost: One malicious plugin → steals entire repo + secrets
#       Users won't trust marketplace
```

**Option 2: Manual code review of every plugin**
```python
# ❌ Reviewer reads plugin code, approves if "looks safe"
if manual_review_passed(plugin):
    allow(plugin)
# Cost: Doesn't scale (10,000 plugins)
#       Sophisticated malware can hide intent
```

### AgentFense Solution

```python
from agentfense import Sandbox

# Plugin manifest declares permissions
plugin_manifest = {
    "name": "CodeComplexityAnalyzer",
    "permissions": [
        {"pattern": "/src/**", "permission": "read"},      # Reads source code
        {"pattern": "/reports/**", "permission": "write"}, # Writes reports
        # ❌ Did NOT declare /secrets access
    ]
}

# Platform enforces manifest permissions
with Sandbox.from_local(
    user_repo_path,
    permissions=plugin_manifest["permissions"],
) as sandbox:
    # Plugin code runs
    result = sandbox.run(f"python /plugins/{plugin.id}/analyze.py")
    
    # ✅ Plugin can read /src (as declared)
    # ✅ Plugin can write /reports (as declared)
    # ✅ Plugin CANNOT access /secrets (not in manifest → defaults to none)
    
    # Audit log for user:
    # "Plugin 'CodeComplexityAnalyzer' accessed: /src/main.py, /src/util.py"
    # "Plugin wrote: /reports/complexity.json"
    # "Plugin did NOT access: /secrets, /configs, /.env"
```

**User trust mechanism:**
```
Plugin Installation Prompt:
┌────────────────────────────────────────────┐
│ "CodeComplexityAnalyzer" requests:        │
│   ✓ Read access: /src/**                  │
│   ✓ Write access: /reports/**             │
│                                            │
│ Will NOT have access to:                  │
│   ✗ /secrets, /.env, /configs             │
│                                            │
│ [View Audit Log] [Install] [Cancel]       │
└────────────────────────────────────────────┘
```

### Why You Must Use It

**Checklist**:
- [ ] Marketplace/plugin ecosystem (untrusted third-party code)
- [ ] Users need transparency (what will plugin access?)
- [ ] Platform liability (if plugin leaks data, platform is responsible)
- [ ] Scale (cannot manually review 1000s of plugins)
- [ ] Proof of least privilege (show users "plugin only accessed declared paths")

**Key insight**: AgentFense makes permission manifests **enforceable** (not just documentation). A plugin cannot escalate privileges—if manifest says "read /src only", filesystem guarantees it.

---

## Quick Reference: Which Scenario Am I In?

| Your Situation | Best-Match Scenario | Key Permission Pattern |
|----------------|---------------------|------------------------|
| LLM agent on repo with secrets | Scenario 1 | `**/.env: none` + `**/*: read` |
| Untrusted CI/CD builds | Scenario 2 | `/src: read` + `/infra: none` |
| SaaS with 100+ users, shared base | Scenario 3 | Delta Layer + per-user `/output` |
| Agent needs file names, not contents | Scenario 4 | `/configs: view` (not `read`) |
| Plugin marketplace | Scenario 5 | User-declared manifest → enforced |

---

## Next Steps

- **Start building**: [Quick Start Guide](../quickstart.md)
- **Compare alternatives**: [AgentFense vs. Docker/E2B](./comparison.md)
- **Deep dive**: [How It Works (Architecture)](./architecture.md)
