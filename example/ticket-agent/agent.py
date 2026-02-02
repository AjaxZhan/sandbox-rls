#!/usr/bin/env python3
"""
Ticket Agent - Demonstrates Sandbox RLS Permission System

Permission levels:
- write: Can read and modify the file
- read:  Can read but not modify
- view:  Can see filename in ls, but cannot read content
- none:  File is completely invisible
"""

import os
import re
from dotenv import load_dotenv
from anthropic import Anthropic
from agentfense import Sandbox

load_dotenv(override=True)

# =============================================================================
# Permission Configuration - THE KEY DEMO POINT
# =============================================================================

PERMISSIONS = [
    # TKT-001: Full access (write)
    {"pattern": "TKT-001*", "permission": "write"},
    
    # TKT-002: Read only
    {"pattern": "TKT-002*", "permission": "read"},
    
    # TKT-003: View only (can see it exists, cannot read)
    {"pattern": "TKT-003*", "permission": "view"},
    
    # secret.txt: Hidden (none) - completely invisible
    {"pattern": "secret*", "permission": "none"},
]

# =============================================================================
# System Prompt
# =============================================================================

SYSTEM_PROMPT = """You are a ticket management assistant. Tickets are stored as .txt files in /workspace/.

File format:
```
title: ...
status: open|in_progress|closed
priority: low|medium|high
assignee: name or none
created: YYYY-MM-DD

<description>
```

Use bash commands: ls, cat, grep, sed, echo, rm, etc.

Output commands in ```bash``` code blocks. After seeing results, summarize for the user.

Important: Some files may have restricted permissions. If a command fails, explain what happened."""

# =============================================================================
# Agent Logic
# =============================================================================

client = Anthropic(
    api_key=os.getenv("ANTHROPIC_API_KEY"),
    base_url=os.getenv("ANTHROPIC_BASE_URL"),
)

def extract_commands(text: str) -> list[str]:
    """Extract bash commands from ```bash``` blocks"""
    return [m.strip() for m in re.findall(r'```bash\n(.*?)```', text, re.DOTALL)]


def run_agent(sandbox: Sandbox):
    """Interactive conversation loop"""
    messages = []
    
    print("=" * 60)
    print("Ticket Agent - Permission Demo")
    print("=" * 60)
    print("\nPermissions configured:")
    print("  TKT-001.txt  → write (full access)")
    print("  TKT-002.txt  → read  (read only)")
    print("  TKT-003.txt  → view  (can see, cannot read)")
    print("  secret.txt   → none  (invisible)")
    print("\nType 'quit' to exit.\n")

    while True:
        user_input = input("You: ").strip()
        if user_input.lower() in ["quit", "exit", "q"]:
            break
        if not user_input:
            continue

        messages.append({"role": "user", "content": user_input})

        while True:
            response = client.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=1024,
                system=SYSTEM_PROMPT,
                messages=messages
            )
            
            text = response.content[0].text
            messages.append({"role": "assistant", "content": text})
            
            commands = extract_commands(text)
            if not commands:
                print(f"\nAgent: {text}\n")
                break

            # Execute commands
            outputs = []
            for cmd in commands:
                print(f"\n$ {cmd}")
                result = sandbox.run(cmd, timeout=10)
                out = result.stdout.strip() if result.stdout else ""
                err = result.stderr.strip() if result.stderr else ""
                
                if result.exit_code == 0:
                    print(out if out else "(success, no output)")
                else:
                    print(f"ERROR: {err or out or 'Command failed'}")
                
                outputs.append(f"$ {cmd}\nexit_code: {result.exit_code}\nstdout: {out}\nstderr: {err}")

            messages.append({"role": "user", "content": "Output:\n" + "\n---\n".join(outputs)})


def main():
    data_dir = os.path.join(os.path.dirname(__file__), "data")
    
    print("Initializing Sandbox...")
    with Sandbox.from_local(
        path=data_dir,
        permissions=PERMISSIONS,
        preset="view-only",
        endpoint="localhost:9000",
    ) as sandbox:
        print(f"Sandbox ID: {sandbox.id}\n")
        run_agent(sandbox)


if __name__ == "__main__":
    main()
