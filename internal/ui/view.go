// Package ui renders the lane view control panel.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	slugStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	downStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	logoStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	badStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

const logo = `██╗      █████╗ ███╗   ██╗███████╗
██║     ██╔══██╗████╗  ██║██╔════╝
██║     ███████║██╔██╗ ██║█████╗
██║     ██╔══██║██║╚██╗██║██╔══╝
███████╗██║  ██║██║ ╚████║███████╗
╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝`

// Logo returns the block "LANE" banner (uncolored).
func Logo() string { return logo }

// PanelState is the full input to RenderPanel (pure; no I/O).
type PanelState struct {
	Stacks   []stack.Stack
	Routers  []traefikapi.Router
	Selected int
	ProxyUp  bool
	TLSOn    bool
	Confirm  string // non-empty → render the down-confirm footer for this slug
	Width    int
	Note     string // transient status line (e.g. "refresh failed")
}

// Reselect returns the index of slug in stacks, or 0 (clamped) if absent.
func Reselect(slug string, stacks []stack.Stack) int {
	for i, s := range stacks {
		if s.Slug == slug {
			return i
		}
	}
	return 0
}

func dot(on bool) string {
	if on {
		return okStyle.Render("●")
	}
	return dimStyle.Render("○")
}

func clamp(i, n int) int {
	if n == 0 || i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// RenderPanel renders the interactive master/detail control panel.
func RenderPanel(st PanelState) string {
	var b strings.Builder
	b.WriteString(logoStyle.Render(logo) + "\n")
	b.WriteString(fmt.Sprintf("🏁  parallel dev stacks      proxy %s   tls %s\n",
		dot(st.ProxyUp), dot(st.TLSOn)))
	b.WriteString(strings.Repeat("─", 60) + "\n")

	if len(st.Stacks) == 0 {
		b.WriteString(dimStyle.Render("  (no stacks — run `lane up` in a project)\n"))
		return b.String()
	}

	routesBySlug := map[string][]traefikapi.Router{}
	for _, r := range st.Routers {
		name := strings.SplitN(r.Name, "@", 2)[0]
		if i := strings.LastIndexByte(name, '-'); i > 0 {
			routesBySlug[name[:i]] = append(routesBySlug[name[:i]], r)
		}
	}

	var left strings.Builder
	for i, s := range st.Stacks {
		marker, label := "  ", s.Slug
		if i == st.Selected {
			marker = "▸ "
			label = selStyle.Render(s.Slug)
		}
		state := okStyle.Render("● running")
		if !s.Running {
			state = badStyle.Render("○ stopped")
		}
		fmt.Fprintf(&left, "%s%-14s %s\n", marker, label, state)
	}

	sel := st.Stacks[clamp(st.Selected, len(st.Stacks))]
	var right strings.Builder
	fmt.Fprintf(&right, "%s\n", selStyle.Render(sel.Slug))
	fmt.Fprintf(&right, "%s\n", dimStyle.Render(sel.ProjectPath))
	if sel.URL != "" {
		fmt.Fprintf(&right, "%s\n", sel.URL)
	}
	if sel.TiltPort > 0 {
		fmt.Fprintf(&right, "%s\n", dimStyle.Render(fmt.Sprintf("tilt → http://tilt-%s.localhost (:%d)", sel.Slug, sel.TiltPort)))
	}
	for _, r := range routesBySlug[sel.Slug] {
		mark := okStyle.Render("✓")
		if r.Status != "enabled" {
			mark = badStyle.Render("✗")
		}
		fmt.Fprintf(&right, "  %s %s → %s\n", mark, r.Rule, r.Service)
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(34).Render(left.String()),
		dimStyle.Render("│ "),
		right.String(),
	) + "\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")

	if st.Confirm != "" {
		b.WriteString(badStyle.Render(fmt.Sprintf(` down "%s"? (y/n)`, st.Confirm)) + "\n")
	} else {
		b.WriteString(dimStyle.Render(" ↑/↓ select  o open  l logs  r restart  x down  q quit") + "\n")
	}
	if st.Note != "" {
		b.WriteString(dimStyle.Render(" "+st.Note) + "\n")
	}
	return b.String()
}

// Render builds the static view string from stacks and live Traefik routers.
func Render(stacks []stack.Stack, routers []traefikapi.Router) string {
	routesBySlug := map[string][]traefikapi.Router{}
	for _, r := range routers {
		// router name like "<slug>-<svc>@docker"
		name := strings.SplitN(r.Name, "@", 2)[0]
		if i := strings.LastIndexByte(name, '-'); i > 0 {
			routesBySlug[name[:i]] = append(routesBySlug[name[:i]], r)
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("🛣  lane — running stacks") + "\n\n")
	if len(stacks) == 0 {
		b.WriteString(dimStyle.Render("  (none — run `lane up` in a project)\n"))
		return b.String()
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Slug < stacks[j].Slug })
	for _, s := range stacks {
		state := slugStyle.Render(s.Slug)
		if !s.Running {
			state = downStyle.Render(s.Slug + " (stopped)")
		}
		b.WriteString(state + "  " + dimStyle.Render(s.ProjectPath) + "\n")
		b.WriteString("  " + s.URL + "\n")
		if s.TiltPort > 0 {
			b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(fmt.Sprintf("tilt → http://tilt-%s.localhost (:%d)", s.Slug, s.TiltPort))))
		}
		for _, r := range routesBySlug[s.Slug] {
			mark := "✓"
			if r.Status != "enabled" {
				mark = "✗"
			}
			b.WriteString(fmt.Sprintf("    %s %s → %s\n", mark, r.Rule, r.Service))
		}
		b.WriteString("\n")
	}
	return b.String()
}
