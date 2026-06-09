package ui

import (
	"strings"
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
)

func TestRender(t *testing.T) {
	out := Render(
		[]stack.Stack{{Slug: "remind", URL: "http://remind.localhost", TiltPort: 10377, ProjectPath: "/p", Running: true}},
		[]traefikapi.Router{{Name: "remind-ui@docker", Rule: "Host(`remind.localhost`)", Service: "remind-ui", Status: "enabled"}},
	)
	if !strings.Contains(out, "remind") || !strings.Contains(out, "remind.localhost") {
		t.Fatalf("render missing content:\n%s", out)
	}
}

func TestRender_Empty(t *testing.T) {
	if !strings.Contains(Render(nil, nil), "none") {
		t.Fatal("empty render should say none")
	}
}

func TestRender_ComposeStackHidesTiltRow(t *testing.T) {
	out := Render(
		[]stack.Stack{{Slug: "demo", URL: "http://demo.localhost", TiltPort: 0, ProjectPath: "/p", Running: true}},
		nil,
	)
	if strings.Contains(out, "tilt →") {
		t.Fatalf("compose stack (TiltPort 0) must not show a tilt row:\n%s", out)
	}
}

func TestLogo(t *testing.T) {
	if n := strings.Count(Logo(), "\n"); n < 5 {
		t.Fatalf("logo should be multi-line, got %d newlines", n)
	}
}

func TestReselect(t *testing.T) {
	stacks := []stack.Stack{{Slug: "a"}, {Slug: "b"}, {Slug: "c"}}
	if i := Reselect("b", stacks); i != 1 {
		t.Fatalf("Reselect(b) = %d, want 1", i)
	}
	if i := Reselect("gone", stacks); i != 0 {
		t.Fatalf("Reselect(missing) = %d, want 0", i)
	}
	if i := Reselect("x", nil); i != 0 {
		t.Fatalf("Reselect(empty) = %d, want 0", i)
	}
}

func TestRenderPanel(t *testing.T) {
	st := PanelState{
		Stacks: []stack.Stack{
			{Slug: "remind", URL: "http://remind.localhost", ProjectPath: "/p/ReMind", TiltPort: 34339, Running: true},
			{Slug: "hsdemo", URL: "http://hsdemo.localhost", ProjectPath: "/p/hs", Running: true},
		},
		Routers:  []traefikapi.Router{{Name: "remind-ui@docker", Rule: "Host(`remind.localhost`)", Service: "remind-ui", Status: "enabled"}},
		Selected: 0, ProxyUp: true, TLSOn: false, Width: 80,
	}
	out := RenderPanel(st)
	for _, want := range []string{"remind", "hsdemo", "http://remind.localhost", "remind-ui", "proxy", "tls"} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderPanel missing %q:\n%s", want, out)
		}
	}
}

func TestRenderPanel_Empty(t *testing.T) {
	if !strings.Contains(RenderPanel(PanelState{Width: 80}), "no stacks") {
		t.Fatal("empty panel should say 'no stacks'")
	}
}

func TestRenderPanel_Confirm(t *testing.T) {
	st := PanelState{Stacks: []stack.Stack{{Slug: "remind", Running: true}}, Selected: 0, Width: 80, Confirm: "remind"}
	if !strings.Contains(RenderPanel(st), `down "remind"? (y/n)`) {
		t.Fatalf("confirm footer missing:\n%s", RenderPanel(st))
	}
}
