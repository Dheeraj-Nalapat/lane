package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail a stack's logs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := projectDir()
		if err != nil {
			return err
		}
		sl := flagSlug
		if sl == "" {
			m, err := manifest.Load(filepath.Join(dir, ".lane.toml"))
			if err != nil {
				return err
			}
			wt, _, _ := gitx.Worktree(dir)
			sl = slug.Resolve(slug.Inputs{ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir), Env: os.Getenv("LANE_SLUG")})
		}

		// Detached run → tail the log file; else stream compose logs.
		logFile := filepath.Join(paths.Run(), sl+".log")
		if _, err := os.Stat(logFile); err == nil {
			c := exec.Command("tail", "-f", logFile)
			c.Stdout, c.Stderr = os.Stdout, os.Stderr
			return c.Run()
		}
		fmt.Printf("no detached log for %s; streaming container logs\n", sl)
		c := exec.Command("docker", "compose", "-p", sl, "logs", "-f")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	},
}

func init() { root.AddCommand(logsCmd) }
