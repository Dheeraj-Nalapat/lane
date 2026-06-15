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
