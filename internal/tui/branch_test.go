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

func TestBranchPicker(t *testing.T) {
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	for _, args := range [][]string{
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"branch", "next"}, {"branch", "feat/other"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{MaxParallel: 3, PortBase: 3100, Claude: config.Claude{Binary: "claude"}, Projects: []config.Project{{Path: dir}}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, m.Init()())

	// Ohne .lazydev/ gibt es nichts zu schreiben.
	m = drive(t, m, keys("b")...)
	if m.mode != modeNormal || !m.messageErr || m.message != "Kein .lazydev/ im Projekt. Mit i anlegen." {
		t.Fatalf("b ohne .lazydev: %v %q", m.mode, m.message)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "alpha · main · ohne .lazydev") {
		t.Errorf("Titel ohne Hinweis auf .lazydev:\n%s", view)
	}

	writeTestFile(t, dir, ".lazydev/config.yaml", "# Team\npreset: nuxt-drizzle\n")
	m = drive(t, m, keys("r")...)
	m = drive(t, m, keys("b")...)
	view := ansi.Strip(m.View())
	if m.mode != modeBranch || !strings.Contains(view, "> main (aktuell)") || !strings.Contains(view, "  next") ||
		strings.Contains(view, "feat/other") {
		t.Fatalf("Branch-Auswahl (%v, %q):\n%s", m.mode, m.message, view)
	}

	central, _ := os.ReadFile(cfgPath)
	m = drive(t, m, keys("j")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	data, _ := os.ReadFile(config.RepoPath(dir))
	if m.mode != modeNormal || string(data) != "# Team\nbaseBranch: next\npreset: nuxt-drizzle\n" {
		t.Fatalf("nach Enter: %v %q\n%s", m.mode, m.message, data)
	}
	if after, _ := os.ReadFile(cfgPath); string(after) != string(central) {
		t.Errorf("zentrale config.yaml geändert:\n%s", after)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "alpha · main (Ziel next)") || strings.Contains(view, "ohne .lazydev") {
		t.Errorf("Titel ohne Ziel-Branch:\n%s", view)
	}

	// Zurück auf main: Der Standard steht nicht in der Datei.
	m = drive(t, m, keys("bk")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	data, _ = os.ReadFile(config.RepoPath(dir))
	if string(data) != "# Team\npreset: nuxt-drizzle\n" {
		t.Errorf("main als Standard:\n%s", data)
	}
}
