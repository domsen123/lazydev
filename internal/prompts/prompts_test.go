package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/claude"
	"lazydev/internal/config"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDefaultParts(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "CLAUDE.md", "x")
	repo := config.Repo{Knowledge: map[string][]string{"all": {"CLAUDE.md"}}}
	p, err := Build(dir, repo, Implement, Vars{Arguments: "prd/auth/token.md", Base: "next", Branch: "feat/token", WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != SourceDefault || claude.FirstLine(p.Text) != "# lazydev implement: prd/auth/token.md" {
		t.Errorf("Quelle %q, Kopfzeile %q", p.Source, claude.FirstLine(p.Text))
	}
	parts := strings.Split(p.Text, "\n\n## Pflichtlektüre")
	if len(parts) != 2 || parts[0] != "# lazydev implement: prd/auth/token.md" {
		t.Fatalf("Knowledge-Block folgt nicht auf die Kopfzeile:\n%s", p.Text)
	}
	knowledge := strings.Index(p.Text, "## Pflichtlektüre")
	body := strings.Index(p.Text, "# PRD umsetzen")
	rules := strings.LastIndex(p.Text, "## Regeln von lazydev")
	if knowledge < 0 || body < knowledge || rules < body || strings.Count(p.Text, "## Regeln von lazydev") != 1 {
		t.Errorf("Reihenfolge der Teile:\n%s", p.Text)
	}
	if !strings.Contains(p.Text, "Lies diese Dateien vollständig, bevor du etwas anderes tust:\n\n- `CLAUDE.md`\n") {
		t.Errorf("Knowledge-Block:\n%s", p.Text)
	}
	for _, want := range []string{"Setze das PRD `prd/auth/token.md` vollständig um", "Branch `feat/token` heißt", "## Ergebnis"} {
		if !strings.Contains(p.Text, want) {
			t.Errorf("Prompt ohne %q", want)
		}
	}
	if strings.ContainsAny(p.Text, "$") {
		t.Errorf("Platzhalter übrig:\n%s", p.Text)
	}
}

func TestBuildEveryStepEndsWithContract(t *testing.T) {
	dir := t.TempDir()
	for _, step := range Steps {
		for _, project := range []bool{false, true} {
			if project {
				write(t, dir, FileName(step), "Eigener Ablauf für "+string(step)+"\n")
			}
			p, err := Build(dir, config.Repo{}, step, Vars{Arguments: "a", Base: "main", Branch: "feat/x", WorkDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			rules := strings.LastIndex(p.Text, "## Regeln von lazydev")
			if rules < 0 || strings.Contains(p.Text[rules:], "Eigener Ablauf") || strings.Contains(p.Text, "## Pflichtlektüre") {
				t.Errorf("%s (Projekt-Prompt %v) endet nicht mit dem Vertrag:\n%s", step, project, p.Text)
			}
			if !strings.HasPrefix(p.Text, "# lazydev "+string(step)+": ") {
				t.Errorf("%s: Kopfzeile %q", step, claude.FirstLine(p.Text))
			}
		}
	}
}

func TestBuildProjectPromptAndPlaceholders(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".lazydev/review.md", "---\ndescription: Review\nallowed-tools: Bash\n---\n# Review im Team\nPrüfe $BRANCH gegen $BASE.\n")
	p, err := Build(dir, config.Repo{}, Review, Vars{Base: "next", Branch: "feat/token", WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != ".lazydev/review.md" || claude.FirstLine(p.Text) != "# lazydev review: feat/token → next" {
		t.Errorf("Quelle %q, Kopfzeile %q", p.Source, claude.FirstLine(p.Text))
	}
	if !strings.Contains(p.Text, "\n\n# Review im Team\nPrüfe feat/token gegen next.\n\n## Regeln von lazydev") ||
		strings.Contains(p.Text, "description:") || strings.Contains(p.Text, "structural-completeness-reviewer") ||
		!strings.Contains(p.Text, "`git diff next...HEAD`") {
		t.Errorf("Projekt-Prompt:\n%s", p.Text)
	}

	// Merge bekommt Branch und Fehlerkontext als $ARGUMENTS.
	p, _ = Build(dir, config.Repo{}, Merge, Vars{Arguments: "feat/token Prüfung rot", Base: "next", Branch: "feat/token", WorkDir: dir})
	if claude.FirstLine(p.Text) != "# lazydev merge: feat/token Prüfung rot" || !strings.Contains(p.Text, "Branch aus `feat/token Prüfung rot`") ||
		strings.Contains(p.Text, "Drizzle") || !strings.Contains(p.Text, "Migrationen behandelst du nach den Regeln aus") {
		t.Errorf("Merge:\n%s", p.Text)
	}

	// Create zeigt nur die erste Zeile der Beschreibung im Kopf.
	p, _ = Build(dir, config.Repo{}, Create, Vars{Arguments: "#12 Avatar\n\nText des Issues", Base: "main", WorkDir: dir})
	if claude.FirstLine(p.Text) != "# lazydev create: #12 Avatar" || !strings.Contains(p.Text, "\n#12 Avatar\n\nText des Issues\n") {
		t.Errorf("Create:\n%s", p.Text)
	}
}

func TestStripFrontmatter(t *testing.T) {
	for in, want := range map[string]string{
		"---\na: b\n---\nText\n": "Text\n",
		"Text\n---\nmehr\n":      "Text\n---\nmehr\n",
		"---\nohne Ende\n":       "---\nohne Ende\n",
	} {
		if got := stripFrontmatter(in); got != want {
			t.Errorf("stripFrontmatter(%q) = %q, erwartet %q", in, got, want)
		}
	}
}

func TestKnowledgeOrderAndMissingFile(t *testing.T) {
	repo := config.Repo{Knowledge: map[string][]string{
		"all":       {"CLAUDE.md", "docs/a.md"},
		"implement": {"docs/b.md", "CLAUDE.md"},
		"review":    {"docs/c.md"},
	}}
	if got := strings.Join(Knowledge(repo, Implement), ","); got != "CLAUDE.md,docs/a.md,docs/b.md" {
		t.Errorf("implement: %s", got)
	}
	if got := strings.Join(Knowledge(repo, Create), ","); got != "CLAUDE.md,docs/a.md" {
		t.Errorf("create: %s", got)
	}

	dir, wt := t.TempDir(), t.TempDir()
	write(t, wt, "CLAUDE.md", "x")
	write(t, wt, "docs/a.md", "x")
	// Im Hauptordner vorhanden, im Arbeitsordner nicht: Build prüft den Arbeitsordner.
	write(t, dir, "docs/b.md", "x")
	_, err := Build(dir, repo, Implement, Vars{Arguments: "prd/x.md", WorkDir: wt})
	if err == nil || err.Error() != "Knowledge-Datei fehlt: docs/b.md" {
		t.Errorf("Fehler: %v", err)
	}
	p, err := Build(dir, repo, Create, Vars{Arguments: "x", WorkDir: wt})
	if err != nil || strings.Join(p.Knowledge, ",") != "CLAUDE.md,docs/a.md" {
		t.Errorf("create: %+v, %v", p.Knowledge, err)
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	repo := config.Repo{BaseBranch: "next", Knowledge: map[string][]string{"all": {"CLAUDE.md"}}}
	if err := Init(dir, repo); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := config.LoadRepo(dir)
	if err != nil || !exists || loaded.Base() != "next" || loaded.Knowledge["all"][0] != "CLAUDE.md" {
		t.Errorf("config.yaml: %+v, %v, %v", loaded, exists, err)
	}
	for _, step := range Steps {
		data, err := os.ReadFile(filepath.Join(dir, FileName(step)))
		if err != nil || string(data) != Default(step) {
			t.Errorf("%s: %v", FileName(step), err)
		}
	}
	if len(InitFiles()) != 5 {
		t.Errorf("InitFiles: %v", InitFiles())
	}

	// Ein vorhandener Ordner bleibt unverändert.
	other := t.TempDir()
	write(t, other, ".lazydev/notiz.txt", "x")
	if err := Init(other, repo); err == nil || !strings.Contains(err.Error(), "existiert schon") {
		t.Errorf("Init bei vorhandenem Ordner: %v", err)
	}
	if _, err := os.Stat(config.RepoPath(other)); err == nil {
		t.Error("Init schrieb in einen vorhandenen Ordner")
	}
}

func TestStateOf(t *testing.T) {
	dir := t.TempDir()
	if s, err := StateOf(dir, Review); err != nil || s != StateDefault || Label(s) != "Standard von lazydev" {
		t.Errorf("ohne Datei: %v, %v", s, err)
	}
	// Frontmatter und Leerraum am Ende zählen nicht.
	write(t, dir, FileName(Review), "---\ndescription: x\n---\n"+strings.TrimSpace(Default(Review))+"\n\n\n")
	if s, err := StateOf(dir, Review); err != nil || s != StateCopy || Label(s) != "Kopie des Standards" {
		t.Errorf("Kopie: %v, %v", s, err)
	}
	write(t, dir, FileName(Review), Default(Review)+"\nEigene Regel.\n")
	if s, err := StateOf(dir, Review); err != nil || s != StateCustom || Label(s) != "eigener Prompt" {
		t.Errorf("eigener Prompt: %v, %v", s, err)
	}
}

// TestDefaultPromptsUseAgents prüft, dass die Agenten, die Standard-Prompts und Verträge aufrufen,
// eingebettet sind.
func TestDefaultPromptsUseAgents(t *testing.T) {
	agents, err := claude.Agents()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"structural-completeness-reviewer", "committer"} {
		if _, ok := agents[name]; !ok || !strings.Contains(Default(Review)+contract(Review), "`"+name+"`") {
			t.Errorf("Review ruft Agent %s nicht auf", name)
		}
	}
	if !strings.Contains(contract(Merge), "`committer`") {
		t.Error("Merge ruft committer nicht auf")
	}
	for _, step := range Steps {
		if !strings.HasPrefix(contract(step), "## Regeln von lazydev\n") || !strings.Contains(contract(step), "`.lazydev/`") {
			t.Errorf("Vertrag %s", step)
		}
		if strings.HasPrefix(Default(step), "---") || strings.Contains(Default(step), "Template") {
			t.Errorf("Standard-Prompt %s", step)
		}
	}
}
