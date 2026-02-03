# AgentFense

> Least-privilege filesystem sandbox & context guardrails for AI agents

Run untrusted AI agent code **against a real codebase** while enforcing **least-privilege access** at the file level.

## The Challenge

The best agent interface remains simple: **bash + filesystem**. With FUSE, you can mount any world and make an agent productive with plain `ls`, `cat`, `grep`, and `find`.

But there's a gap: filesystems are usually **all-or-nothing**. Mount a real repo, and you often expose *everything*—including secrets.

## The Solution

AgentFense fills that gap with four permission levels:

| Level | What the agent can do |
|-------|------------------------|
| `none`  | Path is **invisible** (hidden from `ls`, behaves like it doesn't exist) |
| `view`  | Can **list** names (`ls`), but cannot read file content |
| `read`  | Can read file content |
| `write` | Can read + modify / create files |

**Example policy**: *"You can edit `/docs`, see `/metadata`, read everything else, but `/secrets` does not exist."*

## Quick Example

```python
from agentfense import Sandbox

# One-liner: create sandbox from local directory with "agent-safe" preset
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

The `agent-safe` preset: read all files, write to `/output` and `/tmp`, hide secrets (`.env`, `*.key`, etc.).

## Next Steps

- [Quick Start](get-started/quickstart.md) - Get up and running in 5 minutes
- [Core Concepts](get-started/concepts.md) - Understand the fundamentals
- [Why AgentFense](why/index.md) - Learn about use cases and benefits
- [Python SDK](sdk/python/overview.md) - Dive into the SDK documentation
