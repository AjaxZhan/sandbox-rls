# Security Best Practices

This guide provides production-ready security patterns for deploying AgentFense in real-world environments. Follow these recommendations to maximize security, reliability, and maintainability.

## Permission Design Principles

### 1. Principle of Least Privilege

**Always start with minimal permissions and grant only what's necessary.**

```python
# ✅ Good: Start restrictive, add what's needed
permissions = [
    {"pattern": "**/*", "permission": "none", "priority": 0},      # Default: deny all
    {"pattern": "/src/**", "permission": "read", "priority": 10},  # Explicitly allow source
    {"pattern": "/output/**", "permission": "write", "priority": 10},  # Explicitly allow output
]

# ❌ Bad: Start permissive, try to restrict
permissions = [
    {"pattern": "**/*", "permission": "write"},  # Too permissive
    {"pattern": "/secrets/**", "permission": "none"},  # Easy to miss paths
]
```

**Why:** Over-permissive defaults create security gaps. Attackers exploit what you *forgot* to restrict, not what you *remembered* to allow.

### 2. Defense in Depth

**Layer multiple security controls, don't rely on a single mechanism.**

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./project",
    preset="agent-safe",  # Layer 1: Permission control
    runtime=RuntimeType.DOCKER,  # Layer 2: Container isolation
    resources=ResourceLimits(  # Layer 3: Resource limits
        memory_bytes=512 * 1024 * 1024,
        pids_limit=100,
    ),
    # Layer 4: Network disabled (default in Docker)
) as sandbox:
    result = sandbox.run("untrusted_command")
```

**Layers:**
1. **Filesystem permissions**: Hide secrets, restrict writes
2. **Process isolation**: Prevent escaping to host
3. **Resource limits**: Prevent DoS attacks
4. **Network isolation**: Prevent data exfiltration

### 3. Fail Secure

**When in doubt, deny access. Never fail open.**

```python
# ✅ Good: Default to most restrictive permission
permissions = [
    {"pattern": "**/*", "permission": "none"},  # Default: deny
    # ... explicit allows ...
]

# ❌ Bad: Default to permissive
permissions = [
    {"pattern": "**/*", "permission": "write"},  # Dangerous default
]
```

### 4. Explicit Over Implicit

**Be explicit about security-critical rules, don't rely on pattern matching subtleties.**

```python
# ✅ Good: Explicit rules with high priority
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},  # Explicit
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},  # Explicit
    {"pattern": "**/*.key", "permission": "none", "priority": 100},  # Explicit
]

# ⚠️ Risky: Relies on pattern matching order
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/secrets/**", "permission": "none"},  # No explicit priority
]
```

## Secret Management

### Hide All Secret Patterns

Use comprehensive patterns to catch all secret variations:

```python
SECRET_PATTERNS = [
    # Environment files
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/env.*", "permission": "none", "priority": 100},
    
    # Credentials
    {"pattern": "**/credentials*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    
    # Keys and certificates
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
    {"pattern": "**/*.p12", "permission": "none", "priority": 100},
    {"pattern": "**/*.pfx", "permission": "none", "priority": 100},
    
    # SSH keys
    {"pattern": "**/id_rsa*", "permission": "none", "priority": 100},
    {"pattern": "**/id_ed25519*", "permission": "none", "priority": 100},
    {"pattern": "**/id_ecdsa*", "permission": "none", "priority": 100},
    
    # Cloud provider credentials
    {"pattern": "**/.aws/**", "permission": "none", "priority": 100},
    {"pattern": "**/.gcp/**", "permission": "none", "priority": 100},
    {"pattern": "**/.azure/**", "permission": "none", "priority": 100},
    
    # Token files
    {"pattern": "**/*token*", "permission": "none", "priority": 100},
    {"pattern": "**/*secret*", "permission": "none", "priority": 100},
    
    # Git history (may contain leaked secrets)
    {"pattern": "**/.git/**", "permission": "none", "priority": 50},
]
```

### Use .sandboxignore for Uploads

Create a `.sandboxignore` file to prevent sensitive files from being uploaded:

```
# .sandboxignore (uses .gitignore syntax)

