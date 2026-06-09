// Package slug derives DNS/Docker-safe stack identities.
package slug

import (
	"regexp"
	"strings"
)

const maxLen = 40

var (
	nonSafe   = regexp.MustCompile(`[^a-z0-9-]+`)
	multiDash = regexp.MustCompile(`-{2,}`)
)

// Sanitize lowercases s and reduces it to a safe DNS label: [a-z0-9-], no
// repeated/edge dashes, capped at 40 chars.
func Sanitize(s string) string {
	s = strings.ToLower(s)
	s = nonSafe.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > maxLen {
		s = strings.Trim(s[:maxLen], "-")
	}
	return s
}

// Derive joins a base name with an optional worktree suffix, then sanitizes.
func Derive(base, worktree string) string {
	if worktree != "" {
		base = base + "-" + worktree
	}
	return Sanitize(base)
}

// Inputs feeds the resolution ladder.
type Inputs struct {
	Flag         string // --slug
	Env          string // LANE_SLUG
	ManifestName string // .lane.toml name
	Worktree     string // linked worktree name, "" if main
	DirBase      string // basename of project dir (fallback)
}

// Resolve applies the precedence ladder: flag > env > manifest(+worktree) > dir(+worktree).
func Resolve(in Inputs) string {
	switch {
	case in.Flag != "":
		return Sanitize(in.Flag)
	case in.Env != "":
		return Sanitize(in.Env)
	case in.ManifestName != "":
		return Derive(in.ManifestName, in.Worktree)
	default:
		return Derive(in.DirBase, in.Worktree)
	}
}
