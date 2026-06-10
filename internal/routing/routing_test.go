package routing

import (
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/override"
)

func TestResolve(t *testing.T) {
	services := []compose.Service{
		{Name: "web", Port: 80},
		{Name: "api", Port: 8000},
		{Name: "admin", Port: 9000},
		{Name: "worker", Port: 0},  // no port -> skipped
		{Name: "cron", Port: 7000}, // excluded
	}
	explicit := []override.Route{
		{Service: "web", Port: 80, Hostname: "webapp.localhost"}, // explicit wins
	}
	routes, skipped := Resolve("webapp", services, explicit, true, []string{"cron"})

	byService := map[string]override.Route{}
	for _, r := range routes {
		byService[r.Service] = r
	}
	if byService["web"].Hostname != "webapp.localhost" {
		t.Errorf("web should keep explicit host, got %q", byService["web"].Hostname)
	}
	if byService["api"].Hostname != "webapp-api.localhost" {
		t.Errorf("api auto host = %q", byService["api"].Hostname)
	}
	if byService["admin"].Hostname != "webapp-admin.localhost" {
		t.Errorf("admin auto host = %q", byService["admin"].Hostname)
	}
	if _, ok := byService["cron"]; ok {
		t.Error("cron is excluded; must not be routed")
	}
	if _, ok := byService["worker"]; ok {
		t.Error("worker has no port; must not be routed")
	}
	if len(skipped) != 1 || skipped[0] != "worker" {
		t.Errorf("skipped = %v, want [worker]", skipped)
	}
}

func TestResolve_AutorouteDisabled(t *testing.T) {
	services := []compose.Service{{Name: "api", Port: 8000}}
	explicit := []override.Route{{Service: "web", Port: 80, Hostname: "webapp.localhost"}}
	routes, skipped := Resolve("webapp", services, explicit, false, nil)
	if len(routes) != 1 || routes[0].Service != "web" {
		t.Fatalf("disabled autoroute should yield only explicit routes, got %v", routes)
	}
	if len(skipped) != 0 {
		t.Fatalf("no skip reporting when autoroute disabled, got %v", skipped)
	}
}
