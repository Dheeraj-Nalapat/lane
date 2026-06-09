package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/identity"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/override"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/ports"
	"github.com/dheeraj-nalapat/lane/internal/proxy"
	"github.com/dheeraj-nalapat/lane/internal/runner"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var (
	flagDetach bool
	flagBuild  bool
)

var upCmd = &cobra.Command{
	Use:   "up [path]",
	Short: "Bring a stack up behind the lane proxy",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&flagDetach, "detach", "d", false, "run Tilt in the background (no-op for the compose runner)")
	upCmd.Flags().BoolVar(&flagBuild, "build", false, "force image rebuild (compose runner)")
	root.AddCommand(upCmd)
}

func tiltfileExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Tiltfile"))
	return err == nil
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

	if claimed, ok := dockerx.SlugOwner(sl); ok && claimed != dir {
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}

	composePath := filepath.Join(dir, m.ComposeFile)
	svcs, err := compose.ServiceNames(composePath)
	if err != nil {
		return err
	}

	runnerName := runner.Select(m.Runner, tiltfileExists(dir))
	if m.Runner == "tilt" && !tiltfileExists(dir) {
		fmt.Fprintln(os.Stderr, "lane: warning: runner=tilt but no Tiltfile found in", dir)
	}

	tiltPort := 0
	if runnerName == "tilt" {
		if tiltPort, err = ports.Free(); err != nil {
			return err
		}
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

	env := append(os.Environ(),
		"COMPOSE_PROJECT_NAME="+sl,
		"LANE=1",
		"LANE_SLUG="+sl,
		"LANE_COMPOSE_OVERRIDE="+overridePath,
	)
	if m.APITarget != "" {
		env = append(env, "LANE_API_TARGET=http://"+m.APITarget)
	}

	spec := runner.RunSpec{
		Slug: sl, Dir: dir, ComposePath: composePath, OverridePath: overridePath,
		Routes: routes, Detach: flagDetach, Build: flagBuild,
		TiltPort: tiltPort, DynamicPath: dynamicPath, Env: env,
	}
	r := runner.New(runnerName)

	if flagDryRun {
		fmt.Printf("# slug: %s\n# override (%s):\n%s\n%s", sl, overridePath, body, r.DryRunLines(spec))
		return nil
	}

	if err := os.WriteFile(overridePath, body, 0o644); err != nil {
		return err
	}
	if err := proxy.Ensure(); err != nil {
		return err
	}
	return r.Up(spec)
}

func projectDir(args []string) (string, error) {
	if len(args) == 1 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}
