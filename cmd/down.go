package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [path]",
	Short: "Tear down a stack and remove its generated files",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDown,
}

var flagDownVolumes bool

func init() {
	downCmd.Flags().BoolVar(&flagDownVolumes, "volumes", false, "also remove the stack's named volumes (data reset)")
	root.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	dir, err := projectDir(args)
	if err != nil {
		return err
	}
	m, err := manifest.Load(filepath.Join(dir, ".lane.toml"))
	if err != nil {
		return err
	}
	wt, _, _ := gitx.Worktree(dir)
	sl := slug.Resolve(slug.Inputs{
		Flag: flagSlug, Env: os.Getenv("LANE_SLUG"),
		ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir),
	})

	// Stop a detached Tilt process if present.
	pidFile := filepath.Join(paths.Run(), sl+".pid")
	if b, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			terminate(pid)
		}
		_ = os.Remove(pidFile)
	}

	// Tear down compose resources for this project. We pass only the base
	// compose file: `down` removes containers by project name, so the override
	// isn't required (and may already be gone — keeps `restart` robust).
	composePath := filepath.Join(dir, m.ComposeFile)
	dcArgs := []string{"compose", "-p", sl, "-f", composePath, "down", "--remove-orphans"}
	if flagDownVolumes {
		dcArgs = append(dcArgs, "--volumes")
	}
	dc := exec.Command("docker", dcArgs...)
	dc.Stdout, dc.Stderr = os.Stdout, os.Stderr
	if err := dc.Run(); err != nil {
		return err
	}

	_ = os.Remove(filepath.Join(paths.Overrides(), sl+".override.yml"))
	_ = os.Remove(filepath.Join(paths.TraefikDynamic(), sl+".yml"))
	msg := sl + " torn down"
	if flagDownVolumes {
		msg += " (with volumes)"
	}
	fmt.Printf("lane: %s\n", msg)
	return nil
}
