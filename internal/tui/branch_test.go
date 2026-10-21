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
	m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, m.Init()())

	m = drive(t, m, keys("b")...)
	view := ansi.Strip(m.View())
	if m.mode != modeBranch || !strings.Contains(view, "> main (aktuell)") || !strings.Contains(view, "  next") ||
		strings.Contains(view, "feat/other") {
		t.Fatalf("Branch-Auswahl (%v, %q):\n%s", m.mode, m.message, view)
	}

	m = drive(t, m, keys("j")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	data, _ := os.ReadFile(cfgPath)
	if m.mode != modeNormal || cfg.Projects[0].BaseBranch != "next" || !strings.Contains(string(data), "baseBranch: next") {
		t.Fatalf("nach Enter: %v %+v\n%s", m.mode, cfg.Projects[0], data)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "alpha · main (Ziel next)") {
		t.Errorf("Titel ohne Ziel-Branch:\n%s", view)
	}

	// Zurück auf main: Der Standard steht nicht in der Datei.
	m = drive(t, m, keys("bk")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	data, _ = os.ReadFile(cfgPath)
	if cfg.Projects[0].BaseBranch != "" || strings.Contains(string(data), "baseBranch") {
		t.Errorf("main als Standard: %+v\n%s", cfg.Projects[0], data)
	}
}