# Secrets
.env*
secrets/
credentials*
*.key
*.pem

# Cloud credentials
.aws/
.gcp/
.azure/

# Git history
.git/

# Build artifacts
node_modules/
__pycache__/
dist/
build/
```

### Environment Variable Injection

Don't expose secrets via environment variables in the sandbox:

```python
# ❌ Bad: Secrets in environment variables (accessible to agent)
result = sandbox.run(
    "python app.py",
    env={"DATABASE_PASSWORD": "secret123"}  # Agent can read this!
)

# ✅ Good: Keep secrets server-side, use secure credential injection
# Option 1: Use a secret management service (Vault, AWS Secrets Manager)
# Option 2: Mount secrets read-only with strict permissions
# Option 3: Use application-level auth (API tokens validated server-side)
```

## Upload Safety

### Validate Uploaded Files

```python
from pathlib import Path
from agentfense import Sandbox

def safe_upload(project_path: str) -> Sandbox:
    """Upload project with safety checks."""
    
    # Check for sensitive files before upload
    sensitive_files = []
    for pattern in [".env*", "*.key", "*.pem", "credentials*"]:
        sensitive_files.extend(Path(project_path).rglob(pattern))
    
    if sensitive_files:
        raise ValueError(f"Found sensitive files: {sensitive_files}. Remove before upload.")
    
    # Upload with strict permissions
    return Sandbox.from_local(
        project_path,
        preset="agent-safe",
        ignore_patterns=[
            ".env*",
            "secrets/",
            "*.key",
            "*.pem",
            ".git/",
        ],
    )
```

### Scan for Hardcoded Secrets

Use tools like `detect-secrets` or `truffleHog` before uploading:

```bash
# Install detect-secrets
pip install detect-secrets

# Scan codebase
detect-secrets scan --all-files ./my-project

# Block upload if secrets found
if detect-secrets scan --all-files ./my-project; then
    echo "✅ No secrets detected"
else
    echo "❌ Secrets detected! Fix before upload."
    exit 1
fi
```

## Runtime Configuration

### Production Server Settings

```yaml
# configs/production.yaml

server:
  grpc_addr: ":9000"
  http_addr: ":8080"

storage:
  codebase_path: "/data/agentfense/codebases"  # Isolated storage
  mount_path: "/data/agentfense/mounts"
  db_path: "/data/agentfense/metadata.db"

runtime:
  type: "docker"  # Use Docker for stronger isolation
  default_timeout: "60s"
  max_timeout: "30m"  # Prevent indefinite hangs
  
  docker:
    host: ""
    default_image: "ubuntu:22.04"
    network_mode: "none"  # Disable networking by default
    enable_networking: false  # Enforce no network

sandbox:
  default_expiration: "6h"  # Auto-cleanup stale sandboxes
  max_per_user: 20  # Prevent resource exhaustion
  cleanup_interval: "5m"

logging:
  level: "info"
  format: "json"  # Structured logs for SIEM integration
```

### Resource Limits

Set aggressive resource limits to prevent abuse:

```python
from agentfense import Sandbox, ResourceLimits

# Conservative limits for untrusted workloads
UNTRUSTED_LIMITS = ResourceLimits(
    memory_bytes=256 * 1024 * 1024,  # 256 MB
    cpu_quota=25000,                 # 25% CPU
    pids_limit=50,                   # Max 50 processes
)

# Standard limits for AI agents
AGENT_LIMITS = ResourceLimits(
    memory_bytes=512 * 1024 * 1024,  # 512 MB
    cpu_quota=50000,                 # 50% CPU
    pids_limit=100,                  # Max 100 processes
)

# Higher limits for build tasks
BUILD_LIMITS = ResourceLimits(
    memory_bytes=2 * 1024 * 1024 * 1024,  # 2 GB
    cpu_quota=100000,                      # 100% CPU
    pids_limit=200,
)
```

### Network Isolation

**Default:** Disable networking unless explicitly required.

```yaml
# Server config: disable networking
runtime:
  docker:
    network_mode: "none"
    enable_networking: false
```

**When networking is required:**

```python
from agentfense import Sandbox, RuntimeType

