package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazydev/internal/chatrun"
	"lazydev/internal/config"
	"lazydev/internal/state"
)

// chats ordnet jeder Projekt-ID ihre Benachrichtigungen zu, die älteste zuerst.
type chats map[string][]chatrun.Entry

type chatsLoadedMsg struct{ all []chatrun.Entry }

func loadChatsCmd() tea.Msg {
	all, _ := chatrun.All()
	return chatsLoadedMsg{all: all}
}

// applyChats übernimmt die Chats mit Rückfrage oder Fehler. Beendete Chats ohne offenes Fenster
// löscht applyChats.
func (m *Model) applyChats(all []chatrun.Entry) {
	m.chats = chats{}
	for _, e := range all {
		if e.Chat.Notifies() {
			m.chats[e.ProjectID] = append(m.chats[e.ProjectID], e)
			continue
		}
		open := m.chat != nil && m.chat.projectID == e.ProjectID && m.chat.id == e.ID
		if e.Chat.Status == chatrun.StatusDone && !open {
			chatrun.RemoveIfDone(e.ProjectID, e.ID)
		}
	}
	if m.chat != nil {
		m.chat.refresh()
	}
}

// syncNotifications begrenzt den Cursor der Box. Verschwindet die Box mit Fokus, geht der
// Fokus auf die PRDs.
func (m *Model) syncNotifications() {
	if p := m.selectedProject(); p != nil {
		m.notifCursor = clamp(m.notifCursor, len(m.notifications(p.ID)))
	}
	if m.focus == focusNotifications && !m.showNotifications() {
		m.focus = focusPRDs
	}
}

func (m Model) notifications(projectID string) []chatrun.Entry {
	return m.chats[projectID]
}

// showNotifications meldet, ob das gewählte Projekt eine Benachrichtigung hat.
func (m Model) showNotifications() bool {
	p := m.selectedProject()
	return p != nil && len(m.notifications(p.ID)) > 0
}

// viewNotifications zeigt eine Zeile pro Chat: "?" für eine Rückfrage, "✗" für einen Fehler.
func (m Model) viewNotifications(l layout) string {
	focused := m.focus == focusNotifications
	entries := m.notifications(m.selectedProject().ID)
	var lines []string
	start, end := scrollWindow(m.notifCursor, len(entries), l.notifHeight-2)
	for i := start; i < end; i++ {
		c := entries[i].Chat
		sym := lipgloss.NewStyle().Foreground(colorFocus).Render("?")
		if s, _ := c.Effective(); s == chatrun.StatusError {
			sym = styleError.Render("✗")
		}
		lines = append(lines, cursorLine(i == m.notifCursor, focused, sym+" "+c.Description))
	}
	title := fmt.Sprintf("Benachrichtigungen · %d", len(entries))
	return box(title, lines, l.sideWidth, l.notifHeight, focused)
}

// openNotification wählt das Projekt des Chats, setzt den Cursor der Box darauf und öffnet das
// Fenster.
func (m Model) openNotification(e chatrun.Entry) (Model, tea.Cmd) {
	c, err := openChat(e.ProjectID, e.ID)
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, loadChatsCmd
	}
	for i, p := range m.projects {
		if p.ID == e.ProjectID && i != m.projCursor {
			m.projCursor = i
			m.prdCursor = 0
		}
	}
	for i, n := range m.notifications(e.ProjectID) {
		if n.ID == e.ID {
			m.notifCursor = i
		}
	}
	m.chat = c
	m.mode = modeChat
	return m, nil
}

// nextNotification öffnet die älteste Benachrichtigung über alle Projekte.
func (m Model) nextNotification() (Model, tea.Cmd) {
	var oldest *chatrun.Entry
	for _, p := range m.projects {
		for _, e := range m.notifications(p.ID) {
			if oldest == nil || e.Chat.Before(oldest.Chat) {
				oldest = &e
			}
		}
	}
	if oldest == nil {
		m.setMessage("Keine offenen Rückfragen", false)
		return m, nil
	}
	return m.openNotification(*oldest)
}

// endChat bricht eine laufende Runde ab, löscht den Zustand und schließt das Fenster des Chats.
// Das Log bleibt erhalten.
func (m Model) endChat(projectID, id string) (Model, tea.Cmd) {
	session := ""
	if c, err := chatrun.Load(projectID, id); err == nil {
		session = c.SessionID
	}
	if err := chatrun.Stop(projectID, id); err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, loadChatsCmd
	}
	if m.chat != nil && m.chat.projectID == projectID && m.chat.id == id {
		m.chat = nil
		m.mode = modeNormal
	}
	m.setMessage(chatMessage(session, state.ChatLogPath(projectID, id)), false)
	return m, loadChatsCmd
}

// chatMessage liefert die Meldung nach dem Beenden eines Chats.
func chatMessage(session, logPath string) string {
	text := "Chat beendet."
	if session != "" {
		text += " Fortsetzen: claude --resume " + session + " ·"
	}
	return text + " Log: " + config.ContractPath(logPath)
}

// selectedNotification liefert den Chat unter dem Cursor der Box.
func (m Model) selectedNotification() (chatrun.Entry, bool) {
	p := m.selectedProject()
	if p == nil {
		return chatrun.Entry{}, false
	}
	entries := m.notifications(p.ID)
	if m.notifCursor < 0 || m.notifCursor >= len(entries) {
		return chatrun.Entry{}, false
	}
	return entries[m.notifCursor], true
}

// updateNotifications bedient die Box. handled ist false für Tasten, die wie im Normalmodus
// wirken.
func (m Model) updateNotifications(msg tea.KeyMsg) (next Model, cmd tea.Cmd, handled bool) {
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "enter":
		if e, ok := m.selectedNotification(); ok {
			next, cmd = m.openNotification(e)
			return next, cmd, true
		}
	case "X":
		e, ok := m.selectedNotification()
		if !ok {
			return m, nil, true
		}
		next, cmd = m.ask("Chat beenden? Eine laufende Runde bricht ab. (j/n)", func(m Model) (Model, tea.Cmd) {
			return m.endChat(e.ProjectID, e.ID)
		})
		return next, cmd, true
	default:
		return m, nil, false
	}
	return m, nil, true
}
