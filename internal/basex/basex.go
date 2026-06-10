// Package basex resolves the base stack a worktree borrows from and computes
// which services are borrowed vs run fresh.
package basex

import (
	"fmt"
	"sort"

	"github.com/dheeraj-nalapat/lane/internal/stack"
)

// FindBase returns the slug of the running base stack for project, excluding the
// caller's own slug. Prefers the canonical base (slug == project). Errors when
// the caller is the base, when none is running, or when multiple non-canonical
// candidates exist.
func FindBase(stacks []stack.Stack, project, ownSlug string) (string, error) {
	if ownSlug == project {
		return "", fmt.Errorf("this is the base stack; --base is for worktrees")
	}
	var cands []string
	for _, s := range stacks {
		if s.Project != project || s.Slug == ownSlug || !s.Running {
			continue
		}
		if s.Slug == project {
			return s.Slug, nil // canonical base wins
		}
		cands = append(cands, s.Slug)
	}
	sort.Strings(cands)
	switch len(cands) {
	case 0:
		return "", fmt.Errorf("no running base for %q; start it from the main checkout with `lane up`", project)
	case 1:
		return cands[0], nil
	default:
		return "", fmt.Errorf("multiple candidate base stacks for %q: %v (explicit base selection is not yet supported)", project, cands)
	}
}

// Borrowed returns the base services not in the fresh set, sorted.
func Borrowed(baseServices, fresh []string) []string {
	fset := map[string]bool{}
	for _, f := range fresh {
		fset[f] = true
	}
	var b []string
	for _, s := range baseServices {
		if !fset[s] {
			b = append(b, s)
		}
	}
	sort.Strings(b)
	return b
}