# Use bridge network with egress filtering
with Sandbox.from_local(
    "./project",
    runtime=RuntimeType.DOCKER,
    # Custom Docker config (requires server-side setup)
    labels={"network_policy": "egress_only"},  # Custom label for network policy
) as sandbox:
    # Network access controlled by Docker network policy
    result = sandbox.run("curl https://api.example.com")
```

**Note:** Network policies require server-side Docker configuration (network plugins, firewall rules).

## Monitoring and Auditing

### Enable Comprehensive Logging

```yaml
# Server config
logging:
  level: "info"
  format: "json"
```

**Log all security events:**
- Sandbox creation (who, when, permissions)
- Command execution (command, exit code, duration)
- Permission denials (path, operation, permission level)
- Resource limit violations (memory, CPU, PIDs)

### Alerting Rules

Monitor and alert on suspicious patterns:

```python
# Example: Alert on high permission denial rate (pseudo-code)
def check_permission_denials(logs):
    """Alert if too many permission denials."""
    denials = [log for log in logs if log.get("event") == "permission_denied"]
    
    if len(denials) > 10:  # >10 denials in monitoring window
        alert(
            severity="medium",
            message=f"High permission denial rate: {len(denials)} denials",
            sandbox_id=denials[0].get("sandbox_id"),
        )
```

**Recommended alerts:**

| Event | Threshold | Severity |
|-------|-----------|----------|
| Permission denials | >10/min per sandbox | Medium |
| Resource limit exceeded | Any occurrence | High |
| Sandbox creation rate | >100/hour per user | Medium |
| Long-running sessions | >1 hour | Low |
| Failed sandbox starts | >3 consecutive | High |

### Audit Trail

Maintain immutable audit logs for compliance:

```json
{
  "timestamp": "2026-02-03T10:15:30Z",
  "event": "sandbox_created",
  "user_id": "user_001",
  "sandbox_id": "sb_abc123",
  "codebase_id": "cb_xyz789",
  "permissions": [...],
  "runtime": "docker",
  "image": "python:3.11-slim",
  "resources": {"memory_bytes": 536870912, "pids_limit": 100}
}

{
  "timestamp": "2026-02-03T10:15:35Z",
  "event": "command_executed",
  "sandbox_id": "sb_abc123",
  "command": "python main.py",
  "exit_code": 0,
  "duration_ms": 1250
}

{
  "timestamp": "2026-02-03T10:15:40Z",
  "event": "permission_denied",
  "sandbox_id": "sb_abc123",
  "path": "/workspace/.env",
  "operation": "open",
  "permission": "none",
  "user_id": "user_001"
}
```

**Retention:** Keep audit logs for at least 90 days (compliance-dependent).

## Testing Permission Rules

### Integration Tests

Test permission rules before production:

```python
import pytest
from agentfense import Sandbox

def test_secrets_hidden():
    """Verify secrets are not accessible."""
    permissions = [
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "**/.env*", "permission": "none", "priority": 100},
    ]
    
    with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
        # Attempt to read .env
        result = sandbox.run("cat /workspace/.env")
        assert result.exit_code != 0, "Secret file should be inaccessible"
        assert "No such file" in result.stderr, "File should be invisible"
        
        # Attempt to list .env
        result = sandbox.run("ls -la /workspace/")
        assert ".env" not in result.stdout, "Secret should not appear in listings"

def test_output_writable():
    """Verify output directory is writable."""
    permissions = [
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "/output/**", "permission": "write"},
    ]
    
    with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
        # Write to /output
        result = sandbox.run("echo test > /workspace/output/result.txt")
        assert result.exit_code == 0, "Should be able to write to /output"
        
        # Verify file exists
        result = sandbox.run("cat /workspace/output/result.txt")
        assert result.stdout.strip() == "test"

def test_code_read_only():
    """Verify source code is read-only."""
    permissions = [
        {"pattern": "**/*.py", "permission": "read"},
    ]
    
    with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
        # Attempt to modify code
        result = sandbox.run("echo malicious >> /workspace/app.py")
        assert result.exit_code != 0, "Should not be able to modify code"
