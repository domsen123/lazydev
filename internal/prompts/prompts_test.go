package prompts

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lazydev/internal/lang"
	"lazydev/internal/repo"
)

// de is the config of a German project.
var de = repo.Config{Language: string(lang.DE)}

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

func readContract(t *testing.T, l lang.Lang, step string) string {
	t.Helper()
	data, err := contract(l, step+".md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestContractsExist(t *testing.T) {
	heading := map[lang.Lang]string{lang.EN: "## Rules of lazydev\n", lang.DE: "## Regeln von lazydev\n"}
	for _, l := range lang.Langs() {
		for _, step := range repo.Steps {
			if c := readContract(t, l, step); !strings.HasPrefix(c, heading[l]) {
				t.Errorf("contract %s/%s starts with %q", l, step, strings.SplitN(c, "\n", 2)[0])
			}
		}
	}
}

// TestContractsSameFiles makes sure every language has the same contracts.
func TestContractsSameFiles(t *testing.T) {
	names := map[lang.Lang][]string{}
	for _, l := range lang.Langs() {
		entries, err := fs.ReadDir(contracts, "contract/"+string(l))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			names[l] = append(names[l], e.Name())
		}
	}
	if !reflect.DeepEqual(names[lang.EN], names[lang.DE]) {
		t.Errorf("contracts differ: en %v, de %v", names[lang.EN], names[lang.DE])
	}
}

func TestEnglishContractsHaveNoGermanWords(t *testing.T) {
	entries, err := fs.ReadDir(contracts, "contract/en")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		c := readContract(t, lang.EN, strings.TrimSuffix(e.Name(), ".md"))
		for _, w := range lang.Unique(lang.DE) {
			if strings.Contains(strings.ToLower(c), strings.ToLower(w)) {
				t.Errorf("contract en/%s contains the German %q", e.Name(), w)
			}
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
	knowledge := map[string][]string{
		"all":       {"CLAUDE.md", "docs/a.md"},
		"implement": {"docs/a.md", "docs/b.md"},
		// Knowledge of other steps does not matter.
		"review": {"docs/missing.md"},
	}
	blocks := map[lang.Lang]string{
		lang.EN: "## Required reading\n\nRead these files completely before you do anything else:\n\n",
		lang.DE: "## Pflichtlektüre\n\nLies diese Dateien vollständig, bevor du etwas anderes tust:\n\n",
	}
	rules := map[lang.Lang]string{lang.EN: "## Rules of lazydev", lang.DE: "## Regeln von lazydev"}
	for _, language := range []string{"", "en", "de"} {
		l, _ := lang.Parse(language)
		cfg := repo.Config{Language: language, Knowledge: knowledge}
		v := Vars{Arguments: "prd/x.md", Base: "main", Branch: "feat/x"}
		got, err := Build(dir, work, cfg, "implement", v)
		if err != nil {
			t.Fatal(err)
		}
		want := "# lazydev implement: prd/x.md\n\n" + blocks[l] +
			"- `CLAUDE.md`\n- `docs/a.md`\n- `docs/b.md`\n\n" +
			"# Implement\n\nDo prd/x.md on feat/x for main.\n\n" +
			strings.TrimSpace(strings.NewReplacer("$ARGUMENTS", v.Arguments, "$BASE", v.Base, "$BRANCH", v.Branch).Replace(readContract(t, l, "implement"))) + "\n"
		if got != want {
			t.Errorf("Build %q =\n%s\nwant\n%s", language, got, want)
		}
		if strings.Contains(got, "description:") || strings.Contains(got, "$ARGUMENTS") || strings.Contains(got, "$BASE") || strings.Contains(got, "$BRANCH") {
			t.Errorf("frontmatter or placeholders left:\n%s", got)
		}
		// The contract comes last.
		if i, j := strings.Index(got, "# Implement"), strings.Index(got, rules[l]); i < 0 || j < i {
			t.Errorf("contract not after the prompt:\n%s", got)
		}
		// Build leaves cfg alone.
		if len(cfg.Knowledge["all"]) != 2 {
			t.Errorf("Build changed cfg: %v", cfg.Knowledge)
		}
	}
}

func TestBuildWithoutKnowledge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/review.md", "Review $BRANCH.\n")
	for cfg, rules := range map[*repo.Config]string{&repo.Config{}: "## Rules of lazydev\n", &de: "## Regeln von lazydev\n"} {
		got, err := Build(dir, t.TempDir(), *cfg, "review", Vars{Base: "develop", Branch: "feat/x"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "# lazydev review: feat/x → develop\n\nReview feat/x.\n\n"+rules) {
			t.Errorf("Build =\n%s", got)
		}
		if strings.Contains(got, "Pflichtlektüre") || strings.Contains(got, "Required reading") {
			t.Errorf("knowledge block without knowledge:\n%s", got)
		}
		if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
			t.Errorf("Build does not end with one newline: %q", got[len(got)-10:])
		}
	}
}

