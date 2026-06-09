package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/identity"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/override"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/ports"
	"github.com/dheeraj-nalapat/lane/internal/preflight"
	"github.com/dheeraj-nalapat/lane/internal/proxy"
	"github.com/dheeraj-nalapat/lane/internal/ready"
	"github.com/dheeraj-nalapat/lane/internal/runner"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/dheeraj-nalapat/lane/internal/tlsx"
	"github.com/spf13/cobra"
)

var (
	flagDetach      bool
	flagBuild       bool
	flagJSON        bool
	flagWait        bool
	flagWaitTimeout time.Duration
)

type upURL struct {
	Service string `json:"service"`
	Host    string `json:"host"`
	URL     string `json:"url"`
}
type upResult struct {
	Slug    string  `json:"slug"`
	Runner  string  `json:"runner"`
	TLS     bool    `json:"tls"`
	TiltURL string  `json:"tiltUrl,omitempty"`
	URLs    []upURL `json:"urls"`
}

func buildUpResult(slug, runnerName string, tlsOn bool, routes []override.Route, tiltPort int) upResult {
	res := upResult{Slug: slug, Runner: runnerName, TLS: tlsOn}
	for _, r := range routes {
		res.URLs = append(res.URLs, upURL{Service: r.Service, Host: r.Hostname, URL: "http://" + r.Hostname})
	}
	if tiltPort > 0 {
		res.TiltURL = "http://tilt-" + slug + ".localhost"
	}
	return res
}

func routeURLs(routes []override.Route) []string {
	var u []string
	for _, r := range routes {
		u = append(u, "http://"+r.Hostname)
	}
	return u
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

var upCmd = &cobra.Command{
	Use:   "up [path]",
	Short: "Bring a stack up behind the lane proxy",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&flagDetach, "detach", "d", false, "run Tilt in the background (no-op for the compose runner)")
	upCmd.Flags().BoolVar(&flagBuild, "build", false, "force image rebuild (compose runner)")
	upCmd.Flags().BoolVar(&flagJSON, "json", false, "print the result as JSON (implies detach)")
	upCmd.Flags().BoolVar(&flagWait, "wait", false, "wait until routes are serving before returning (implies detach)")
	upCmd.Flags().DurationVar(&flagWaitTimeout, "wait-timeout", 90*time.Second, "max time to wait with --wait")
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
	if err := preflight.DockerRunning(); err != nil {
		return err
	}
	if err := preflight.ComposeReady(); err != nil {
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

	var routes []override.Route
	for _, r := range m.Routes {
		routes = append(routes, override.Route{
			Service: r.Service, Port: r.Port,
			Hostname: identity.RenderHost(r.Host, sl),
		})
	}
	runnerName := runner.Select(m.Runner, tiltfileExists(dir))

	if claimed, ok := dockerx.SlugOwner(sl); ok {
		if claimed == dir {
			if flagJSON {
				return printJSON(buildUpResult(sl, runnerName, tlsx.Enabled(), routes, 0))
			}
			fmt.Printf("stack %q already running — use `lane restart` to recreate, or `lane down` to stop\n", sl)
			return nil
		}
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}

	composePath := filepath.Join(dir, m.ComposeFile)
	svcs, err := compose.ServiceNames(composePath)
	if err != nil {
		return err
	}
	built, err := compose.BuiltServices(composePath)
	if err != nil {
		return err
	}

	if m.Runner == "tilt" && !tiltfileExists(dir) {
		fmt.Fprintln(os.Stderr, "lane: warning: runner=tilt but no Tiltfile found in", dir)
	}

	tiltPort := 0
	if runnerName == "tilt" {
		if tiltPort, err = ports.Free(); err != nil {
			return err
		}
	}

	tlsOn := tlsx.Enabled()
	body, err := override.Generate(override.Spec{
		Slug: sl, ProjectPath: dir, Network: proxy.Network,
		Services: svcs, Routes: routes, TiltPort: tiltPort, TLS: tlsOn,
		BuiltServices: built,
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

	wantResult := flagJSON || flagWait
	detach := flagDetach || (wantResult && runnerName == "tilt")

	spec := runner.RunSpec{
		Slug: sl, Dir: dir, ComposePath: composePath, OverridePath: overridePath,
		Routes: routes, Detach: detach, Build: flagBuild,
		TiltPort: tiltPort, DynamicPath: dynamicPath, Env: env, TLS: tlsOn,
		Quiet: flagJSON,
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
	if err := r.Up(spec); err != nil {
		return err
	}
	if flagWait {
		if err := ready.WaitReady(routeURLs(routes), flagWaitTimeout, nil); err != nil {
			return err
		}
	}
	if flagJSON {
		return printJSON(buildUpResult(sl, runnerName, tlsOn, routes, tiltPort))
	}
	return nil
}

func projectDir(args []string) (string, error) {
	if len(args) == 1 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}
