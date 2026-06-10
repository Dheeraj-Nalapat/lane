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

func TestParsePS_Project(t *testing.T) {
	lines := `{"Names":"webapp-ui-1","Labels":"lane.managed=true,lane.slug=webapp,lane.project=webapp,lane.project.path=/p","State":"running"}`
	stacks := parsePS([]byte(lines))
	if len(stacks) != 1 {
		t.Fatalf("got %d stacks, want 1", len(stacks))
	}
	if stacks[0].Project != "webapp" {
		t.Fatalf("Project = %q, want webapp", stacks[0].Project)
	}
}

func TestParseRunningServices(t *testing.T) {
	// Two running containers + one exited, all in project "webapp".
	out := []byte(`{"Labels":"com.docker.compose.project=webapp,com.docker.compose.service=api","State":"running"}
{"Labels":"com.docker.compose.service=web,com.docker.compose.project=webapp","State":"running"}
{"Labels":"com.docker.compose.project=webapp,com.docker.compose.service=db","State":"exited"}
`)
	got := parseRunningServices(out)
	if !got["api"] || !got["web"] {
		t.Fatalf("api and web should be running: %v", got)
	}
	if got["db"] {
		t.Fatalf("db is exited; must not be running: %v", got)
	}
}
