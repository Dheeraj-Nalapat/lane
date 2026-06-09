# Interactive `view` Control Panel (C2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `lane view` into a full-screen, live, actionable master/detail control panel (select a stack → open / logs / restart / down), with a plain fallback for non-TTY use.

**Architecture:** Pure render + state in `internal/ui` (`Logo`, `PanelState`, `RenderPanel`, `Reselect`) — unit-tested. The bubbletea `Model` lives in `cmd/panel.go`, refreshing from `dockerx`/`traefikapi` every 2s and shelling actions out to the `lane` binary via `tea.ExecProcess`. `cmd/view.go` picks interactive (TTY) vs `--plain` (static `ui.Render`).

**Tech Stack:** Go 1.22, bubbletea + lipgloss (existing), `github.com/mattn/go-isatty` (already an indirect dep). No new downloads.

Spec: `docs/2026-06-09-interactive-view-design.md`.

---

## File Structure

```
internal/ui/view.go     + Logo(), PanelState, RenderPanel(), Reselect()  (pure)
internal/ui/view_test.go + panel render/reselect tests
cmd/panel.go             NEW — bubbletea Model (state, 2s tick, keys, actions, confirm)
cmd/view.go              rewrite: drop --watch; --plain + TTY detection; launch panel
cmd/panel_test.go        NEW — actionArgs test
README.md, CHANGELOG.md  document the interactive view
```

---

### Task 1: `internal/ui` — Logo, PanelState, RenderPanel, Reselect

**Files:** Modify `internal/ui/view.go`, `internal/ui/view_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/view_test.go`:
```go
func TestLogo(t *testing.T) {
	if n := strings.Count(Logo(), "\n"); n < 5 {
		t.Fatalf("logo should be multi-line, got %d newlines", n)
	}
}

func TestReselect(t *testing.T) {
	stacks := []stack.Stack{{Slug: "a"}, {Slug: "b"}, {Slug: "c"}}
	if i := Reselect("b", stacks); i != 1 {
		t.Fatalf("Reselect(b) = %d, want 1", i)
	}
	if i := Reselect("gone", stacks); i != 0 {
		t.Fatalf("Reselect(missing) = %d, want 0 (clamp)", i)
	}
	if i := Reselect("x", nil); i != 0 {
		t.Fatalf("Reselect on empty = %d, want 0", i)
	}
}

func TestRenderPanel(t *testing.T) {
	st := PanelState{
		Stacks: []stack.Stack{
			{Slug: "remind", URL: "http://remind.localhost", ProjectPath: "/p/ReMind", TiltPort: 34339, Running: true},
			{Slug: "hsdemo", URL: "http://hsdemo.localhost", ProjectPath: "/p/hs", Running: true},
		},
		Routers:  []traefikapi.Router{{Name: "remind-ui@docker", Rule: "Host(`remind.localhost`)", Service: "remind-ui", Status: "enabled"}},
		Selected: 0, ProxyUp: true, TLSOn: false, Width: 80,
	}
	out := RenderPanel(st)
	for _, want := range []string{"remind", "hsdemo", "http://remind.localhost", "remind-ui", "proxy", "tls"} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderPanel missing %q:\n%s", want, out)
		}
	}
}

func TestRenderPanel_Empty(t *testing.T) {
	if !strings.Contains(RenderPanel(PanelState{Width: 80}), "no stacks") {
		t.Fatal("empty panel should say 'no stacks'")
	}
}

func TestRenderPanel_Confirm(t *testing.T) {
	st := PanelState{
		Stacks:   []stack.Stack{{Slug: "remind", Running: true}},
		Selected: 0, Width: 80, Confirm: "remind",
	}
	if !strings.Contains(RenderPanel(st), `down "remind"? (y/n)`) {
		t.Fatalf("confirm footer missing:\n%s", RenderPanel(st))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/`
Expected: FAIL — `Logo`/`PanelState`/`RenderPanel`/`Reselect` undefined.

- [ ] **Step 3: Implement (append to `internal/ui/view.go`)**

