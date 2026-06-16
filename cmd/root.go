package cmd

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version is the lane version, set via -ldflags at release time (GoReleaser).
// When unset (e.g. `go install` or a local `go build`), it falls back to the
// version embedded in the binary's build info.
var Version = "dev"

// resolveVersion returns Version when it was set via -ldflags, otherwise it
// derives a meaningful version from the binary's embedded build info: the
// module version for `go install`, or the VCS revision for a local build.
func resolveVersion() string {
	if Version != "dev" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}

	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		v := "dev-" + revision
		if modified == "true" {
			v += "-dirty"
		}
		return v
	}

	return Version
}

var (
	flagSlug    string
	flagDryRun  bool
	flagVerbose bool
	flagPath    string
)

var root = &cobra.Command{
	Use:           "lane",
	Short:         "Run many project stacks at once with zero port conflicts",
	Version:       resolveVersion(),
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	root.PersistentFlags().StringVar(&flagSlug, "slug", "", "override the stack slug")
	root.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "print what would happen, then exit")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
	root.PersistentFlags().StringVarP(&flagPath, "path", "C", "", "project directory (default: current directory)")
}

// Execute runs the CLI.
func Execute() error { return root.Execute() }
