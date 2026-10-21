package claude

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"lazydev/internal/state"
)

// agentFiles enthält die Agenten, die die eingebetteten Prompts aufrufen, z. B. committer.
//
//go:embed agents/*.md
var agentFiles embed.FS

// Agent ist ein Eintrag im JSON für claude --agents.
type Agent struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// Agents liefert die eingebetteten Agenten nach Name. Der Name steht in der Frontmatter.
func Agents() (map[string]Agent, error) {
	entries, err := agentFiles.ReadDir("agents")
	if err != nil {
		return nil, err
	}
	agents := map[string]Agent{}
	for _, e := range entries {
		data, err := agentFiles.ReadFile(path.Join("agents", e.Name()))
		if err != nil {
			return nil, err
		}
		name, a, err := parseAgent(string(data))
		if err != nil {
			return nil, fmt.Errorf("Agent %s: %w", e.Name(), err)
		}
		agents[name] = a
	}
	return agents, nil
}

// parseAgent liest Frontmatter und Inhalt einer Agent-Datei. Die Frontmatter ist kein gültiges
// YAML, weil description unquotiert ": " enthalten darf. Deshalb gilt je Zeile: Schlüssel bis zum
// ersten ":", Rest als Wert.
func parseAgent(text string) (string, Agent, error) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return "", Agent{}, fmt.Errorf("Frontmatter fehlt")
	}
	front, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return "", Agent{}, fmt.Errorf("Frontmatter ohne Ende")
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
		return "", Agent{}, fmt.Errorf("name, description oder Inhalt fehlt")
	}
	return name, a, nil
}

// AgentsPath ist die Datei für claude --agents im Zustandsverzeichnis.
func AgentsPath() string {
	return filepath.Join(state.Dir(), "agents.json")
}

// WriteAgents schreibt die eingebetteten Agenten nach AgentsPath. Mehrere Prozesse schreiben
// dieselbe Datei, deshalb über eine temporäre Datei und rename.
func WriteAgents() error {
	agents, err := Agents()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return err
	}
	p := AgentsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "agents-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}
