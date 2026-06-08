package scaffold

import (
	"strings"
	"testing"
)

func TestGuessAndRender(t *testing.T) {
	compose := `services:
  server:
    ports: ["8000:8000"]
  ui:
    ports: ["80:80"]
`
	svc, port := GuessWebEntry(compose)
	if svc != "ui" || port != 80 {
		t.Fatalf("guessed %s:%d, want ui:80", svc, port)
	}
	out := RenderManifest("myproj", "docker-compose.yml", svc, port)
	for _, want := range []string{`name = "myproj"`, `service = "ui"`, "port = 80"} {
		if !strings.Contains(out, want) {
			t.Errorf("manifest missing %q:\n%s", want, out)
		}
	}
}
