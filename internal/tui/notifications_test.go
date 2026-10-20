package tui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/chatrun"
	"lazydev/internal/state"
)

// writeChat legt den Zustand eines Chats an, ohne eine Runde zu starten.
func writeChat(t *testing.T, projectID, id string, c chatrun.Chat) {
	t.Helper()
	if err := os.MkdirAll(state.ChatsDir(projectID), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(c)
	if err := os.WriteFile(state.ChatPath(projectID, id), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func question(desc, updated string) chatrun.Chat {
	return chatrun.Chat{Title: "Neues PRD (Einfach): alpha", Description: desc, Dir: os.TempDir(), Status: chatrun.StatusQuestion,
		SessionID: "sess-" + desc, StartedAt: updated, UpdatedAt: updated}
}

func TestEnvelopeWidth(t *testing.T) {
	if w := ansi.StringWidth("✉"); w != 1 {
		t.Errorf("✉ ist %d Zellen breit", w)
	}
}

func TestNotificationBox(t *testing.T) {
	m := setupRun(t)
	before := m.layout()
	if before.sideWidth != 0 || strings.Contains(ansi.Strip(m.View()), "Benachrichtigungen") {
		t.Fatalf("Box ohne Benachrichtigung: %+v", before)
	}
	// Ohne Box überspringt der Fokus die Box.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusLog {
		t.Errorf("Fokus nach zweimal Tab ohne Box: %v", m.focus)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.focus != focusPRDs {
		t.Errorf("Fokus nach ← ohne Box: %v", m.focus)
	}
	m = drive(t, m, keys("N")...)
	if m.messageErr || m.message != "Keine offenen Rückfragen" {
		t.Errorf("N ohne Benachrichtigung: %q", m.message)
	}

	writeChat(t, "alpha", "b", question("Zweite Frage", "2026-10-20T10:00:00+02:00"))
	writeChat(t, "alpha", "a", question("Erste Frage", "2026-10-20T09:00:00+02:00"))
	failed := question("Kaputt", "2026-10-20T11:00:00+02:00")
	failed.Status, failed.Error = chatrun.StatusError, "claude endete mit Exit-Code 3"
	writeChat(t, "alpha", "c", failed)
	done := question("Fertig", "2026-10-20T08:00:00+02:00")
	done.Status = chatrun.StatusDone
	writeChat(t, "alpha", "d", done)
	m = drive(t, m, loadChatsCmd())

	l := m.layout()
	if l.sideWidth != l.leftWidth || l.notifHeight != l.topHeight || l.issuesHeight != 0 || l.rightWidth != before.rightWidth-l.leftWidth {
		t.Errorf("Layout mit Box: %+v, ohne: %+v", l, before)
	}
	view := ansi.Strip(m.View())
	first, second := strings.Index(view, "? Erste Frage"), strings.Index(view, "? Zweite Frage")
	if !strings.Contains(view, "Benachrichtigungen · 3") || first < 0 || second < first || !strings.Contains(view, "✗ Kaputt") ||
		!strings.Contains(view, " 3✉") || strings.Contains(view, "Fertig") {
		t.Errorf("Box:\n%s", view)
	}
	if _, err := os.Stat(state.ChatPath("alpha", "d")); !os.IsNotExist(err) {
		t.Errorf("beendeter Chat bleibt: %v", err)
	}
	for _, width := range []int{80, 200} {
		m = drive(t, m, tea.WindowSizeMsg{Width: width, Height: 30})
		checkViewSize(t, m)
	}
	m = drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})

	// Mit Box liegt sie zwischen PRDs und Log.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.focus != focusNotifications || !strings.Contains(ansi.Strip(m.helpLine()), "Enter öffnen") {
		t.Fatalf("Fokus nach → mit Box: %v", m.focus)
	}
	m = drive(t, m, keys("j")...)
	if m.notifCursor != 1 {
		t.Errorf("Cursor nach j: %d", m.notifCursor)
	}

	// X fragt nach. Nach n bleibt lazydev im Normalmodus, nach j ist der Zustand gelöscht.
	os.WriteFile(state.ChatLogPath("alpha", "b"), []byte("{}\n"), 0o644)
	m = drive(t, m, keys("X")...)
	if m.mode != modeConfirm || !strings.Contains(m.confirm.question, "Chat beenden?") {
		t.Fatalf("X ohne Rückfrage: %v", m.mode)
	}
	m = drive(t, m, keys("n")...)
	if m.mode != modeNormal {
		t.Errorf("nach n: %v", m.mode)
	}
	m = drive(t, m, keys("Xj")...)
	if !strings.Contains(m.message, "Chat beendet. Fortsetzen: claude --resume sess-Zweite Frage · Log: ") {
		t.Errorf("Meldung nach X: %q", m.message)
	}
	if _, err := os.Stat(state.ChatPath("alpha", "b")); !os.IsNotExist(err) {
		t.Errorf("Zustand nach X: %v", err)
	}
	if _, err := os.Stat(state.ChatLogPath("alpha", "b")); err != nil {
		t.Errorf("Log nach X: %v", err)
	}
	m = drive(t, m, loadChatsCmd())
	if len(m.notifications("alpha")) != 2 {
		t.Errorf("Benachrichtigungen nach X: %+v", m.notifications("alpha"))
	}

	// Verschwindet die Box mit Fokus, geht der Fokus auf die PRDs.
	os.Remove(state.ChatPath("alpha", "a"))
	os.Remove(state.ChatPath("alpha", "c"))
	m = drive(t, m, loadChatsCmd())
	if m.focus != focusPRDs || m.layout().sideWidth != 0 {
		t.Errorf("Fokus ohne Box: %v", m.focus)
	}
}

