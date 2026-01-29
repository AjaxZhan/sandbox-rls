# Git Create Commit

## Overview

Create a short, focused commit message and commit staged changes. Then push current branch to origin and sync with remote updates.

## Steps

1. **Review changes**
    - Check the diff: `git diff --cached` (if changes are staged) or `git diff` (if unstaged)
    - Understand what changed and why
2. **Ask for issue key (optional)**
    - Check the branch name for an issue key (Linear, Jira, GitHub issue, etc.)
    - If an issue key (e.g., POW-123, PROJ-456, #123) is not already available in the chat or commit context, optionally ask the user if they want to include one
    - This is optional - commits can be made without an issue key
3. **Stage changes (if not already staged)**
    - `git add -A`
4. **Create short commit message**
    - Base the message on the actual changes in the diff
    - Example: `git commit -m "fix(auth): handle expired token refresh"`
    - Example with issue key: `git commit -m "PROJ-123: fix(auth): handle expired token refresh"`
5. **Fetch and rebase onto latest main (optional but recommended)**
    - `git fetch origin`
    - `git rebase origin/main || git rebase --abort` (if not on main, rebase your feature branch onto latest main)
6. **Push current branch**
    - `git push -u origin HEAD`
7. **If push rejected due to remote updates**
    - Rebase and push: `git pull --rebase && git push`


## Template

- `git commit -m "<type>(<scope>): <short summary>"`
- With issue key: `git commit -m "<issue-key>: <type>(<scope>): <short summary>"`

## Rules

- **Length:** <= 72 characters
- **Imperative mood:** Use "fix", "add", "update" (not "fixed", "added", "updated")
- **Capitalize:** First letter of summary should be capitalized
- **No period:** Don't end the subject line with a period
- **Describe why:** Not just what - "fix stuff" is meaningless

## Notes

- Prefer `rebase` over `merge` for a linear history.
- If you need to force push after a rebase: you need to ask the user if they want to force push: `git push --force-with-lease`.