package runner

import (
	"fmt"
	"os"
	"os/exec"
)

type composeRunner struct{}

func (composeRunner) Name() string { return "compose" }

// buildComposeArgs builds the `docker <args>` for bringing a stack up detached.
func buildComposeArgs(slug, composePath, overridePath string, build bool) []string {
	args := []string{"compose", "-p", slug, "-f", composePath, "-f", overridePath, "up", "-d"}
	if build {
		args = append(args, "--build")
	}
	return args
}

func (composeRunner) DryRunLines(s RunSpec) string {
	return fmt.Sprintf("# runner: compose\n# command: docker %v\n",
		buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build))
}

func (composeRunner) Up(s RunSpec) error {
	printURLs(s)
	c := exec.Command("docker", buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build)...)
	c.Dir = s.Dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	fmt.Printf("up (detached). logs: lane logs --slug %s\n", s.Slug)
	return nil
}
