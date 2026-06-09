// Package preflight runs environment checks shared by doctor and the action
// commands (up/proxy/tls).
package preflight

import (
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var verRe = regexp.MustCompile(`v(\d+)\.(\d+)\.\d+`)

// composeOK reports whether a `docker compose version` line is >= 2.20.
func composeOK(line string) bool {
	mm := verRe.FindStringSubmatch(line)
	if mm == nil {
		return false
	}
	major, _ := strconv.Atoi(mm[1])
	minor, _ := strconv.Atoi(mm[2])
	return major > 2 || (major == 2 && minor >= 20)
}

// DockerRunning returns an actionable error if the Docker daemon is unreachable.
func DockerRunning() error {
	if err := exec.Command("docker", "info").Run(); err != nil {
		return errors.New("Docker doesn't appear to be running — start Docker and retry")
	}
	return nil
}

// ComposeOK runs `docker compose version` and reports whether it is >= 2.20,
// along with the raw version line.
func ComposeOK() (bool, string) {
	out, _ := exec.Command("docker", "compose", "version").CombinedOutput()
	line := strings.TrimSpace(string(out))
	return composeOK(line), line
}

// ComposeReady returns an actionable error if Docker Compose is too old.
func ComposeReady() error {
	if ok, line := ComposeOK(); !ok {
		return errors.New("Docker Compose >= 2.20 is required (the !reset override needs it); found: " + line)
	}
	return nil
}

// IsPortConflict reports whether command output indicates a host-port clash.
func IsPortConflict(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "address already in use") ||
		strings.Contains(s, "port is already allocated")
}
