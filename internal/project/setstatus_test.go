package project

import (
	"os"
	"path/filepath"
	"testing"

	"lazydev/internal/git"
)

// setupStatusRepo legt ein Repo auf main mit einem getrackten und einem ungetrackten PRD an.
func setupStatusRepo(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "t")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t@t")
	}
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "prd/auth/token.md", "# PRD: Token\n\n> Status: geplant · Scope: Backend\n> zweite Zeile\n")
	write(t, dir, "other.txt", "a\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, "prd/auth/neu.md", "# PRD: Neu\n> Status: geplant\n")
	return dir
}

func TestChangeStatusCommitsTrackedPRD(t *testing.T) {
	dir := setupStatusRepo(t)
	// Andere geänderte Dateien im Hauptordner stören nicht und landen nicht im Commit.
	write(t, dir, "other.txt", "b\n")

	committed, err := ChangeStatus(dir, "main", "prd/auth/token.md", "umgesetzt (2026-10-21)")
	if err != nil || !committed {
		t.Fatalf("ChangeStatus = %v, %v", committed, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "prd/auth/token.md"))
	if want := "# PRD: Token\n\n> Status: umgesetzt (2026-10-21) · Scope: Backend\n> zweite Zeile\n"; string(data) != want {
		t.Errorf("Datei:\n%s", data)
	}
	msg, _ := git.Run(dir, "log", "-1", "--format=%B")
	if msg != "docs(prd): mark token as done" {
		t.Errorf("Commit-Nachricht %q", msg)
	}
	files, _ := git.Run(dir, "show", "--name-only", "--format=", "HEAD")
	if files != "prd/auth/token.md" {
		t.Errorf("Dateien im Commit: %q", files)
	}
	if status, _ := git.Run(dir, "status", "--porcelain"); status != " M other.txt\n?? prd/auth/neu.md" {
		t.Errorf("Status nach dem Commit:\n%s", status)
	}

	for value, word := range map[string]string{"verworfen (2026-10-21)": "discarded", "geplant": "planned"} {
		if _, err := ChangeStatus(dir, "main", "prd/auth/token.md", value); err != nil {
			t.Fatal(err)
		}
		if msg, _ := git.Run(dir, "log", "-1", "--format=%s"); msg != "docs(prd): mark token as "+word {
			t.Errorf("%s: Commit-Nachricht %q", value, msg)
		}
	}
}

func TestChangeStatusUntracked(t *testing.T) {
	dir := setupStatusRepo(t)
	// Für ungetrackte PRDs gilt die Voraussetzung für den Commit nicht.
	run(t, dir, "checkout", "-q", "-b", "other")
	head, _ := git.Run(dir, "rev-parse", "HEAD")

	committed, err := ChangeStatus(dir, "main", "prd/auth/neu.md", "verworfen (2026-10-21)")
	if err != nil || committed {
		t.Fatalf("ChangeStatus = %v, %v", committed, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "prd/auth/neu.md"))
	if string(data) != "# PRD: Neu\n> Status: verworfen (2026-10-21)\n" {
		t.Errorf("Datei:\n%s", data)
	}
	if after, _ := git.Run(dir, "rev-parse", "HEAD"); after != head {
		t.Error("ungetracktes PRD committet")
	}
}

func TestChangeStatusBlocked(t *testing.T) {
	dir := setupStatusRepo(t)
	const before = "# PRD: Token\n\n> Status: geplant · Scope: Backend\n> zweite Zeile\n"
	head, _ := git.Run(dir, "rev-parse", "HEAD")
	check := func(want string) {
		t.Helper()
		committed, err := ChangeStatus(dir, "main", "prd/auth/token.md", "umgesetzt (2026-10-21)")
		if committed || err == nil || err.Error() != want {
			t.Errorf("ChangeStatus = %v, %v, erwartet %q", committed, err, want)
		}
		if after, _ := git.Run(dir, "rev-parse", "HEAD"); after != head {
			t.Error("Commit trotz Fehler")
		}
	}

	run(t, dir, "checkout", "-q", "-b", "other")
	check("Hauptordner steht auf other statt main")
	if data, _ := os.ReadFile(filepath.Join(dir, "prd/auth/token.md")); string(data) != before {
		t.Errorf("Datei auf falschem Branch geändert:\n%s", data)
	}
	run(t, dir, "checkout", "-q", "main")

	changed := before + "\nNotiz\n"
	write(t, dir, "prd/auth/token.md", changed)
	check("prd/auth/token.md hat uncommittete Änderungen")
	if data, _ := os.ReadFile(filepath.Join(dir, "prd/auth/token.md")); string(data) != changed {
		t.Errorf("Datei mit Änderungen geändert:\n%s", data)
	}

	write(t, dir, "prd/auth/leer.md", "# PRD: Leer\n\nText\n")
	if _, err := ChangeStatus(dir, "main", "prd/auth/leer.md", "geplant"); err == nil || err.Error() != "prd/auth/leer.md hat keine Statuszeile" {
		t.Errorf("ohne Statuszeile: %v", err)
	}
}
