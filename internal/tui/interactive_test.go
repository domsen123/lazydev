package tui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/chatrun"
	"lazydev/internal/config"
	"lazydev/internal/prdtemplate"
	"lazydev/internal/state"
)

// fakeHerdr protokolliert die Aufrufe von herdr in einer Datei.
func fakeHerdr(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + calls + "\necho >> " + calls + "\n" +
		"[ \"$2\" = split ] && echo '{\"result\":{\"pane\":{\"pane_id\":\"w:p1\"}}}'\nexit 0\n"
	os.WriteFile(filepath.Join(dir, "herdr"), []byte(script), 0o755)
	t.Setenv("HERDR_BIN_PATH", filepath.Join(dir, "herdr"))
	t.Setenv("HERDR_ENV", "1")
	return calls
}

// runCmd führt einen Befehl aus und gibt seine Nachricht an das Modell.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("kein Befehl")
	}
	next, _ := m.Update(cmd())
	return next.(Model)
}

// fakeClaude antwortet wie claude -p und protokolliert die Argumente. Die erste Runde endet mit
// einer Frage, eine Runde mit --resume ohne Frage. FAKE_CLAUDE_MODE=exit lässt die Runde scheitern.
func fakeClaude(t *testing.T) (bin, args string) {
	t.Helper()
	dir := t.TempDir()
	args = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + args + "\necho >> " + args + "\n" +
		`[ "$FAKE_CLAUDE_MODE" = exit ] && exit 3` + "\n" +
		`echo '{"type":"system","subtype":"init","session_id":"chat-1"}'` + "\n" +
		`case "$*" in *--resume*)` + "\n" +
		`echo '{"type":"assistant","message":{"content":[{"type":"text","text":"PRD gespeichert unter prd/x.md."}]}}'` + "\n" +
		`echo '{"type":"result","is_error":false,"total_cost_usd":0.05,"result":"PRD gespeichert unter prd/x.md."}'` + "\n" +
		`exit 0;; esac` + "\n" +
		`echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"_ARCHITECTURE.md"}},{"type":"text","text":"Frage 1: Wie groß darf das Bild sein?"}]}}'` + "\n" +
		`echo '{"type":"result","is_error":false,"total_cost_usd":0.05,"result":"Frage 1: Wie groß darf das Bild sein?"}'` + "\n"
	bin = filepath.Join(dir, "claude")
	os.WriteFile(bin, []byte(script), 0o755)
	return bin, args
}

// inlineRounds lässt jede Chat-Runde in einer Goroutine statt in einem eigenen Prozess laufen.
func inlineRounds(t *testing.T, cfg *config.Config) {
	t.Helper()
	var wg sync.WaitGroup
	restore := chatrun.SetSpawn(func(projectID, id string) (int, error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chatrun.Round(cfg, projectID, id)
		}()
		return 0, nil
	})
	t.Cleanup(func() {
		wg.Wait()
		restore()
	})
}

// waitRound wartet, bis kein Chat mehr läuft, und gibt die Chats an das Modell.
func waitRound(t *testing.T, m Model) Model {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		all, _ := chatrun.All()
		running := false
		for _, e := range all {
			running = running || e.Chat.Status == chatrun.StatusRunning
		}
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Chat-Runde endet nicht")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return drive(t, m, loadChatsCmd())
}

// checkViewSize prüft, dass View genau m.height Zeilen liefert und keine breiter als m.width ist.
func checkViewSize(t *testing.T, m Model) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Errorf("Ansicht hat %d Zeilen statt %d", len(lines), m.height)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > m.width {
			t.Errorf("Zeile %d hat %d Zellen, mehr als %d: %q", i+1, w, m.width, ansi.Strip(l))
		}
	}
}

