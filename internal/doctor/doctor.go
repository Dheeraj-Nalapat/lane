// Package doctor runs berth preflight checks.
package doctor

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Check is one diagnostic result.
type Check struct {
	Name string
	OK   bool
	Hint string
}

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

func cmdOut(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Run executes all checks.
func Run() []Check {
	var checks []Check

	_, err := cmdOut("docker", "info")
	checks = append(checks, Check{"docker daemon", err == nil, "start Docker"})

	cv, _ := cmdOut("docker", "compose", "version")
	checks = append(checks, Check{"compose >= 2.20", composeOK(cv), "upgrade Docker Compose"})

	_, err = exec.LookPath("tilt")
	checks = append(checks, Check{"tilt installed", err == nil, "install Tilt: https://tilt.dev"})

	// *.localhost resolves to loopback?
	addrs, _ := net.LookupHost("berth-check.localhost")
	loop := false
	for _, a := range addrs {
		if a == "127.0.0.1" || a == "::1" {
			loop = true
		}
	}
	checks = append(checks, Check{"*.localhost → loopback", loop, "ensure your resolver maps .localhost to 127.0.0.1"})

	return checks
}

// Report formats the checks; returns (text, allOK).
func Report() (string, bool) {
	var b strings.Builder
	all := true
	for _, c := range Run() {
		mark := "✓"
		if !c.OK {
			mark, all = "✗", false
			b.WriteString(fmt.Sprintf("%s %s — %s\n", mark, c.Name, c.Hint))
		} else {
			b.WriteString(fmt.Sprintf("%s %s\n", mark, c.Name))
		}
	}
	return b.String(), all
}
