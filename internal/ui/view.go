// Package ui renders the berth view control panel.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dheerajnalapat/berth/internal/stack"
	"github.com/dheerajnalapat/berth/internal/traefikapi"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	slugStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	downStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

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
	b.WriteString(titleStyle.Render("⚓ berth — running stacks") + "\n\n")
	if len(stacks) == 0 {
		b.WriteString(dimStyle.Render("  (none — run `berth up` in a project)\n"))
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
		b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(fmt.Sprintf("tilt → http://tilt-%s.localhost (:%d)", s.Slug, s.TiltPort))))
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
