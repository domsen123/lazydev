// Package prompts baut die Prompts der Schritte create, implement, review und merge. Ein Prompt
// besteht aus der Kopfzeile von lazydev, dem Knowledge-Block, dem Prompt des Projekts aus
// .lazydev/<schritt>.md oder dem Standard-Prompt und dem eingebetteten Vertrag des Schritts.
package prompts

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/config"
)

// Step ist ein Schritt mit eigenem Prompt.
type Step string

const (
	// Create ist der Chat zum PRD.
	Create Step = "create"
	// Implement setzt ein PRD im Worktree um.
	Implement Step = "implement"
	// Review prüft und committet die Umsetzung.
	Review Step = "review"
	// Merge bringt den Branch bei einem Konflikt oder roter Prüfung auf den Ziel-Branch.
	Merge Step = "merge"
)

// Steps listet die Schritte in ihrer Reihenfolge.
var Steps = []Step{Create, Implement, Review, Merge}

// SourceDefault ist Prompt.Source, wenn das Projekt keinen eigenen Prompt hat.
const SourceDefault = "Standard"

//go:embed default/*.md contract/*.md
var files embed.FS

// Vars sind die Werte für die Platzhalter. WorkDir ist der Ordner, in dem Claude läuft. Dort
// prüft Build, ob die Knowledge-Dateien existieren.
type Vars struct {
	Arguments string
	Base      string
	Branch    string
	WorkDir   string
}

// Prompt ist der fertige Prompt eines Schritts. Source ist ".lazydev/<schritt>.md" oder
// SourceDefault, Knowledge die Pfade aus dem Knowledge-Block.
type Prompt struct {
	Text      string
	Source    string
	Knowledge []string
}

// Default liefert den eingebetteten Standard-Prompt eines Schritts.
func Default(step Step) string {
	return embedded("default", step)
}

func contract(step Step) string {
	return embedded("contract", step)
}

func embedded(dir string, step Step) string {
	data, err := files.ReadFile(dir + "/" + string(step) + ".md")
	if err != nil {
		panic(fmt.Sprintf("eingebetteter Prompt %s/%s fehlt", dir, step))
	}
	return string(data)
}

// FileName ist der Prompt eines Schritts relativ zum Projekt, z. B. ".lazydev/implement.md".
func FileName(step Step) string {
	return config.RepoDir + "/" + string(step) + ".md"
}

// Build liest den Prompt des Schritts aus .lazydev/ im Hauptordner projectDir, sonst den
// Standard-Prompt, und setzt die vier Teile zusammen. Fehlt eine Knowledge-Datei in v.WorkDir,
// liefert Build einen Fehler.
func Build(projectDir string, repo config.Repo, step Step, v Vars) (Prompt, error) {
	knowledge := Knowledge(repo, step)
	for _, p := range knowledge {
		if _, err := os.Stat(filepath.Join(v.WorkDir, p)); err != nil {
			return Prompt{}, fmt.Errorf("Knowledge-Datei fehlt: %s", p)
		}
	}
	body, source := Default(step), SourceDefault
	data, err := os.ReadFile(filepath.Join(projectDir, FileName(step)))
	switch {
	case err == nil:
		body, source = string(data), FileName(step)
	case !errors.Is(err, fs.ErrNotExist):
		return Prompt{}, err
	}

	r := strings.NewReplacer("$ARGUMENTS", v.Arguments, "$BASE", v.Base, "$BRANCH", v.Branch)
	parts := []string{header(step, v)}
	if len(knowledge) > 0 {
		parts = append(parts, knowledgeBlock(knowledge))
	}
	parts = append(parts, r.Replace(stripFrontmatter(body)), r.Replace(contract(step)))
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return Prompt{Text: strings.Join(parts, "\n\n") + "\n", Source: source, Knowledge: knowledge}, nil
}

// header ist die erste Zeile des Prompts. Sie steht im Log und nennt den Schritt, z. B.
// "# lazydev implement: prd/auth/token.md". Ohne Argument nennt sie Branch und Ziel-Branch.
func header(step Step, v Vars) string {
	arg, _, _ := strings.Cut(strings.TrimSpace(v.Arguments), "\n")
	if arg == "" {
		arg = v.Branch + " → " + v.Base
	}
	return fmt.Sprintf("# lazydev %s: %s", step, arg)
}

// Knowledge liefert die Knowledge-Dateien eines Schritts: erst all, dann der Schritt, ohne
// doppelte Pfade.
func Knowledge(repo config.Repo, step Step) []string {
	var paths []string
	for _, p := range append(slices.Clone(repo.Knowledge[config.KnowledgeAll]), repo.Knowledge[string(step)]...) {
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	return paths
}

func knowledgeBlock(paths []string) string {
	var b strings.Builder
	b.WriteString("## Pflichtlektüre\n\nLies diese Dateien vollständig, bevor du etwas anderes tust:\n\n")
	for _, p := range paths {
		b.WriteString("- `" + p + "`\n")
	}
	return b.String()
}

// stripFrontmatter entfernt einen Block von der ersten Zeile "---" bis zur nächsten. So lassen
// sich Commands aus .claude/commands/ übernehmen.
func stripFrontmatter(s string) string {
	first, rest, ok := strings.Cut(s, "\n")
	if !ok || strings.TrimSpace(first) != "---" {
		return s
	}
	for {
		line, after, found := strings.Cut(rest, "\n")
		if strings.TrimSpace(line) == "---" {
			return after
		}
		if !found {
			// Ohne Ende ist es keine Frontmatter.
			return s
		}
		rest = after
	}
}

// Init legt .lazydev/ im Hauptordner projectDir an: config.yaml aus repo und die vier
// Standard-Prompts, damit das Projekt sie anpassen kann. Existiert .lazydev/, schreibt Init nichts.
func Init(projectDir string, repo config.Repo) error {
	dir := filepath.Join(projectDir, config.RepoDir)
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("%s/ existiert schon", config.RepoDir)
	}
	data, err := repo.Marshal()
	if err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(config.RepoPath(projectDir), data, 0o644); err != nil {
		return err
	}
	for _, step := range Steps {
		if err := os.WriteFile(filepath.Join(projectDir, FileName(step)), []byte(Default(step)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// InitFiles listet die Dateien, die Init anlegt, relativ zum Projekt.
func InitFiles() []string {
	out := []string{config.RepoDir + "/config.yaml"}
	for _, step := range Steps {
		out = append(out, FileName(step))
	}
	return out
}
