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
