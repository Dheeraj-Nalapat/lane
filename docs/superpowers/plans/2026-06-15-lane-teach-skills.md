# `lane skills` + `lane teach` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two commands — `lane skills` (show available agent integrations) and `lane teach` (install them into a project or global config) — with content embedded in the binary.

**Architecture:** A new `internal/agentskills` package owns the embedded content (`go:embed`), the integration registry, auto-detection, and the write/merge logic (pure, unit-tested). The `cmd` package adds thin `teach`/`skills` cobra commands that resolve the project dir (reusing `projectDir()`), select integrations, call the package, and print a table or JSON. The repo's published `agent/` tree is mirrored inside the package and kept in lockstep by a parity test.

**Tech Stack:** Go 1.24, cobra, `embed`, stdlib (`os`, `strings`, `encoding/json`, `text/tabwriter`). Tests: standard `testing` with temp dirs. CI runs `gofmt -l .` and `go test ./...`.

**Spec:** `docs/superpowers/specs/2026-06-15-lane-teach-skills-design.md`

---

## File structure

**Create:**
- `internal/agentskills/content/claude/SKILL.md` — embedded copy (mirror of `agent/claude/skills/lane/SKILL.md`)
- `internal/agentskills/content/cursor/lane.mdc` — embedded copy (mirror of `agent/cursor/lane.mdc`)
- `internal/agentskills/content/agents/AGENTS.snippet.md` — new lane section body for AGENTS.md
- `agent/agents/AGENTS.snippet.md` — published mirror of the above
- `internal/agentskills/embed.go` — `go:embed` directives → string vars
- `internal/agentskills/integrations.go` — `Integration` type, `All()`, `Get()`, `Detect()`
- `internal/agentskills/install.go` — `Result`, statuses/scopes, `Apply()`, `mergeAgents()`, write helpers
- `internal/agentskills/integrations_test.go`
- `internal/agentskills/install_test.go`
- `internal/agentskills/embed_test.go`
- `cmd/teach.go` — `lane teach`
- `cmd/teach_test.go`
- `cmd/skills.go` — `lane skills`
- `cmd/skills_test.go`

**Modify:**
- `docs/guide/agents.md`, `docs/guide/commands.md`, `README.md` — document the commands

---

## Task 1: Embedded content + parity test

**Files:**
- Create: `internal/agentskills/content/claude/SKILL.md`
- Create: `internal/agentskills/content/cursor/lane.mdc`
- Create: `internal/agentskills/content/agents/AGENTS.snippet.md`
- Create: `agent/agents/AGENTS.snippet.md`
- Create: `internal/agentskills/embed.go`
- Test: `internal/agentskills/embed_test.go`

- [ ] **Step 1: Copy the existing skill + rule into the package mirror**

```bash
mkdir -p internal/agentskills/content/claude internal/agentskills/content/cursor internal/agentskills/content/agents agent/agents
cp agent/claude/skills/lane/SKILL.md internal/agentskills/content/claude/SKILL.md
cp agent/cursor/lane.mdc            internal/agentskills/content/cursor/lane.mdc
```

(Do not edit the copies — the parity test in Step 5 enforces they stay byte-identical to `agent/`.)

- [ ] **Step 2: Author the AGENTS.md section body** (identical bytes in both locations)

Write this exact content to BOTH `internal/agentskills/content/agents/AGENTS.snippet.md` and `agent/agents/AGENTS.snippet.md`:

```markdown
## Running & testing with lane

This project uses [lane](https://github.com/Dheeraj-Nalapat/lane) to run its
Docker/Tilt stack behind a shared proxy, so each git worktree gets an isolated,
port-conflict-free stack. Multiple agents (one per worktree) can bring up stacks
and test in parallel without colliding on host ports.

The loop:

- `lane up --wait --json` — bring up an isolated stack for this worktree, wait
  until it serves, and print `{"slug":...,"urls":[{"url":"http://<slug>.localhost"}]}`
  on stdout (human logs go to stderr; exit code `0` = ready).
- Run tests / requests against the returned `url`(s).
- `lane down` — tear the stack down (the repo is left byte-for-byte unchanged).

Test only what changed (faster, lighter):

- `lane up <service...> --wait --json` — bring up only the services you changed
  (their dependencies come up automatically). Each is auto-routed at
  `http://<slug>-<service>.localhost`; the JSON `urls[]` carry a per-service
  `running` flag.