Add these to `internal/ui/view.go` (keep the existing `Render` and styles):
```go
var (
	logoStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	selStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	badStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

const logo = `██╗      █████╗ ███╗   ██╗███████╗
██║     ██╔══██╗████╗  ██║██╔════╝
██║     ███████║██╔██╗ ██║█████╗
██║     ██╔══██║██║╚██╗██║██╔══╝
███████╗██║  ██║██║ ╚████║███████╗
╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝`

// Logo returns the block "LANE" banner (uncolored).
func Logo() string { return logo }

// PanelState is the full input to RenderPanel (pure; no I/O).
type PanelState struct {
	Stacks   []stack.Stack
	Routers  []traefikapi.Router
	Selected int
	ProxyUp  bool
	TLSOn    bool
	Confirm  string // non-empty → render the down-confirm footer for this slug
	Width    int
	Note     string // transient status line (e.g. "refresh failed")
}

// Reselect returns the index of slug in stacks, or 0 (clamped) if absent.
func Reselect(slug string, stacks []stack.Stack) int {
	for i, s := range stacks {
		if s.Slug == slug {
			return i
		}
	}
	return 0
}

func dot(on bool) string {
	if on {
		return okStyle.Render("●")
	}
	return dimStyle.Render("○")
}

// RenderPanel renders the interactive master/detail control panel.
func RenderPanel(st PanelState) string {
	var b strings.Builder
	b.WriteString(logoStyle.Render(logo) + "\n")
	b.WriteString(fmt.Sprintf("🏁  parallel dev stacks      proxy %s   tls %s\n",
		dot(st.ProxyUp), dot(st.TLSOn)))
	b.WriteString(strings.Repeat("─", 60) + "\n")

	if len(st.Stacks) == 0 {
		b.WriteString(dimStyle.Render("  (no stacks — run `lane up` in a project)\n"))
		return b.String()
	}

	routesBySlug := map[string][]traefikapi.Router{}
	for _, r := range st.Routers {
		name := strings.SplitN(r.Name, "@", 2)[0]
		if i := strings.LastIndexByte(name, '-'); i > 0 {
			routesBySlug[name[:i]] = append(routesBySlug[name[:i]], r)
		}
	}

	// Left: stack list.
	var left strings.Builder
	for i, s := range st.Stacks {
		marker, label := "  ", s.Slug
		if i == st.Selected {
			marker = "▸ "
			label = selStyle.Render(s.Slug)
		}
		st8 := okStyle.Render("● running")
		if !s.Running {
			st8 = badStyle.Render("○ stopped")
		}
		fmt.Fprintf(&left, "%s%-14s %s\n", marker, label, st8)
	}

	// Right: detail of the selected stack.
	sel := st.Stacks[clamp(st.Selected, len(st.Stacks))]
	var right strings.Builder
	fmt.Fprintf(&right, "%s\n", selStyle.Render(sel.Slug))
	fmt.Fprintf(&right, "%s\n", dimStyle.Render(sel.ProjectPath))
	if sel.URL != "" {
		fmt.Fprintf(&right, "%s\n", sel.URL)
	}
	if sel.TiltPort > 0 {
		fmt.Fprintf(&right, "%s\n", dimStyle.Render(fmt.Sprintf("tilt → http://tilt-%s.localhost (:%d)", sel.Slug, sel.TiltPort)))
	}
	for _, r := range routesBySlug[sel.Slug] {
		mark := okStyle.Render("✓")
		if r.Status != "enabled" {
			mark = badStyle.Render("✗")
		}
		fmt.Fprintf(&right, "  %s %s → %s\n", mark, r.Rule, r.Service)
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(34).Render(left.String()),
		dimStyle.Render("│ "),
		right.String(),
	) + "\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")

	if st.Confirm != "" {
		b.WriteString(badStyle.Render(fmt.Sprintf(` down "%s"? (y/n)`, st.Confirm)) + "\n")
	} else {
		b.WriteString(dimStyle.Render(" ↑/↓ select  o open  l logs  r restart  x down  q quit") + "\n")
	}
	if st.Note != "" {
		b.WriteString(dimStyle.Render(" " + st.Note + "\n"))
	}
	return b.String()
}

func clamp(i, n int) int {
	if n == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/`
