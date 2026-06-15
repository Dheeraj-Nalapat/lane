# lane — `lane skills` + `lane teach` (self-installing agent skills) — Design Spec

**Date:** 2026-06-15
**Status:** Design complete — ready for implementation planning.
**Follows from:** `docs/2026-06-09-agent-integration-design.md`, which deferred
exactly this as a v1 non-goal: *"`AGENTS.md` and a `lane agent init` generator
(easy later from the same content)."*

## Context

lane already ships agent integrations in the repo:

- `agent/claude/skills/lane/SKILL.md` — a Claude Code plugin skill
- `agent/cursor/lane.mdc` — a Cursor project rule

Today these reach users **only** through the Claude plugin marketplace
(`git-subdir`). A user who just has the `lane` binary has no way to drop these
into a project so their coding agent (Claude Code, Cursor, Codex, …) learns to
drive lane. This feature makes the **binary itself** the distribution mechanism.

## Goal

Two new commands:

- **`lane skills`** — *show* what lane can teach an agent (read-only).
- **`lane teach`** — *install* those integrations into the current project (or,
  where supported, global config), idempotently.

Single source of truth stays under `agent/` in the repo and is embedded into the
binary at build time, so the install is self-contained, offline, and
version-matched.

## Decisions

| Item | Decision |
|---|---|
| Commands | `lane skills` (show) and `lane teach` (install) |
| Content source | `go:embed` the files under `agent/` into a new `internal/agentskills` package |
| Harnesses | Claude Code skill, Cursor rule, AGENTS.md (transitively covers Codex/Copilot/Gemini) |
| Default selection | **Auto-detect** by project layout; explicit flags/args override |
| Project vs global | Default project-local; `--global` for Claude (and a paste-flow for Cursor) |
| Idempotency | Overwrite lane-owned files; merge a marked block into AGENTS.md |
| Honesty | Per-target status: `created` / `updated` / `unchanged` / `skipped` |

## Non-goals (YAGNI)

- MCP server; per-agent auth.
- Downloading the latest content over the network (embed is version-matched).
- Harnesses beyond the three above as first-class targets (AGENTS.md covers the
  rest).
- A global, file-based Cursor rule — **not possible** (see §5).

## 1. Command surface

### `lane skills`

Read-only. Lists each available integration with:

- a short name (`claude`, `cursor`, `agents`),
- a one-line description,
- the path it would install to for the current context (project-local by
  default; `--global` shows global paths),
- its current status in that context (`present` / `absent`, and for AGENTS.md
  whether the lane block is present).

Flags: `--global`, `--json`. `--json` emits a machine-readable array (same shape
the `cmd` JSON commands already use — JSON to stdout, human text to stderr).

### `lane teach`

```
lane teach [harness...] [--claude] [--cursor] [--agents-md] [--global] [--dry-run]
```

- **No harness selected → auto-detect** (see §3). If nothing is detected, install
  all three project-local and say so explicitly.
- Positional args (`claude`, `cursor`, `agents`) and the equivalent boolean
  flags both select harnesses; using any explicit selector disables auto-detect.
- `--global` — write to user config where supported (Claude). See §5 for Cursor
  and AGENTS.md behavior under `--global`.
- `--dry-run` — print the planned actions and target paths; write nothing.
  (Reuses lane's existing `--dry-run` convention.)
- Honors the global `-C` / `--path` flag for resolving the project root.

Exit codes follow lane's convention: `0` on success (including all-`unchanged`
no-ops), `1` on error.

## 2. Content source — embedded

New package `internal/agentskills`:

- Exposes a registry of integrations, each declaring:
  `key`, `title`, `description`, embedded `content`, project-local relative
  path, optional global path resolver, a detection probe, and a writer strategy
  (`own-file` vs `agents-block` vs `cursor`).

**Embed layout.** `go:embed` paths must be at or below the embedding `.go`
file's directory — `..` is not allowed — so the package cannot embed the repo's
top-level `agent/` tree directly. Decision: the embedded content lives **inside
the package** at `internal/agentskills/content/`:

```
internal/agentskills/content/claude/SKILL.md
internal/agentskills/content/cursor/lane.mdc
internal/agentskills/content/agents/AGENTS.snippet.md   # new
```

