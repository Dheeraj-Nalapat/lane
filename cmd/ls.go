package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/spf13/cobra"
)

var flagLsJSON bool

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List running lane stacks",
	RunE: func(cmd *cobra.Command, args []string) error {
		stacks, err := dockerx.List()
		if err != nil {
			return err
		}
		if flagLsJSON {
			if stacks == nil {
				stacks = []stack.Stack{} // marshal [] not null
			}
			b, err := json.MarshalIndent(stacks, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tURL\tTILT\tSTATE\tPATH")
		for _, s := range stacks {
			state := "stopped"
			if s.Running {
				state = "running"
			}
			tilt := "-"
			if s.TiltPort > 0 {
				tilt = fmt.Sprintf("%d", s.TiltPort)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Slug, s.URL, tilt, state, s.ProjectPath)
		}
		return w.Flush()
	},
}

func init() {
	lsCmd.Flags().BoolVar(&flagLsJSON, "json", false, "output as JSON")
	root.AddCommand(lsCmd)
}
