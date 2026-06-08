package override

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	out, err := Generate(Spec{
		Slug:        "remind-featx",
		ProjectPath: "/home/u/remind",
		Network:     "berth",
		Services:    []string{"server", "agent-server", "ui", "worker"},
		TiltPort:    10377,
		Routes: []Route{
			{Service: "ui", Port: 80, Hostname: "remind-featx.localhost"},
			{Service: "server", Port: 8000, Hostname: "api.remind-featx.localhost"},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	s := string(out)

	// Every service gets ports + container_name reset.
	for _, svc := range []string{"server:", "agent-server:", "ui:", "worker:"} {
		if !strings.Contains(s, svc) {
			t.Errorf("missing service %q in:\n%s", svc, s)
		}
	}
	if c := strings.Count(s, "ports: !reset []"); c != 4 {
		t.Errorf("got %d 'ports: !reset []', want 4\n%s", c, s)
	}
	if c := strings.Count(s, "container_name: !reset null"); c != 4 {
		t.Errorf("got %d 'container_name: !reset null', want 4", c)
	}

	// Routed services get a Traefik host rule + the service port label.
	if !strings.Contains(s, "Host(`remind-featx.localhost`)") {
		t.Errorf("missing ui host rule\n%s", s)
	}
	if !strings.Contains(s, "Host(`api.remind-featx.localhost`)") {
		t.Errorf("missing server host rule")
	}
	if !strings.Contains(s, "loadbalancer.server.port=80") {
		t.Errorf("missing ui service port label")
	}

	// berth identity labels appear.
	if !strings.Contains(s, "berth.slug=remind-featx") {
		t.Errorf("missing berth.slug label")
	}
	if !strings.Contains(s, "berth.project.path=/home/u/remind") {
		t.Errorf("missing berth.project.path label")
	}
	if !strings.Contains(s, "berth.tilt.port=10377") {
		t.Errorf("missing berth.tilt.port label")
	}

	// The external network is declared.
	if !strings.Contains(s, "external: true") {
		t.Errorf("missing external network declaration")
	}
}
