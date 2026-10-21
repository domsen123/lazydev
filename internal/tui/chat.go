package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/chatrun"
	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/state"
)

type chatRole int

const (
	roleUser chatRole = iota
	roleClaude
	roleTool
	roleError
)

type chatEntry struct {
	role chatRole
	text string
}

// chat ist das Fenster eines Chats über der TUI. Die Runden laufen als losgelöste Prozesse
// "lazydev chat". Das Fenster liest nur deren Zustand und Log.
type chat struct {
	projectID string
	id        string
	state     *chatrun.Chat
	entries   []chatEntry
	input     textarea.Model
	// scroll zählt die Zeilen, um die der Verlauf vom Ende nach oben verschoben ist.
	scroll int
	// logPath ist das Log aller Runden. logOffset ist die Stelle hinter der letzten gelesenen Zeile.
	logPath   string
	logOffset int64
}

// openChat lädt den Zustand eines Chats und baut den Verlauf aus dem Log auf.
func openChat(projectID, id string) (*chat, error) {
	st, err := chatrun.Load(projectID, id)
	if err != nil {
		return nil, err
	}
	in := textarea.New()
	in.Placeholder = "Antwort an Claude …"
	in.ShowLineNumbers = false
	in.Prompt = "> "
	in.SetHeight(3)
	in.CharLimit = 0
	in.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	in.Focus()
	c := &chat{projectID: projectID, id: id, state: st, input: in, logPath: state.ChatLogPath(projectID, id)}
	c.readLog()
	return c, nil
}

// refresh lädt den Zustand neu und liest neue Log-Zeilen. Ist der Zustand gelöscht, bleibt der
// letzte Stand.
func (c *chat) refresh() {
	if st, err := chatrun.Load(c.projectID, c.id); err == nil {
		c.state = st
	}
	c.readLog()
}

// readLog liest vollständige Zeilen ab logOffset. Eine halb geschriebene Zeile liest der
// nächste Aufruf.
func (c *chat) readLog() {
	f, err := os.Open(c.logPath)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err := f.Seek(c.logOffset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return
	}
	c.logOffset += int64(end + 1)
	for _, line := range bytes.Split(data[:end], []byte("\n")) {
		c.handleEvent(line)
	}
}

// status liefert den Status für die Anzeige.
func (c *chat) status() chatrun.Status {
	s, _ := c.state.Effective()
	return s
}

