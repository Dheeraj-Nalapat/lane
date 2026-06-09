package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [path]",
	Short: "Recreate a stack (down then up)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Best-effort teardown (a not-running stack is fine), then bring up fresh.
		if err := runDown(cmd, args); err != nil {
			fmt.Fprintln(os.Stderr, "lane: restart: down step:", err)
		}
		return runUp(cmd, args)
	},
}

func init() { root.AddCommand(restartCmd) }
