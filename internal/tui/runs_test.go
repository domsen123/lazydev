package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// setupRun legt ein Projekt an, dessen PRD ein Lauf in seinen Worktree verschoben hat.
func setupRun(t *testing.T) Model {
	t.Helper()
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	wt := filepath.Join(root, "wt")
	os.MkdirAll(filepath.Join(wt, "prd", "auth"), 0o755)
	os.WriteFile(filepath.Join(wt, "prd", "auth", "moved.md"), []byte("# PRD: Verschoben\n\n> Status: geplant\n"), 0o644)

	r := &state.Run{PRD: "prd/auth/moved.md", Branch: "feat/moved", Worktree: wt, Port: 3105,
		Phase: project.PhaseMergeQueue, Note: "autoMerge ist aus"}
	cost := 0.25
	r.StartStep("implement")
	r.Steps[0].CostUSD = &cost
	if err := state.Save("alpha", "moved", r); err != nil {
		t.Fatal(err)
	}
	log := `{"type":"lazydev","time":"2026-10-20T14:02:00+02:00","text":"Schritt Umsetzung"}
{"ts":"2026-10-20T14:03:00+02:00","type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"server/a.ts"}}]}}
{"ts":"2026-10-20T14:04:00+02:00","type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":"kaputt"}]}}
`
	os.WriteFile(state.LogPath("alpha", "moved", "implement"), []byte(log), 0o644)

	cfg := &config.Config{MaxParallel: 3, PortBase: 3100, Claude: config.Claude{Binary: "claude"}, Projects: []config.Project{{Path: dir}}}
	m := New(filepath.Join(root, "config.yaml"), cfg)
	m = drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	cmd := m.Init()
	return drive(t, m, cmd())
}

func TestRunShownWithWorktreePRD(t *testing.T) {
	m := setupRun(t)
	if len(m.projects) != 1 || len(m.projects[0].PRDs) != 2 {
		t.Fatalf("PRDs: %+v", m.projects)
	}
	// Das verschobene PRD steht alphabetisch vor token.
	view := ansi.Strip(m.View())
	for _, want := range []string{"auth/moved", "Merge-Queue", "3105", "feat/moved", "0.25 USD", "autoMerge ist aus",
		clock("2026-10-20T14:03:00+02:00") + " Edit server/a.ts", "Fehler: kaputt", "lazydev Schritt Umsetzung"} {
		if !strings.Contains(view, want) {
			t.Errorf("Ansicht ohne %q:\n%s", want, view)
		}
	}
}

func TestLogEventAndConfirm(t *testing.T) {
	m := setupRun(t)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusLog || m.logCursor != 2 {
		t.Fatalf("Fokus %v, Cursor %d", m.focus, m.logCursor)
	}
	m = drive(t, m, keys("k")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeEvent || !strings.Contains(m.View(), `"file_path": "server/a.ts"`) {
		t.Errorf("Ereignis:\n%s", m.View())
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	// x ohne laufenden Runner meldet einen Fehler, s fragt nach und bricht mit n ab.
	m = drive(t, m, keys("x")...)
	if !m.messageErr || !strings.Contains(m.message, "kein Runner") {
		t.Errorf("x: %q", m.message)
	}
	m = drive(t, m, keys("s")...)
	if m.mode != modeConfirm || !strings.Contains(m.confirm.question, "ab Schritt implement") {
		t.Fatalf("s: Modus %v, %+v", m.mode, m.confirm)
	}
	m = drive(t, m, keys("n")...)
	if m.mode != modeNormal || m.message != "Abgebrochen" {
		t.Errorf("n: %v %q", m.mode, m.message)
	}
	// X fragt zweimal.
	m = drive(t, m, keys("Xj")...)
	if m.mode != modeConfirm || !strings.Contains(m.confirm.question, "Wirklich") {
		t.Errorf("zweite Rückfrage fehlt: %+v", m.confirm)
	}
}

func TestLogCacheReadsIncrementally(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	r := &state.Run{}
	r.StartStep("setup")
	path := state.LogPath("p", "x", "setup")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"type":"lazydev","text":"eins"}`+"\n"+`{"type":"lazydev","te`), 0o644)
	c := logCache{}
	if got := c.lines("p", "x", r); len(got) != 1 {
		t.Fatalf("erste Lesung: %+v", got)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`xt":"zwei"}` + "\n")
	f.Close()
	got := c.lines("p", "x", r)
	if len(got) != 2 || got[1].text != "lazydev zwei" {
		t.Errorf("zweite Lesung: %+v", got)
	}
}

func TestHelpAndCost(t *testing.T) {
	m := setupRun(t)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "0.25 $") || !strings.Contains(view, "alpha · main · 0.25 USD") || !strings.Contains(view, "? Hilfe") {
		t.Errorf("Kosten oder Hilfe-Hinweis fehlen:\n%s", view)
	}
	m = drive(t, m, keys("?")...)
	if m.mode != modeHelp || !strings.Contains(ansi.Strip(m.View()), "Dev-Server im Worktree starten") {
		t.Errorf("Hilfe:\n%s", ansi.Strip(m.View()))
	}
	m = drive(t, m, keys("j")...)
	if m.mode != modeNormal {
		t.Error("Hilfe schließt nicht")
	}
}
