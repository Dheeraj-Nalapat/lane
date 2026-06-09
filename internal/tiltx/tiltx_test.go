package tiltx

import (
	"strings"
	"testing"
)

func TestRenderDynamic(t *testing.T) {
	out, err := RenderDynamicRoute("remind", 10377)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"Host(`tilt-remind.localhost`)",
		"http://host.docker.internal:10377",
		"remind-tilt",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestUpArgs(t *testing.T) {
	got := UpArgs(10377)
	want := []string{"up", "--host", "0.0.0.0", "--port", "10377", "--", "--docker"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("UpArgs = %v, want %v", got, want)
	}
}
