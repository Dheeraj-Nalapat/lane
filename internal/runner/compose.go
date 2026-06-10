package runner

import (
	"fmt"
	"os"
	"os/exec"
)

type composeRunner struct{}

func (composeRunner) Name() string { return "compose" }

// buildComposeArgs builds the `docker <args>` for bringing a stack up detached.
// Global flags (--profile, -p, -f) must precede `up`; service names follow it.
func buildComposeArgs(slug, composePath, overridePath string, build, noDeps bool, profiles, services []string) []string {
	args := []string{"compose"}
	for _, p := range profiles {
		args = append(args, "--profile", p)
	}
	args = append(args, "-p", slug, "-f", composePath, "-f", overridePath, "up", "-d")
	if noDeps {
		args = append(args, "--no-deps")
	}
	if build {
		args = append(args, "--build")
	}
	args = append(args, services...)
	return args
}

func (composeRunner) DryRunLines(s RunSpec) string {
	return fmt.Sprintf("# runner: compose\n# command: docker %v\n",
		buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build, s.NoDeps, s.Profiles, s.Services))
}

func (composeRunner) Up(s RunSpec) error {
	printURLs(s)
	c := exec.Command("docker", buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build, s.NoDeps, s.Profiles, s.Services)...)
	c.Dir = s.Dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	emit(s, "up (detached). logs: lane logs --slug %s\n", s.Slug)
	return nil
}
