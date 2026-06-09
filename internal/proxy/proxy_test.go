package proxy

import (
	"strings"
	"testing"
)

func TestRenderCompose(t *testing.T) {
	out, err := renderCompose("lane", "/tmp/lane/traefik/dynamic")
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"image: traefik:v3.1",
		"--providers.docker.network=lane",
		"host.docker.internal:host-gateway",
		"/tmp/lane/traefik/dynamic:/dynamic:ro",
		"external: true",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered compose missing %q\n%s", want, s)
		}
	}
}
