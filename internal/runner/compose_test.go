package runner

import (
	"strings"
	"testing"
)

func TestBuildComposeArgs(t *testing.T) {
	got := buildComposeArgs("remind", "/p/docker-compose.yml", "/h/.lane/overrides/remind.override.yml", false, false, nil, nil)
	want := "compose -p remind -f /p/docker-compose.yml -f /h/.lane/overrides/remind.override.yml up -d"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

func TestBuildComposeArgs_Build(t *testing.T) {
	got := buildComposeArgs("x", "a.yml", "b.yml", true, false, nil, nil)
	if got[len(got)-1] != "--build" {
		t.Fatalf("expected --build last, got %v", got)
	}
}

func TestBuildComposeArgs_ProfilesAndServices(t *testing.T) {
	got := buildComposeArgs("webapp", "compose.yml", "ovr.yml", true, false,
		[]string{"minimal", "debug"}, []string{"api", "web"})
	want := []string{
		"compose", "--profile", "minimal", "--profile", "debug",
		"-p", "webapp", "-f", "compose.yml", "-f", "ovr.yml",
		"up", "-d", "--build", "api", "web",
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}

func TestBuildComposeArgs_NoDeps(t *testing.T) {
	got := buildComposeArgs("webapp", "compose.yml", "ovr.yml", false, true, nil, []string{"api"})
	want := []string{"compose", "-p", "webapp", "-f", "compose.yml", "-f", "ovr.yml", "up", "-d", "--no-deps", "api"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}