Expected: PASS (existing `Render` tests still green).

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat(ui): Logo, PanelState, RenderPanel, Reselect for the control panel"
```

---

### Task 2: `cmd/view.go` — plain/TTY, drop `--watch`

**Files:** Rewrite `cmd/view.go`

- [ ] **Step 1: Replace `cmd/view.go`**

```go
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
	"github.com/dheeraj-nalapat/lane/internal/ui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var flagPlain bool

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Live control panel of running stacks (interactive on a TTY)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagPlain || !isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Print(snapshot())
			return nil
		}
		_, err := tea.NewProgram(newPanelModel(), tea.WithAltScreen()).Run()
		return err
	},
}

func init() {
	viewCmd.Flags().BoolVar(&flagPlain, "plain", false, "print a static snapshot instead of the interactive panel")
	root.AddCommand(viewCmd)
}

// snapshot is the static, scriptable rendering (also the non-TTY fallback).
func snapshot() string {
	stacks, _ := dockerx.List()
	routers, _ := traefikapi.Default().Routers()
	return ui.Render(stacks, routers)
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: FAIL — `newPanelModel` undefined (added in Task 3). That's expected; proceed to Task 3.

- [ ] **Step 3: Commit (with Task 3)**

Deferred — commit together with Task 3 once `cmd/panel.go` exists.

---

### Task 3: `cmd/panel.go` — bubbletea Model + actions

**Files:** Create `cmd/panel.go`, `cmd/panel_test.go`

- [ ] **Step 1: Write the failing test**

`cmd/panel_test.go`:
```go
package cmd

import (
	"strings"
	"testing"
)

func TestActionArgs(t *testing.T) {
	got := strings.Join(actionArgs("restart", "remind"), " ")
	if got != "restart --slug remind" {
		t.Fatalf("actionArgs = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestActionArgs`
Expected: FAIL — `actionArgs` undefined.

- [ ] **Step 3: Implement**

`cmd/panel.go`:
```go
package cmd

import (
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
	"github.com/dheeraj-nalapat/lane/internal/ui"
)

type panelTick struct{}
type loaded struct {
	stacks  []stack.Stack
	routers []traefikapi.Router
	err     error
}
type execDone struct{ err error }

type panelModel struct {
	st      ui.PanelState
	selSlug string // remembered selection across refreshes
}

func newPanelModel() panelModel { return panelModel{} }

func loadCmd() tea.Msg {
	stacks, err := dockerx.List()
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Slug < stacks[j].Slug })
	routers, _ := traefikapi.Default().Routers()
	return loaded{stacks: stacks, routers: routers, err: err}
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return panelTick{} })
}

func (m panelModel) Init() tea.Cmd { return tea.Batch(loadCmd, tickCmd()) }

// self returns the path to this binary (for shelling out lane subcommands).
func self() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return "lane"
}

func actionArgs(verb, slug string) []string { return []string{verb, "--slug", slug} }

func (m *panelModel) exec(verb string) tea.Cmd {
	if len(m.st.Stacks) == 0 {
		return nil
	}
	slug := m.st.Stacks[m.st.Selected].Slug
	c := exec.Command(self(), actionArgs(verb, slug)...)
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDone{err} })
}

func openURL(url string) {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	_ = exec.Command(opener, url).Start()
}

func (m panelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loaded:
		m.st.Stacks = msg.stacks
		m.st.Routers = msg.routers
		m.st.ProxyUp = proxyRunning()
		m.st.TLSOn = tlsEnabled()
		if msg.err != nil {
			m.st.Note = "refresh failed (retrying)"
		} else {
			m.st.Note = ""
		}
		if m.selSlug == "" && len(m.st.Stacks) > 0 {
			m.selSlug = m.st.Stacks[0].Slug
		}
		m.st.Selected = ui.Reselect(m.selSlug, m.st.Stacks)
		return m, nil

	case panelTick:
		return m, tea.Batch(loadCmd, tickCmd())

	case execDone:
		return m, loadCmd // refresh after an action

	case tea.WindowSizeMsg:
		m.st.Width = msg.Width
		return m, nil

	case tea.KeyMsg:
		if m.st.Confirm != "" {
			if msg.String() == "y" {
				slug := m.st.Confirm
				m.st.Confirm = ""
				c := exec.Command(self(), actionArgs("down", slug)...)
				return m, tea.ExecProcess(c, func(err error) tea.Msg { return execDone{err} })
			}
			m.st.Confirm = ""
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.st.Selected > 0 {
				m.st.Selected--
			}
			m.rememberSel()
		case "down", "j":
			if m.st.Selected < len(m.st.Stacks)-1 {
				m.st.Selected++
			}
			m.rememberSel()
		case "o":
			if len(m.st.Stacks) > 0 {
				openURL(m.st.Stacks[m.st.Selected].URL)
			}
		case "l":
			return m, m.exec("logs")
		case "r":
			return m, m.exec("restart")
		case "x":
			if len(m.st.Stacks) > 0 {
				m.st.Confirm = m.st.Stacks[m.st.Selected].Slug
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *panelModel) rememberSel() {
	if len(m.st.Stacks) > 0 {
		m.selSlug = m.st.Stacks[m.st.Selected].Slug
	}
}

func (m panelModel) View() string { return ui.RenderPanel(m.st) }
```

> `proxyRunning()` and `tlsEnabled()` are tiny package-local helpers added in
> Step 4 so `cmd` doesn't import `proxy`/`tlsx` in two places inconsistently.

- [ ] **Step 4: Add the header-status helpers**

In `cmd/panel.go`, add imports `"github.com/dheeraj-nalapat/lane/internal/proxy"` and `"github.com/dheeraj-nalapat/lane/internal/tlsx"`, and:
```go
func proxyRunning() bool { return proxy.Running() }
func tlsEnabled() bool   { return tlsx.Enabled() }
```

- [ ] **Step 5: Run test + build**

Run: `go test ./cmd/ -run TestActionArgs && go build ./...`
Expected: PASS, builds (now that `newPanelModel` exists).

- [ ] **Step 6: Commit (Tasks 2 + 3 together)**

```bash
git add cmd/view.go cmd/panel.go cmd/panel_test.go
git commit -m "feat(view): interactive master/detail control panel; --plain fallback; drop --watch"
```

---

### Task 4: Docs

**Files:** Modify `README.md`, `CHANGELOG.md`

- [ ] **Step 1: README — update the `view` row + add a short blurb**

In `README.md` Commands table, replace the `lane view` row with:
```
| `lane view` | Live, interactive control panel (master/detail): select a stack and `o`pen / `l`ogs / `r`estart / `x` down it; auto-refreshing. Non-TTY (piped/CI) prints a static snapshot; `--plain` forces it. |
```

- [ ] **Step 2: CHANGELOG**

Under `## [Unreleased]` → `### Added`, append:
```markdown
- `lane view` is now an interactive control panel (select a stack; open / logs /
  restart / down), auto-refreshing; falls back to a static snapshot when piped
  (`--plain` to force). Replaces the `--watch` flag.
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: interactive lane view"
```

---

### Task 5: Live smoke (needs Docker; manual TTY)

**No code. Verification.**

- [ ] **Step 1: Build + plain path (scriptable, no TTY needed)**

```bash
go build -o ~/.local/bin/lane . && hash -r
lane view --plain        # static snapshot prints
lane view | cat          # piped → static snapshot (TTY auto-fallback), does NOT launch the TUI
```
Expected: both print the static tree; no alt-screen takeover when piped.

- [ ] **Step 2: Interactive panel (in a real terminal, with a stack up)**

```bash
lane proxy up
cd <whoami-project> && lane up
lane view                # full-screen panel: logo, proxy ●, the stack + routes
```
Manually verify: `↑/↓` moves selection; `o` opens the browser; `l` suspends to logs then returns; `r` restarts (stack recreated); `x` shows `down "<slug>"? (y/n)` and `y` tears it down (panel refreshes to empty); `q` quits cleanly (terminal restored).

- [ ] **Step 3: Tear down**

```bash
cd <whoami-project> && lane down 2>/dev/null; lane proxy down
```

---

## Final verification

- [ ] `go test ./...` all pass (ui panel/reselect, actionArgs, plus existing); `go vet ./...` clean; `gofmt -l .` empty.
- [ ] `lane view --plain` and `lane view | cat` print the static snapshot (no TUI).
- [ ] Interactive panel: select + open/logs/restart/down work; `x` confirms; `q` restores the terminal.
- [ ] `--watch` is gone; `--plain` documented.
