package claude

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAgents(t *testing.T) {
	agents, err := Agents()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"committer", "structural-completeness-reviewer"} {
		a, ok := agents[name]
		if !ok {
			t.Fatalf("Agent %s fehlt: %v", name, agents)
		}
		if strings.HasPrefix(a.Prompt, "---") || strings.Contains(a.Prompt, "\nname: ") || a.Description == "" {
			t.Errorf("Frontmatter von %s nicht entfernt: %+v", name, a)
		}
	}
	if a := agents["structural-completeness-reviewer"]; a.Model != "sonnet" || strings.Contains(a.Description, `\n`) {
		t.Errorf("Reviewer: Modell %q, Beschreibung %q", a.Model, a.Description)
	}
	if a := agents["committer"]; strings.Join(a.Tools, ",") != "Bash,Read,Grep,Glob" || a.Model != "haiku" {
		t.Errorf("Committer: %+v", a)
	}
}

func TestWriteAgents(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := WriteAgents(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(AgentsPath())
	if err != nil {
		t.Fatal(err)
	}
	var agents map[string]Agent
	if err := json.Unmarshal(data, &agents); err != nil || agents["committer"].Prompt == "" {
		t.Errorf("agents.json: %v\n%s", err, data)
	}
}

func TestParseAgentNeedsFrontmatter(t *testing.T) {
	for _, text := range []string{"kein Frontmatter", "---\nname: x\n", "---\nname: x\n---\nPrompt\n"} {
		if _, _, err := parseAgent(text); err == nil {
			t.Errorf("parseAgent(%q) ohne Fehler", text)
		}
	}
}
