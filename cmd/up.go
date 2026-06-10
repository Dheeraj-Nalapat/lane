package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/dheeraj-nalapat/lane/internal/routing"
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
	flagProfiles    []string
)

type upURL struct {
	Service string `json:"service"`
	Host    string `json:"host"`
	URL     string `json:"url"`
	Running bool   `json:"running"`
}
type upResult struct {
	Slug    string  `json:"slug"`
	Runner  string  `json:"runner"`
	TLS     bool    `json:"tls"`
	TiltURL string  `json:"tiltUrl,omitempty"`
	URLs    []upURL `json:"urls"`
}

func buildUpResult(slug, runnerName string, tlsOn bool, routes []override.Route, tiltPort int, running map[string]bool) upResult {
	res := upResult{Slug: slug, Runner: runnerName, TLS: tlsOn}
	for _, r := range routes {
		res.URLs = append(res.URLs, upURL{
			Service: r.Service, Host: r.Hostname, URL: "http://" + r.Hostname,
			Running: running[r.Service],
		})
	}
	if tiltPort > 0 {
		res.TiltURL = "http://tilt-" + slug + ".localhost"
	}
	return res
}

// runningRouteURLs returns the URLs of routes whose service is running.
func runningRouteURLs(routes []override.Route, running map[string]bool) []string {
	var u []string
	for _, r := range routes {
		if running[r.Service] {
			u = append(u, "http://"+r.Hostname)
		}
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
	Use:   "up [services...]",
	Short: "Bring a stack up behind the lane proxy",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&flagDetach, "detach", "d", false, "run Tilt in the background (no-op for the compose runner)")
	upCmd.Flags().BoolVar(&flagBuild, "build", false, "force image rebuild (compose runner)")
	upCmd.Flags().BoolVar(&flagJSON, "json", false, "print the result as JSON (implies detach)")
	upCmd.Flags().BoolVar(&flagWait, "wait", false, "wait until routes are serving before returning (implies detach)")
	upCmd.Flags().DurationVar(&flagWaitTimeout, "wait-timeout", 90*time.Second, "max time to wait with --wait")
	upCmd.Flags().StringSliceVarP(&flagProfiles, "profile", "p", nil, "compose profile(s) to activate (repeatable)")
	root.AddCommand(upCmd)
}

func tiltfileExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Tiltfile"))
	return err == nil
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := projectDir()
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

	composePath := filepath.Join(dir, m.ComposeFile)
	svcInfos, err := compose.Services(composePath)
	if err != nil {
		return err
	}
	var svcs, built []string
	for _, s := range svcInfos {
		svcs = append(svcs, s.Name)
		if s.Build {
			built = append(built, s.Name)
		}
	}

	var explicit []override.Route
	for _, r := range m.Routes {
		explicit = append(explicit, override.Route{
			Service: r.Service, Port: r.Port,
			Hostname: identity.RenderHost(r.Host, sl),
		})
	}
	routes, skipped := routing.Resolve(sl, svcInfos, explicit, m.AutorouteEnabled(), m.Autoroute.Exclude)
	if len(skipped) > 0 && !flagJSON {
		fmt.Fprintf(os.Stderr, "lane: not auto-routed (no single exposed port): %s\n", strings.Join(skipped, ", "))
	}

	runnerName := runner.Select(m.Runner, tiltfileExists(dir))
	selection := len(args) > 0 || len(flagProfiles) > 0

	if claimed, ok := dockerx.SlugOwner(sl); ok {
		if claimed != dir {
			return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
		}
		// Stack is already owned by this project. With no selection, a whole-stack
		// `up` is a no-op; with a selection we fall through to bring the named
		// services up additively (compose `up <svc>` leaves running ones alone).
		if !selection {
			if flagJSON {
				running, _ := dockerx.RunningServices(sl)
				return printJSON(buildUpResult(sl, runnerName, tlsx.Enabled(), routes, 0, running))
			}
			fmt.Printf("stack %q already running — use `lane restart` to recreate, or `lane down` to stop\n", sl)
			return nil
		}
		if runnerName == "tilt" {
			return fmt.Errorf("stack %q is already running under Tilt; enable resources in the Tilt UI or run `lane restart`", sl)
		}
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
		Slug: sl, Project: m.Name, ProjectPath: dir, Network: proxy.Network,
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
		Quiet:    flagJSON,
		Services: args,
		Profiles: flagProfiles,
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
	if flagWait || flagJSON {
		running, err := dockerx.RunningServices(sl)
		if err != nil {
			return err
		}
		if flagWait {
			if err := ready.WaitReady(runningRouteURLs(routes, running), flagWaitTimeout, nil); err != nil {
				return err
			}
		}
		if flagJSON {
			return printJSON(buildUpResult(sl, runnerName, tlsOn, routes, tiltPort, running))
		}
	}
	return nil
}

func projectDir() (string, error) {
	if flagPath != "" {
		return filepath.Abs(flagPath)
	}
	return os.Getwd()
}
