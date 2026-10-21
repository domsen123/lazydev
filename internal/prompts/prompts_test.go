package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/repo"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contract(t *testing.T, step string) string {
	t.Helper()
	data, err := contracts.ReadFile("contract/" + step + ".md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestContractsExist(t *testing.T) {
	for _, step := range repo.Steps {
		if c := contract(t, step); !strings.HasPrefix(c, "## Regeln von lazydev\n") {
			t.Errorf("contract %s starts with %q", step, strings.SplitN(c, "\n", 2)[0])
		}
	}
}

func TestBuild(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir, work := t.TempDir(), t.TempDir()
	writeFile(t, dir, ".lazydev/implement.md", "---\ndescription: implement\nallowed-tools: Bash\n---\n\n# Implement\n\nDo $ARGUMENTS on $BRANCH for $BASE.\n\n")
	for _, f := range []string{"CLAUDE.md", "docs/a.md", "docs/b.md"} {
		writeFile(t, work, f, "x\n")
	}
	cfg := repo.Config{Knowledge: map[string][]string{
		"all":       {"CLAUDE.md", "docs/a.md"},
		"implement": {"docs/a.md", "docs/b.md"},
		// Knowledge of other steps does not matter.
		"review": {"docs/missing.md"},
	}}
	v := Vars{Arguments: "prd/x.md", Base: "main", Branch: "feat/x"}
	got, err := Build(dir, work, cfg, "implement", v)
	if err != nil {
		t.Fatal(err)
	}
	want := "# lazydev implement: prd/x.md\n\n" +
		"## Pflichtlektüre\n\nLies diese Dateien vollständig, bevor du etwas anderes tust:\n\n" +
		"- `CLAUDE.md`\n- `docs/a.md`\n- `docs/b.md`\n\n" +
		"# Implement\n\nDo prd/x.md on feat/x for main.\n\n" +
		strings.TrimSpace(strings.NewReplacer("$ARGUMENTS", v.Arguments, "$BASE", v.Base, "$BRANCH", v.Branch).Replace(contract(t, "implement"))) + "\n"
	if got != want {
		t.Errorf("Build =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "description:") || strings.Contains(got, "$ARGUMENTS") || strings.Contains(got, "$BASE") || strings.Contains(got, "$BRANCH") {
		t.Errorf("frontmatter or placeholders left:\n%s", got)
	}
	// The contract comes last.
	if i, j := strings.Index(got, "# Implement"), strings.Index(got, "## Regeln von lazydev"); i < 0 || j < i {
		t.Errorf("contract not after the prompt:\n%s", got)
	}
	// Build leaves cfg alone.
	if len(cfg.Knowledge["all"]) != 2 {
		t.Errorf("Build changed cfg: %v", cfg.Knowledge)
	}
}

func TestBuildWithoutKnowledge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/review.md", "Review $BRANCH.\n")
	got, err := Build(dir, t.TempDir(), repo.Config{}, "review", Vars{Base: "develop", Branch: "feat/x"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "# lazydev review: feat/x → develop\n\nReview feat/x.\n\n## Regeln von lazydev\n") {
		t.Errorf("Build =\n%s", got)
	}
	if strings.Contains(got, "Pflichtlektüre") {
		t.Errorf("knowledge block without knowledge:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("Build does not end with one newline: %q", got[len(got)-10:])
	}
}

func TestBuildMissingPrompt(t *testing.T) {
	_, err := Build(t.TempDir(), t.TempDir(), repo.Config{}, "merge", Vars{})
	if err == nil || !strings.Contains(err.Error(), ".lazydev/merge.md") || !strings.Contains(err.Error(), "press i") {
		t.Errorf("Build error = %v", err)
	}
}

func TestBuildMissingKnowledge(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	writeFile(t, dir, ".lazydev/implement.md", "x\n")
	// The file exists in the main folder only, but Claude runs in work.
	writeFile(t, dir, "docs/only-main.md", "x\n")
	cfg := repo.Config{Knowledge: map[string][]string{"implement": {"docs/only-main.md"}}}
	_, err := Build(dir, work, cfg, "implement", Vars{Arguments: "prd/x.md"})
	if err == nil || !strings.Contains(err.Error(), "docs/only-main.md") {
		t.Errorf("Build error = %v", err)
	}
}

func TestBuildPromptFromMainFolderOnly(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	writeFile(t, work, ".lazydev/create.md", "from worktree\n")
	if _, err := Build(dir, work, repo.Config{}, "create", Vars{Arguments: "idea"}); err == nil {
		t.Error("prompt read from the worktree")
	}
	writeFile(t, dir, ".lazydev/create.md", "from main\n")
	got, err := Build(dir, work, repo.Config{}, "create", Vars{Arguments: "idea"})
	if err != nil || !strings.Contains(got, "from main") || strings.Contains(got, "from worktree") {
		t.Errorf("Build = %q, %v", got, err)
	}
}

func TestHeader(t *testing.T) {
	tests := []struct {
		step string
		v    Vars
		want string
	}{
		{"implement", Vars{Arguments: "prd/auth/token.md"}, "# lazydev implement: prd/auth/token.md"},
		{"create", Vars{Arguments: "  first line\nsecond line"}, "# lazydev create: first line"},
		{"review", Vars{Base: "main", Branch: "feat/x"}, "# lazydev review: feat/x → main"},
		{"merge", Vars{Arguments: " \n ", Base: "main", Branch: "feat/x"}, "# lazydev merge: feat/x → main"},
	}
	for _, tt := range tests {
		if got := Header(tt.step, tt.v); got != tt.want {
			t.Errorf("Header(%s, %+v) = %q, want %q", tt.step, tt.v, got, tt.want)
		}
	}
}

func TestStripFrontmatter(t *testing.T) {
	for in, want := range map[string]string{
		"---\na: b\n---\nbody\n":   "body\n",
		"---\n---\nbody":           "body",
		"--- \na: b\n ---\nbody":   "body",
		"---\na: b\n---":           "",
		"---\nunclosed\nbody\n":    "---\nunclosed\nbody\n",
		"body\n---\na: b\n---\n":   "body\n---\na: b\n---\n",
		"---":                      "---",
		"":                         "",
		"# Title\n\n---\n\nmore\n": "# Title\n\n---\n\nmore\n",
	} {
		if got := stripFrontmatter(in); got != want {
			t.Errorf("stripFrontmatter(%q) = %q, want %q", in, got, want)
		}
	}
}