func TestBuildUnblock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	tests := []struct {
		language string
		want     []string
		end      string
	}{
		{"de", []string{
			"Kläre die Blocker des PRDs `prd/auth/token.md` mit dem Nutzer.",
			"Du änderst ausschließlich das PRD `prd/auth/token.md`.",
			"Weiter mit lazydev start <prd> oder s in der TUI.",
		}, "- Nenne zum Schluss: `Weiter mit lazydev start <prd> oder s in der TUI.`\n"},
		{"", []string{
			"Resolve the blockers of the PRD `prd/auth/token.md` with the user.",
			"You change only the PRD `prd/auth/token.md`.",
			"Continue with lazydev start <prd> or s in the TUI.",
		}, "- At the end, say: `Continue with lazydev start <prd> or s in the TUI.`\n"},
	}
	for _, tt := range tests {
		dir, work := t.TempDir(), t.TempDir()
		if tt.language != "" {
			writeFile(t, dir, ".lazydev/config.yaml", "language: "+tt.language+"\n")
		}
		if err := repo.Init(dir); err != nil {
			t.Fatal(err)
		}
		writeFile(t, work, "CLAUDE.md", "x\n")
		cfg := repo.Config{Language: tt.language, Knowledge: map[string][]string{"unblock": {"CLAUDE.md"}}}
		got, err := Build(dir, work, cfg, "unblock", Vars{Arguments: "prd/auth/token.md", Base: "main", Branch: "feat/token"})
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range append(tt.want, "# lazydev unblock: prd/auth/token.md\n", "- `CLAUDE.md`\n", "`## Blocker`") {
			if !strings.Contains(got, s) {
				t.Errorf("%q: prompt lacks %q:\n%s", tt.language, s, got)
			}
		}
		if !strings.HasSuffix(got, tt.end) {
			t.Errorf("%q: the contract does not come last", tt.language)
		}
	}
}

func TestBuildVerify(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	tests := []struct {
		cfg  repo.Config
		want []string
	}{
		{de, []string{
			"Prüfe das Feature des PRDs `prd/auth/token.md` in der laufenden App",
			"nur unter `/state/runs/app/token.evidence` ab",
			"## Belege\n",
			"verifiziert | abgebrochen",
		}},
		{repo.Config{}, []string{
			"Check the feature of the PRD `prd/auth/token.md` in the running app",
			"only under `/state/runs/app/token.evidence`",
			"## Evidence\n",
			"verified | aborted",
		}},
	}
	for _, tt := range tests {
		dir := t.TempDir()
		if err := repo.Init(dir); err != nil {
			t.Fatal(err)
		}
		if err := repo.WritePrompt(dir, "verify", tt.cfg.Lang()); err != nil {
			t.Fatal(err)
		}
		v := Vars{Arguments: "prd/auth/token.md", Base: "main", Branch: "feat/token", Evidence: "/state/runs/app/token.evidence", Port: 3501}
		got, err := Build(dir, t.TempDir(), tt.cfg, "verify", v)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range append(tt.want, "# lazydev verify: prd/auth/token.md\n", "port `3501`") {
			if !strings.Contains(strings.ToLower(got), strings.ToLower(s)) {
				t.Errorf("verify prompt lacks %q:\n%s", s, got)
			}
		}
		if strings.Contains(got, "$EVIDENCE") || strings.Contains(got, "$PORT") {
			t.Errorf("placeholders left:\n%s", got)
		}
		// Without a port and evidence folder the placeholders go away too.
		got, _ = Build(dir, t.TempDir(), tt.cfg, "verify", Vars{Arguments: "prd/x.md"})
		if strings.Contains(got, "$EVIDENCE") || strings.Contains(got, "$PORT") || strings.Contains(strings.ToLower(got), "port `0`") {
			t.Errorf("placeholders left:\n%s", got)
		}
	}
}

func TestBuildReviewPullRequest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/review.md", "Review $BRANCH.\n")
	v := Vars{Base: "main", Branch: "feat/x", Evidence: "/ev"}
	for _, tt := range []struct {
		language, title string
	}{{"de", "# <Titel>"}, {"", "# <title>"}} {
		ff, err := Build(dir, t.TempDir(), repo.Config{Language: tt.language}, "review", v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(ff, "pr.md") {
			t.Errorf("review without merge: pr writes a pull request:\n%s", ff)
		}
		pr, err := Build(dir, t.TempDir(), repo.Config{Language: tt.language, Merge: repo.MergePR}, "review", v)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"`/ev/pr.md`", tt.title, ".github/pull_request_template.md", "## Evidence", "`/ev/report.md`",
			"No runtime verification ran (.lazydev/verify.md is missing)."} {
			if !strings.Contains(pr, s) {
				t.Errorf("review with merge: pr lacks %q:\n%s", s, pr)
			}
		}
		if !strings.HasPrefix(pr, strings.TrimSuffix(ff, "\n")) {
			t.Errorf("the block does not come after the contract:\n%s", pr)
		}
	}
}

