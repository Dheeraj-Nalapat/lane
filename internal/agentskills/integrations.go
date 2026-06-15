package agentskills

import (
	"os"
	"path/filepath"
)

// Write strategies for an integration's content.
const (
	StrategyOwnFile     = "own-file"     // lane owns the file; write/overwrite it
	StrategyAgentsBlock = "agents-block" // merge a marked block into the file
)

// Integration is one agent-harness target lane can install for.
type Integration struct {
	Key                string // "claude", "cursor", "agents"
	Title              string // human label
	Description        string // one-line summary
	Content            string // embedded content (body for agents-block)
	Strategy           string // StrategyOwnFile | StrategyAgentsBlock
	ProjectRel         string // path relative to the project root
	DetectRel          string // path under the project whose existence triggers auto-detect
	SupportsGlobalFile bool   // can be written to a global file (Claude only)
}

// All returns the registry in display order.
func All() []Integration {
	return []Integration{
		{
			Key:                "claude",
			Title:              "Claude Code skill",
			Description:        "Teaches Claude Code to drive lane (skill file).",
			Content:            claudeSkill,
			Strategy:           StrategyOwnFile,
			ProjectRel:         filepath.Join(".claude", "skills", "lane", "SKILL.md"),
			DetectRel:          ".claude",
			SupportsGlobalFile: true,
		},
		{
			Key:                "cursor",
			Title:              "Cursor rule",
			Description:        "Teaches Cursor to drive lane (project rule).",
			Content:            cursorRule,
			Strategy:           StrategyOwnFile,
			ProjectRel:         filepath.Join(".cursor", "rules", "lane.mdc"),
			DetectRel:          ".cursor",
			SupportsGlobalFile: false,
		},
		{
			Key:                "agents",
			Title:              "AGENTS.md section",
			Description:        "Adds a lane section to AGENTS.md (Codex, Copilot, Gemini, …).",
			Content:            agentsSnippet,
			Strategy:           StrategyAgentsBlock,
			ProjectRel:         "AGENTS.md",
			DetectRel:          "AGENTS.md",
			SupportsGlobalFile: false,
		},
	}
}

// Get returns the integration with the given key.
func Get(key string) (Integration, bool) {
	for _, it := range All() {
		if it.Key == key {
			return it, true
		}
	}
	return Integration{}, false
}

// Detect returns, in registry order, the keys of integrations whose detect path
// exists under dir.
func Detect(dir string) []string {
	var keys []string
	for _, it := range All() {
		if _, err := os.Stat(filepath.Join(dir, it.DetectRel)); err == nil {
			keys = append(keys, it.Key)
		}
	}
	return keys
}
