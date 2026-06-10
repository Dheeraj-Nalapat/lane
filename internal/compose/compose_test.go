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

func TestServices_PortDiscovery(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  exposed:
    image: a
    expose: [8000]
  shortport:
    image: b
    ports: ["3000:80"]
  longport:
    image: c
    ports:
      - target: 9000
        published: 9001
  multi:
    image: d
    expose: [80, 443]
  none:
    image: e
  built:
    build: .
    expose: ["5000"]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Services(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	byName := map[string]Service{}
	for _, s := range got {
		byName[s.Name] = s
	}
	cases := map[string]int{
		"exposed":   8000, // single expose
		"shortport": 80,   // container side of short syntax
		"longport":  9000, // target of long syntax
		"multi":     0,    // ambiguous -> 0
		"none":      0,    // nothing -> 0
		"built":     5000,
	}
	for name, wantPort := range cases {
		if byName[name].Port != wantPort {
			t.Errorf("%s: Port = %d, want %d", name, byName[name].Port, wantPort)
		}
	}
	if !byName["built"].Build {
		t.Error("built: Build = false, want true")
	}
	if byName["exposed"].Build {
		t.Error("exposed: Build = true, want false")
	}
}
