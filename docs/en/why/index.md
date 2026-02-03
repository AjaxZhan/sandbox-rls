# Why AgentFense?

## TL;DR

**What it is**: A filesystem-backed sandbox that enforces path-level least privilege for AI agents and automation, letting them execute bash commands on real codebases without exposing secrets.

**Core value**: Run untrusted code against production repos with filesystem-level access control—agents can use bash/grep/find naturally, but `.env` is literally invisible, not just "off-limits".

**Use it when**: You need to grant automation (AI agents, CI scripts, third-party tools) precise filesystem access—not "read the whole repo" or "mount nothing", but "read docs/, write output/, secrets don't exist".

---

## The Problem: Filesystem Access is All-or-Nothing

### What Developers Face Today

When you need to run automation against a real codebase, you face an impossible choice:

**Option 1: Mount the entire repository**
```bash
docker run -v $(pwd):/workspace my-agent
# ❌ Exposes .env, .key files, customer_data/, everything
# ❌ Agent can read secrets, overwrite critical files
# ❌ No audit trail of what was actually accessed
```

**Option 2: Don't mount at all, use API-based file access**
```python
# Agent asks: "read file X"
content = api.read_file(path)
# ❌ Loses bash productivity (no ls, grep, find)
# ❌ 10x more API calls, 10x slower
# ❌ Agent can't use familiar shell commands
```

**Option 3: Copy files in/out**
```bash
cp safe_files/ /tmp/sandbox/
docker run /tmp/sandbox agent-command
cp /tmp/sandbox/output/ ./results/
# ❌ Stale data if source changes
# ❌ Storage overhead (duplicate every file)
# ❌ Can't handle large repos (100GB+ codebases)
```

### The Real Cost

Let's quantify what this actually costs teams:

| Pain Point | Traditional Approach | Cost |
|------------|---------------------|------|
| **Security incident** | Mount entire repo, agent leaks `.env` to logs | 1 leaked key = incident response (hours) + rotation (all services) + postmortem |
| **Development velocity** | API-based file access → 10x more roundtrips | 30-second task becomes 5 minutes; 100 daily tasks = 8 hours wasted/week |
| **Compliance audit** | No visibility into what agent accessed | Manual code review of every agent run; fails audit requirements |
| **Multi-tenant isolation** | Run separate container per user/task | 10GB per container × 100 users = 1TB wasted; crashes at 50 concurrent users |
| **Maintenance burden** | Custom permission logic in every agent script | 200 lines of "check if path is secret" code × 15 scripts = 3000 LOC to maintain |

**Concrete example**: A typical LLM agent running code review on a monorepo:
- Without path-level control: Reads 10,000 files including `.env` → leaks API keys in context → credential rotation + downtime
- With AgentFense: Sees only `/src`, `/tests` (15% of files) → completes review → zero secrets exposed

---

## Why This Matters: From "Can Run" to "Can Control"

Traditional sandboxes solve **isolation** (run untrusted code safely). AgentFense solves **least privilege** (control what isolated code can see).

### The Shift: Filesystem-Level Access Control

Instead of "give the agent read access to the container", you define:

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},       # Default: read everything
    {"pattern": "/docs/**", "permission": "write"},  # Can modify docs
    {"pattern": "/secrets/**", "permission": "none"}, # Literally invisible
]
```

**What this enables**:

1. **Secrets are invisible, not just "blocked"**
   - `ls /workspace` → `.env` doesn't appear
   - `cat .env` → "No such file" (not "Permission denied")
   - Agent can't even know secrets exist → can't prompt-inject around restrictions

2. **Visibility control (the `view` level)**
   - Agent can see `/metadata/schema.json` exists (for documentation)
   - But cannot read its contents (prevent data leakage)
   - Useful for: listing available configs without exposing values

3. **Path-level write boundaries**
   - Agent writes to `/output`, `/tmp` only
   - Cannot accidentally `rm -rf /src`
   - Automated rollback if agent crashes

4. **Multi-sandbox codebase sharing (Delta Layer)**
   - 100 sandboxes share 1 codebase (10GB) → 10GB + 100×5MB = 10.5GB total
   - Each sandbox's writes isolated via Copy-On-Write
   - No cross-contamination between concurrent tasks

---

## The Value: Security + Efficiency + Auditability

### 1. Security: Prevent Leaks by Design

**Problem**: AI agents often log everything they see. If they see `.env`, they leak it.

**Solution**: Make secrets **invisible** (not just "forbidden").

```python
# Traditional mount: agent can read .env
result = agent.run("find . -name '*.env'")
# → Finds .env → reads it → logs it → leaked

# AgentFense: .env doesn't exist in agent's reality
result = sandbox.run("find . -name '*.env'")  
# → Finds nothing → cannot leak what it can't see
```

**Measurable impact**:
- **Zero secret exposure**: Paths with `none` permission cannot appear in `ls`, `find`, or any syscall
- **90% reduction in audit surface**: If agent can only see 10% of files, you only audit 10% of accesses
- **Compliance-ready**: Every file access logged with permission check result

### 2. Efficiency: Bash Productivity Without API Tax

**Problem**: API-based file access is 10x slower than filesystem operations.

**Solution**: Agent uses native bash—filesystem enforces permissions transparently.

```python
# API-based: each file operation is a network call
for file in files:
    content = api.read(file)  # 50ms per call
    process(content)
