package dockerx

import "testing"

func TestParsePS(t *testing.T) {
	// One JSON line per container, as `docker ps --format '{{json .}}'` emits.
	lines := `{"Names":"remind-ui-1","Labels":"lane.managed=true,lane.slug=remind,lane.url=http://remind.localhost,lane.tilt.port=10377,lane.project.path=/home/u/remind","State":"running"}
{"Names":"remind-server-1","Labels":"lane.managed=true,lane.slug=remind,lane.project.path=/home/u/remind","State":"running"}
{"Names":"x-ui-1","Labels":"lane.managed=true,lane.slug=x,lane.url=http://x.localhost,lane.tilt.port=10500,lane.project.path=/home/u/x","State":"running"}`
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
