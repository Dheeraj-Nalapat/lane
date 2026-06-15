package agentskills

import _ "embed"

//go:embed content/claude/SKILL.md
var claudeSkill string

//go:embed content/cursor/lane.mdc
var cursorRule string

//go:embed content/agents/AGENTS.snippet.md
var agentsSnippet string
