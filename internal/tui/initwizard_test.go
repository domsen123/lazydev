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

	// Ein zweites i öffnet den Modus „bearbeiten“.
	m = drive(t, m, keys("i")...)
	if view := ansi.Strip(m.View()); m.mode != modeInit || !strings.Contains(view, ".lazydev/ bearbeiten · alpha") ||
		!strings.Contains(view, "> next (aktuell)") {
		t.Errorf("i bei vorhandenem Ordner (%v %q):\n%s", m.mode, m.message, view)
	}
}

// setupEditWizard legt .lazydev/ mit Kommentaren, Hooks, Freigaben und einer nicht getrackten
// Knowledge-Datei an. review.md ist eine Kopie des Standards, implement.md ein eigener Prompt,
// create.md und merge.md fehlen.
func setupEditWizard(t *testing.T) (m Model, dir, cfgPath string) {
	t.Helper()
	t.Setenv("VISUAL", "true")
	t.Setenv("HERDR_ENV", "")
	m, dir, cfgPath = setupWizard(t)
	writeTestFile(t, dir, ".lazydev/config.yaml", `# Team-Einstellungen
baseBranch: next
preset: nuxt-drizzle
# Pflichtlektüre
knowledge:
  all: [CLAUDE.md]
  review: [docs/a.md, GONE.md]
allowedTools:
  - "Bash(make:*)"
hooks:
  setup: ./s.sh # Setup
`)
	writeTestFile(t, dir, prompts.FileName(prompts.Review), prompts.Default(prompts.Review))
	writeTestFile(t, dir, prompts.FileName(prompts.Implement), "Eigener Prompt\n")
	// Die Meldung zu den Übergangsfeldern der zentralen config.yaml kommt einmal, hier beim Laden.
	return drive(t, m, m.loadProjects("")()), dir, cfgPath
}

