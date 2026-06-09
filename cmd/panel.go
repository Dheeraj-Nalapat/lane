package cmd

import (
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/proxy"
	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/dheeraj-nalapat/lane/internal/tlsx"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
	"github.com/dheeraj-nalapat/lane/internal/ui"
)

type panelTick struct{}
type loaded struct {
	stacks  []stack.Stack
	routers []traefikapi.Router
	err     error
}
type execDone struct{ err error }

type panelModel struct {
	st      ui.PanelState
	selSlug string // remembered selection across refreshes
}

func newPanelModel() panelModel { return panelModel{} }

func loadCmd() tea.Msg {
	stacks, err := dockerx.List()
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Slug < stacks[j].Slug })
	routers, _ := traefikapi.Default().Routers()
	return loaded{stacks: stacks, routers: routers, err: err}
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return panelTick{} })
}

func (m panelModel) Init() tea.Cmd { return tea.Batch(loadCmd, tickCmd()) }

// self returns the path to this binary (for shelling out lane subcommands).
func self() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return "lane"
}

func actionArgs(verb, slug string) []string { return []string{verb, "--slug", slug} }

func (m *panelModel) exec(verb string) tea.Cmd {
	if len(m.st.Stacks) == 0 {
		return nil
	}
	slug := m.st.Stacks[m.st.Selected].Slug
	c := exec.Command(self(), actionArgs(verb, slug)...)
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDone{err} })
}

func openURL(url string) {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	_ = exec.Command(opener, url).Start()
}

func (m *panelModel) rememberSel() {
	if len(m.st.Stacks) > 0 {
		m.selSlug = m.st.Stacks[m.st.Selected].Slug
	}
}

func (m panelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loaded:
		m.st.Stacks = msg.stacks
		m.st.Routers = msg.routers
		m.st.ProxyUp = proxy.Running()
		m.st.TLSOn = tlsx.Enabled()
		if msg.err != nil {
			m.st.Note = "refresh failed (retrying)"
		} else {
			m.st.Note = ""
		}
		if m.selSlug == "" && len(m.st.Stacks) > 0 {
			m.selSlug = m.st.Stacks[0].Slug
		}
		m.st.Selected = ui.Reselect(m.selSlug, m.st.Stacks)
		return m, nil

	case panelTick:
		return m, tea.Batch(loadCmd, tickCmd())

	case execDone:
		return m, loadCmd

	case tea.WindowSizeMsg:
		m.st.Width = msg.Width
		return m, nil

	case tea.KeyMsg:
		if m.st.Confirm != "" {
			if msg.String() == "y" {
				slug := m.st.Confirm
				m.st.Confirm = ""
				c := exec.Command(self(), actionArgs("down", slug)...)
				return m, tea.ExecProcess(c, func(err error) tea.Msg { return execDone{err} })
			}
			m.st.Confirm = ""
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.st.Selected > 0 {
				m.st.Selected--
			}
			m.rememberSel()
		case "down", "j":
			if m.st.Selected < len(m.st.Stacks)-1 {
				m.st.Selected++
			}
			m.rememberSel()
		case "o":
			if len(m.st.Stacks) > 0 {
				openURL(m.st.Stacks[m.st.Selected].URL)
			}
		case "l":
			return m, m.exec("logs")
		case "r":
			return m, m.exec("restart")
		case "x":
			if len(m.st.Stacks) > 0 {
				m.st.Confirm = m.st.Stacks[m.st.Selected].Slug
			}
		}
		return m, nil
	}
	return m, nil
}

func (m panelModel) View() string { return ui.RenderPanel(m.st) }
