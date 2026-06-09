package runner

import (
	"strings"
	"testing"
)

func TestBuildComposeArgs(t *testing.T) {
	got := buildComposeArgs("remind", "/p/docker-compose.yml", "/h/.lane/overrides/remind.override.yml", false)
	want := "compose -p remind -f /p/docker-compose.yml -f /h/.lane/overrides/remind.override.yml up -d"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

func TestBuildComposeArgs_Build(t *testing.T) {
	got := buildComposeArgs("x", "a.yml", "b.yml", true)
	if got[len(got)-1] != "--build" {
		t.Fatalf("expected --build last, got %v", got)
	}
}
