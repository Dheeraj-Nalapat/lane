package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheeraj-nalapat/lane/internal/agentskills"
	"github.com/spf13/cobra"
)

var (
	flagTeachClaude bool
	flagTeachCursor bool
	flagTeachAgents bool
	flagTeachGlobal bool
	flagTeachJSON   bool
)

var teachCmd = &cobra.Command{
	Use:   "teach [claude|cursor|agents...]",
	Short: "Install lane's agent skills into this project (or global config)",
	Long: `Install lane's agent integrations so coding agents learn to drive lane.

With no arguments, lane auto-detects which harnesses this project uses
(.claude/, .cursor/, AGENTS.md) and installs for those; if none are detected it
installs all three. Select explicitly with positional args (claude, cursor,
agents) or the matching flags. Use --global to install to user config where
supported (Claude). Cursor's global rules are UI-only, so --global --cursor
prints the rule to paste into Cursor Settings → Rules. Use --dry-run to preview.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := projectDir()
		if err != nil {
			return err
		}
		keys, err := resolveSelection(args, dir)
		if err != nil {
			return err
		}
		var results []agentskills.Result
		for _, k := range keys {
			it, _ := agentskills.Get(k)
			res, err := agentskills.Apply(it, dir, flagTeachGlobal, flagDryRun)
			if err != nil {
				return err
			}
			results = append(results, res)
		}
		return reportTeach(results, flagTeachJSON)
	},
}

// resolveSelection turns args + flags into an ordered list of integration keys.
// Explicit selection (args or flags) disables auto-detect.
func resolveSelection(args []string, dir string) ([]string, error) {
	set := map[string]bool{}
	for _, a := range args {
		if _, ok := agentskills.Get(a); !ok {
			return nil, fmt.Errorf("unknown harness %q (want: claude, cursor, agents)", a)
		}
		set[a] = true
	}
	if flagTeachClaude {
		set["claude"] = true
	}
	if flagTeachCursor {
		set["cursor"] = true
	}
	if flagTeachAgents {
		set["agents"] = true
	}
	if len(set) == 0 {
		detected := agentskills.Detect(dir)
		if len(detected) == 0 {
			for _, it := range agentskills.All() {
				set[it.Key] = true
			}
		} else {
			for _, k := range detected {
				set[k] = true
			}
		}
	}
	var keys []string
	for _, it := range agentskills.All() { // preserve registry order
		if set[it.Key] {
			keys = append(keys, it.Key)
		}
	}
	return keys, nil
}

func reportTeach(results []agentskills.Result, asJSON bool) error {
	if asJSON {
		if results == nil {
			results = []agentskills.Result{}
		}
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	w := tabwriter.NewWriter(os.Stderr, 0, 2, 2, ' ', 0)
	for _, r := range results {
		line := fmt.Sprintf("%s\t%s\t%s", r.Key, r.Target, r.Status)
		if r.Reason != "" {
			line += "\t" + r.Reason
		}
		fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	for _, r := range results { // Cursor global: emit the rule to paste
		if r.Status == agentskills.StatusSkipped && r.Content != "" {
			fmt.Fprintf(os.Stderr, "\nPaste this into Cursor Settings → Rules → User Rules:\n\n%s\n", r.Content)
		}
	}
	return nil
}

func init() {
	teachCmd.Flags().BoolVar(&flagTeachClaude, "claude", false, "install the Claude Code skill")
	teachCmd.Flags().BoolVar(&flagTeachCursor, "cursor", false, "install the Cursor rule")
	teachCmd.Flags().BoolVar(&flagTeachAgents, "agents-md", false, "install the AGENTS.md section")
	teachCmd.Flags().BoolVar(&flagTeachGlobal, "global", false, "install to global config where supported")
	teachCmd.Flags().BoolVar(&flagTeachJSON, "json", false, "output as JSON")
	root.AddCommand(teachCmd)
}