// handleEvent übernimmt eine Zeile des Logs in den Verlauf: Eingaben, Text der Hauptsitzung
// vollständig, Tool-Aufrufe und Subagenten als kurze Zeilen.
func (c *chat) handleEvent(line []byte) {
	var e struct {
		Type            string `json:"type"`
		Text            string `json:"text"`
		Shown           string `json:"shown"`
		ParentToolUseID string `json:"parent_tool_use_id"`
		Message         struct {
			Content []struct {
				Type  string         `json:"type"`
				Text  string         `json:"text"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &e) != nil {
		return
	}
	switch e.Type {
	case "user_prompt":
		text := e.Shown
		if text == "" {
			text = e.Text
		}
		c.entries = append(c.entries, chatEntry{role: roleUser, text: text})
		return
	case "lazydev":
		c.entries = append(c.entries, chatEntry{role: roleError, text: e.Text})
		return
	case "system", "result":
		// Sitzung, Kosten und Fehler stehen im Zustand.
		return
	case "assistant":
		for _, part := range e.Message.Content {
			switch {
			case part.Type == "tool_use":
				c.entries = append(c.entries, chatEntry{role: roleTool, text: claude.ToolLine(part.Name, part.Input)})
			case part.Type == "text" && e.ParentToolUseID == "" && strings.TrimSpace(part.Text) != "":
				c.entries = append(c.entries, chatEntry{role: roleClaude, text: strings.TrimSpace(part.Text)})
			}
		}
		return
	}
	// Fehler von Tools, Ablehnungen, stderr und Subagenten.
	for _, d := range claude.Describe(line) {
		role := roleTool
		if t := strings.TrimSpace(d); strings.HasPrefix(t, "Fehler:") || strings.HasPrefix(t, "abgelehnt:") || strings.HasPrefix(t, "stderr:") {
			role = roleError
		}
		c.entries = append(c.entries, chatEntry{role: role, text: strings.TrimSpace(d)})
	}
}

// render zeichnet das Fenster mit Verlauf, Status und Eingabe in width × height Zellen.
func (c *chat) render(width, height int) string {
	inner := width - 2 - 2*dialogPadX
	rows := height - 2 - 2*dialogPadY
	// Der Rahmen „Antwort“ braucht 2 Spalten, dazu kommt 1 Leerzeichen vor dem Prompt.
	c.input.SetWidth(inner - 3)
	status := styleMuted.Render(fmt.Sprintf("%.2f USD", c.state.CostUSD))
	if c.status() == chatrun.StatusRunning {
		status = styleSelected.Render("● Claude arbeitet …") + styleMuted.Render("  ·  ") + status
	}
	inputLines := strings.Split(c.input.View(), "\n")
	for i, l := range inputLines {
		inputLines[i] = " " + l
	}
	answer := strings.Split(box("Antwort", inputLines, inner, len(inputLines)+2, false), "\n")
	historyRows := max(1, rows-1-len(answer))

	var lines []string
	for i, e := range c.entries {
		if i > 0 && e.role != roleTool && e.role != roleError {
			lines = append(lines, "")
		}
		lines = append(lines, renderEntry(e, inner)...)
	}
	maxScroll := max(0, len(lines)-historyRows)
	c.scroll = min(c.scroll, maxScroll)
	end := len(lines) - c.scroll
	start := max(0, end-historyRows)
	visible := lines[start:end]
	for len(visible) < historyRows {
		visible = append([]string{""}, visible...)
	}

	body := append(slices.Clone(visible), status)
	body = append(body, answer...)
	return dialog(c.state.Title, body, width, height)
}

func renderEntry(e chatEntry, width int) []string {
	var prefix string
	style := lipgloss.NewStyle()
	switch e.role {
	case roleUser:
		prefix = "Du: "
		style = styleSelected
	case roleClaude:
		prefix = "Claude: "
	case roleTool:
		prefix = "  · "
		style = styleMuted
	case roleError:
		prefix = "  ✗ "
		style = styleError
	}
	text := e.text
	if e.role == roleTool || e.role == roleError {
		text = ansi.Truncate(claude.FirstLine(text), width-len(prefix), "…")
	}
	wrapped := lipgloss.NewStyle().Width(width - 2).Render(prefix + text)
	var out []string
	for _, l := range strings.Split(wrapped, "\n") {
		out = append(out, style.Render(strings.TrimRight(l, " ")))
	}
	return out
}

// overlay dimmt bg und legt fg zentriert darüber. Beide sind mehrzeilige Strings mit ANSI-Stilen.
func overlay(bg, fg string, width int) string {
	bgLines := strings.Split(bg, "\n")
	for i, l := range bgLines {
		bgLines[i] = styleMuted.Render(ansi.Strip(l))
	}
	fgLines := strings.Split(fg, "\n")
	fgWidth := 0
	for _, l := range fgLines {
		fgWidth = max(fgWidth, ansi.StringWidth(l))
	}
	x := max(0, (width-fgWidth)/2)
	y := max(0, (len(bgLines)-len(fgLines))/2)
	for i, l := range fgLines {
		row := y + i
		if row >= len(bgLines) {
			break
		}
		b := bgLines[row]
		left := ansi.Truncate(b, x, "")
		left += strings.Repeat(" ", max(0, x-ansi.StringWidth(left)))
		right := ansi.Cut(b, x+fgWidth, max(x+fgWidth, ansi.StringWidth(b)))
		bgLines[row] = left + "\x1b[0m" + l + "\x1b[0m" + right
	}
	return strings.Join(bgLines, "\n")
}

// dialogSize liefert die Größe der großen Fenster für das Terminal: Chat, „Neues PRD“, Ziel-Branch
// und „.lazydev/ anlegen“.
func (m Model) dialogSize() (int, int) {
	return max(20, min(m.width-6, 110)), max(10, m.height-4)
}

func (m Model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.chat
	switch msg.String() {
	case "esc":
		m.chat = nil
		m.mode = modeNormal
		logPath := config.ContractPath(c.logPath)
		if c.status() == chatrun.StatusDone {
			// Ein beendeter Chat erzeugt keine Benachrichtigung, sein Zustand ist überflüssig.
			if err := chatrun.RemoveIfDone(c.projectID, c.id); err != nil {
				m.setMessage(firstLine(err.Error()), true)
			} else if c.state.SessionID != "" {
				m.setMessage("Chat geschlossen. Fortsetzen: claude --resume "+c.state.SessionID+" · Log: "+logPath, false)
			}
		} else {
			m.setMessage("Chat läuft im Hintergrund weiter · Log: "+logPath, false)
		}
		return m, tea.Batch(m.loadProjects(""), loadChatsCmd)
	case "ctrl+x":
		projectID, id := c.projectID, c.id
		m.returnMode = modeChat
		return m.ask("Chat beenden? Eine laufende Runde bricht ab. (j/n)", func(m Model) (Model, tea.Cmd) {
			return m.endChat(projectID, id)
		})
	case "ctrl+r":
		if c.status() != chatrun.StatusError {
			return m, nil
		}
		return m.sendChat(c.state.Shown, c.state.Prompt)
	case "enter":
		text := strings.TrimSpace(c.input.Value())
		if text == "" || c.status() == chatrun.StatusRunning {
			return m, nil
		}
		c.input.Reset()
		return m.sendChat(text, text)
	case "pgup":
		_, h := m.dialogSize()
		c.scroll += max(1, h/2)
		return m, nil
	case "pgdown":
		_, h := m.dialogSize()
		c.scroll = max(0, c.scroll-max(1, h/2))
		return m, nil
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return m, cmd
}

// sendChat startet eine Runde im offenen Chat. Danach gilt seine Benachrichtigung als gelesen.
func (m Model) sendChat(shown, prompt string) (Model, tea.Cmd) {
	c := m.chat
	err := chatrun.Send(c.projectID, c.id, shown, prompt)
	c.scroll = 0
	c.refresh()
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
	} else {
		m.setMessage("", false)
	}
	return m, loadChatsCmd
}
