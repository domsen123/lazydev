package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/prompts"
	"lazydev/internal/state"
)

// setupWizard legt ein Projekt mit getrackten .md-Dateien und den Branches main und next an. Die
// zentrale config.yaml trägt noch die Übergangsfelder.
func setupWizard(t *testing.T) (m Model, dir, cfgPath string) {
	t.Helper()
	root := t.TempDir()
	dir = newTestProject(t, root, "alpha")
	for rel, content := range map[string]string{
		"CLAUDE.md": "x", "ARCHITECTURE.md": "x", "docs/a.md": "x", "prd/alt/done.md": "# PRD: Alt\n\n> Status: umgesetzt (2026-10-01)\n",
	} {
		writeTestFile(t, dir, rel, content)
	}
	for _, args := range [][]string{
		{"add", "CLAUDE.md", "ARCHITECTURE.md", "docs/a.md", "prd/alt/done.md"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init"},
		{"branch", "next"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// Nur lokal vorhanden, deshalb nicht in der Liste.
	writeTestFile(t, dir, "LOKAL.md", "x")
	off := false
	cfgPath = filepath.Join(root, "config.yaml")
	cfg := &config.Config{MaxParallel: 3, PortBase: 3100, Claude: config.Claude{Binary: "claude"}, Projects: []config.Project{{
		Path: dir, AutoMerge: &off, BaseBranch: "next", AllowedTools: []string{"Bash(make:*)"}, Hooks: config.Hooks{Setup: "./s.sh"},
	}}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	m = drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 120, Height: 40})
	return drive(t, m, m.Init()()), dir, cfgPath
}

func TestInitWizard(t *testing.T) {
	m, dir, cfgPath := setupWizard(t)

	// Esc bricht ohne Änderung ab.
	m = drive(t, m, keys("i")...)
	if m.mode != modeInit {
		t.Fatalf("i: %v %q", m.mode, m.message)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if _, err := os.Stat(filepath.Join(dir, ".lazydev")); m.mode != modeNormal || err == nil {
		t.Fatalf("nach Esc: %v, %v", m.mode, err)
	}

	m = drive(t, m, keys("i")...)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, ".lazydev/ anlegen · alpha") || !strings.Contains(view, "1/4 Ziel-Branch") || !strings.Contains(view, "> next") {
		t.Errorf("Seite Ziel-Branch ohne Vorauswahl next:\n%s", view)
	}
	checkViewSize(t, m)

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	view = ansi.Strip(m.View())
	for _, want := range []string{"2/4 Knowledge-Dateien", "> [x] ARCHITECTURE.md", "  [x] CLAUDE.md", "  [ ] docs/a.md"} {
		if !strings.Contains(view, want) {
			t.Errorf("Seite Knowledge ohne %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "LOKAL.md") || strings.Contains(view, "done.md") {
		t.Errorf("ungetrackte Datei oder PRD in der Liste:\n%s", view)
	}
	if got := ansi.Strip(m.helpLine()); !strings.Contains(got, "Leertaste markieren") || !strings.Contains(got, "Backspace zurück") {
		t.Errorf("Fußzeile: %q", got)
	}
	// docs/a.md markieren, ARCHITECTURE.md abwählen.
	m = drive(t, m, keys("jj ")...)
	m = drive(t, m, keys("kk")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeySpace})

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	view = ansi.Strip(m.View())
	if !strings.Contains(view, "3/4 Schritte je Datei") || !strings.Contains(view, "> CLAUDE.md  [c] [i] [ ] [ ]") ||
		!strings.Contains(view, "  docs/a.md  [c] [i] [ ] [ ]") || strings.Contains(view, "ARCHITECTURE.md") {
		t.Errorf("Seite Schritte:\n%s", view)
	}
	// CLAUDE.md für alle Schritte, docs/a.md nur für implement.
	m = drive(t, m, keys("34j1")...)
	view = ansi.Strip(m.View())
	if !strings.Contains(view, "  CLAUDE.md  [c] [i] [r] [m]") || !strings.Contains(view, "> docs/a.md  [ ] [i] [ ] [ ]") {
		t.Errorf("nach dem Umschalten:\n%s", view)
	}
	// Backspace geht zurück, die Auswahl bleibt.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(ansi.Strip(m.View()), "2/4 Knowledge-Dateien") {
		t.Errorf("Backspace:\n%s", ansi.Strip(m.View()))
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyEnter})

	view = ansi.Strip(m.View())
	for _, want := range []string{"4/4 Zusammenfassung", "baseBranch: next", "  all:", "- CLAUDE.md", "implement:", "- docs/a.md",
		"- Bash(make:*)", "setup: ./s.sh", "lazydev legt an:", ".lazydev/config.yaml", ".lazydev/review.md"} {
		if !strings.Contains(view, want) {
			t.Errorf("Zusammenfassung ohne %q:\n%s", want, view)
		}
	}
	checkViewSize(t, m)

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal || m.messageErr || m.message != ".lazydev/ angelegt. Committe den Ordner, damit das Team ihn bekommt." {
		t.Fatalf("nach dem Anlegen: %v %q", m.mode, m.message)
	}
	repo, exists, err := config.LoadRepo(dir)
	if err != nil || !exists {
		t.Fatalf("config.yaml: %v, %v", exists, err)
	}
	if repo.BaseBranch != "next" || repo.Preset != "" || !slices.Equal(repo.Knowledge["all"], []string{"CLAUDE.md"}) ||
		!slices.Equal(repo.Knowledge["implement"], []string{"docs/a.md"}) || len(repo.Knowledge) != 2 ||
		!slices.Equal(repo.AllowedTools, []string{"Bash(make:*)"}) || repo.Hooks.Setup != "./s.sh" {
		t.Errorf("config.yaml: %+v", repo)
	}
	for _, step := range prompts.Steps {
		if data, err := os.ReadFile(filepath.Join(dir, prompts.FileName(step))); err != nil || string(data) != prompts.Default(step) {
			t.Errorf("%s: %v", prompts.FileName(step), err)
		}
	}
	saved, _ := config.Load(cfgPath)
	p := saved.Projects[0]
	if p.BaseBranch != "" || p.Preset != "" || p.AllowedTools != nil || p.Hooks != (config.Hooks{}) || p.AutoMergeEnabled() {
		t.Errorf("zentrale config.yaml: %+v", p)
	}
	if out, _ := exec.Command("git", "-C", dir, "status", "--porcelain", "--", ".lazydev").Output(); string(out) != "?? .lazydev/\n" {
		t.Errorf("lazydev hat committet: %q", out)
	}
	if view := ansi.Strip(m.View()); strings.Contains(view, "ohne .lazydev") || !strings.Contains(view, "(Ziel next)") {
		t.Errorf("Titel nach dem Anlegen:\n%s", view)
	}

	// Ein zweites i öffnet nichts.
	m = drive(t, m, keys("i")...)
	if m.mode != modeNormal || !m.messageErr || m.message != ".lazydev/ existiert schon. Bearbeite die Dateien im Editor." {
		t.Errorf("i bei vorhandenem Ordner: %v %q", m.mode, m.message)
	}
}

// TestInitWizardWithoutKnowledge prüft, dass der Wizard ohne markierte Datei die Seite Schritte
// überspringt und das Preset erkennt.
func TestInitWizardWithoutKnowledge(t *testing.T) {
	m, dir, _ := setupWizard(t)
	writeTestFile(t, dir, "drizzle.config.ts", "")
	writeTestFile(t, dir, "server/database/migrations/.keep", "")
	m = drive(t, m, keys("i")...)
	m = drive(t, m, keys("k")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(t, m, keys(" j ")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "4/4 Zusammenfassung") || strings.Contains(view, "knowledge") ||
		strings.Contains(view, "baseBranch") || !strings.Contains(view, "preset: nuxt-drizzle") {
		t.Errorf("Zusammenfassung ohne Knowledge:\n%s", view)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(ansi.Strip(m.View()), "2/4 Knowledge-Dateien") {
		t.Errorf("Backspace ohne markierte Datei:\n%s", ansi.Strip(m.View()))
	}
}

func TestIgnoredLocalSettings(t *testing.T) {
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	writeTestFile(t, dir, ".lazydev/config.yaml", "baseBranch: next\n")
	cfg := &config.Config{MaxParallel: 3, PortBase: 3100, Claude: config.Claude{Binary: "claude"},
		Projects: []config.Project{{Path: dir, BaseBranch: "dev", Hooks: config.Hooks{Dev: "./dev.sh"}}}}
	m := drive(t, New(filepath.Join(root, "config.yaml"), cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, m.Init()())
	if m.message != "alpha: lokale Einstellungen ignoriert (baseBranch, hooks), es gilt .lazydev/config.yaml" {
		t.Errorf("Meldung: %q", m.message)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "(Ziel next)") {
		t.Errorf("Ziel-Branch aus .lazydev/config.yaml fehlt:\n%s", view)
	}
	// Die Meldung kommt nur einmal.
	m = drive(t, m, keys("r")...)
	if m.message != "" {
		t.Errorf("Meldung nach erneutem Laden: %q", m.message)
	}
}

func TestStartAsksForTrust(t *testing.T) {
	m := setupRun(t)
	dir := m.selectedProject().Path
	writeTestFile(t, dir, ".lazydev/config.yaml", "baseBranch: main\nhooks:\n  setup: ./s.sh\n")

	m = drive(t, m, keys("s")...)
	view := ansi.Strip(m.View())
	if m.mode != modeConfirm || m.confirm.question != "Erlauben? (j/n)" ||
		!strings.Contains(view, ".lazydev/config.yaml von alpha setzt Hooks oder Freigaben:") || !strings.Contains(view, "setup: ./s.sh") ||
		strings.Contains(view, "baseBranch") {
		t.Fatalf("Rückfrage zum Vertrauen (%v):\n%s", m.mode, view)
	}
	checkViewSize(t, m)
	repo, _, _ := config.LoadRepo(dir)
	m = drive(t, m, keys("n")...)
	if m.mode != modeNormal || state.Trusted("alpha", repo.TrustHash()) {
		t.Fatalf("nach n: %v", m.mode)
	}

	m = drive(t, m, keys("sj")...)
	if !state.Trusted("alpha", repo.TrustHash()) || m.mode != modeConfirm || !strings.Contains(m.confirm.question, "ab Schritt implement") {
		t.Fatalf("nach j: %v %+v", m.mode, m.confirm)
	}
	m = drive(t, m, keys("n")...)

	// Bestätigt fragt s nur noch nach dem Start.
	m = drive(t, m, keys("s")...)
	if m.mode != modeConfirm || !strings.Contains(m.confirm.question, "ab Schritt implement") {
		t.Errorf("s nach Bestätigung: %+v", m.confirm)
	}
}
