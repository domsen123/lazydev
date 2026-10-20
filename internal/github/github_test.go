package github

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseRemote(t *testing.T) {
	cases := []struct {
		url  string
		repo string
		ok   bool
	}{
		{"https://github.com/acme/app", "acme/app", true},
		{"https://github.com/acme/app.git", "acme/app", true},
		{"https://github.com/acme/app/", "acme/app", true},
		{"https://github.com/acme/app.git/", "acme/app", true},
		{"git@github.com:acme/app", "acme/app", true},
		{"git@github.com:acme/app.git", "acme/app", true},
		{"git@github.com:acme/app/", "acme/app", true},
		{"ssh://git@github.com/acme/app", "acme/app", true},
		{"ssh://git@github.com/acme/app.git", "acme/app", true},
		{"ssh://git@github.com/acme/app/", "acme/app", true},
		{"https://gitlab.com/acme/app.git", "", false},
		{"git@gitlab.com:acme/app.git", "", false},
		{"https://github.example.com/acme/app", "", false},
		{"git@github.example.com:acme/app", "", false},
		{"https://github.com//app", "", false},
		{"git@github.com:/app.git", "", false},
		{"https://github.com/acme", "", false},
		{"https://github.com/acme/.git", "", false},
		{"/home/user/repos/app.git", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		repo, ok := ParseRemote(c.url)
		if repo != c.repo || ok != c.ok {
			t.Errorf("ParseRemote(%q) = %q, %v, erwartet %q, %v", c.url, repo, ok, c.repo, c.ok)
		}
	}
}

func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return dir
}

func TestRepo(t *testing.T) {
	dir := gitInit(t)
	if repo, ok := Repo(dir); ok || repo != "" {
		t.Errorf("Repo ohne origin: %q, %v", repo, ok)
	}
	exec.Command("git", "-C", dir, "remote", "add", "upstream", "git@github.com:acme/upstream.git").Run()
	if _, ok := Repo(dir); ok {
		t.Error("Repo nutzt ein anderes Remote als origin")
	}
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:acme/app.git").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if repo, ok := Repo(dir); !ok || repo != "acme/app" {
		t.Errorf("Repo mit origin: %q, %v", repo, ok)
	}
	exec.Command("git", "-C", dir, "remote", "set-url", "origin", "https://gitlab.com/acme/app.git").Run()
	if _, ok := Repo(dir); ok {
		t.Error("Repo mit origin auf gitlab.com")
	}
}

// fakeGH protokolliert die Argumente und Ordner in args und gibt out aus. FAKE_GH_FAIL lässt gh
// mit Exit-Code 1 scheitern.
func fakeGH(t *testing.T, out string) (args string) {
	t.Helper()
	dir := t.TempDir()
	args = filepath.Join(dir, "args")
	os.WriteFile(filepath.Join(dir, "out.json"), []byte(out), 0o644)
	script := "#!/bin/sh\npwd >> " + args + "\nprintf '%s|' \"$@\" >> " + args + "\necho >> " + args + "\n" +
		`[ -n "$FAKE_GH_FAIL" ] && { echo "  $FAKE_GH_FAIL" >&2; echo "zweite Zeile" >&2; exit 1; }` + "\n" +
		"cat " + filepath.Join(dir, "out.json") + "\n"
	bin := filepath.Join(dir, "gh")
	os.WriteFile(bin, []byte(script), 0o755)
	prev := Binary
	Binary = bin
	t.Cleanup(func() { Binary = prev })
	return args
}

func TestList(t *testing.T) {
	out := `[{"number":1,"title":"Alt","body":"","url":"https://github.com/acme/app/issues/1","createdAt":"2026-01-01T10:00:00Z"},
{"number":3,"title":"Neu","body":"Text","url":"https://github.com/acme/app/issues/3","createdAt":"2026-03-01T10:00:00Z"},
{"number":2,"title":"Mitte","body":"","url":"https://github.com/acme/app/issues/2","createdAt":"2026-02-01T10:00:00Z"}]`
	argsPath := fakeGH(t, out)
	dir := t.TempDir()
	issues, err := List(dir, "acme/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 3 || issues[0].Number != 3 || issues[1].Number != 2 || issues[2].Number != 1 {
		t.Fatalf("Sortierung: %+v", issues)
	}
	if issues[0].Body != "Text" || !issues[0].CreatedAt.Equal(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("Felder: %+v", issues[0])
	}
	got, _ := os.ReadFile(argsPath)
	want := "issue|list|--repo|acme/app|--state|open|--limit|30|--json|number,title,body,url,createdAt|"
	if lines := strings.Split(string(got), "\n"); lines[0] != dir || lines[1] != want {
		t.Errorf("Aufruf von gh:\n%s", got)
	}

	t.Setenv("FAKE_GH_FAIL", "gh auth login required")
	if _, err := List(dir, "acme/app"); err == nil || err.Error() != "gh auth login required\nzweite Zeile" {
		t.Errorf("Fehler bei Exit-Code 1: %v", err)
	}
	if err := OpenWeb(dir, "acme/app", 3); err == nil || !strings.HasPrefix(err.Error(), "gh auth login required") {
		t.Errorf("Fehler von OpenWeb: %v", err)
	}
}

func TestListWithoutBinary(t *testing.T) {
	prev := Binary
	Binary = filepath.Join(t.TempDir(), "gh")
	t.Cleanup(func() { Binary = prev })
	if _, err := List(t.TempDir(), "acme/app"); err == nil {
		t.Error("kein Fehler ohne gh")
	}
}

func TestOpenWeb(t *testing.T) {
	argsPath := fakeGH(t, "")
	if err := OpenWeb(t.TempDir(), "acme/app", 7); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(argsPath)
	if !strings.Contains(string(got), "\nissue|view|7|--repo|acme/app|--web|\n") {
		t.Errorf("Aufruf von gh:\n%s", got)
	}
}

func TestCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c, err := LoadCache("alpha")
	if c != nil || err != nil {
		t.Fatalf("LoadCache ohne Datei: %+v, %v", c, err)
	}
	saved := &Cache{Repo: "acme/app", FetchedAt: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
		Issues: []Issue{{Number: 4, Title: "Login", Body: "Text", URL: "https://github.com/acme/app/issues/4",
			CreatedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)}}}
	if err := SaveCache("alpha", saved); err != nil {
		t.Fatal(err)
	}
	c, err = LoadCache("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if c.Repo != saved.Repo || !c.FetchedAt.Equal(saved.FetchedAt) || len(c.Issues) != 1 || c.Issues[0].Title != "Login" ||
		!c.Issues[0].CreatedAt.Equal(saved.Issues[0].CreatedAt) {
		t.Errorf("Rundlauf: %+v", c)
	}
	files, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_STATE_HOME"), "lazydev", "issues", "*"))
	if len(files) != 1 || filepath.Base(files[0]) != "alpha.json" {
		t.Errorf("Dateien im Zwischenspeicher: %v", files)
	}
}
