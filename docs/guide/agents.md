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

## Teaching your agent to drive lane

lane ships the recipe above as ready-made agent files and installs them from the
binary — no copy-paste. Two commands:

```bash
lane skills   # show what lane can install, and what's already present
lane teach    # install it into this project
```

### What gets installed

| Harness | Installs | Where |
|---|---|---|
| **Claude Code** | a skill (`SKILL.md`) | `.claude/skills/lane/` (project) or `~/.claude/skills/lane/` (global) |
| **Cursor** | a project rule (`lane.mdc`) | `.cursor/rules/` |
| **AGENTS.md** | a lane section (Codex, Copilot, Gemini, …) | `./AGENTS.md` |

Each tells the agent the same thing: use `lane up --wait --json` for an isolated
per-worktree stack, test against the returned URL, `lane down` when finished —
plus the selective bring-up and base-borrowing shortcuts.

### `lane skills`

Read-only. Lists each integration, the path it targets, and its state
(`not installed`, `installed (current)`, `installed (outdated)`). `--json` for
machine output; `--global` shows the global-config targets.

```text
KEY     TITLE              TARGET                             STATE
claude  Claude Code skill  .claude/skills/lane/SKILL.md       not installed
cursor  Cursor rule        .cursor/rules/lane.mdc             installed (current)
agents  AGENTS.md section  AGENTS.md                          not installed
```

### `lane teach`

Installs the integrations. With no arguments it **auto-detects** which harnesses
the project already uses (`.claude/`, `.cursor/`, `AGENTS.md`) and installs for
those; if none are detected it installs all three. Select explicitly with
positional args or flags:

```bash
lane teach                      # auto-detect
lane teach claude cursor        # only these (also: --claude / --cursor / --agents-md)
lane teach --global             # Claude skill into ~/.claude (user config)
lane teach --dry-run            # preview without writing
lane teach --json               # machine-readable results
```

It's **idempotent**: lane-owned files (Claude skill, Cursor rule) are
overwritten with the version-matched content, and the AGENTS.md edit is confined
to a `<!-- lane:start -->`/`<!-- lane:end -->` block so the rest of your
`AGENTS.md` is preserved. Re-running reports `created` / `updated` / `unchanged`
per target.

Cursor's **global** rules are UI-only, so `lane teach --global --cursor` can't
write a file — it prints the rule for you to paste into *Cursor Settings →
Rules → User Rules* instead.

The Claude skill is also installable as a marketplace plugin
(`/plugin marketplace add Dheeraj-Nalapat/lane`).
