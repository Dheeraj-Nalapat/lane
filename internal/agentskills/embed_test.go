package agentskills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedContentNonEmpty(t *testing.T) {
	for name, s := range map[string]string{
		"claudeSkill":   claudeSkill,
		"cursorRule":    cursorRule,
		"agentsSnippet": agentsSnippet,
	} {
		if strings.TrimSpace(s) == "" {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestClaudeSkillHasFrontmatter(t *testing.T) {
	if !strings.Contains(claudeSkill, "name:") || !strings.Contains(claudeSkill, "description:") {
		t.Error("claude skill missing name:/description: frontmatter")
	}
}

func TestMirrorParity(t *testing.T) {
	// Embedded copies must match the published agent/ tree exactly.
	cases := map[string]string{
		filepath.Join("..", "..", "agent", "claude", "skills", "lane", "SKILL.md"): claudeSkill,
		filepath.Join("..", "..", "agent", "cursor", "lane.mdc"):                   cursorRule,
		filepath.Join("..", "..", "agent", "agents", "AGENTS.snippet.md"):          agentsSnippet,
	}
	for path, embedded := range cases {
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(want) != embedded {
			t.Errorf("embedded content drifted from %s", path)
		}
	}
}
