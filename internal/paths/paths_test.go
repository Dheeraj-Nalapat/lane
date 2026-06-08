package paths

import (
	"strings"
	"testing"
)

func TestPaths(t *testing.T) {
	t.Setenv("BERTH_HOME", "/tmp/berthtest")
	if got := Home(); got != "/tmp/berthtest" {
		t.Fatalf("Home = %q", got)
	}
	if !strings.HasSuffix(Overrides(), "/overrides") {
		t.Fatalf("Overrides = %q", Overrides())
	}
	if !strings.HasSuffix(Run(), "/run") {
		t.Fatalf("Run = %q", Run())
	}
	if !strings.HasSuffix(TraefikDynamic(), "/traefik/dynamic") {
		t.Fatalf("TraefikDynamic = %q", TraefikDynamic())
	}
}
