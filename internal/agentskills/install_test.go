package agentskills

import (
	"os"
	"path/filepath"
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
