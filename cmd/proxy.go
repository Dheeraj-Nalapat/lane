package cmd

import (
	"fmt"

	"github.com/dheerajnalapat/berth/internal/proxy"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy [up|down|status]",
	Short: "Manage the shared Traefik proxy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "up":
			if err := proxy.Up(); err != nil {
				return err
			}
			fmt.Println("berth proxy is up (http://*.localhost → your stacks)")
			return nil
		case "down":
			return proxy.Down()
		case "status":
			if proxy.Running() {
				fmt.Println("running")
			} else {
				fmt.Println("stopped")
			}
			return nil
		default:
			return fmt.Errorf("unknown subcommand %q (use up|down|status)", args[0])
		}
	},
}

func init() { root.AddCommand(proxyCmd) }
