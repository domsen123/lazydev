package runner

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"mobile-bearer-token-auth":     "mobile_bearer_token_auth",
		"new-token":                    "new_token",
		"Foo  Bar!":                    "foo_bar",
		"--x--":                        "x",
		"v2.1-api":                     "v2_1_api",
		"Übersicht":                    "bersicht",
		strings.Repeat("a", 39) + "-b": strings.Repeat("a", 39),
		strings.Repeat("ab", 30):       strings.Repeat("ab", 20),
		"":                             "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
		if got := Slug(in); len(got) > maxSlugLen {
			t.Errorf("Slug(%q) longer than %d", in, maxSlugLen)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	if got, want := WorktreePath("app", "x"), filepath.Join(data, "lazydev", "worktrees", "app", "x"); got != want {
		t.Errorf("WorktreePath = %q, want %q", got, want)
	}
	t.Setenv("XDG_DATA_HOME", "")
	home, _ := os.UserHomeDir()
	if got, want := WorktreePath("app", "x"), filepath.Join(home, ".local", "share", "lazydev", "worktrees", "app", "x"); got != want {
		t.Errorf("WorktreePath without XDG_DATA_HOME = %q, want %q", got, want)
	}
}

func TestAllocatePort(t *testing.T) {
	isolate(t)
	free := func(used map[int]bool) int {
		for p := PortBase; p < PortBase+portSearchRange; p++ {
			if !used[p] && portFree(p) {
				return p
			}
		}
		t.Fatal("no free port")
		return 0
	}
	first := free(nil)
	if got, err := allocatePort(); err != nil || got != first {
		t.Fatalf("allocatePort = %d, %v, want %d", got, err, first)
	}

	// Ports of unmerged runs of any project are taken, merged runs free theirs.
	state.Save("app", "a", &state.Run{Port: first, Phase: project.PhaseImplement})
	second := free(map[int]bool{first: true})
	state.Save("web", "b", &state.Run{Port: second, Phase: project.PhaseError})
	third := free(map[int]bool{first: true, second: true})
	state.Save("app", "c", &state.Run{Port: third, Phase: project.PhaseMerged})
	if got, err := allocatePort(); err != nil || got != third {
		t.Errorf("allocatePort = %d, %v, want %d", got, err, third)
	}

	// A port with a listener is skipped.
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(third))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if portFree(third) {
		t.Errorf("port %d with listener counts as free", third)
	}
	if got, err := allocatePort(); err != nil || got == third || got == first || got == second {
		t.Errorf("allocatePort = %d, %v, taken: %d %d %d", got, err, first, second, third)
	}
}

func TestFind(t *testing.T) {
	isolate(t)
	if _, err := Find("app"); err == nil {
		t.Error("Find without projects")
	}
	root := t.TempDir()
	a, b, web := filepath.Join(root, "one", "app"), filepath.Join(root, "two", "app"), filepath.Join(root, "web")
	if err := config.SaveProjects([]string{a, b, web}); err != nil {
		t.Fatal(err)
	}
	ids := config.IDs([]string{a, b, web})

	if got, err := Find("web"); err != nil || got != (Target{ID: "web", Dir: web}) {
		t.Errorf("Find by ID: %+v, %v", got, err)
	}
	if got, err := Find(b); err != nil || got != (Target{ID: ids[1], Dir: b}) {
		t.Errorf("Find by path: %+v, %v", got, err)
	}
	if got, err := Find(ids[0]); err != nil || got.Dir != a {
		t.Errorf("Find by hashed ID: %+v, %v", got, err)
	}
	if got, err := Find(filepath.Join(root, "web") + "/"); err != nil || got.ID != "web" {
		t.Errorf("Find by path with slash: %+v, %v", got, err)
	}
	// The bare folder name is ambiguous and no ID.
	if _, err := Find("app"); err == nil || !strings.Contains(err.Error(), `no project "app"`) {
		t.Errorf("Find of an ambiguous name: %v", err)
	}
}

