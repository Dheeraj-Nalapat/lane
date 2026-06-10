package dockerx

import (
	"strings"
	"testing"
)

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

func TestParseContainers(t *testing.T) {
	out := []byte(`{"Names":"webapp-db-1","Labels":"com.docker.compose.service=db,com.docker.compose.project=webapp","State":"running"}
{"Names":"webapp-api-1","Labels":"com.docker.compose.project=webapp,com.docker.compose.service=api","State":"running"}
{"Names":"webapp-old-1","Labels":"com.docker.compose.service=old,com.docker.compose.project=webapp","State":"exited"}
`)
	got := parseContainers(out)
	byService := map[string]string{}
	for _, c := range got {
		byService[c.Service] = c.Name
	}
	if byService["db"] != "webapp-db-1" || byService["api"] != "webapp-api-1" {
		t.Fatalf("unexpected containers: %v", got)
	}
	if _, ok := byService["old"]; ok {
		t.Fatal("exited container must be excluded")
	}
}

func TestParseForeignContainers(t *testing.T) {
	// `docker network inspect <net> --format '{{json .Containers}}'` output.
	out := []byte(`{
		"abc": {"Name": "webapp-featx-api-1"},
		"def": {"Name": "webapp-db-1"},
		"ghi": {"Name": "webapp-auth-1"}
	}`)
	got := parseForeignContainers(out, "webapp-featx")
	want := []string{"webapp-auth-1", "webapp-db-1"} // sorted; the featx one is ours
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNetworkArgs(t *testing.T) {
	c := connectArgs("webapp-featx_default", "webapp-db-1", "db")
	if strings.Join(c, " ") != "network connect --alias db webapp-featx_default webapp-db-1" {
		t.Fatalf("connectArgs = %v", c)
	}
	d := disconnectArgs("webapp-featx_default", "webapp-db-1")
	if strings.Join(d, " ") != "network disconnect webapp-featx_default webapp-db-1" {
		t.Fatalf("disconnectArgs = %v", d)
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
