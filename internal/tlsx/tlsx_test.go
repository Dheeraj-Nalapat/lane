package tlsx

import (
	"strings"
	"testing"
)

func TestCertNames(t *testing.T) {
	got := strings.Join(CertNames(), " ")
	want := "*.localhost localhost"
	if got != want {
		t.Fatalf("CertNames = %q, want %q", got, want)
	}
}

func TestMkcertArgs(t *testing.T) {
	got := strings.Join(mkcertArgs("/c/cert.pem", "/c/key.pem"), " ")
	want := "-cert-file /c/cert.pem -key-file /c/key.pem *.localhost localhost"
	if got != want {
		t.Fatalf("mkcertArgs = %q, want %q", got, want)
	}
}

func TestRenderTLSConfig(t *testing.T) {
	s := string(RenderTLSConfig())
	for _, want := range []string{"defaultCertificate", "/certs/cert.pem", "/certs/key.pem"} {
		if !strings.Contains(s, want) {
			t.Errorf("RenderTLSConfig missing %q:\n%s", want, s)
		}
	}
}

func TestEnabled_FollowsCert(t *testing.T) {
	t.Setenv("LANE_HOME", t.TempDir())
	if Enabled() {
		t.Fatal("Enabled() true with no cert")
	}
}
