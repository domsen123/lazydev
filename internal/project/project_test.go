package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAssignIDs(t *testing.T) {
	ids := AssignIDs([]string{"/a/app", "/b/app", "/c/other"})
	if ids[2] != "other" {
		t.Errorf("ohne Kollision: %q", ids[2])
	}
	if ids[0] == ids[1] || len(ids[0]) != len("app-")+6 {
		t.Errorf("Kollision nicht aufgelöst: %q %q", ids[0], ids[1])
	}
	again := AssignIDs([]string{"/c/other", "/b/app", "/a/app"})
	if again[2] != ids[0] || again[1] != ids[1] {
		t.Errorf("IDs hängen von der Reihenfolge ab: %v vs. %v", ids, again)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "prd/auth/token.md", "# PRD: Token\n\n> Status: geplant · Scope: X\n")
	write(t, dir, "prd/profile/avatar.md", "# PRD: Avatar\n> Status: umgesetzt (2026-10-01)\n")
	write(t, dir, "prd/billing/export.md", "# PRD: Export\n> Status: geplant\n")
	write(t, dir, "prd/notes.txt", "kein PRD")

	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "add", "prd/profile", "prd/billing")
	run(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init")
	run(t, dir, "branch", "feat/export")

	p := Load("x", dir, "main")
	if p.Err != nil {
		t.Fatal(p.Err)
	}
	if p.Branch != "main" {
		t.Errorf("Branch = %q", p.Branch)
	}
	if len(p.PRDs) != 3 {
		t.Fatalf("PRDs = %+v", p.PRDs)
	}
	byName := map[string]PRD{}
	for _, d := range p.PRDs {
		byName[d.Name] = d
	}

	tok := byName["token"]
	if tok.Area != "auth" || tok.Tracked || tok.Branch != "" || tok.Phase() != PhasePlanned {
		t.Errorf("token = %+v", tok)
	}
	if av := byName["avatar"]; !av.Tracked || av.Branch != "main" || av.Phase() != PhaseMerged {
		t.Errorf("avatar = %+v", av)
	}
	if ex := byName["export"]; ex.Branch != "feat/export" || ex.Label() != "billing/export" {
		t.Errorf("export = %+v", ex)
	}
}

func TestAddedTimes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "prd/a/x.md", "x")
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "add", "prd")
	cmd := exec.Command("git", "-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x")
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE=2020-01-02T03:04:05Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	want := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := addedTimes(dir, PRDDir)["prd/a/x.md"]; !got.Equal(want) {
		t.Errorf("addedTimes = %v, erwartet %v", got, want)
	}
	if got := Load("x", dir, "main").PRDs[0].Created; !got.Equal(want) {
		t.Errorf("Created = %v, erwartet %v", got, want)
	}
}

func TestLoadWithoutGitOrPRDs(t *testing.T) {
	p := Load("x", t.TempDir(), "main")
	if p.Err == nil {
		t.Error("Fehler für fehlendes Git-Repo erwartet")
	}
	if len(p.PRDs) != 0 {
		t.Errorf("PRDs = %+v", p.PRDs)
	}
	if p := Load("x", filepath.Join(t.TempDir(), "fehlt"), "main"); p.Err == nil {
		t.Error("Fehler für fehlenden Ordner erwartet")
	}
}

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

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
