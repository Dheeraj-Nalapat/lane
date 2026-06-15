package agentskills

import "strings"

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
