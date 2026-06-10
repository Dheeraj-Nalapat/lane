package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/tiltx"
)

type tiltRunner struct{}

func (tiltRunner) Name() string { return "tilt" }

func (tiltRunner) DryRunLines(s RunSpec) string {
	dyn, _ := tiltx.RenderDynamicRoute(s.Slug, s.TiltPort, s.TLS)
	return fmt.Sprintf("# runner: tilt\n# tilt port: %d\n# tilt dynamic (%s):\n%s\n# command: tilt %v\n# env adds: COMPOSE_PROJECT_NAME, LANE, LANE_SLUG, LANE_COMPOSE_OVERRIDE\n",
		s.TiltPort, s.DynamicPath, dyn, tiltx.UpArgs(s.TiltPort, s.Services))
}

func (tiltRunner) Up(s RunSpec) error {
	dyn, err := tiltx.RenderDynamicRoute(s.Slug, s.TiltPort, s.TLS)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.DynamicPath, dyn, 0o644); err != nil {
		return err
	}

	printURLs(s)
	emit(s, "  → http://tilt-%s.localhost  (Tilt UI)\n", s.Slug)

	tcmd := exec.Command("tilt", tiltx.UpArgs(s.TiltPort, s.Services)...)
	tcmd.Dir = s.Dir
	tcmd.Env = s.Env
	if len(s.Profiles) > 0 {
		tcmd.Env = append(tcmd.Env, "COMPOSE_PROFILES="+strings.Join(s.Profiles, ","))
	}

	if s.Detach {
		logf, err := os.Create(filepath.Join(paths.Run(), s.Slug+".log"))
		if err != nil {
			return err
		}
		tcmd.Stdout, tcmd.Stderr = logf, logf
		if err := tcmd.Start(); err != nil {
			return err
		}
		_ = os.WriteFile(filepath.Join(paths.Run(), s.Slug+".pid"),
			[]byte(fmt.Sprint(tcmd.Process.Pid)), 0o644)
		emit(s, "detached (pid %d). logs: lane logs --slug %s\n", tcmd.Process.Pid, s.Slug)
		return nil
	}
	tcmd.Stdout, tcmd.Stderr, tcmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return tcmd.Run()
}