These package files are the bytes the binary ships. The repo's published
`agent/` tree (consumed by the Claude marketplace `git-subdir`) is a **mirror**
of the same content. A unit test enforces parity: it `os.ReadFile`s the repo
files (via a `../../agent/...` test-relative path — allowed at runtime, unlike
`go:embed`) and asserts they are byte-identical to the embedded copies, so the
two locations can never silently drift. `AGENTS.snippet.md` is new content (the
lane section merged into a project's `AGENTS.md`) and lives in both places.

The `cmd` package depends only on `internal/agentskills`; all content and
filesystem logic lives in the package so it is unit-testable without the CLI.

## 3. Install targets & detection

| Integration | key | Project-local path | Global path | Detected by |
|---|---|---|---|---|
| Claude skill | `claude` | `.claude/skills/lane/SKILL.md` | `~/.claude/skills/lane/SKILL.md` | `.claude/` exists |
| Cursor rule | `cursor` | `.cursor/rules/lane.mdc` | — (UI-only, see §5) | `.cursor/` exists |
| AGENTS.md | `agents` | `./AGENTS.md` (marked block) | — (project-local) | `AGENTS.md` exists |

- Project root resolves via the same path logic other commands use (`-C`/`--path`,
  else CWD). It does **not** require a git repo or a `.lane.toml`.
- Global Claude path resolves to `~/.claude/skills/lane/` (Claude Code's
  user-config location).
- Auto-detect with **zero** matches → install all three project-local. A clean
  repo should still get taught; the output makes clear what was created.
- Parent directories are created as needed (`.claude/skills/lane/`,
  `.cursor/rules/`).

## 4. Idempotency & write strategies

Every target reports exactly one status: `created`, `updated`, `unchanged`, or
`skipped`.

- **`own-file` (Claude skill, Cursor rule, project-local):** lane owns these
  files. Write embedded content. If the file is absent → `created`; if present
  but content differs → `updated`; if byte-identical → `unchanged`. This lets
  re-running refresh stale content after a lane upgrade.
- **`agents-block` (AGENTS.md):** never clobber the user's file. The lane content
  is wrapped in HTML-comment markers:

  ```
  <!-- lane:start -->
  ...lane section...
  <!-- lane:end -->
  ```

  - File absent → create it containing only the marked block (`created`).
  - File present, no lane block → append the block after a blank line
    (`updated`).
  - File present, lane block exists, interior differs → replace only the block's
    interior, preserving everything else (`updated`).
  - File present, block byte-identical → `unchanged`.

- **`cursor` global:** see §5.

`--dry-run` computes the same status without writing, so users can preview a
refresh.

## 5. Cursor & `--global`

Cursor's **project** rules are file-based (`.cursor/rules/lane.mdc`) — installed
normally. Cursor's **global** rules ("User Rules") are **UI-only**: configured in
*Cursor Settings → Rules*, with no supported home-directory file or
`~/.cursor/rules` path ([Cursor Docs — Rules](https://cursor.com/docs/rules)).

Therefore, under `lane teach --global --cursor` (or when global auto-detect would
include Cursor), lane does **not** fabricate a file. Instead it:

1. prints the rule content to stdout, clearly delimited, and
2. prints a one-line instruction: paste it into *Cursor Settings → Rules → User
   Rules*.

This target reports status `skipped` (with the reason "manual: paste into Cursor
Settings → Rules") so the summary stays honest. `lane skills --global` shows the
same note for Cursor instead of a path.

`--global` for **AGENTS.md** has no universal location, so AGENTS.md stays
project-local regardless of `--global`; the summary notes this when `--global` is
combined with AGENTS.md selection.

## 6. Output

Human output (stderr-friendly, stdout for JSON): a short per-target summary,
e.g.

```
claude   .claude/skills/lane/SKILL.md      created
cursor   .cursor/rules/lane.mdc            updated
agents   AGENTS.md                          unchanged
```

`--json` (both commands) emits an array of
`{key, title, target, scope, status, reason?}` objects to stdout; human text to
stderr.

## 7. Testing

`internal/agentskills` unit tests (temp dirs, no CLI):

- AGENTS.md merge across all cases: absent / present-without-block /
  present-with-block-differing / present-with-block-identical, asserting status
  and that surrounding user content is preserved.
- Idempotency: running `teach` twice yields `unchanged` the second time for every
  `own-file` and `agents-block` target.
- Detection: which integrations fire for fake project layouts
  (`.claude/` only, `.cursor/` only, `AGENTS.md` only, none → all three, several
  present).
- `--dry-run`: no files written; statuses still computed.
- Embed sanity: each embedded blob is non-empty; the Claude skill parses
  `name:`/`description:` frontmatter; the Cursor rule parses its frontmatter.
- Mirror parity: embedded `content/` bytes are byte-identical to the published
  `agent/` files (guards against drift between the two locations).

`cmd` smoke tests in the style of `cmd/root_test.go` / `cmd/panel_test.go`:
command wiring, flag parsing, `--json` shape, exit codes.

## 8. Docs

- Update `docs/guide/agents.md`: replace the "Skill files live under `agent/`"
  note with `lane teach` / `lane skills` as the primary install path (marketplace
  remains a secondary option for Claude).
- Add the commands to `docs/guide/commands.md`.
- README "Quick start" / agents section: one line pointing at `lane teach`.
