// Package prompts builds the prompt of a step: header, knowledge block, .lazydev/<step>.md and the
// contract of the step. The contract is built in and comes last, so a project cannot change the
// rules lazydev relies on.
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

	"lazydev/internal/repo"
)

//go:embed contract/*.md
var contracts embed.FS

// Vars are the values of the placeholders $ARGUMENTS, $BASE and $BRANCH.
type Vars struct {
	Arguments, Base, Branch string
}

// Build builds the prompt of step for the project at dir. The prompt comes from .lazydev/ of the
// main folder only, a missing file is an error. Knowledge files must exist in workDir, where
// Claude runs.
func Build(dir, workDir string, cfg repo.Config, step string, v Vars) (string, error) {
	var knowledge []string
	for _, f := range append(slices.Clone(cfg.Knowledge[repo.KnowledgeAll]), cfg.Knowledge[step]...) {
		if _, err := os.Stat(filepath.Join(workDir, f)); err != nil {
			return "", fmt.Errorf("knowledge file missing: %s", f)
		}
		if !slices.Contains(knowledge, f) {
			knowledge = append(knowledge, f)
		}
	}
	body, err := os.ReadFile(repo.PromptPath(dir, step))
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s/%s.md is missing, press i to add it", repo.Dir, step)
	} else if err != nil {
		return "", err
	}
	contract, _ := contracts.ReadFile("contract/" + step + ".md")

	parts := []string{Header(step, v)}
	if len(knowledge) > 0 {
		var b strings.Builder
		b.WriteString("## Pflichtlektüre\n\nLies diese Dateien vollständig, bevor du etwas anderes tust:\n\n")
		for _, f := range knowledge {
			b.WriteString("- `" + f + "`\n")
		}
		parts = append(parts, b.String())
	}
	r := strings.NewReplacer("$ARGUMENTS", v.Arguments, "$BASE", v.Base, "$BRANCH", v.Branch)
	parts = append(parts, r.Replace(stripFrontmatter(string(body))), r.Replace(string(contract)))
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, "\n\n") + "\n", nil
}

// Header is the first line of a prompt. It shows in the log and names the step, e.g.
// "# lazydev implement: prd/auth/token.md". Without arguments it names branch and base branch.
func Header(step string, v Vars) string {
	arg, _, _ := strings.Cut(strings.TrimSpace(v.Arguments), "\n")
	if arg == "" {
		arg = v.Branch + " → " + v.Base
	}
	return fmt.Sprintf("# lazydev %s: %s", step, arg)
}

// stripFrontmatter removes a block from a first line "---" to the next one, so commands from
// .claude/commands/ work as prompts.
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
			return s
		}
		rest = after
	}
}
