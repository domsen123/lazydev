package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
)

func TestDetailRendersMarkdownAndScrolls(t *testing.T) {
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	var b strings.Builder
	b.WriteString("# PRD: Token\n\n> Status: geplant · Scope: **Backend** mit `code`\n")
	for _, item := range []string{"eins", "zwei", "drei", "vier", "fünf", "sechs", "sieben", "acht"} {
		b.WriteString("> - Punkt " + item + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "prd", "auth", "token.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	cfg, _ := config.Load(cfgPath)
	if _, err := cfg.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(t, m, m.Init()())

	view := ansi.Strip(m.View())
	for _, want := range []string{"Status    geplant", "Datei     prd/auth/token.md", "Scope: Backend mit"} {
		if !strings.Contains(view, want) {
			t.Errorf("%q fehlt in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "**Backend**") || strings.Contains(view, "Punkt acht") {
		t.Errorf("Markdown roh oder Box nicht begrenzt:\n%s", view)
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRight}, tea.KeyMsg{Type: tea.KeyRight}, tea.KeyMsg{Type: tea.KeyEnd})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "• Punkt acht") || !strings.Contains(view, "100 %") {
		t.Errorf("Detailansicht scrollt nicht ans Ende:\n%s", view)
	}
	m = drive(t, m, keys("kkkkkkkkkk")...)
	if !m.detail.AtTop() {
		t.Errorf("k scrollt nicht nach oben: YOffset=%d", m.detail.YOffset)
	}
}
