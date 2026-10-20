package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
)

// drive schickt Nachrichten an das Modell und führt zurückgegebene Lade-Befehle aus.
func drive(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	for _, msg := range msgs {
		next, cmd := m.Update(msg)
		m = next.(Model)
		if cmd == nil {
			continue
		}
		if loaded, ok := cmd().(projectsLoadedMsg); ok {
			next, _ = m.Update(loaded)
			m = next.(Model)
		}
	}
	return m
}

func keys(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return msgs
}

func newTestProject(t *testing.T, root, name string) string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := filepath.Join(root, name)
	prd := filepath.Join(dir, "prd", "auth")
	if err := os.MkdirAll(prd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prd, "token.md"), []byte("# PRD: Token\n\n> Status: geplant · Scope: Backend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return dir
}

func TestAddAndRemoveProject(t *testing.T) {
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	cfgPath := filepath.Join(root, "config.yaml")
	cfg, _ := config.Load(cfgPath)

	m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, m.Init()())
	if !strings.Contains(ansi.Strip(m.View()), "drücke a") {
		t.Errorf("Hinweis für leere Liste fehlt:\n%s", ansi.Strip(m.View()))
	}

	m = drive(t, m, keys("a")...)
	m.input.SetValue(filepath.Join(root, "al"))
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != dir+"/" {
		t.Fatalf("Tab-Vervollständigung: %q", m.input.Value())
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	view := ansi.Strip(m.View())
	for _, want := range []string{"alpha", "auth/token", "○ geplant", "Scope: Backend", "ungetrackt"} {
		if !strings.Contains(view, want) {
			t.Errorf("%q fehlt in:\n%s", want, view)
		}
	}
	if saved, _ := config.Load(cfgPath); len(saved.Projects) != 1 {
		t.Errorf("Konfiguration nicht gespeichert: %+v", saved.Projects)
	}

	m = drive(t, m, keys("dn")...)
	if len(m.cfg.Projects) != 1 {
		t.Error("n hat trotzdem entfernt")
	}
	m = drive(t, m, keys("dj")...)
	if saved, _ := config.Load(cfgPath); len(saved.Projects) != 0 || len(m.projects) != 0 {
		t.Errorf("Projekt nicht entfernt: %+v", saved.Projects)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("Projektordner gelöscht")
	}
}

func TestAddProjectError(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg, _ := config.Load(cfgPath)
	m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 100, Height: 24})
	m = drive(t, m, keys("a")...)
	m.input.SetValue(filepath.Join(root, "fehlt"))
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeAddProject || !m.messageErr {
		t.Errorf("Fehler nicht angezeigt: mode=%v message=%q", m.mode, m.message)
	}
	if _, err := os.Stat(cfgPath); err == nil {
		t.Error("Konfiguration trotz Fehler geschrieben")
	}
}

func TestViewFitsTerminal(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg, _ := config.Load(cfgPath)
	for _, n := range []string{"alpha", "beta"} {
		if _, err := cfg.AddProject(newTestProject(t, root, n)); err != nil {
			t.Fatal(err)
		}
	}
	for _, size := range [][2]int{{80, 20}, {160, 50}} {
		m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = drive(t, m, m.Init()())
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
