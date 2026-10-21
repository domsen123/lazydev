package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/git"
)

// setupStatus legt ein Projekt auf main an, in dem auth/token getrackt und committet ist.
func setupStatus(t *testing.T) (Model, string) {
	t.Helper()
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "t")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t@t")
	}
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cfg := &config.Config{Projects: []config.Project{{Path: dir}}}
	m := drive(t, New(filepath.Join(root, "config.yaml"), cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, m.Init()())
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	return m, dir
}

// runAction schickt msg und führt die Aktion im Hintergrund aus. Danach liest es die Projekte neu ein.
func runAction(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(Model)
	if cmd == nil {
		t.Fatalf("keine Aktion, Meldung %q", m.message)
	}
	done, ok := cmd().(actionDoneMsg)
	if !ok {
		t.Fatal("keine actionDoneMsg")
	}
	next, _ = m.Update(done)
	m = next.(Model)
	return drive(t, m, m.loadProjects("")())
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func TestStatusPickerCommitsTrackedPRD(t *testing.T) {
	m, dir := setupStatus(t)
	m.selectPRD("token")

	m = drive(t, m, keys("c")...)
	view := ansi.Strip(m.View())
	if m.mode != modeStatus || !strings.Contains(view, "Status · auth/token") || !strings.Contains(view, "> geplant (aktuell)") ||
		!strings.Contains(view, "  umgesetzt") || !strings.Contains(view, "  verworfen") {
		t.Fatalf("Fenster (%v, %q):\n%s", m.mode, m.message, view)
	}
	if got, want := ansi.Strip(m.helpLine()), " ↑ ↓ Status wählen · Enter übernehmen · Esc abbrechen"; got != want {
		t.Errorf("Fußzeile im Fenster:\n got %q\nwant %q", got, want)
	}

	// Der aktuelle Wert schließt das Fenster ohne Änderung.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal || m.confirm != nil {
		t.Fatalf("gleicher Wert: %v", m.mode)
	}
	// Esc bricht ab.
	m = drive(t, m, keys("cj")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal || m.prdStatus != nil {
		t.Fatalf("Esc: %v", m.mode)
	}

	m = drive(t, m, keys("cj")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	value := "umgesetzt (" + today() + ")"
	if m.mode != modeConfirm || m.confirm.question != "Status von auth/token auf "+value+" setzen und committen? (j/n)" {
		t.Fatalf("Rückfrage (%v): %+v", m.mode, m.confirm)
	}
	m = runAction(t, m, keys("j")[0])
	if m.messageErr || m.message != "Status von auth/token: "+value+", committet" {
		t.Errorf("Meldung %q", m.message)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "prd", "auth", "token.md"))
	if string(data) != "# PRD: Token\n\n> Status: "+value+" · Scope: Backend\n" {
		t.Errorf("Datei:\n%s", data)
	}
	if msg, _ := git.Run(dir, "log", "-1", "--format=%B"); msg != "docs(prd): mark token as done" {
		t.Errorf("Commit-Nachricht %q", msg)
	}

	// Das PRD liegt jetzt in der Gruppe „erledigt“.
	rows := m.prdList(m.selectedProject())
	if len(rows) != 1 || rows[0].prd != nil || rows[0].done != 1 {
		t.Errorf("PRD-Liste: %+v", rows)
	}
}

func TestStatusPickerWrongBranch(t *testing.T) {
	m, dir := setupStatus(t)
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "-b", "other").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	head, _ := git.Run(dir, "rev-parse", "HEAD")
	m.selectPRD("token")
	m = drive(t, m, keys("cj")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = runAction(t, m, keys("j")[0])
	if !m.messageErr || m.message != "Hauptordner steht auf other statt main" {
		t.Errorf("Meldung %q", m.message)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "prd", "auth", "token.md"))
	if string(data) != "# PRD: Token\n\n> Status: geplant · Scope: Backend\n" {
		t.Errorf("Datei geändert:\n%s", data)
	}
	if after, _ := git.Run(dir, "rev-parse", "HEAD"); after != head {
		t.Error("Commit trotz falschem Branch")
	}
}

func TestStatusPickerUntrackedDiscarded(t *testing.T) {
	m, dir := setupStatus(t)
	writeTestFile(t, dir, "prd/auth/neu.md", "# PRD: Neu\n> Status: geplant · Scope: x\n")
	head, _ := git.Run(dir, "rev-parse", "HEAD")
	m = drive(t, m, keys("r")...)
	m.selectPRD("neu")

	// Ohne Rückfrage, ohne Commit.
	m = drive(t, m, keys("cjj")...)
	value := "verworfen (" + today() + ")"
	m = runAction(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.messageErr || m.message != "Status von auth/neu: "+value {
		t.Errorf("Meldung %q", m.message)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "prd", "auth", "neu.md"))
	if string(data) != "# PRD: Neu\n> Status: "+value+" · Scope: x\n" {
		t.Errorf("Datei:\n%s", data)
	}
	if after, _ := git.Run(dir, "rev-parse", "HEAD"); after != head {
		t.Error("ungetracktes PRD committet")
	}

	// Das verworfene PRD liegt in „erledigt“, mit ✗, und s startet es nicht.
	rows := m.prdList(m.selectedProject())
	if len(rows) != 2 || rows[0].prd == nil || rows[0].prd.Name != "token" || rows[1].prd != nil || rows[1].done != 1 {
		t.Fatalf("PRD-Liste: %+v", rows)
	}
	m.toggleDone()
	m.selectPRD("neu")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "✗ verworfen") || !strings.Contains(view, "▾ 1 erledigt") {
		t.Errorf("verworfenes PRD:\n%s", view)
	}
	m = drive(t, m, keys("s")...)
	if !m.messageErr || m.message != "auth/neu ist verworfen" || m.mode != modeNormal {
		t.Errorf("s auf verworfenem PRD: %v %q", m.mode, m.message)
	}

	// Ein verworfenes PRD lässt sich zurück auf geplant setzen.
	m = drive(t, m, keys("c")...)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "> verworfen (aktuell)") {
		t.Fatalf("Fenster:\n%s", view)
	}
	m = drive(t, m, keys("kk")...)
	m = runAction(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	data, _ = os.ReadFile(filepath.Join(dir, "prd", "auth", "neu.md"))
	if string(data) != "# PRD: Neu\n> Status: geplant · Scope: x\n" {
		t.Errorf("zurück auf geplant:\n%s", data)
	}
}

func TestStatusPickerBlocked(t *testing.T) {
	// Ein PRD mit Lauf-Zustand lässt sich nicht ändern.
	m := setupRun(t)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = drive(t, m, keys("c")...)
	if m.mode != modeNormal || !m.messageErr || m.message != "auth/moved hat einen Lauf von lazydev. Verwirf ihn zuerst mit X." {
		t.Errorf("PRD mit Lauf: %v %q", m.mode, m.message)
	}

	// Ohne Statuszeile legt lazydev keine an.
	m, dir := setupStatus(t)
	writeTestFile(t, dir, "prd/auth/leer.md", "# PRD: Leer\n\nText\n")
	m = drive(t, m, keys("r")...)
	m.selectPRD("leer")
	m = drive(t, m, keys("c")...)
	if m.mode != modeNormal || !m.messageErr || m.message != "auth/leer hat keine Statuszeile" {
		t.Errorf("ohne Statuszeile: %v %q", m.mode, m.message)
	}
}
