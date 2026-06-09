package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dheerajnalapat/lane/internal/compose"
	"github.com/dheerajnalapat/lane/internal/dockerx"
	"github.com/dheerajnalapat/lane/internal/gitx"
	"github.com/dheerajnalapat/lane/internal/identity"
	"github.com/dheerajnalapat/lane/internal/manifest"
	"github.com/dheerajnalapat/lane/internal/override"
	"github.com/dheerajnalapat/lane/internal/paths"
	"github.com/dheerajnalapat/lane/internal/ports"
	"github.com/dheerajnalapat/lane/internal/proxy"
	"github.com/dheerajnalapat/lane/internal/slug"
	"github.com/dheerajnalapat/lane/internal/tiltx"
	"github.com/spf13/cobra"
)

var flagDetach bool

var upCmd = &cobra.Command{
	Use:   "up [path]",
	Short: "Bring a stack up behind the lane proxy",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&flagDetach, "detach", "d", false, "run Tilt in the background")
	root.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := projectDir(args)
	if err != nil {
		return err
	}

	m, err := manifest.Load(filepath.Join(dir, ".lane.toml"))
	if err != nil {
		return err
	}

	wt, _, err := gitx.Worktree(dir)
	if err != nil {
		return err
	}
	sl := slug.Resolve(slug.Inputs{
		Flag: flagSlug, Env: os.Getenv("LANE_SLUG"),
		ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir),
	})

	// Collision: same slug already claimed by a different path?
	if claimed, ok := dockerx.SlugOwner(sl); ok && claimed != dir {
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}

	tiltPort, err := ports.Free()
	if err != nil {
		return err
	}

	composePath := filepath.Join(dir, m.ComposeFile)
	svcs, err := compose.ServiceNames(composePath)
	if err != nil {
		return err
	}

	var routes []override.Route
	for _, r := range m.Routes {
		routes = append(routes, override.Route{
			Service: r.Service, Port: r.Port,
			Hostname: identity.RenderHost(r.Host, sl),
		})
	}

	body, err := override.Generate(override.Spec{
		Slug: sl, ProjectPath: dir, Network: proxy.Network,
		Services: svcs, Routes: routes, TiltPort: tiltPort,
	})
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}
	overridePath := filepath.Join(paths.Overrides(), sl+".override.yml")
	dynamicPath := filepath.Join(paths.TraefikDynamic(), sl+".yml")
	dynamic, err := tiltx.RenderDynamicRoute(sl, tiltPort)
	if err != nil {
		return err
	}

	env := append(os.Environ(),
		"COMPOSE_PROJECT_NAME="+sl,
		"LANE=1",
		"LANE_SLUG="+sl,
		"LANE_COMPOSE_OVERRIDE="+overridePath,
	)
	if m.APITarget != "" {
		env = append(env, "LANE_API_TARGET=http://"+m.APITarget)
	}

	if flagDryRun {
		fmt.Printf("# slug: %s\n# tilt port: %d\n# override (%s):\n%s\n# tilt dynamic (%s):\n%s\n# command: tilt %v\n# env adds: COMPOSE_PROJECT_NAME, LANE, LANE_SLUG, LANE_COMPOSE_OVERRIDE\n",
			sl, tiltPort, overridePath, body, dynamicPath, dynamic, tiltx.UpArgs(tiltPort))
		return nil
	}

	if err := os.WriteFile(overridePath, body, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(dynamicPath, dynamic, 0o644); err != nil {
		return err
	}
	if err := proxy.Ensure(); err != nil {
		return err
	}

	printURLs(sl, routes)

	tcmd := exec.Command("tilt", tiltx.UpArgs(tiltPort)...)
	tcmd.Dir = dir
	tcmd.Env = env

	if flagDetach {
		logf, err := os.Create(filepath.Join(paths.Run(), sl+".log"))
		if err != nil {
			return err
		}
		tcmd.Stdout, tcmd.Stderr = logf, logf
		if err := tcmd.Start(); err != nil {
			return err
		}
		pidFile := filepath.Join(paths.Run(), sl+".pid")
		_ = os.WriteFile(pidFile, []byte(fmt.Sprint(tcmd.Process.Pid)), 0o644)
		fmt.Printf("detached (pid %d). logs: lane logs --slug %s\n", tcmd.Process.Pid, sl)
		return nil
	}

	tcmd.Stdout, tcmd.Stderr, tcmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return tcmd.Run()
}

func projectDir(args []string) (string, error) {
	if len(args) == 1 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}

func printURLs(sl string, routes []override.Route) {
	fmt.Printf("lane: %s\n", sl)
	for _, r := range routes {
		fmt.Printf("  → http://%s  (%s:%d)\n", r.Hostname, r.Service, r.Port)
	}
	fmt.Printf("  → http://tilt-%s.localhost  (Tilt UI)\n", sl)
}
