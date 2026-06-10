package basex

import (
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/stack"
)

func stacks() []stack.Stack {
	return []stack.Stack{
		{Slug: "webapp", Project: "webapp", Running: true},
		{Slug: "webapp-featx", Project: "webapp", Running: true},
		{Slug: "other", Project: "other", Running: true},
	}
}

func TestFindBase_Canonical(t *testing.T) {
	got, err := FindBase(stacks(), "webapp", "webapp-featx")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "webapp" {
		t.Fatalf("base = %q, want webapp", got)
	}
}

func TestFindBase_IsBase(t *testing.T) {
	if _, err := FindBase(stacks(), "webapp", "webapp"); err == nil {
		t.Fatal("expected error when run from the base itself")
	}
}

func TestFindBase_None(t *testing.T) {
	only := []stack.Stack{{Slug: "x-featx", Project: "x", Running: false}}
	if _, err := FindBase(only, "x", "x-featx"); err == nil {
		t.Fatal("expected error when no running base")
	}
}

func TestFindBase_Multiple(t *testing.T) {
	ss := []stack.Stack{
		{Slug: "webapp-a", Project: "webapp", Running: true},
		{Slug: "webapp-b", Project: "webapp", Running: true},
	}
	if _, err := FindBase(ss, "webapp", "webapp-featx"); err == nil {
		t.Fatal("expected error for multiple candidates (no canonical)")
	}
}

func TestBorrowed(t *testing.T) {
	got := Borrowed([]string{"web", "api", "db", "auth"}, []string{"api"})
	want := []string{"auth", "db", "web"} // sorted, minus fresh
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