func TestEditWizard(t *testing.T) {
	m, dir, cfgPath := setupEditWizard(t)
	central, _ := os.ReadFile(cfgPath)

	m = drive(t, m, keys("i")...)
	view := ansi.Strip(m.View())
	if m.mode != modeInit || !strings.Contains(view, ".lazydev/ bearbeiten · alpha") || !strings.Contains(view, "1/5 Ziel-Branch") ||
		!strings.Contains(view, "> next (aktuell)") {
		t.Fatalf("Seite Ziel-Branch (%v %q):\n%s", m.mode, m.message, view)
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	view = ansi.Strip(m.View())
	for _, want := range []string{"2/5 Knowledge-Dateien", "> [ ] ARCHITECTURE.md", "  [x] CLAUDE.md", "  [x] docs/a.md",
		"  [x] GONE.md (nicht getrackt)"} {
		if !strings.Contains(view, want) {
			t.Errorf("Seite Knowledge ohne %q:\n%s", want, view)
		}
	}
	// ARCHITECTURE.md neu markieren: create und implement.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeySpace}, tea.KeyMsg{Type: tea.KeyEnter})
	view = ansi.Strip(m.View())
	for _, want := range []string{"3/5 Schritte je Datei", "> ARCHITECTURE.md  [c] [i] [ ] [ ]", "  CLAUDE.md        [c] [i] [r] [m]",
		"  docs/a.md        [ ] [ ] [r] [ ]", "  GONE.md          [ ] [ ] [r] [ ]"} {
		if !strings.Contains(view, want) {
			t.Errorf("Seite Schritte ohne %q:\n%s", want, view)
		}
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	view = ansi.Strip(m.View())
	for _, want := range []string{"4/5 Prompts", "Änderungen an Prompts gelten sofort.",
		"> create     .lazydev/create.md     Standard von lazydev", "  implement  .lazydev/implement.md  eigener Prompt",
		"  review     .lazydev/review.md     Kopie des Standards", "  merge      .lazydev/merge.md      Standard von lazydev"} {
		if !strings.Contains(view, want) {
			t.Errorf("Seite Prompts ohne %q:\n%s", want, view)
		}
	}
	if got := ansi.Strip(m.helpLine()); !strings.Contains(got, "↑ ↓ wählen · o Editor · r Standard · Enter weiter · Backspace zurück · Esc abbrechen") {
		t.Errorf("Fußzeile: %q", got)
	}
	checkViewSize(t, m)

	// r bewirkt beim Standard nichts.
	m = drive(t, m, keys("r")...)
	if m.mode != modeInit {
		t.Errorf("r beim Standard: %v", m.mode)
	}

	// o legt die fehlende merge.md mit dem Standard an.
	m = drive(t, m, keys("jjjo")...)
	if data, err := os.ReadFile(filepath.Join(dir, prompts.FileName(prompts.Merge))); err != nil || string(data) != prompts.Default(prompts.Merge) {
		t.Errorf("merge.md nach o: %v", err)
	}
	m = drive(t, m, wizardEditedMsg{})
	if m.mode != modeInit || m.wizard.page != pagePrompts || m.wizard.states[3] != prompts.StateCopy {
		t.Errorf("nach dem Editor: %v %d %v", m.mode, m.wizard.page, m.wizard.states)
	}

	// r fragt nach und löscht review.md nach j.
	m = drive(t, m, keys("kr")...)
	if m.mode != modeConfirm || m.confirm.question != ".lazydev/review.md löschen und den Standard von lazydev nutzen? (j/n)" ||
		!strings.Contains(ansi.Strip(m.View()), "4/5 Prompts") {
		t.Fatalf("Rückfrage zu r (%v): %+v", m.mode, m.confirm)
	}
	m = drive(t, m, keys("j")...)
	if _, err := os.Stat(filepath.Join(dir, prompts.FileName(prompts.Review))); m.mode != modeInit || err == nil {
		t.Errorf("nach j: %v, %v", m.mode, err)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "> review     .lazydev/review.md     Standard von lazydev") {
		t.Errorf("review nach dem Zurücksetzen:\n%s", view)
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	view = ansi.Strip(m.View())
	for _, want := range []string{"5/5 Zusammenfassung", "baseBranch: next", "- GONE.md", "- ARCHITECTURE.md", "setup: ./s.sh",
		"Prompts:", "create: Standard von lazydev", "implement: eigener Prompt", "review: Standard von lazydev", "merge: Kopie des Standards"} {
		if !strings.Contains(view, want) {
			t.Errorf("Zusammenfassung ohne %q:\n%s", want, view)
		}
	}
	if got := ansi.Strip(m.helpLine()); !strings.Contains(got, "Enter speichern") {
		t.Errorf("Fußzeile: %q", got)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(ansi.Strip(m.View()), "4/5 Prompts") {
		t.Errorf("Backspace:\n%s", ansi.Strip(m.View()))
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal || m.messageErr || m.message != ".lazydev/config.yaml gespeichert. Committe die Änderung, damit das Team sie bekommt." {
		t.Fatalf("nach dem Speichern: %v %q", m.mode, m.message)
	}

	data, _ := os.ReadFile(config.RepoPath(dir))
	for _, want := range []string{"# Team-Einstellungen\nbaseBranch: next\npreset: nuxt-drizzle\n# Pflichtlektüre\nknowledge:\n",
		"allowedTools:\n  - \"Bash(make:*)\"\nhooks:\n  setup: ./s.sh # Setup\n"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config.yaml ohne %q:\n%s", want, data)
		}
	}
	repo, _, err := config.LoadRepo(dir)
	if err != nil || !slices.Equal(repo.Knowledge["all"], []string{"CLAUDE.md"}) || !slices.Equal(repo.Knowledge["create"], []string{"ARCHITECTURE.md"}) ||
		!slices.Equal(repo.Knowledge["implement"], []string{"ARCHITECTURE.md"}) || !slices.Equal(repo.Knowledge["review"], []string{"docs/a.md", "GONE.md"}) ||
		len(repo.Knowledge) != 4 {
		t.Errorf("knowledge: %+v, %v", repo.Knowledge, err)
	}
	if after, _ := os.ReadFile(cfgPath); string(after) != string(central) {
		t.Errorf("zentrale config.yaml geändert:\n%s", after)
	}

	// Ohne Änderung schreibt lazydev nichts.
	m = drive(t, m, keys("i")...)
	for range 5 {
		m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	}
	if m.mode != modeNormal || m.messageErr || m.message != "Keine Änderung an .lazydev/config.yaml" {
		t.Errorf("ohne Änderung: %v %q", m.mode, m.message)
	}
	if after, _ := os.ReadFile(config.RepoPath(dir)); string(after) != string(data) {
		t.Errorf("config.yaml ohne Änderung neu geschrieben:\n%s", after)
	}
}

// TestEditWizardUntrackedRemoved prüft, dass eine nicht getrackte Datei nach dem Abwählen entfällt
// und main als Ziel-Branch nicht in der Datei steht.
func TestEditWizardUntrackedRemoved(t *testing.T) {
	m, dir, _ := setupEditWizard(t)
	m = drive(t, m, keys("i")...)
	// Ziel-Branch main, GONE.md abwählen.
	m = drive(t, m, keys("k")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(t, m, keys("jjj")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeySpace})
	for range 4 {
		m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	}
	if m.mode != modeNormal || m.messageErr {
		t.Fatalf("nach dem Speichern: %v %q", m.mode, m.message)
	}
	data, _ := os.ReadFile(config.RepoPath(dir))
	if strings.Contains(string(data), "baseBranch") || strings.Contains(string(data), "GONE.md") ||
		!strings.Contains(string(data), "# Team-Einstellungen\npreset: nuxt-drizzle") {
		t.Errorf("config.yaml:\n%s", data)
	}
}

func TestEditWizardInvalidConfig(t *testing.T) {
	m, dir, _ := setupWizard(t)
	writeTestFile(t, dir, ".lazydev/config.yaml", "foo: 1\n")
	m = drive(t, m, keys("i")...)
	if m.mode != modeNormal || m.wizard != nil || !m.messageErr || !strings.Contains(m.message, `unbekanntes Feld "foo"`) {
		t.Errorf("ungültige config.yaml: %v %q", m.mode, m.message)
	}
}

// TestEditWizardPromptsImmediate prüft, dass Zurücksetzen auch nach Esc gilt und n nichts löscht.
func TestEditWizardPromptsImmediate(t *testing.T) {
	m, dir, _ := setupEditWizard(t)
	m = drive(t, m, keys("i")...)
	for range 3 {
		m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	}
	path := filepath.Join(dir, prompts.FileName(prompts.Implement))
	m = drive(t, m, keys("jrn")...)
	if _, err := os.Stat(path); m.mode != modeInit || err != nil {
		t.Errorf("nach n: %v, %v", m.mode, err)
	}
	m = drive(t, m, keys("rj")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if _, err := os.Stat(path); m.mode != modeNormal || err == nil {
		t.Errorf("nach Esc: %v, %v", m.mode, err)
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
