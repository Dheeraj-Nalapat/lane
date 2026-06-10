package tiltx

import (
	"strings"
	"testing"
)

func TestRenderDynamic_NoTLS(t *testing.T) {
	out, err := RenderDynamicRoute("remind", 10377, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "Host(`tilt-remind.localhost`)") {
		t.Errorf("missing web router:\n%s", s)
	}
	if strings.Contains(s, "websecure") || strings.Contains(s, "tls") {
		t.Errorf("no-TLS output must not contain websecure/tls:\n%s", s)
	}
}

func TestRenderDynamic_TLS(t *testing.T) {
	out, err := RenderDynamicRoute("remind", 10377, true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"remind-tilt-tls", "websecure", "tls: true"} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS output missing %q:\n%s", want, s)
		}
	}
}

func TestUpArgs_Resources(t *testing.T) {
	got := UpArgs(10377, []string{"api", "web"})
	want := []string{"up", "--host", "0.0.0.0", "--port", "10377", "api", "web", "--", "--docker"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("UpArgs = %v, want %v", got, want)
	}
}

func TestUpArgs_NoResources(t *testing.T) {
	got := UpArgs(10377, nil)
	want := []string{"up", "--host", "0.0.0.0", "--port", "10377", "--", "--docker"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("UpArgs = %v, want %v", got, want)
	}
}
