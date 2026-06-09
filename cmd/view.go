package cmd

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
	"github.com/dheeraj-nalapat/lane/internal/ui"
	"github.com/spf13/cobra"
)

var flagWatch bool

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Rich control panel of running stacks and live routing",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagWatch {
			fmt.Print(snapshot())
			return nil
		}
		_, err := tea.NewProgram(model{}).Run()
		return err
	},
}

func init() {
	viewCmd.Flags().BoolVar(&flagWatch, "watch", false, "live-refresh the view")
	root.AddCommand(viewCmd)
}

func snapshot() string {
	stacks, _ := dockerx.List()
	routers, _ := traefikapi.Default().Routers()
	return ui.Render(stacks, routers)
}

type tick struct{}
type model struct{ body string }

func (m model) Init() tea.Cmd { return tea.Batch(refresh, tea.EnterAltScreen) }

func refresh() tea.Msg { return tick{} }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tick:
		m.body = snapshot()
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tick{} })
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string { return m.body + "\n(press any key to quit)\n" }
