package claude

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"lazydev/internal/fsutil"
	"lazydev/internal/lang"
	"lazydev/internal/state"
)

// agents/<lang>/ holds the agents the prompts call, e.g. committer, per language with the same
// names. They are built in, nothing comes from ~/.claude.
//
//go:embed agents/en/*.md agents/de/*.md
var agentFiles embed.FS

// Agent is an entry of the JSON for claude --agents.
type Agent struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// Agents returns the built-in agents in language l by name.
func Agents(l lang.Lang) (map[string]Agent, error) {
	dir := path.Join("agents", string(l))
	entries, err := agentFiles.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	agents := map[string]Agent{}
	for _, e := range entries {
		data, err := agentFiles.ReadFile(path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		name, a, err := parseAgent(string(data))
		if err != nil {
			return nil, fmt.Errorf("agent %s: %w", e.Name(), err)
		}
		agents[name] = a
	}
	return agents, nil
}

// AgentsJSON returns the built-in agents in language l as one line of JSON. An interactive claude
// takes --agents inline only, a file path works with -p alone.
func AgentsJSON(l lang.Lang) (string, error) {
	agents, err := Agents(l)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(agents)
	return string(data), err
}

// AgentsPath is the file WriteAgents writes for language l: agents.json for the default language,
// agents.<lang>.json for the others, so runners of projects in different languages do not overwrite
// each other's agents.
func AgentsPath(l lang.Lang) string {
	if l == lang.Default || l == "" {
		return filepath.Join(state.Dir(), "agents.json")
	}
	return filepath.Join(state.Dir(), "agents."+string(l)+".json")
}

// WriteAgents writes the built-in agents in language l to AgentsPath and returns its path. Several
// runners write the same file, so it goes through a temporary file.
func WriteAgents(l lang.Lang) (string, error) {
	agents, err := Agents(l)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return "", err
	}
	p := AgentsPath(l)
	return p, fsutil.WriteAtomic(p, data, 0o644)
}

// parseAgent reads frontmatter and body of an agent file. The frontmatter is no valid YAML,
// because a description may contain ": ", so each line splits at the first ":".
func parseAgent(text string) (string, Agent, error) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return "", Agent{}, errors.New("frontmatter missing")
	}
	front, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return "", Agent{}, errors.New("frontmatter without end")
	}
	var name string
	a := Agent{Prompt: strings.TrimSpace(body)}
	for line := range strings.SplitSeq(front, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			a.Description = strings.ReplaceAll(value, `\n`, "\n")
		case "model":
			a.Model = value
		case "tools":
			for t := range strings.SplitSeq(value, ",") {
				if t = strings.TrimSpace(t); t != "" {
					a.Tools = append(a.Tools, t)
				}
			}
		}
	}
	if name == "" || a.Description == "" || a.Prompt == "" {
		return "", Agent{}, errors.New("name, description or body missing")
	}
	return name, a, nil
}
