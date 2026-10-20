package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
)

// longPRD liefert ein PRD, das nicht ins Fenster passt. last steht im letzten Abschnitt.
func longPRD(last string) string {
	var b strings.Builder
	b.WriteString("# PRD: Token\n\n> Status: geplant · Scope: Backend\n\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "## Abschnitt %d\n\nText mit **fett** Nummer %d.\n\n", i, i)
	}
	b.WriteString("## Schluss\n\n" + last + "\n")
	return b.String()
}

// setupPRDView legt ein Projekt mit einem langen PRD an und lädt es.
func setupPRDView(t *testing.T) (Model, string) {
	t.Helper()
	t.Setenv("HERDR_ENV", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	path := filepath.Join(dir, "prd", "auth", "token.md")
	if err := os.WriteFile(path, []byte(longPRD("Ganz unten im Hauptordner")), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Claude: config.Claude{Binary: "claude"}, Projects: []config.Project{{Path: dir}}}
	m := drive(t, New(filepath.Join(root, "config.yaml"), cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	return drive(t, m, m.Init()()), path
}

func TestPRDViewOpenAndClose(t *testing.T) {
	m, _ := setupPRDView(t)
	m = drive(t, m, keys("v")...)
	if m.mode != modePRDView || m.prdView == nil {
		t.Fatalf("v öffnet kein Fenster: Modus %v, Meldung %q", m.mode, m.message)
	}
	view := ansi.Strip(m.View())
	for _, want := range []string{"PRD: auth/token", "Status    geplant", "Abschnitt 1", " %"} {
		if !strings.Contains(view, want) {
			t.Errorf("%q fehlt:\n%s", want, view)
		}
	}
	if strings.Contains(view, "**") {
		t.Errorf("Markdown nicht gerendert:\n%s", view)
	}
	if !strings.Contains(view, "j/k scrollen · PgUp/PgDn Seite · g/G Anfang/Ende · o Editor · Esc schließen") {
		t.Errorf("Tastenhilfe fehlt:\n%s", view)
	}

	m = drive(t, m, keys("G")...)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Ganz unten im Hauptordner") || !strings.Contains(view, "100 %") {
		t.Errorf("G springt nicht ans Ende:\n%s", view)
	}
	m = drive(t, m, keys("g")...)
	if m.prdView.viewport.YOffset != 0 {
		t.Errorf("g springt nicht an den Anfang: %d", m.prdView.viewport.YOffset)
	}
	m = drive(t, m, keys("jj")...)
	m = drive(t, m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if m.prdView.viewport.YOffset != 3 {
		t.Errorf("j und Mausrad: YOffset %d", m.prdView.viewport.YOffset)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	if want := 3 + m.prdView.viewport.Height; m.prdView.viewport.YOffset != want {
		t.Errorf("PgDn: YOffset %d, erwartet %d", m.prdView.viewport.YOffset, want)
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal || m.prdView != nil {
		t.Errorf("Esc schließt nicht: %v", m.mode)
	}
	m = drive(t, m, keys("vq")...)
	if m.mode != modeNormal || m.prdView != nil {
		t.Errorf("q schließt nicht: %v", m.mode)
	}
}

func TestPRDViewFromWorktree(t *testing.T) {
	t.Setenv("HERDR_ENV", "")
	m := setupRun(t)
	if d := m.selectedPRD(); d == nil || d.Name != "moved" {
		t.Fatalf("gewählt: %+v", d)
	}
	r := m.selectedRun()
	os.WriteFile(filepath.Join(r.Worktree, r.PRD), []byte(longPRD("Inhalt aus dem Worktree")), 0o644)
	m = drive(t, m, keys("vG")...)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Inhalt aus dem Worktree") {
		t.Errorf("Datei nicht aus dem Worktree:\n%s", view)
	}
	if m.prdView.dir != r.Worktree {
		t.Errorf("Ordner: %q", m.prdView.dir)
	}
}

func TestPRDViewNoRefresh(t *testing.T) {
	m, path := setupPRDView(t)
	m = drive(t, m, keys("vG")...)
	os.WriteFile(path, []byte(longPRD("Neuer Inhalt")), 0o644)
	m = drive(t, m, runsLoadedMsg{runs: loadRuns()}, m.loadProjects("")())
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Ganz unten im Hauptordner") || strings.Contains(view, "Neuer Inhalt") {
		t.Errorf("Inhalt hat sich geändert:\n%s", view)
	}
}

func TestPRDViewUnreadable(t *testing.T) {
	m, path := setupPRDView(t)
	os.Remove(path)
	m = drive(t, m, keys("v")...)
	if m.mode != modeNormal || m.prdView != nil || !m.messageErr || !strings.HasPrefix(m.message, "PRD nicht lesbar:") {
		t.Errorf("Modus %v, Meldung %q", m.mode, m.message)
	}
}

func TestPRDViewEdited(t *testing.T) {
	m, path := setupPRDView(t)
	m = drive(t, m, keys("vjjjj")...)
	offset := m.prdView.viewport.YOffset
	if offset != 4 {
		t.Fatalf("YOffset %d", offset)
	}
	os.WriteFile(path, []byte(strings.Replace(longPRD("x"), "Abschnitt 3\n", "Geänderter Abschnitt\n", 1)), 0o644)
	m = drive(t, m, prdEditedMsg{path: path})
	if !strings.Contains(ansi.Strip(m.View()), "Geänderter Abschnitt") {
		t.Errorf("neuer Inhalt fehlt:\n%s", ansi.Strip(m.View()))
	}
	if m.prdView.viewport.YOffset != offset {
		t.Errorf("YOffset %d, erwartet %d", m.prdView.viewport.YOffset, offset)
	}

	m = drive(t, m, prdEditedMsg{path: path, err: fmt.Errorf("exit status 1")})
	if m.message != "Editor: exit status 1" || m.prdView == nil {
		t.Errorf("Editor-Fehler: %q", m.message)
	}

	os.Remove(path)
	m = drive(t, m, prdEditedMsg{path: path})
	if m.mode != modeNormal || m.prdView != nil || !strings.HasPrefix(m.message, "PRD nicht lesbar:") {
		t.Errorf("gelöschte Datei: Modus %v, Meldung %q", m.mode, m.message)
	}
}

func TestEditorCommand(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if got := editorCommand(); strings.Join(got, "|") != "vi" {
		t.Errorf("ohne Variablen: %q", got)
	}
	t.Setenv("EDITOR", "code -w")
	if got := editorCommand(); strings.Join(got, "|") != "code|-w" {
		t.Errorf("EDITOR: %q", got)
	}
	t.Setenv("VISUAL", "nvim")
	if got := editorCommand(); strings.Join(got, "|") != "nvim" {
		t.Errorf("VISUAL: %q", got)
	}
}

func TestPRDViewEditorHerdr(t *testing.T) {
	m, path := setupPRDView(t)
	calls := fakeHerdr(t)
	t.Setenv("VISUAL", "nvim")
	m = drive(t, m, keys("v")...)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = runCmd(t, next.(Model), cmd)
	got, _ := os.ReadFile(calls)
	if !strings.Contains(string(got), "nvim "+path) {
		t.Errorf("herdr-Aufrufe:\n%s", got)
	}
	if m.mode != modePRDView || m.prdView == nil {
		t.Errorf("Fenster geschlossen: %v", m.mode)
	}
}

func TestPRDViewFitsTerminal(t *testing.T) {
	m, _ := setupPRDView(t)
	for _, size := range [][2]int{{80, 20}, {160, 50}} {
		m = drive(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if m.mode != modePRDView {
			m = drive(t, m, keys("v")...)
		}
		lines := strings.Split(m.View(), "\n")
		if len(lines) != size[1] {
			t.Errorf("%dx%d: %d Zeilen", size[0], size[1], len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w > size[0] {
				t.Errorf("%dx%d: Zeile %d ist %d breit", size[0], size[1], i, w)
			}
		}
	}
}