func TestChatRetryAndEnd(t *testing.T) {
	m := setupRun(t)
	bin, argsPath := fakeClaude(t)
	m.cfg.Claude.Binary = bin
	inlineRounds(t, m.cfg)
	failed := question("Retry", "2026-10-20T11:00:00+02:00")
	failed.Status, failed.Error = chatrun.StatusError, "claude endete mit Exit-Code 3"
	failed.SessionID = ""
	failed.Prompt, failed.Shown = "/create-prd Retry", "Retry"
	writeChat(t, "alpha", "r", failed)
	m = drive(t, m, loadChatsCmd())

	m = drive(t, m, keys("N")...)
	if m.mode != modeChat || m.chat == nil || m.chat.id != "r" {
		t.Fatalf("N öffnet den Chat nicht: %v", m.mode)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})
	m = waitRound(t, m)
	got, _ := os.ReadFile(argsPath)
	if !strings.Contains(string(got), "-p|/create-prd Retry|") {
		t.Errorf("Ctrl+R wiederholt die Eingabe nicht:\n%s", got)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Du: Retry") || m.chat.state.Status != chatrun.StatusQuestion {
		t.Errorf("Chat nach Ctrl+R: %+v\n%s", m.chat.state, view)
	}

	// Ctrl+R wirkt nur nach einem Fehler.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})
	if lines := strings.Count(string(mustRead(t, argsPath)), "\n"); lines != 1 {
		t.Errorf("Ctrl+R ohne Fehler startet eine Runde: %d Aufrufe", lines)
	}

	// Ctrl+X fragt nach. Nach n bleibt das Fenster offen.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	if m.mode != modeConfirm || !strings.Contains(ansi.Strip(m.View()), "Retry") {
		t.Fatalf("Ctrl+X ohne Rückfrage: %v", m.mode)
	}
	m = drive(t, m, keys("n")...)
	if m.mode != modeChat || m.chat == nil {
		t.Fatalf("nach n: %v", m.mode)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	m = drive(t, m, keys("j")...)
	if m.mode != modeNormal || m.chat != nil || !strings.Contains(m.message, "Chat beendet. Fortsetzen: claude --resume chat-1 · Log: ") {
		t.Errorf("nach Ctrl+X: %v %q", m.mode, m.message)
	}
	if _, err := os.Stat(state.ChatPath("alpha", "r")); !os.IsNotExist(err) {
		t.Errorf("Zustand nach Ctrl+X: %v", err)
	}
	if _, err := os.Stat(state.ChatLogPath("alpha", "r")); err != nil {
		t.Errorf("Log nach Ctrl+X: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
