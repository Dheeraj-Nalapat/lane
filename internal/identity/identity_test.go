package identity

import "testing"

func TestResolveHostnames(t *testing.T) {
	got := RenderHost("api.{slug}", "remind-featx")
	if got != "api.remind-featx.localhost" {
		t.Fatalf("RenderHost = %q", got)
	}
	if RenderHost("{slug}", "remind") != "remind.localhost" {
		t.Fatalf("default host wrong")
	}
}