- `lane up <service> --base --wait --json` — run the changed service fresh and
  borrow the rest from a running base stack of the same project (saves resources).

Notes:

- `lane ls --json` lists running stacks. Exit `0` = success (including already
  running), `1` = error.
- `-C <dir>` / `--path <dir>` acts on a project without `cd`-ing into it.
- The slug derives from the git worktree, so two worktrees of one repo get
  distinct URLs automatically — no port coordination needed.
```

- [ ] **Step 3: Write the embed directives**

Create `internal/agentskills/embed.go`:

```go
package agentskills

import _ "embed"

//go:embed content/claude/SKILL.md
var claudeSkill string

//go:embed content/cursor/lane.mdc
var cursorRule string

//go:embed content/agents/AGENTS.snippet.md
var agentsSnippet string
```

- [ ] **Step 4: Write the embed + parity test (failing)**

Create `internal/agentskills/embed_test.go`:

```go
package agentskills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedContentNonEmpty(t *testing.T) {
	for name, s := range map[string]string{
		"claudeSkill":   claudeSkill,
		"cursorRule":    cursorRule,
		"agentsSnippet": agentsSnippet,
	} {
		if strings.TrimSpace(s) == "" {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestClaudeSkillHasFrontmatter(t *testing.T) {
	if !strings.Contains(claudeSkill, "name:") || !strings.Contains(claudeSkill, "description:") {
		t.Error("claude skill missing name:/description: frontmatter")
	}
}

func TestMirrorParity(t *testing.T) {
	// Embedded copies must match the published agent/ tree exactly.
	cases := map[string]string{
		filepath.Join("..", "..", "agent", "claude", "skills", "lane", "SKILL.md"): claudeSkill,
		filepath.Join("..", "..", "agent", "cursor", "lane.mdc"):                   cursorRule,
		filepath.Join("..", "..", "agent", "agents", "AGENTS.snippet.md"):          agentsSnippet,
	}
	for path, embedded := range cases {
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(want) != embedded {
			t.Errorf("embedded content drifted from %s", path)
		}
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/agentskills/ -run 'Embedded|Frontmatter|Parity' -v`
Expected: PASS (3 tests). If parity fails, re-copy the file in Step 1 — do not edit the mirror by hand.

- [ ] **Step 6: Commit**

```bash
git add internal/agentskills/ agent/agents/AGENTS.snippet.md
git commit -m "feat(agentskills): embed skill/rule/AGENTS content with parity test"
```

---

## Task 2: Integration registry + detection

**Files:**
- Create: `internal/agentskills/integrations.go`
- Test: `internal/agentskills/integrations_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/agentskills/integrations_test.go`:

```go
package agentskills

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAllKeys(t *testing.T) {
	var keys []string
	for _, it := range All() {
		keys = append(keys, it.Key)
	}
	if !reflect.DeepEqual(keys, []string{"claude", "cursor", "agents"}) {
		t.Fatalf("All() keys = %v", keys)
	}
}

func TestGet(t *testing.T) {
	if _, ok := Get("cursor"); !ok {
		t.Error("Get(cursor) not found")
	}
	if _, ok := Get("nope"); ok {
		t.Error("Get(nope) should not be found")
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	if got := Detect(dir); len(got) != 0 {
		t.Fatalf("empty dir Detect = %v, want none", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Detect(dir)
	if !reflect.DeepEqual(got, []string{"cursor", "agents"}) {
		t.Fatalf("Detect = %v, want [cursor agents]", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/agentskills/ -run 'AllKeys|Get|Detect' -v`
Expected: FAIL — `All`, `Get`, `Detect` undefined.

- [ ] **Step 3: Implement the registry**

Create `internal/agentskills/integrations.go`:

```go
package agentskills

import (
	"os"
	"path/filepath"
)

// Write strategies for an integration's content.
const (
	StrategyOwnFile     = "own-file"     // lane owns the file; write/overwrite it
	StrategyAgentsBlock = "agents-block" // merge a marked block into the file
)

// Integration is one agent-harness target lane can install for.
type Integration struct {
	Key                string // "claude", "cursor", "agents"
	Title              string // human label
	Description        string // one-line summary
	Content            string // embedded content (body for agents-block)
	Strategy           string // StrategyOwnFile | StrategyAgentsBlock
	ProjectRel         string // path relative to the project root
	DetectRel          string // path under the project whose existence triggers auto-detect
	SupportsGlobalFile bool   // can be written to a global file (Claude only)
}

// All returns the registry in display order.
func All() []Integration {
	return []Integration{
		{
			Key:                "claude",
			Title:              "Claude Code skill",
			Description:        "Teaches Claude Code to drive lane (skill file).",
			Content:            claudeSkill,
			Strategy:           StrategyOwnFile,
			ProjectRel:         filepath.Join(".claude", "skills", "lane", "SKILL.md"),
			DetectRel:          ".claude",
			SupportsGlobalFile: true,
		},
		{
			Key:                "cursor",
			Title:              "Cursor rule",
			Description:        "Teaches Cursor to drive lane (project rule).",
			Content:            cursorRule,
			Strategy:           StrategyOwnFile,
			ProjectRel:         filepath.Join(".cursor", "rules", "lane.mdc"),
			DetectRel:          ".cursor",
			SupportsGlobalFile: false,
		},
		{
			Key:                "agents",
			Title:              "AGENTS.md section",
			Description:        "Adds a lane section to AGENTS.md (Codex, Copilot, Gemini, …).",
			Content:            agentsSnippet,
			Strategy:           StrategyAgentsBlock,
			ProjectRel:         "AGENTS.md",
			DetectRel:          "AGENTS.md",
			SupportsGlobalFile: false,
		},
	}
}

// Get returns the integration with the given key.
func Get(key string) (Integration, bool) {
	for _, it := range All() {
		if it.Key == key {
			return it, true
		}
	}
	return Integration{}, false
}

// Detect returns, in registry order, the keys of integrations whose detect path
// exists under dir.
func Detect(dir string) []string {
	var keys []string
	for _, it := range All() {
		if _, err := os.Stat(filepath.Join(dir, it.DetectRel)); err == nil {
			keys = append(keys, it.Key)
		}
	}
	return keys
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/agentskills/ -run 'AllKeys|Get|Detect' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/agentskills/integrations.go internal/agentskills/integrations_test.go
git commit -m "feat(agentskills): integration registry + auto-detection"
```

---

## Task 3: AGENTS.md block merge (pure logic)

**Files:**
- Create: `internal/agentskills/install.go` (merge + markers only in this task)
- Test: `internal/agentskills/install_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/agentskills/install_test.go`:

```go
package agentskills

import (
	"strings"
	"testing"
)

func TestMergeAgents_Absent(t *testing.T) {
	got, changed := mergeAgents("", "BODY")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if !strings.Contains(got, agentsStart) || !strings.Contains(got, "BODY") || !strings.Contains(got, agentsEnd) {
		t.Fatalf("got = %q", got)
	}
}

func TestMergeAgents_AppendPreservesUserContent(t *testing.T) {
	existing := "# My project\n\nmy own notes\n"
	got, changed := mergeAgents(existing, "BODY")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if !strings.HasPrefix(got, "# My project") {
		t.Fatalf("user content not preserved: %q", got)
	}
	if !strings.Contains(got, agentsStart) {
		t.Fatal("lane block not appended")
	}
}

func TestMergeAgents_ReplacesBlockOnly(t *testing.T) {
	existing := "intro\n\n" + agentsStart + "\nOLD\n" + agentsEnd + "\n\noutro\n"
	got, changed := mergeAgents(existing, "NEW")
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if strings.Contains(got, "OLD") {
		t.Fatal("old block content not replaced")
	}
	if !strings.Contains(got, "NEW") || !strings.HasPrefix(got, "intro") || !strings.Contains(got, "outro") {
		t.Fatalf("surrounding content not preserved: %q", got)
	}
}

func TestMergeAgents_Idempotent(t *testing.T) {
	once, _ := mergeAgents("intro\n", "BODY")
	twice, changed := mergeAgents(once, "BODY")
	if changed {
		t.Fatal("second merge changed = true, want false")
	}
	if once != twice {
		t.Fatal("merge not idempotent")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/agentskills/ -run MergeAgents -v`
Expected: FAIL — `mergeAgents`, `agentsStart`, `agentsEnd` undefined.

- [ ] **Step 3: Implement the merge in `install.go`**

Create `internal/agentskills/install.go` with exactly this (more is added in Task 4):

```go
package agentskills

import "strings"

const (
	agentsStart = "<!-- lane:start -->"
	agentsEnd   = "<!-- lane:end -->"
)

// renderAgentsBlock wraps body in the lane markers.
func renderAgentsBlock(body string) string {
	return agentsStart + "\n" + strings.TrimSpace(body) + "\n" + agentsEnd
}

// mergeAgents returns AGENTS.md content with the lane block created, appended,
// or replaced in place, plus whether the content changed.
func mergeAgents(existing, body string) (string, bool) {
	block := renderAgentsBlock(body)
	si := strings.Index(existing, agentsStart)
	ei := strings.Index(existing, agentsEnd)
	if si >= 0 && ei > si {
		merged := existing[:si] + block + existing[ei+len(agentsEnd):]
		return merged, merged != existing
	}
	if strings.TrimSpace(existing) == "" {
		return block + "\n", true
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n", true
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/agentskills/ -run MergeAgents -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/agentskills/install.go internal/agentskills/install_test.go
git commit -m "feat(agentskills): idempotent AGENTS.md block merge"
```

---

## Task 4: Apply (write to disk) + statuses

**Files:**
- Modify: `internal/agentskills/install.go` (add types + `Apply` + write helpers)
- Test: `internal/agentskills/install_test.go` (add Apply tests)

- [ ] **Step 1: Add the failing tests**

Append to `internal/agentskills/install_test.go`:

```go
func TestApply_OwnFile_CreateThenUnchanged(t *testing.T) {
	dir := t.TempDir()
	it, _ := Get("cursor")

	r1, err := Apply(it, dir, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Status != StatusCreated {
		t.Fatalf("first apply status = %q, want created", r1.Status)
	}
	r2, err := Apply(it, dir, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Status != StatusUnchanged {
		t.Fatalf("second apply status = %q, want unchanged", r2.Status)
	}
}

func TestApply_OwnFile_Updated(t *testing.T) {
	dir := t.TempDir()
	it, _ := Get("cursor")
	if _, err := Apply(it, dir, false, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, it.ProjectRel), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Apply(it, dir, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpdated {
		t.Fatalf("status = %q, want updated", r.Status)
	}
}

func TestApply_Agents_LifeCycle(t *testing.T) {
	dir := t.TempDir()
	it, _ := Get("agents")

	r1, _ := Apply(it, dir, false, false)
	if r1.Status != StatusCreated {
		t.Fatalf("status = %q, want created", r1.Status)
	}
	r2, _ := Apply(it, dir, false, false)
	if r2.Status != StatusUnchanged {
		t.Fatalf("status = %q, want unchanged", r2.Status)
	}
}

func TestApply_CursorGlobalIsManual(t *testing.T) {
	dir := t.TempDir()
	it, _ := Get("cursor")
	r, err := Apply(it, dir, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusSkipped || r.Content == "" {
		t.Fatalf("cursor global: status=%q content-empty=%v, want skipped + content", r.Status, r.Content == "")
	}
}

func TestApply_DryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	it, _ := Get("cursor")
	r, err := Apply(it, dir, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusCreated {
		t.Fatalf("dry-run status = %q, want created", r.Status)
	}
	if _, err := os.Stat(filepath.Join(dir, it.ProjectRel)); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote a file")
	}
}
```

Add these imports to the top of `install_test.go` (the file currently imports only `strings` and `testing`):

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/agentskills/ -run Apply -v`
Expected: FAIL — `Apply`, `Status*`, `Result` undefined.

- [ ] **Step 3: Implement `Apply` + helpers**

Append to `internal/agentskills/install.go` and add the imports. Replace the import line `import "strings"` with:

```go
import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)
```

Then append:

```go
// Status is the outcome of applying (or planning) one integration.
type Status string

const (
	StatusCreated   Status = "created"
	StatusUpdated   Status = "updated"
	StatusUnchanged Status = "unchanged"
	StatusSkipped   Status = "skipped"
)

// Scope is where an integration is installed.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Result reports what happened (or, with dryRun, would happen) for one integration.
type Result struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Target  string `json:"target"`
	Scope   Scope  `json:"scope"`
	Status  Status `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Content string `json:"-"` // populated for manual (paste) targets; not serialized
}

// Apply installs one integration under projectDir (or global config where
// supported). With dryRun, it computes the status without touching disk.
func Apply(it Integration, projectDir string, global, dryRun bool) (Result, error) {
	// Cursor's global rules are UI-only — there is no file to write.
	if global && it.Key == "cursor" {
		return Result{
			Key:     it.Key,
			Title:   it.Title,
			Scope:   ScopeGlobal,
			Target:  "Cursor Settings → Rules",
			Status:  StatusSkipped,
			Reason:  "manual: paste into Cursor Settings → Rules",
			Content: it.Content,
		}, nil
	}

	scope := ScopeProject
	path := filepath.Join(projectDir, it.ProjectRel)
	if global && it.SupportsGlobalFile {
		home, err := os.UserHomeDir()
		if err != nil {
			return Result{}, err
		}
		path = filepath.Join(home, it.ProjectRel)
		scope = ScopeGlobal
	}

	res := Result{Key: it.Key, Title: it.Title, Target: path, Scope: scope}

	var status Status
	var err error
	if it.Strategy == StrategyAgentsBlock {
		status, err = applyAgents(path, it.Content, dryRun)
	} else {
		status, err = applyOwnFile(path, it.Content, dryRun)
	}
	if err != nil {
		return Result{}, err
	}
	res.Status = status
	return res, nil
}

func applyOwnFile(path, content string, dryRun bool) (Status, error) {
	old, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if !dryRun {
			if err := writeFile(path, content); err != nil {
				return "", err
			}
		}
		return StatusCreated, nil
	case err != nil:
		return "", err
	}
	if string(old) == content {
		return StatusUnchanged, nil
	}
	if !dryRun {
		if err := writeFile(path, content); err != nil {
			return "", err
		}
	}
	return StatusUpdated, nil
}

func applyAgents(path, body string, dryRun bool) (Status, error) {
	old, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	merged, changed := mergeAgents(string(old), body)
	if !existed {
		if !dryRun {
			if err := writeFile(path, merged); err != nil {
				return "", err
			}
		}
		return StatusCreated, nil
	}
	if !changed {
		return StatusUnchanged, nil
	}
	if !dryRun {
		if err := writeFile(path, merged); err != nil {
			return "", err
		}
	}
	return StatusUpdated, nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
```

- [ ] **Step 4: Run the full package suite**

Run: `go test ./internal/agentskills/ -v`
Expected: PASS (all tests from Tasks 1–4).

- [ ] **Step 5: Commit**

```bash
git add internal/agentskills/install.go internal/agentskills/install_test.go
git commit -m "feat(agentskills): Apply with create/update/unchanged/skipped statuses"
```

---

## Task 5: `lane teach` command

**Files:**
- Create: `cmd/teach.go`
- Test: `cmd/teach_test.go`

Note: `cmd/up.go` already defines `projectDir()` and `cmd/root.go` defines the persistent `flagDryRun` (`--dry-run`) — reuse both.

- [ ] **Step 1: Write the failing test**

Create `cmd/teach_test.go`:

```go
package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTeachCommandRegistered(t *testing.T) {
	for _, c := range root.Commands() {
		if c.Name() == "teach" {
			return
		}
	}
	t.Fatal("teach command not registered")
}

func TestResolveSelection_ExplicitArgs(t *testing.T) {
	resetTeachFlags()
	got, err := resolveSelection([]string{"cursor", "claude"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// registry order: claude before cursor
	if !reflect.DeepEqual(got, []string{"claude", "cursor"}) {
		t.Fatalf("got %v", got)
	}
}

func TestResolveSelection_UnknownArg(t *testing.T) {
	resetTeachFlags()
	if _, err := resolveSelection([]string{"nope"}, t.TempDir()); err == nil {
		t.Fatal("expected error for unknown harness")
	}
}

func TestResolveSelection_AutoDetect(t *testing.T) {
	resetTeachFlags()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveSelection(nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"cursor"}) {
		t.Fatalf("got %v, want [cursor]", got)
	}
}

func TestResolveSelection_NoneDetectedInstallsAll(t *testing.T) {
	resetTeachFlags()
	got, err := resolveSelection(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"claude", "cursor", "agents"}) {
		t.Fatalf("got %v, want all", got)
	}
}

func resetTeachFlags() {
	flagTeachClaude = false
	flagTeachCursor = false
	flagTeachAgents = false
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/ -run 'Teach|ResolveSelection' -v`
Expected: FAIL — `resolveSelection`, `flagTeach*`, teach command undefined.

- [ ] **Step 3: Implement `cmd/teach.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheeraj-nalapat/lane/internal/agentskills"
	"github.com/spf13/cobra"
)

var (
	flagTeachClaude bool
	flagTeachCursor bool
	flagTeachAgents bool
	flagTeachGlobal bool
	flagTeachJSON   bool
)

var teachCmd = &cobra.Command{
	Use:   "teach [claude|cursor|agents...]",
	Short: "Install lane's agent skills into this project (or global config)",
	Long: `Install lane's agent integrations so coding agents learn to drive lane.

With no arguments, lane auto-detects which harnesses this project uses
(.claude/, .cursor/, AGENTS.md) and installs for those; if none are detected it
installs all three. Select explicitly with positional args (claude, cursor,
agents) or the matching flags. Use --global to install to user config where
supported (Claude). Cursor's global rules are UI-only, so --global --cursor
prints the rule to paste into Cursor Settings → Rules. Use --dry-run to preview.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := projectDir()
		if err != nil {
			return err
		}
		keys, err := resolveSelection(args, dir)
		if err != nil {
			return err
		}
		var results []agentskills.Result
		for _, k := range keys {
			it, _ := agentskills.Get(k)
			res, err := agentskills.Apply(it, dir, flagTeachGlobal, flagDryRun)
			if err != nil {
				return err
			}
			results = append(results, res)
		}
		return reportTeach(results, flagTeachJSON)
	},
}

// resolveSelection turns args + flags into an ordered list of integration keys.
// Explicit selection (args or flags) disables auto-detect.
func resolveSelection(args []string, dir string) ([]string, error) {
	set := map[string]bool{}
	for _, a := range args {
		if _, ok := agentskills.Get(a); !ok {
			return nil, fmt.Errorf("unknown harness %q (want: claude, cursor, agents)", a)
		}
		set[a] = true
	}
	if flagTeachClaude {
		set["claude"] = true
	}
	if flagTeachCursor {
		set["cursor"] = true
	}
	if flagTeachAgents {
		set["agents"] = true
	}
	if len(set) == 0 {
		detected := agentskills.Detect(dir)
		if len(detected) == 0 {
			for _, it := range agentskills.All() {
				set[it.Key] = true
			}
		} else {
			for _, k := range detected {
				set[k] = true
			}
		}
	}
	var keys []string
	for _, it := range agentskills.All() { // preserve registry order
		if set[it.Key] {
			keys = append(keys, it.Key)
		}
	}
	return keys, nil
}

func reportTeach(results []agentskills.Result, asJSON bool) error {
	if asJSON {
		if results == nil {
			results = []agentskills.Result{}
		}
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	w := tabwriter.NewWriter(os.Stderr, 0, 2, 2, ' ', 0)
	for _, r := range results {
		line := fmt.Sprintf("%s\t%s\t%s", r.Key, r.Target, r.Status)
		if r.Reason != "" {
			line += "\t" + r.Reason
		}
		fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	for _, r := range results { // Cursor global: emit the rule to paste
		if r.Status == agentskills.StatusSkipped && r.Content != "" {
			fmt.Fprintf(os.Stderr, "\nPaste this into Cursor Settings → Rules → User Rules:\n\n%s\n", r.Content)
		}
	}
	return nil
}

func init() {
	teachCmd.Flags().BoolVar(&flagTeachClaude, "claude", false, "install the Claude Code skill")
	teachCmd.Flags().BoolVar(&flagTeachCursor, "cursor", false, "install the Cursor rule")
	teachCmd.Flags().BoolVar(&flagTeachAgents, "agents-md", false, "install the AGENTS.md section")
	teachCmd.Flags().BoolVar(&flagTeachGlobal, "global", false, "install to global config where supported")
	teachCmd.Flags().BoolVar(&flagTeachJSON, "json", false, "output as JSON")
	root.AddCommand(teachCmd)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./cmd/ -run 'Teach|ResolveSelection' -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Manual smoke test in a throwaway dir**

Run:
```bash
go build -o /tmp/lane . && cd "$(mktemp -d)" && /tmp/lane teach && ls -R .claude .cursor AGENTS.md 2>&1 | head; cd -
```
Expected: prints a 3-line summary (`created` for each), and the files exist. Running it again prints `unchanged` for all three.

- [ ] **Step 6: Commit**

```bash
git add cmd/teach.go cmd/teach_test.go
git commit -m "feat(cmd): lane teach — install agent skills (auto-detect, --global, --json)"
```

---

## Task 6: `lane skills` command

**Files:**
- Create: `cmd/skills.go`
- Test: `cmd/skills_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/skills_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/agentskills"
)

func TestSkillsCommandRegistered(t *testing.T) {
	for _, c := range root.Commands() {
		if c.Name() == "skills" {
			return
		}
	}
	t.Fatal("skills command not registered")
}

func TestSkillState(t *testing.T) {
	cases := map[agentskills.Status]string{
		agentskills.StatusCreated:   "not installed",
		agentskills.StatusUnchanged: "installed (current)",
		agentskills.StatusUpdated:   "installed (outdated)",
		agentskills.StatusSkipped:   "manual",
	}
	for in, want := range cases {
		if got := skillState(in); got != want {
			t.Errorf("skillState(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/ -run 'Skills|SkillState' -v`
Expected: FAIL — `skills` command and `skillState` undefined.

- [ ] **Step 3: Implement `cmd/skills.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheeraj-nalapat/lane/internal/agentskills"
	"github.com/spf13/cobra"
)

var (
	flagSkillsGlobal bool
	flagSkillsJSON   bool
)

// skillInfo is the display/JSON shape for `lane skills`.
type skillInfo struct {
	Key         string             `json:"key"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Target      string             `json:"target"`
	Scope       agentskills.Scope  `json:"scope"`
	State       string             `json:"state"`
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Show the agent skills lane can install (install with: lane teach)",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := projectDir()
		if err != nil {
			return err
		}
		var infos []skillInfo
		for _, it := range agentskills.All() {
			// dryRun=true never writes; the status tells us what's installed.
			res, err := agentskills.Apply(it, dir, flagSkillsGlobal, true)
			if err != nil {
				return err
			}
			infos = append(infos, skillInfo{
				Key:         it.Key,
				Title:       it.Title,
				Description: it.Description,
				Target:      res.Target,
				Scope:       res.Scope,
				State:       skillState(res.Status),
			})
		}
		return reportSkills(infos, flagSkillsJSON)
	},
}

// skillState maps an Apply (dry-run) status to a human label for `lane skills`.
func skillState(s agentskills.Status) string {
	switch s {
	case agentskills.StatusCreated:
		return "not installed"
	case agentskills.StatusUnchanged:
		return "installed (current)"
	case agentskills.StatusUpdated:
		return "installed (outdated)"
	case agentskills.StatusSkipped:
		return "manual"
	}
	return string(s)
}

func reportSkills(infos []skillInfo, asJSON bool) error {
	if asJSON {
		if infos == nil {
			infos = []skillInfo{}
		}
		b, err := json.MarshalIndent(infos, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tTITLE\tTARGET\tSTATE")
	for _, i := range infos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Key, i.Title, i.Target, i.State)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "\nInstall with: lane teach   (see: lane teach --help)")
	return nil
}

func init() {
	skillsCmd.Flags().BoolVar(&flagSkillsGlobal, "global", false, "show global-config targets where supported")
	skillsCmd.Flags().BoolVar(&flagSkillsJSON, "json", false, "output as JSON")
	root.AddCommand(skillsCmd)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./cmd/ -run 'Skills|SkillState' -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Smoke test**

Run: `go build -o /tmp/lane . && /tmp/lane skills`
Expected: a table of the three integrations with `KEY/TITLE/TARGET/STATE` and a trailing "Install with: lane teach" line. `/tmp/lane skills --json` prints a JSON array.

- [ ] **Step 6: Commit**

```bash
git add cmd/skills.go cmd/skills_test.go
git commit -m "feat(cmd): lane skills — show installable agent integrations"
```

---

## Task 7: Docs + full verification

**Files:**
- Modify: `docs/guide/agents.md`
- Modify: `docs/guide/commands.md`
- Modify: `README.md`

- [ ] **Step 1: Update `docs/guide/agents.md`**

Replace the final "**Skill files:**" paragraph (the one beginning "a Claude Code skill (installable as a plugin …") with:

```markdown
**Install the skills.** Run `lane teach` in your project — lane auto-detects
which harnesses you use (`.claude/`, `.cursor/`, `AGENTS.md`) and installs its
skill/rule for each (run `lane skills` first to see what's available). Use
`lane teach --global` for the Claude skill in user config. The Claude skill is
also installable as a marketplace plugin
(`/plugin marketplace add Dheeraj-Nalapat/lane`).
```

- [ ] **Step 2: Add the commands to `docs/guide/commands.md`**

Read the file, then add two entries following the existing format used for `lane init` / `lane ls`. Use this content:

```markdown
### `lane skills`

Show the agent integrations lane can install (Claude Code skill, Cursor rule,
AGENTS.md section) and whether each is already present. `--json` for machine
output; `--global` to show global-config targets.

### `lane teach`

Install those integrations into the current project. With no arguments it
auto-detects which harnesses the project uses and installs for those (installs
all three if none are detected). Select explicitly with `claude`, `cursor`,
`agents` (or `--claude` / `--cursor` / `--agents-md`). `--global` installs the
Claude skill to user config; for Cursor it prints the rule to paste into Cursor
Settings → Rules (global Cursor rules are UI-only). `--dry-run` previews;
`--json` reports results as JSON.
```

- [ ] **Step 3: Add a README pointer**

In `README.md`, in the agents/quick-start area, add this line near the existing
agent guidance (after the `lane view` block in "Quick start"):

```markdown
# Teach your coding agent to drive lane (Claude Code / Cursor / AGENTS.md):
lane teach            # auto-detects the harnesses in this project
lane skills           # see what's available first
```

- [ ] **Step 4: Format, vet, and run the full suite**

Run:
```bash
gofmt -w . && test -z "$(gofmt -l .)" && go vet ./... && go test ./...
```
Expected: no gofmt output, `go vet` clean, all tests PASS. (CI runs `gofmt -l .` and `go test ./...`, so both must be clean.)

- [ ] **Step 5: Final manual end-to-end check**

Run:
```bash
go build -o /tmp/lane . \
  && d="$(mktemp -d)" && mkdir -p "$d/.cursor" \
  && /tmp/lane -C "$d" teach \
  && /tmp/lane -C "$d" teach \
  && /tmp/lane -C "$d" skills
```
Expected: first `teach` reports `cursor … created` (auto-detected from `.cursor/`); second reports `unchanged`; `skills` shows cursor as `installed (current)` and the others as `not installed`.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/agents.md docs/guide/commands.md README.md
git commit -m "docs: document lane teach + lane skills"
```

---

## Self-review notes

- **Spec coverage:** §1 commands → Tasks 5–6; §2 embed → Task 1; §3 detection/targets → Tasks 2 & 4; §4 idempotency/merge → Tasks 3–4; §5 Cursor `--global` paste flow → Task 4 (`Apply` manual branch) + Task 5 (`reportTeach` paste output); §6 output → Tasks 5–6; §7 testing → tests in Tasks 1–6; §8 docs → Task 7.
- **`--global` + AGENTS.md:** `Apply` ignores `global` for the agents/cursor-project strategies (AGENTS.md stays project-local), matching the spec; no special-casing needed beyond the cursor manual branch.
- **Type consistency:** `Integration`, `Result`, `Status*`, `Scope*`, `Apply`, `mergeAgents`, `Detect`, `resolveSelection`, `skillState` are named identically everywhere they appear across tasks.
