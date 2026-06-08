package compose

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestServiceNames(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  server:
    image: x
  ui:
    image: y
volumes:
  data:
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ServiceNames(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	sort.Strings(got)
	want := []string{"server", "ui"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}
