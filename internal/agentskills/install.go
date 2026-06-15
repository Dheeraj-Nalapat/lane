package agentskills

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	agentsStart = "<!-- lane:start -->"
	agentsEnd   = "<!-- lane:end -->"
)

// renderAgentsBlock wraps body in the lane markers.
func renderAgentsBlock(body string) string {
	return agentsStart + "\n" + strings.TrimSpace(body) + "\n" + agentsEnd
}

// mergeAgents returns AGENTS.md content with the lane block created, appended,
// or replaced in place, plus whether the content changed.
func mergeAgents(existing, body string) (string, bool) {
	block := renderAgentsBlock(body)
	si := strings.Index(existing, agentsStart)
	ei := strings.Index(existing, agentsEnd)
	if si >= 0 && ei > si {
		merged := existing[:si] + block + existing[ei+len(agentsEnd):]
		return merged, merged != existing
	}
	if strings.TrimSpace(existing) == "" {
		return block + "\n", true
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n", true
}

// Status is the outcome of applying (or planning) one integration.
type Status string

const (
	StatusCreated   Status = "created"
	StatusUpdated   Status = "updated"
	StatusUnchanged Status = "unchanged"
	StatusSkipped   Status = "skipped"
)

// Scope is where an integration is installed.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Result reports what happened (or, with dryRun, would happen) for one integration.
type Result struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Target  string `json:"target"`
	Scope   Scope  `json:"scope"`
	Status  Status `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Content string `json:"-"` // populated for manual (paste) targets; not serialized
}

// Apply installs one integration under projectDir (or global config where
// supported). With dryRun, it computes the status without touching disk.
func Apply(it Integration, projectDir string, global, dryRun bool) (Result, error) {
	// Cursor's global rules are UI-only — there is no file to write.
	if global && it.Key == "cursor" {
		return Result{
			Key:     it.Key,
			Title:   it.Title,
			Scope:   ScopeGlobal,
			Target:  "Cursor Settings → Rules",
			Status:  StatusSkipped,
			Reason:  "manual: paste into Cursor Settings → Rules",
			Content: it.Content,
		}, nil
	}

	scope := ScopeProject
	path := filepath.Join(projectDir, it.ProjectRel)
	if global && it.SupportsGlobalFile {
		home, err := os.UserHomeDir()
		if err != nil {
			return Result{}, err
		}
		path = filepath.Join(home, it.ProjectRel)
		scope = ScopeGlobal
	}

	res := Result{Key: it.Key, Title: it.Title, Target: path, Scope: scope}

	var status Status
	var err error
	if it.Strategy == StrategyAgentsBlock {
		status, err = applyAgents(path, it.Content, dryRun)
	} else {
		status, err = applyOwnFile(path, it.Content, dryRun)
	}
	if err != nil {
		return Result{}, err
	}
	res.Status = status
	return res, nil
}

func applyOwnFile(path, content string, dryRun bool) (Status, error) {
	old, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if !dryRun {
			if err := writeFile(path, content); err != nil {
				return "", err
			}
		}
		return StatusCreated, nil
	case err != nil:
		return "", err
	}
	if string(old) == content {
		return StatusUnchanged, nil
	}
	if !dryRun {
		if err := writeFile(path, content); err != nil {
			return "", err
		}
	}
	return StatusUpdated, nil
}

func applyAgents(path, body string, dryRun bool) (Status, error) {
	old, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	merged, changed := mergeAgents(string(old), body)
	if !existed {
		if !dryRun {
			if err := writeFile(path, merged); err != nil {
				return "", err
			}
		}
		return StatusCreated, nil
	}
	if !changed {
		return StatusUnchanged, nil
	}
	if !dryRun {
		if err := writeFile(path, merged); err != nil {
			return "", err
		}
	}
	return StatusUpdated, nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
