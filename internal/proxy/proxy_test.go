package proxy

import (
	"strings"
	"testing"
)

func TestRenderCompose_NoTLS(t *testing.T) {
	out, err := renderCompose("lane", "/d/dynamic", "/d/certs", false)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"image: traefik:v3.1", "--providers.docker.network=lane", "external: true"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, absent := range []string{":443", "websecure", "/certs"} {
		if strings.Contains(s, absent) {
			t.Errorf("TLS-off output unexpectedly contains %q:\n%s", absent, s)
		}
	}
}

func TestRenderCompose_TLS(t *testing.T) {
	out, err := renderCompose("lane", "/d/dynamic", "/d/certs", true)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"--entryPoints.websecure.address=:443", `"443:443"`, "/d/certs:/certs:ro"} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS-on output missing %q:\n%s", want, s)
		}
	}
}
