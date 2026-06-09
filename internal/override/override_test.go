package override

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	out, err := Generate(Spec{
		Slug:        "remind-featx",
		ProjectPath: "/home/u/remind",
		Network:     "lane",
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

	// lane identity labels appear.
	if !strings.Contains(s, "lane.slug=remind-featx") {
		t.Errorf("missing lane.slug label")
	}
	if !strings.Contains(s, "lane.project.path=/home/u/remind") {
		t.Errorf("missing lane.project.path label")
	}
	if !strings.Contains(s, "lane.tilt.port=10377") {
		t.Errorf("missing lane.tilt.port label")
	}

	// The external network is declared.
	if !strings.Contains(s, "external: true") {
		t.Errorf("missing external network declaration")
	}
}

func TestGenerate_NoTiltPortLabelWhenZero(t *testing.T) {
	out, err := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"}, TiltPort: 0,
		Routes: []Route{{Service: "web", Port: 80, Hostname: "demo.localhost"}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if strings.Contains(string(out), "lane.tilt.port") {
		t.Fatalf("compose stack (TiltPort 0) must not emit lane.tilt.port:\n%s", out)
	}
}

func TestGenerate_TLSRouter(t *testing.T) {
	out, _ := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"}, TiltPort: 0, TLS: true,
		Routes: []Route{{Service: "web", Port: 80, Hostname: "demo.localhost"}},
	})
	s := string(out)
	for _, want := range []string{
		"traefik.http.routers.demo-web-tls.rule=Host(`demo.localhost`)",
		"traefik.http.routers.demo-web-tls.entrypoints=websecure",
		"traefik.http.routers.demo-web-tls.tls=true",
		"traefik.http.routers.demo-web-tls.service=demo-web",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS router missing %q:\n%s", want, s)
		}
	}
}

func TestGenerate_NoTLSRouterWhenOff(t *testing.T) {
	out, _ := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"}, TLS: false,
		Routes: []Route{{Service: "web", Port: 80, Hostname: "demo.localhost"}},
	})
	if strings.Contains(string(out), "-tls") {
		t.Fatalf("TLS off must not emit a -tls router:\n%s", out)
	}
}

func TestGenerate_ResetsBuiltImage(t *testing.T) {
	out, _ := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"server", "redis"}, BuiltServices: []string{"server"},
		Routes: []Route{{Service: "server", Port: 8000, Hostname: "demo.localhost"}},
	})
	s := string(out)
	if c := strings.Count(s, "image: !reset null"); c != 1 {
		t.Fatalf("got %d image resets, want 1:\n%s", c, s)
	}
}
