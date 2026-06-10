package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".lane.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Valid(t *testing.T) {
	p := write(t, `
name = "remind"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80

[[routes]]
service = "server"
port = 8000
host = "api.{slug}"
`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.Name != "remind" {
		t.Fatalf("Name = %q", m.Name)
	}
	if m.ComposeFile != "infra/docker-compose.yml" {
		t.Fatalf("ComposeFile = %q", m.ComposeFile)
	}
	if len(m.Routes) != 2 {
		t.Fatalf("got %d routes, want 2", len(m.Routes))
	}
	if m.Routes[1].Host != "api.{slug}" {
		t.Fatalf("route[1].Host = %q", m.Routes[1].Host)
	}
}

func TestLoad_MissingName(t *testing.T) {
	p := write(t, `compose_file = "docker-compose.yml"`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoad_NoRoutesNowAllowed(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("routes are optional now; got error: %v", err)
	}
	if len(m.Routes) != 0 {
		t.Fatalf("got %d routes, want 0", len(m.Routes))
	}
	if !m.AutorouteEnabled() {
		t.Fatal("autoroute should default to enabled")
	}
}

func TestLoad_AutorouteBlock(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"

[autoroute]
enabled = false
exclude = ["worker", "cron"]`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.AutorouteEnabled() {
		t.Fatal("autoroute should be disabled")
	}
	if len(m.Autoroute.Exclude) != 2 || m.Autoroute.Exclude[0] != "worker" {
		t.Fatalf("Exclude = %v", m.Autoroute.Exclude)
	}
}

func TestLoad_RunnerValid(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"
runner = "compose"
[[routes]]
service = "ui"
port = 80
`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.Runner != "compose" {
		t.Fatalf("Runner = %q, want compose", m.Runner)
	}
}

func TestLoad_RunnerInvalid(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"
runner = "nomad"
[[routes]]
service = "ui"
port = 80
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for invalid runner")
	}
}

func TestLoad_RunnerDefaultsEmpty(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"
[[routes]]
service = "ui"
port = 80
`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.Runner != "" {
		t.Fatalf("Runner = %q, want empty", m.Runner)
	}
}
