# Git Create Commit

## Overview

Create a well-structured commit message and commit staged changes. Then push current branch to origin and sync with remote updates.

## Steps

1. **Review changes**
    - Check the diff: `git diff --cached` (if changes are staged) or `git diff` (if unstaged)
    - Understand what changed and why
    - Assess complexity: simple (typo, small fix) vs complex (new feature, refactor, bug fix with context)

2. **Ask for issue key (optional)**
    - Check the branch name for an issue key (Linear, Jira, GitHub issue, etc.)
    - If an issue key (e.g., POW-123, PROJ-456, #123) is not already available in the chat or commit context, optionally ask the user if they want to include one
    - This is optional - commits can be made without an issue key

3. **Stage changes (if not already staged)**
    - `git add -A`

4. **Create commit message**
    - **Simple changes**: Single-line subject is sufficient
    - **Complex changes**: Include subject + body (see Message Structure below)
    - Base the message on the actual changes in the diff

5. **Fetch and rebase onto latest main (optional but recommended)**
    - `git fetch origin`
    - `git rebase origin/main || git rebase --abort` (if not on main, rebase your feature branch onto latest main)

6. **Push current branch**
    - `git push -u origin HEAD`

7. **If push rejected due to remote updates**
    - Rebase and push: `git pull --rebase && git push`

## Message Structure

### Simple changes (single-line)

```bash
git commit -m "<type>(<scope>): <short summary>"
```

Example: `git commit -m "fix(auth): correct typo in error message"`

### Complex changes (subject + body)

```
<type>(<scope>): <short summary>

<body: explain WHY this change is needed>

<optional footer: references, breaking changes>
```

Use HEREDOC for multi-line commits:

```bash
git commit -m "$(cat <<'EOF'
feat(api): add rate limiting to public endpoints

The API was vulnerable to abuse without rate limits. This change adds
token bucket rate limiting to protect server resources and ensure fair
usage across clients.

- Add RateLimiter middleware with configurable limits
- Return 429 status with Retry-After header when exceeded
- Log rate limit events for monitoring

Closes #456
EOF
)"
```

## When to Write a Body

Include a body when:
- The change fixes a bug (explain the root cause)
- The change adds a feature (explain the motivation)
- The change refactors code (explain why the new structure is better)
- The change has non-obvious side effects
- The change reverts a previous commit (reference the original)

Skip the body for:
- Typo fixes
- Simple formatting changes
- Dependency version bumps
- Obvious one-line changes

## Subject Line Rules

- **Length:** <= 50 characters ideal, 72 max
- **Imperative mood:** Use "fix", "add", "update" (not "fixed", "added", "updated")
- **Capitalize:** First letter of summary should be capitalized
- **No period:** Don't end the subject line with a period

## Body Rules

- **Blank line:** Always separate subject from body with a blank line
- **Line wrap:** Wrap at 72 characters
- **Explain why:** Focus on motivation and context, not just what changed
- **Use bullet points:** For multiple related changes

## Commit Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, no code change |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `chore` | Build process, dependencies, tooling |

## Examples

**Simple fix:**
```
fix(ui): correct button alignment on mobile
```

**Bug fix with context:**
```
fix(auth): handle expired token refresh correctly

The previous implementation silently failed when tokens expired during
long-running sessions, causing users to see cryptic 401 errors without
any way to recover.

- Add automatic token refresh before API calls
- Show "Session expired, please login again" message on refresh failure
- Add debug logging for token lifecycle events

Fixes #123
```

**Feature with breaking change:**
```
feat(api)!: change response format to JSON:API spec

BREAKING CHANGE: All API responses now follow JSON:API specification.
Clients must update their response parsing logic.

Migration guide: https://example.com/migration
```

## Notes

- Prefer `rebase` over `merge` for a linear history
- If you need to force push after a rebase: ask the user first, then use `git push --force-with-lease`
- **Never add another Co-author named cursoragent!!!**