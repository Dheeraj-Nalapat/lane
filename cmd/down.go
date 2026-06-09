package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dheerajnalapat/lane/internal/gitx"
	"github.com/dheerajnalapat/lane/internal/manifest"
	"github.com/dheerajnalapat/lane/internal/paths"
	"github.com/dheerajnalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [path]",
	Short: "Tear down a stack and remove its generated files",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDown,
}

func init() { root.AddCommand(downCmd) }

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

	// Tear down compose resources for this project (keeps named volumes).
	overridePath := filepath.Join(paths.Overrides(), sl+".override.yml")
	composePath := filepath.Join(dir, m.ComposeFile)
	dc := exec.Command("docker", "compose", "-p", sl, "-f", composePath, "-f", overridePath, "down", "--remove-orphans")
	dc.Stdout, dc.Stderr = os.Stdout, os.Stderr
	if err := dc.Run(); err != nil {
		return err
	}

	_ = os.Remove(overridePath)
	_ = os.Remove(filepath.Join(paths.TraefikDynamic(), sl+".yml"))
	fmt.Printf("lane: %s torn down\n", sl)
	return nil
}