func TestCheckSummary(t *testing.T) {
	cases := []struct {
		name string
		s    claude.Summary
		exit int
		want string
	}{
		{"ok", claude.Summary{HasResult: true, Result: "fertig"}, 0, ""},
		{"exit with result", claude.Summary{HasResult: true, IsError: true, Result: "\nkaputt\nmehr"}, 3, "claude ended with exit code 3: kaputt"},
		{"exit without result", claude.Summary{}, 1, "claude ended with exit code 1"},
		{"signal", claude.Summary{}, -1, "claude ended with exit code -1"},
		{"no result", claude.Summary{SessionID: "s"}, 0, "claude ended without result event"},
		{"error result", claude.Summary{HasResult: true, IsError: true, Result: "Rate limit\nlater"}, 0, "claude reports an error: Rate limit"},
		{"empty error result", claude.Summary{HasResult: true, IsError: true}, 0, "claude reports an error: "},
	}
	for _, c := range cases {
		err := checkSummary(c.s, c.exit)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCheckReport(t *testing.T) {
	const impl, merge = "Offene Fragen", "Offene Punkte"
	cases := []struct {
		name, report, step, want, questions, err string
	}{
		{"done", "## Ergebnis\numgesetzt\n", "implement", resultImplemented, impl, ""},
		{"case and blanks", "Text\n\n## Ergebnis\n\n  Umgesetzt  \n\n## Offene Fragen\n- keine\n", "implement", resultImplemented, impl, ""},
		{"first line counts", "## Ergebnis\numgesetzt\nmit Details\n", "implement", resultImplemented, impl, ""},
		{"merge ready", "## Ergebnis\nBereit für Fast-Forward\n", "merge", resultMergeReady, merge, ""},
		{"no section", "umgesetzt", "implement", resultImplemented, impl, "report of implement without „## Ergebnis“"},
		{"empty", "", "review", resultImplemented, impl, "report of review without „## Ergebnis“"},
		{"empty section", "## Ergebnis\n\n## Offene Fragen\n- x\n", "implement", resultImplemented, impl, `unknown result "" in the report of implement`},
		{"aborted with question", "## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- Feld fehlt im PRD\n- zweite\n", "implement", resultImplemented, impl, "implement blocked: Feld fehlt im PRD"},
		{"keine before a question", "## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- keine\n- Feld fehlt\n", "implement", resultImplemented, impl, "implement blocked: Feld fehlt"},
		{"notes do not block", "## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- keine\n\n## Hinweise\n- einmal cd benutzt\n", "implement", resultImplemented, impl, "implement aborted"},
		{"notes with a result", "## Ergebnis\numgesetzt\n\n## Offene Fragen\n- keine\n\n## Hinweise\n- einmal cd benutzt\n", "implement", resultImplemented, impl, ""},
		{"aborted with keine", "## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- keine\n", "implement", resultImplemented, impl, "implement aborted"},
		{"aborted with bare keine", "## Ergebnis\nAbgebrochen\n\n## Offene Fragen\nkeine\n", "implement", resultImplemented, impl, "implement aborted"},
		{"aborted without questions", "## Ergebnis\nabgebrochen\n", "implement", resultImplemented, impl, "implement aborted"},
		{"aborted reads the given section", "## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- falsch\n\n## Offene Punkte\n- Konflikt in x\n", "merge", resultMergeReady, merge, "merge blocked: Konflikt in x"},
		{"unknown", "## Ergebnis\nteilweise umgesetzt\n", "implement", resultImplemented, impl, `unknown result "teilweise umgesetzt" in the report of implement`},
		{"other result of another step", "## Ergebnis\numgesetzt\n", "merge", resultMergeReady, merge, `unknown result "umgesetzt" in the report of merge`},
	}
	for _, c := range cases {
		err := checkReport(c.report, c.step, c.want, c.questions)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.err {
			t.Errorf("%s: %q, want %q", c.name, got, c.err)
		}
	}
	err := checkReport("## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- Erste\n\n  Zweite  \n- keine\n", "implement", resultImplemented, "Offene Fragen")
	var blocked *blockedError
	if !errors.As(err, &blocked) || blocked.step != "implement" || strings.Join(blocked.points, "|") != "Erste|Zweite" {
		t.Errorf("blocked: %#v", err)
	}
}

func TestCheckLazydevUntouched(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	write(t, dir, ".lazydev/implement.md", "prompt\n")
	write(t, dir, "a.txt", "a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	runGit(t, dir, "switch", "-q", "-c", "feat/x")
	r := &state.Run{Worktree: dir, Base: "main"}
	check := func(want string) {
		t.Helper()
		err := checkLazydevUntouched(r)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != want {
			t.Errorf("checkLazydevUntouched = %q, want %q", got, want)
		}
	}
	write(t, dir, "a.txt", "changed\n")
	check("")
	write(t, dir, ".lazydev/implement.md", "changed\n")
	check(".lazydev/ changed in the worktree: .lazydev/implement.md")
	runGit(t, dir, "commit", "-q", "-am", "change")
	check(".lazydev/ changed in the worktree: .lazydev/implement.md")
	// Committed and changed again counts once, a new file adds to it.
	write(t, dir, ".lazydev/implement.md", "again\n")
	write(t, dir, ".lazydev/sub/new.md", "x\n")
	check(".lazydev/ changed in the worktree: .lazydev/implement.md, .lazydev/sub/new.md")
	// A change on the base branch after the branch point does not count.
	runGit(t, dir, "checkout", "-q", "--", ".")
	runGit(t, dir, "clean", "-q", "-fd", ".lazydev")
	runGit(t, dir, "reset", "-q", "--hard", "main")
	runGit(t, dir, "switch", "-q", "main")
	write(t, dir, ".lazydev/merge.md", "main\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "main change")
	runGit(t, dir, "switch", "-q", "feat/x")
	check("")
}

func TestLogger(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "sub", "step.jsonl")
	log, err := openLog(path)
	if err != nil {
		t.Fatal(err)
	}
	log.printf("Step %s", "setup")
	w := log.lines()
	w.Write([]byte("one\r\ntw"))
	w.Write([]byte("o\nthree"))
	w.Close()
	log.Write([]byte(`{"type":"result"}` + "\n"))
	log.close()
	lines := strings.Split(strings.TrimSpace(read(t, path)), "\n")
	if len(lines) != 5 {
		t.Fatalf("lines:\n%s", strings.Join(lines, "\n"))
	}
	for i, text := range []string{"Step setup", "one", "two", "three"} {
		if !strings.Contains(lines[i], `"type":"lazydev"`) || !strings.Contains(lines[i], `"text":"`+text+`"`) || !strings.Contains(lines[i], `"time":"`) {
			t.Errorf("line %d: %s", i, lines[i])
		}
	}
	if lines[4] != `{"type":"result"}` {
		t.Errorf("stream line changed: %s", lines[4])
	}

	// Without a path the logger writes nothing.
	empty, err := openLog("")
	if err != nil {
		t.Fatal(err)
	}
	empty.printf("nothing")
	empty.close()
}
