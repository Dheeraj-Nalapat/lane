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

func TestBuiltServices(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  server:
    image: app/server
    build:
      context: .
  redis:
    image: redis:7
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := BuiltServices(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 1 || got[0] != "server" {
		t.Fatalf("BuiltServices = %v, want [server]", got)
	}
}
