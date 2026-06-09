# lane — Interactive `view` Control Panel (C2) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** C2 of the generic-release effort (C split into C1 hardening +
C2 interactive view).

## Context

`lane view` today prints a static styled tree (with a `--watch` tick). C1 added
`restart` and `down`. C2 turns `view` into a full-screen, live, **actionable**
control panel — select a stack and open / tail logs / restart / down it — built
on the C1 lifecycle commands.

## Goal

A live TUI cockpit for the running stacks: see health at a glance and act on a
selected stack, without leaving the terminal — while keeping a machine-readable
path for scripts/agents.

## Decisions

| Item | Decision |
|---|---|
| Mode | `lane view` → interactive when stdout is a TTY; **auto-fallback to plain** when piped/non-TTY |
| Plain | `lane view --plain` → always the static snapshot (replaces the removed `--watch`) |
| Layout | **Master/detail** — left stack list, right detail pane |
| Branding | Hardcoded block "LANE" ASCII logo (lipgloss-colored) + 🏁 accent |
| Refresh | Auto every 2s; selection preserved **by slug** |
| Actions | `o` open · `l` logs · `r` restart · `x` down · `?` help · `q` quit |
| Action exec | `l`/`r`/`x` shell out to the `lane` binary via bubbletea `ExecProcess`; `o` opens browser |
| Confirm | `x` (down) prompts `y/n`; `r` runs without prompt (recoverable) |

## Non-goals

- Live request metrics; multi-pane scrolling beyond the detail pane; editing
  `.lane.toml` from the TUI; the agent JSON output (that's sub-project G).

## Layout (master/detail)

```
██╗      █████╗ ███╗   ██╗███████╗
██║     ██╔══██╗████╗  ██║██╔════╝     🏁  parallel dev stacks
██║     ███████║██╔██╗ ██║█████╗
██║     ██╔══██║██║╚██╗██║██╔══╝       proxy ● up    tls ○ off
███████╗██║  ██║██║ ╚████║███████╗
╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝
──────────────────────────────────┬──────────────────────────
▸ remind        ● running          │ remind
  hsdemo        ● running          │ /home/u/project/ReMind
                                   │ http://remind.localhost
                                   │ tilt → http://tilt-remind.localhost (:34339)
                                   │ routes:
                                   │  ✓ remind.localhost       → remind-ui
                                   │  ✓ tilt-remind.localhost  → tilt
──────────────────────────────────┴──────────────────────────
 ↑/↓ select  o open  l logs  r restart  x down  ? help  q quit
```

- **Header:** block-LANE logo (lipgloss-colored), 🏁 accent + tagline, and live
  `proxy ●/○` (`proxy.Running()`) and `tls ●/○` (`tlsx.Enabled()`).
- **Left list:** each stack `slug` + `● running` / `○ stopped`; selected row
  marked `▸` and highlighted.
- **Right pane:** selected stack's project path, app URL(s), `tilt → URL (:port)`
  if a Tilt stack, and each live Traefik route with ✓ (enabled) / ✗.
- **Footer:** keybinding hints; when `x` is pressed, the footer becomes the
  confirm prompt `down "<slug>"? (y/n)`.
- **Empty state:** logo + `(no stacks — run \`lane up\` in a project)`.

## Components

### `internal/ui` (render — pure, testable)

- `Logo() string` — the hardcoded 6-line block "LANE" banner.
- `RenderPanel(m PanelState) string` — full-screen frame from a plain struct:
  ```go
  type PanelState struct {
      Stacks   []stack.Stack
      Routers  []traefikapi.Router
      Selected int            // index into Stacks
      ProxyUp  bool
      TLSOn    bool
      Confirm  string         // non-empty → show "down <slug>? (y/n)" in footer
      Width    int            // for column sizing
  }
  ```
  Pure (no I/O) → unit-testable. The existing `Render` (static tree) stays for
  `--plain`.

### `internal/ui/panel.go` (bubbletea Model)

`model` holds `PanelState` + the data sources. Implements `tea.Model`:
- **Init:** kick a refresh + a 2s ticker (`tea.Tick`).
- **Update:**
  - `tick` → re-query `dockerx.List()` + `traefikapi.Default().Routers()`,
    rebuild `Stacks/Routers`, **re-resolve `Selected` by remembering the selected
    slug** (clamp if it vanished); schedule the next tick.
  - `KeyMsg`: `up`/`down`/`k`/`j` move selection; `o` open; `l`/`r`/`x` actions;
    `?` toggle a help overlay; `q`/`ctrl-c` quit.
  - In confirm mode (`Confirm != ""`): `y` runs the down, any other key cancels.
- **View:** `ui.RenderPanel(state)`.

### Actions

The selected stack's slug drives every action. `actionArgs(verb, slug)` →
`[]string{verb, "--slug", slug}` (pure, tested). Execution:
- `o` → `exec.Command(opener, url).Start()` (xdg-open / open), non-blocking; no
  TUI suspend.
- `l` → `tea.ExecProcess(exec.Command(self, "logs", "--slug", slug), ...)` —
  suspends the TUI, streams logs until the user exits, then resumes + refreshes.
- `r` → `tea.ExecProcess(self, "restart", "--slug", slug)` then refresh.
- `x` → confirm; on `y`, `tea.ExecProcess(self, "down", "--slug", slug)` then
  refresh.

`self` is `os.Executable()` (falls back to `"lane"`). Shelling out reuses the C1
commands verbatim — no duplicated lifecycle logic in the TUI.

### `cmd/view.go`

```
lane view            → if isatty(stdout): run the bubbletea panel; else print ui.Render (plain)
lane view --plain    → always ui.Render (static snapshot)
```
TTY detection via `golang.org/x/term`'s `term.IsTerminal(int(os.Stdout.Fd()))`
(pulled in transitively by bubbletea; `go get golang.org/x/term` if not already
required). The non-TTY fallback keeps `view` safe in pipes/CI/agents.

## Error handling

- Docker/Traefik query failures during a refresh → keep the last good state and
  show a dim "refresh failed (retrying)" note in the footer, rather than
  crashing the TUI.
- An action's `ExecProcess` returning an error → surface it in the footer note on
  resume; never panic.
- No stacks → empty state (not an error).

## Testing

- `ui.Logo()` — returns the 6 banner lines (sanity).
- `ui.RenderPanel` — table cases: header shows `proxy ●`/`tls ○` per flags;
  selected row marked; right pane lists the selected stack's routes; empty state
  contains "no stacks"; confirm mode shows `down "<slug>"? (y/n)`.
- Selection-by-slug: given a state selected on `b`, after a refresh that reorders
  `[c,a,b]`, `Selected` still points at `b` (helper `reselect(slug, stacks) int`,
  pure + tested).
- `actionArgs("restart","remind")` == `["restart","--slug","remind"]`.
- bubbletea `Update`/`View` wiring is exercised manually (live smoke); the pure
  pieces above carry the unit coverage.

## Backward compatibility / dependencies

- `--plain` preserves today's scriptable output; non-TTY auto-fallback means
  nothing that pipes `lane view` breaks. Removing `--watch` is safe (it was
  introduced in the same unreleased line; the live panel replaces it).
- Reuses existing deps (`bubbletea`, `lipgloss`); adds a tiny TTY-detection
  import. No other new dependencies.
