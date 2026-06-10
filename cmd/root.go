package cmd

import "github.com/spf13/cobra"

// Version is the lane version, set via -ldflags at release time (GoReleaser).
var Version = "dev"

var (
	flagSlug    string
	flagDryRun  bool
	flagVerbose bool
	flagPath    string
)

var root = &cobra.Command{
	Use:           "lane",
	Short:         "Run many project stacks at once with zero port conflicts",
	Version:       Version,
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