```

### Pre-Deployment Checklist

Before deploying to production:

- [ ] All secrets hidden (test with `cat`, `ls`, `find`, `grep`)
- [ ] Output directories writable
- [ ] Source code read-only (for agent-safe preset)
- [ ] Resource limits enforced (test with memory/CPU-intensive tasks)
- [ ] Network disabled (test with `curl`, `wget`, `ping`)
- [ ] Sandboxes auto-expire (check cleanup interval)
- [ ] Audit logs enabled and forwarded to SIEM
- [ ] Alerts configured for suspicious activity

## Incident Response

### Detecting Compromises

**Signs of potential compromise:**
1. High permission denial rate (agent probing for secrets)
2. Unusual commands (privilege escalation attempts)
3. Resource exhaustion (DoS attacks)
4. Long-running sessions (persistence attempts)

### Response Workflow

1. **Isolate:** Immediately destroy the sandbox
2. **Investigate:** Review audit logs for the sandbox
3. **Contain:** Block the user if malicious
4. **Remediate:** Fix exposed secrets, patch vulnerabilities
5. **Learn:** Update permission rules to prevent recurrence

```python
# Example: Emergency sandbox termination
from agentfense import SandboxClient

client = SandboxClient()

# Destroy compromised sandbox
client.destroy_sandbox("sb_suspicious_123")

# Block user (application-level, not SDK)
# block_user(user_id="user_malicious")
```

## Multi-Tenancy

### Isolate Users

Use separate codebases per user/tenant:

```python
from agentfense import SandboxClient

client = SandboxClient()

# User A's codebase
codebase_a = client.create_codebase(name="project-a", owner_id="user_a")

# User B's codebase (isolated from A)
codebase_b = client.create_codebase(name="project-b", owner_id="user_b")

# Sandboxes are isolated by codebase
sandbox_a = client.create_sandbox(codebase_id=codebase_a.id, permissions=[...])
sandbox_b = client.create_sandbox(codebase_id=codebase_b.id, permissions=[...])
```

### Quota Enforcement

Prevent resource exhaustion per tenant:

```yaml
sandbox:
  max_per_user: 20  # Max concurrent sandboxes per user
```

### Separate Environments

For high-security scenarios, use dedicated server instances per tenant:

```
Tenant A → AgentFense Server A (isolated infrastructure)
Tenant B → AgentFense Server B (isolated infrastructure)
```

## Secure Development Workflow

### 1. Define Security Requirements

Before implementing a feature:
- What data needs protection?
- What operations must the agent perform?
- What's the acceptable risk level?

### 2. Design Permissions

Create permission rules based on requirements:

```python
# Example: Code review agent
CODE_REVIEW_PERMISSIONS = [
    # Read source code
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    
    # View docs structure
    {"pattern": "/docs/**", "permission": "view"},
    
    # Write review comments
    {"pattern": "/review/**", "permission": "write"},
    
    # Hide secrets
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
]
```

### 3. Test Permissions

Write integration tests (see [Testing Permission Rules](#testing-permission-rules)).

### 4. Deploy with Monitoring

Enable logging and alerts before production.

### 5. Iterate Based on Incidents

Update permission rules after each security incident or near-miss.

## Summary

| Practice | Why It Matters | How to Implement |
|----------|----------------|------------------|
| **Least privilege** | Minimize attack surface | Start with `none`, explicitly allow |
| **Defense in depth** | No single point of failure | Permissions + isolation + resource limits |
| **Hide secrets** | Prevent credential theft | Use comprehensive secret patterns |
| **Resource limits** | Prevent DoS attacks | Set memory, CPU, PID limits |
| **Network isolation** | Prevent data exfiltration | Disable networking by default |
| **Audit logging** | Detect and investigate incidents | Log all sandbox operations (JSON format) |
| **Testing** | Catch misconfigurations | Write integration tests for permission rules |
| **Monitoring** | Early detection of attacks | Alert on permission denials, resource violations |

## Next Steps

- **[Permission System](permissions.md)** - Deep dive into permission rules
- **[Presets Guide](presets.md)** - Pre-configured permission templates
- **[Security Model](index.md)** - Overall security architecture
