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

**Test only what you changed.** Bring up a subset and reach each service by name:

```bash
lane up api --wait --json            # only api (+ deps); api at http://<slug>-api.localhost
```

Each service is auto-routed at `<slug>-<service>.localhost`, and `--json` reports
per-service `running` status (`--wait` waits only on what you started). See
[Selecting & reaching services](selecting-services.md).

**Borrow the rest from a base stack** to save resources — run the changed
services fresh, reuse everything else from a running base of the same project:

```bash
lane up api --base --wait --json     # api fresh; db, auth, web borrowed from the base
```

See [Borrowing from a base stack](base-stacks.md).

**Install the skills.** Run `lane teach` in your project — lane auto-detects
which harnesses you use (`.claude/`, `.cursor/`, `AGENTS.md`) and installs its
skill/rule for each (run `lane skills` first to see what's available). Use
`lane teach --global` for the Claude skill in user config. The Claude skill is
also installable as a marketplace plugin
(`/plugin marketplace add Dheeraj-Nalapat/lane`).
