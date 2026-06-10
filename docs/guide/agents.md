---
description: Use lane with AI coding agents like Claude Code and Cursor — give each git worktree an isolated, port-conflict-free stack so multiple agents test in parallel with race-safe, machine-readable lane up --wait --json.
---

# Using lane with coding agents (parallel testing)

lane gives each git worktree an isolated, port-conflict-free stack, so multiple
agents (Claude, Cursor, …) can test in parallel. The loop:

```bash
lane up --wait --json   # isolated stack for this worktree; waits until serving; prints {slug, urls[]}
# ...run tests against the returned url...
lane down
```

`--json` prints machine-readable output on stdout (human logs on stderr); exit
`0` = success (incl. already-running), `1` = error. `lane ls --json` lists
stacks. Parallel `lane up`s are race-safe (the shared proxy bring-up is locked).

**Skill files:** a Claude Code skill (installable as a plugin —
`/plugin marketplace add Dheeraj-Nalapat/lane`) and a Cursor rule live under
`agent/` in the repo.
