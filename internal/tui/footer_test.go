package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestFooterFollowsFocusAndState(t *testing.T) {
	m := setupRun(t)
	help := func() string { return ansi.Strip(m.helpLine()) }

	if got, want := help(), " ? Hilfe · q beenden · a Projekt · d entfernen · b Ziel-Branch · D Docker · n neues PRD"; got != want {
		t.Errorf("Projekte:\n got %q\nwant %q", got, want)
	}

	// Der Lauf scheiterte in der Umsetzung: s setzt fort, ohne Sitzung fehlt o.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got, want := help(), " ? Hilfe · q beenden · n neues PRD · v ansehen · s fortsetzen · X verwerfen · p dev · w Shell · S sortieren"; got != want {
		t.Errorf("PRD mit Fehler:\n got %q\nwant %q", got, want)
	}

	// Nach dem Review steht m bereit, die Sitzung öffnet o.
	r := m.selectedRun()
	r.Steps = nil
	for _, name := range []string{"setup", "implement", "review"} {
		r.StartStep(name)
		r.EndStep(0, true)
	}
	r.Steps[2].SessionID = "abc"
	if err := state.Save("alpha", "moved", r); err != nil {
		t.Fatal(err)
	}
	m = drive(t, m, runsLoadedMsg{runs: loadRuns()})
	if got, want := help(), " ? Hilfe · q beenden · n neues PRD · v ansehen · s fortsetzen · m mergen · X verwerfen · o Sitzung · p dev · w Shell · S sortieren"; got != want {
		t.Errorf("Merge-Queue:\n got %q\nwant %q", got, want)
	}

	// Ohne Lauf und ohne Branch gibt es nur starten, nichts zu verwerfen.
	m = drive(t, m, keys("j")...)
	if got, want := help(), " ? Hilfe · q beenden · n neues PRD · v ansehen · s starten · S sortieren"; got != want {
		t.Errorf("geplant:\n got %q\nwant %q", got, want)
	}

	// Ein gemergtes PRD wandert in die Gruppe der erledigten und zeigt kein s, m und x mehr.
	r.Phase = project.PhaseMerged
	r.StartStep("merge")
	r.EndStep(0, true)
	if err := state.Save("alpha", "moved", r); err != nil {
		t.Fatal(err)
	}
	// Nach dem Merge liegt das PRD im Hauptordner, der Worktree ist weg.
	main := filepath.Join(m.selectedProject().Path, "prd", "auth", "moved.md")
	if err := os.WriteFile(main, []byte("# PRD: Verschoben\n\n> Status: umgesetzt (2026-10-20)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(r.Worktree); err != nil {
		t.Fatal(err)
	}
	m = drive(t, m, runsLoadedMsg{runs: loadRuns()}, m.loadProjects("")())
	if got, want := help(), " ? Hilfe · q beenden · n neues PRD · e erledigte · S sortieren"; got != want {
		t.Errorf("Gruppenzeile:\n got %q\nwant %q", got, want)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(t, m, keys("j")...)
	if got, want := help(), " ? Hilfe · q beenden · n neues PRD · v ansehen · X verwerfen · e erledigte · S sortieren"; got != want {
		t.Errorf("gemergt:\n got %q\nwant %q", got, want)
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got, want := help(), " ? Hilfe · q beenden · j/k scrollen · Enter Ereignis · G Ende · l Log groß"; got != want {
		t.Errorf("Log:\n got %q\nwant %q", got, want)
	}
}
