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
