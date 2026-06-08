package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheerajnalapat/berth/internal/dockerx"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List running berth stacks",
	RunE: func(cmd *cobra.Command, args []string) error {
		stacks, err := dockerx.List()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tURL\tTILT\tSTATE\tPATH")
		for _, s := range stacks {
			state := "stopped"
			if s.Running {
				state = "running"
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", s.Slug, s.URL, s.TiltPort, state, s.ProjectPath)
		}
		return w.Flush()
	},
}

func init() { root.AddCommand(lsCmd) }
