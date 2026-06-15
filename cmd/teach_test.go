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
