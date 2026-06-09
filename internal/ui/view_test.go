package ui

import (
	"strings"
	"testing"

	"github.com/dheerajnalapat/lane/internal/stack"
	"github.com/dheerajnalapat/lane/internal/traefikapi"
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
