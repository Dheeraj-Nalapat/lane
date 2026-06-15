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
	flagSkillsGlobal bool
	flagSkillsJSON   bool
)

// skillInfo is the display/JSON shape for `lane skills`.
type skillInfo struct {
	Key         string            `json:"key"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Target      string            `json:"target"`
	Scope       agentskills.Scope `json:"scope"`
	State       string            `json:"state"`
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Show the agent skills lane can install (install with: lane teach)",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := projectDir()
		if err != nil {
			return err
		}
		var infos []skillInfo
		for _, it := range agentskills.All() {
			// dryRun=true never writes; the status tells us what's installed.
			res, err := agentskills.Apply(it, dir, flagSkillsGlobal, true)
			if err != nil {
				return err
			}
			infos = append(infos, skillInfo{
				Key:         it.Key,
				Title:       it.Title,
				Description: it.Description,
				Target:      res.Target,
				Scope:       res.Scope,
				State:       skillState(res.Status),
			})
		}
		return reportSkills(infos, flagSkillsJSON)
	},
}

// skillState maps an Apply (dry-run) status to a human label for `lane skills`.
func skillState(s agentskills.Status) string {
	switch s {
	case agentskills.StatusCreated:
		return "not installed"
	case agentskills.StatusUnchanged:
		return "installed (current)"
	case agentskills.StatusUpdated:
		return "installed (outdated)"
	case agentskills.StatusSkipped:
		return "manual"
	}
	return string(s)
}

func reportSkills(infos []skillInfo, asJSON bool) error {
	if asJSON {
		if infos == nil {
			infos = []skillInfo{}
		}
		b, err := json.MarshalIndent(infos, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tTITLE\tTARGET\tSTATE")
	for _, i := range infos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Key, i.Title, i.Target, i.State)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "\nInstall with: lane teach   (see: lane teach --help)")
	return nil
}

func init() {
	skillsCmd.Flags().BoolVar(&flagSkillsGlobal, "global", false, "show global-config targets where supported")
	skillsCmd.Flags().BoolVar(&flagSkillsJSON, "json", false, "output as JSON")
	root.AddCommand(skillsCmd)
}