func TestNewPRDChat(t *testing.T) {
	m := setupRun(t)
	bin, argsPath := fakeClaude(t)
	m.cfg.Claude.Binary = bin
	m.cfg.Projects[0].Preset = config.PresetNuxtDrizzle
	inlineRounds(t, m.cfg)
	m = drive(t, m, keys("n")...)
	if m.mode != modeTemplate || prdtemplate.All[m.template].Name != prdtemplate.Nuxt {
		t.Fatalf("Picker nach n: Modus %v, Template %d", m.mode, m.template)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Template wählen") || !strings.Contains(view, "> Nuxt-Feature") {
		t.Errorf("Picker ohne Vorauswahl Nuxt-Feature:\n%s", view)
	}
	checkViewSize(t, m)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Neues PRD · Nuxt-Feature") ||
		!strings.Contains(view, "Beschreibe das Feature in einem Satz.") {
		t.Errorf("Fenster für die Beschreibung fehlt:\n%s", view)
	}
	checkViewSize(t, m)
	m.input.SetValue("Avatar-Upload")
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeChat || m.chat == nil {
		t.Fatalf("kein Chat nach der Beschreibung: %v %q", m.mode, m.message)
	}
	m = waitRound(t, m)
	if m.chat.state.Status != chatrun.StatusQuestion || m.chat.state.SessionID != "chat-1" || m.chat.state.PID != 0 {
		t.Fatalf("Chat nach der ersten Runde: %+v", m.chat.state)
	}
	view := ansi.Strip(m.View())
	for _, want := range []string{"Neues PRD (Nuxt-Feature): alpha", "Du: /create-prd Avatar-Upload", "· Read _ARCHITECTURE.md",
		"Claude: Frage 1: Wie groß darf das Bild sein?", "0.05 USD", "Antwort"} {
		if !strings.Contains(view, want) {
			t.Errorf("Chat ohne %q:\n%s", want, view)
		}
	}
	checkViewSize(t, m)

	// Esc blendet nur aus, die Rückfrage bleibt als Benachrichtigung.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal || m.chat != nil || !strings.Contains(m.message, "Chat läuft im Hintergrund weiter") {
		t.Errorf("nach Esc: %v %q", m.mode, m.message)
	}
	m = drive(t, m, loadChatsCmd())
	view = ansi.Strip(m.View())
	if !strings.Contains(view, "Benachrichtigungen · 1") || !strings.Contains(view, "? Avatar-Upload") || !strings.Contains(view, "alpha") ||
		!strings.Contains(view, " 1✉") {
		t.Errorf("Box oder Indikator fehlt:\n%s", view)
	}
	checkViewSize(t, m)

	m = drive(t, m, keys("N")...)
	if m.mode != modeChat || m.chat == nil || !strings.Contains(ansi.Strip(m.View()), "Claude: Frage 1") {
		t.Fatalf("N öffnet den Chat nicht: %v", m.mode)
	}
	m.chat.input.SetValue("2 MB")
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = waitRound(t, m)
	if len(m.notifications("alpha")) != 0 || strings.Contains(ansi.Strip(m.View()), "Benachrichtigungen") {
		t.Errorf("Benachrichtigung nach der Antwort: %+v", m.notifications("alpha"))
	}
	got, _ := os.ReadFile(argsPath)
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "-p|/create-prd Avatar-Upload|") || strings.Contains(lines[0], "--resume") ||
		!strings.Contains(lines[1], "-p|2 MB|") || !strings.Contains(lines[1], "--resume|chat-1|") ||
		!strings.Contains(lines[1], "Edit(prd/**)") || !strings.Contains(lines[0], "--model|opus|") {
		t.Errorf("Aufrufe von claude:\n%s", got)
	}
	if m.chat.state.CostUSD < 0.099 || m.chat.state.Status != chatrun.StatusDone {
		t.Errorf("Zustand nach der zweiten Runde: %+v", m.chat.state)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Du: 2 MB") || !strings.Contains(view, "Claude: PRD gespeichert") {
		t.Errorf("Verlauf nach der zweiten Runde:\n%s", view)
	}

	logs, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_STATE_HOME"), "lazydev", "chats", "alpha", "*.jsonl"))
	if len(logs) != 1 {
		t.Fatalf("Chat-Logs: %v", logs)
	}
	logData, _ := os.ReadFile(logs[0])
	if !strings.Contains(string(logData), `"type":"user_prompt","text":"2 MB"`) || !strings.Contains(string(logData), `"session_id":"chat-1"`) ||
		!strings.Contains(string(logData), `{"ts":"`) {
		t.Errorf("Chat-Log:\n%s", logData)
	}

	// Esc schließt einen beendeten Chat und löscht seinen Zustand, das Log bleibt.
	id := m.chat.id
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal || m.chat != nil || !strings.Contains(m.message, "Chat geschlossen. Fortsetzen: claude --resume chat-1") {
		t.Errorf("nach Esc: %v %q", m.mode, m.message)
	}
	if _, err := os.Stat(state.ChatPath("alpha", id)); !os.IsNotExist(err) {
		t.Errorf("Zustand nach Esc: %v", err)
	}
	if _, err := os.Stat(logs[0]); err != nil {
		t.Errorf("Log nach Esc: %v", err)
	}
}

func TestNewPRDSimpleTemplate(t *testing.T) {
	m := setupRun(t)
	bin, argsPath := fakeClaude(t)
	m.cfg.Claude.Binary = bin
	inlineRounds(t, m.cfg)
	m = drive(t, m, keys("n")...)
	if prdtemplate.All[m.template].Name != prdtemplate.Simple {
		t.Fatalf("Vorauswahl ohne Preset: %d", m.template)
	}
	// Esc bricht den Picker ab, l und h wechseln das Template.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Fatalf("nach Esc: %v", m.mode)
	}
	m = drive(t, m, keys("nlh")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if view := ansi.Strip(m.View()); m.mode != modeNewPRD || !strings.Contains(view, "Neues PRD · Einfach") {
		t.Fatalf("Eingabe nach dem Picker: %v\n%s", m.mode, view)
	}
	m.input.SetValue("Template-Picker")
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = waitRound(t, m)

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Neues PRD (Einfach): alpha") || !strings.Contains(view, "Du: Template-Picker") {
		t.Errorf("Chat:\n%s", view)
	}
	got, _ := os.ReadFile(argsPath)
	if !strings.Contains(string(got), "Template: einfach") || !strings.Contains(string(got), "Template-Picker") ||
		strings.Contains(string(got), "$ARGUMENTS") || strings.Contains(string(got), "/create-prd") {
		t.Errorf("Aufruf von claude:\n%s", got)
	}
}

func TestOpenSessionNeedsSession(t *testing.T) {
	m := setupRun(t)
	calls := fakeHerdr(t)
	m = drive(t, m, keys("o")...)
	if !m.messageErr || !strings.Contains(m.message, "keine Claude-Sitzung") {
		t.Errorf("o ohne Sitzung: %q", m.message)
	}

	r, _ := state.Load("alpha", "moved")
	r.Steps[0].SessionID = "sess-9"
	state.Save("alpha", "moved", r)
	m = drive(t, m, keys("r")...)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = runCmd(t, next.(Model), cmd)
	got, _ := os.ReadFile(calls)
	if !strings.Contains(string(got), "--cwd|"+r.Worktree+"|") || !strings.Contains(string(got), "run|w:p1|claude --resume sess-9|") {
		t.Errorf("herdr-Aufrufe:\n%s", got)
	}
}

func TestDevWithoutCommand(t *testing.T) {
	m := setupRun(t)
	m = drive(t, m, keys("p")...)
	if !m.messageErr || !strings.Contains(m.message, "hooks.dev") {
		t.Errorf("p ohne Preset: %q", m.message)
	}
}
