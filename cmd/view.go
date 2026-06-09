package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
	"github.com/dheeraj-nalapat/lane/internal/ui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var flagPlain bool

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Live control panel of running stacks (interactive on a TTY)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagPlain || !isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Print(snapshot())
			return nil
		}
		_, err := tea.NewProgram(newPanelModel(), tea.WithAltScreen()).Run()
		return err
	},
}

func init() {
	viewCmd.Flags().BoolVar(&flagPlain, "plain", false, "print a static snapshot instead of the interactive panel")
	root.AddCommand(viewCmd)
}

// snapshot is the static, scriptable rendering (also the non-TTY fallback).
func snapshot() string {
	stacks, _ := dockerx.List()
	routers, _ := traefikapi.Default().Routers()
	return ui.Render(stacks, routers)
}