# 1000 files × 50ms = 50 seconds

# AgentFense: agent runs bash directly
sandbox.run("grep -r 'TODO' /workspace/src")  
# → Native grep speed → 2 seconds for 1000 files
```

**Measurable impact**:
- **25x faster** for bulk operations (grep, find, wc)
- **Zero learning curve**: Agents already know bash/ls/cat
- **Lower token cost**: No need to explain "use API calls for file access"

### 3. Cost: Multi-Tenant Without Multi-Container

**Problem**: Running 100 Docker containers for 100 users → 1TB memory, crashes at scale.

**Solution**: 100 sandboxes share 1 codebase via Delta Layer (COW).

| Approach | Storage | Memory | Max Concurrent |
|----------|---------|--------|----------------|
| Docker per user | 10GB × 100 = 1TB | 512MB × 100 = 50GB | 20-30 (OOM crash) |
| AgentFense + Delta | 10GB + 100×5MB = 10.5GB | 8MB × 100 = 800MB | 200+ (tested) |

**Measurable impact**:
- **95% storage reduction** (10.5GB vs 1TB)
- **98% memory reduction** (800MB vs 50GB)
- **10x concurrency increase** (200 vs 20)

### 4. Auditability: Know What Was Accessed

**Problem**: No visibility into "did the agent touch sensitive files?"

**Solution**: Every file operation logged with permission checks.

```
[INFO] Sandbox sb_123: Read allowed: /docs/README.md
[WARN] Sandbox sb_123: Write denied: /src/auth.py (permission: read)
[INFO] Sandbox sb_123: Path invisible: /secrets/.env (permission: none)
```

**Measurable impact**:
- **Complete audit trail**: Every access logged (allowed or denied)
- **Forensics-ready**: After incident, replay "what did agent X see?"
- **Compliance proof**: Export logs showing secrets were never accessible

---

## When You MUST Use AgentFense

Check these conditions—if 3+ apply, AgentFense is the right tool:

- [ ] **You run untrusted or semi-trusted code** (AI agents, third-party plugins, CI jobs from external contributors)
- [ ] **Your codebase contains secrets** (`.env`, `*.key`, `credentials.json`, customer data) that must never be exposed
- [ ] **You need path-level granularity** (not "read the whole repo" or "read nothing")—e.g., "read docs/, write output/, secrets invisible"
- [ ] **Agents must use bash/shell commands** (not just API calls)—for performance, familiarity, or complex operations (grep/find/awk)
- [ ] **Multi-user or multi-task concurrency** (SaaS platform, CI/CD with parallel jobs, agent swarm)—cannot afford 1 container per task
- [ ] **Compliance/audit requirements** (SOC2, HIPAA, PCI-DSS)—need proof that secrets were never accessible, not just "we told the agent not to read them"
- [ ] **Fail-safe isolation** (even if agent is compromised/prompt-injected, it cannot escape permissions)
- [ ] **You're building a platform** (not a one-off script)—need reusable, declarative permission model

**If you checked 3+**: AgentFense gives you a permission system you can't build yourself without months of FUSE/kernel/security engineering.

**If you checked 6+**: You're in AgentFense's sweet spot—filesystem-level least privilege is your only scalable path.

---

## What Makes AgentFense Different

### It's Not Just "Another Sandbox"

| Dimension | Traditional Sandboxes | AgentFense |
|-----------|----------------------|------------|
| **Granularity** | Container/VM-level isolation | Path-level permissions |
| **Visibility** | All files visible (block reads) | `none` = invisible (zero knowledge) |
| **Approach** | "Agent can see everything, we block access" | "Agent's reality is limited to allowed paths" |
| **Auditability** | Log agent commands (guess what it accessed) | Log every file operation (know what was accessed) |
| **Multi-tenancy** | 1 container per user (expensive) | 1 codebase + N sandboxes (cheap) |

### It's About **Mechanism**, Not **Policy**

AgentFense doesn't tell you "agents shouldn't see secrets" (you already know that). It gives you **a mechanism to enforce it at the kernel level**, so even a compromised/jailbroken agent cannot bypass restrictions.

**Policy** (what you want):
> "This agent can read docs, write logs, but secrets should be hidden."

**Mechanism** (how AgentFense enforces it):
> FUSE filesystem intercepts every `open()`, `readdir()`, `stat()` syscall → checks permission rules → returns ENOENT for `none`, EACCES for violations → secrets literally don't exist in agent's namespace.

You get **defense in depth**: even if the agent is malicious, it cannot escape the filesystem boundary.

---

## Next Steps

- **Try it in 5 minutes**: [Quick Start](../quickstart.md)
- **See real scenarios**: [Use Cases](./use-cases.md)
- **Compare solutions**: [AgentFense vs. Alternatives](./comparison.md)
- **Understand how it works**: [Architecture](./architecture.md)
