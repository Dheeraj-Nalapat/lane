package dockerx

import "testing"

func TestParsePS(t *testing.T) {
	// One JSON line per container, as `docker ps --format '{{json .}}'` emits.
	lines := `{"Names":"remind-ui-1","Labels":"berth.managed=true,berth.slug=remind,berth.url=http://remind.localhost,berth.tilt.port=10377,berth.project.path=/home/u/remind","State":"running"}
{"Names":"remind-server-1","Labels":"berth.managed=true,berth.slug=remind,berth.project.path=/home/u/remind","State":"running"}
{"Names":"x-ui-1","Labels":"berth.managed=true,berth.slug=x,berth.url=http://x.localhost,berth.tilt.port=10500,berth.project.path=/home/u/x","State":"running"}`
	stacks := parsePS([]byte(lines))
	if len(stacks) != 2 {
		t.Fatalf("got %d stacks, want 2", len(stacks))
	}
	bySlug := map[string]int{}
	for _, s := range stacks {
		bySlug[s.Slug] = s.TiltPort
	}
	if bySlug["remind"] != 10377 {
		t.Fatalf("remind tilt port = %d", bySlug["remind"])
	}
}