func TestUnblockAndResolveKnowVerify(t *testing.T) {
	for _, step := range []string{"unblock", "resolve"} {
		if c := readContract(t, lang.DE, step); !strings.Contains(c, "`Schritt: verify` und `Schritt: merge` auf `umgesetzt (<heute als YYYY-MM-DD>)`") {
			t.Errorf("contract de/%s does not set umgesetzt after verify", step)
		}
		if c := readContract(t, lang.EN, step); !strings.Contains(c, "after `Step: verify` and `Step: merge` to `implemented (<today as YYYY-MM-DD>)`") {
			t.Errorf("contract en/%s does not set implemented after verify", step)
		}
	}
}

func TestImplementContractHasNotes(t *testing.T) {
	headings := map[lang.Lang][3]string{
		lang.DE: {"## Offene Fragen\n", "## Hinweise\n", "## Nicht geprüft\n"},
		lang.EN: {"## Open questions\n", "## Notes\n", "## Not checked\n"},
	}
	for l, h := range headings {
		c := readContract(t, l, "implement")
		q, n, u := strings.Index(c, h[0]), strings.Index(c, h[1]), strings.Index(c, h[2])
		if q < 0 || n < q || u < n {
			t.Errorf("%s: sections out of order: %d, %d, %d", l, q, n, u)
		}
	}
}

func TestEnglishCreateContract(t *testing.T) {
	c := readContract(t, lang.EN, "create")
	for _, s := range []string{"Write the PRD in English.", "`> Status: planned · Scope:", "`## Context`", "`## Product decisions (final)`",
		"`## Scope`", "**In scope**", "**Out of scope**", "`## Implementation`", "`## Technical notes`", "`## Success criteria`"} {
		if !strings.Contains(c, s) {
			t.Errorf("create contract lacks %q", s)
		}
	}
	if a, _ := contract(lang.EN, "create.auto.md"); !strings.Contains(string(a), "`## Decisions`") {
		t.Error("create auto contract lacks ## Decisions")
	}
	if !strings.Contains(readContract(t, lang.EN, "verify"), "`## Success criteria`") {
		t.Error("verify contract does not refer to ## Success criteria")
	}
}

func TestBuildMissingPrompt(t *testing.T) {
	_, err := Build(t.TempDir(), t.TempDir(), repo.Config{}, "merge", Vars{})
	if err == nil || !strings.Contains(err.Error(), ".lazydev/merge.md") || !strings.Contains(err.Error(), "lazydev add") {
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

func TestAuto(t *testing.T) {
	headings := map[lang.Lang]string{lang.DE: "## Autonomer Modus\n", lang.EN: "## Autonomous mode\n"}
	for l, heading := range headings {
		for _, step := range []string{"create", "unblock"} {
			got := Auto(l, step, Vars{Arguments: "/wt/prd/a/b.md"})
			if !strings.HasPrefix(got, heading) || strings.Contains(got, "$ARGUMENTS") {
				t.Errorf("Auto(%s, %s) = %q", l, step, got)
			}
		}
		if !strings.Contains(Auto(l, "unblock", Vars{Arguments: "/wt/prd/a/b.md"}), "`/wt/prd/a/b.md`") {
			t.Errorf("%s: unblock auto lacks the PRD path", l)
		}
		if got := Auto(l, "implement", Vars{}); got != "" {
			t.Errorf("Auto(%s, implement) = %q, want empty", l, got)
		}
	}
}

func TestPlan(t *testing.T) {
	runs := []PlanRun{{Name: "alt", Phase: "implement", File: "/wt/alt/prd/a/alt.md"}}
	for l, want := range map[lang.Lang][]string{
		lang.DE: {"Fast-Forward in `main`", "## Reihenfolge", "Nach: keine"},
		lang.EN: {"fast-forward. Decide", "into `main`", "## Order", "After: none"},
	} {
		got := Plan(l, Vars{Arguments: "prd/a/neu.md", Base: "main"}, runs)
		for _, s := range append(want, "# lazydev plan: prd/a/neu.md\n", "- `alt` (Phase implement): `/wt/alt/prd/a/alt.md`\n") {
			if !strings.Contains(got, s) {
				t.Errorf("%s: plan prompt lacks %q:\n%s", l, s, got)
			}
		}
		if strings.Contains(got, "$RUNS") || strings.Contains(got, "$ARGUMENTS") {
			t.Errorf("placeholder left:\n%s", got)
		}
	}
}

func TestResolve(t *testing.T) {
	for l, want := range map[lang.Lang][]string{
		lang.DE: {"Ändere ausschließlich das PRD `prd/a/neu.md`", "## Entscheidungen", "geklärt"},
		lang.EN: {"Change only the PRD `prd/a/neu.md`", "## Decisions", "## Result\nresolved"},
	} {
		got := Resolve(l, Vars{Arguments: "prd/a/neu.md"})
		for _, s := range append(want, "# lazydev resolve: prd/a/neu.md\n") {
			if !strings.Contains(got, s) {
				t.Errorf("%s: resolve prompt lacks %q", l, s)
			}
		}
	}
}
