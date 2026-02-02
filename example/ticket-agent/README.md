# Ticket Agent - Sandbox RLS Demo

An AI Agent that manages tickets via bash commands, demonstrating **file-level permission control**.

## Quick Start

```bash
cd example/ticket-agent
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

# Configure .env
echo "ANTHROPIC_API_KEY=your-key" > .env
echo "ANTHROPIC_BASE_URL=https://your-api-endpoint" >> .env  # optional

# Start sandbox server (in another terminal)
cd ../.. && ./bin/sandbox-server

# Run agent
python agent.py
```

## Permission Demo

The agent has 4 files with different permission levels:

| File | Permission | Behavior |
|------|------------|----------|
| `TKT-001.txt` | **write** | Can read and modify |
| `TKT-002.txt` | **read** | Can read, cannot modify |
| `TKT-003.txt` | **view** | Can see in `ls`, cannot read content |
| `secret.txt` | **none** | Completely invisible |

## Demo Script

Run these prompts in order to demonstrate each permission level:

### 1. List files (none permission hides secret.txt)

```
You: List all files
```

**Expected**: Shows TKT-001, TKT-002, TKT-003. No secret.txt!

### 2. Read TKT-001 (write permission)

```
You: Show me TKT-001
```

**Expected**: ✓ Success, content displayed

### 3. Modify TKT-001 (write permission)

```
You: Change TKT-001 status to closed
```

**Expected**: ✓ Success, file modified

### 4. Read TKT-002 (read permission)

```
You: Show me TKT-002
```

**Expected**: ✓ Success, content displayed

### 5. Try to modify TKT-002 (read permission blocks write)

```
You: Change TKT-002 status to closed
```

**Expected**: ✗ Permission denied

### 6. Try to read TKT-003 (view permission blocks read)

```
You: Show me TKT-003
```

**Expected**: ✗ Permission denied (file exists but cannot read)

### 7. Try to access secret.txt (none permission)

```
You: Is there a secret.txt file? Try to read it.
```

**Expected**: File not found (completely invisible)

## Key Code

```python
PERMISSIONS = [
    {"pattern": "TKT-001*", "permission": "write"},  # Full access
    {"pattern": "TKT-002*", "permission": "read"},   # Read only
    {"pattern": "TKT-003*", "permission": "view"},   # See name only
    {"pattern": "secret*", "permission": "none"},    # Invisible
]

with Sandbox.from_local(
    path=data_dir,
    permissions=PERMISSIONS,
    ...
) as sandbox:
    # Agent commands run inside isolated sandbox
    sandbox.run("cat /workspace/TKT-001.txt")  # ✓ allowed
    sandbox.run("cat /workspace/secret.txt")   # ✗ file not found
```

## Architecture

```
User → "Show TKT-001"
       ↓
Claude → generates: cat /workspace/TKT-001.txt
       ↓
Sandbox → checks permission → allowed → returns content
       ↓
Claude → "Here's TKT-001: ..."
```

The Sandbox enforces permissions at the **filesystem level** - the Agent cannot bypass them even if it tries.
