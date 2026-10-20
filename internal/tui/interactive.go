package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/chatrun"
	"lazydev/internal/devserver"
	"lazydev/internal/herdr"
	"lazydev/internal/hooks"
	"lazydev/internal/prdtemplate"
	"lazydev/internal/state"
	"lazydev/internal/worktree"
)

// openInteractive startet args in dir. In Herdr öffnet sich ein neuer Pane, sonst unterbricht
// die TUI ihre Oberfläche, bis der Befehl endet. Ohne args startet eine Shell.
func (m Model) openInteractive(dir string, args []string, what string) (Model, tea.Cmd) {
	if herdr.Available() {
		return m, action(func() (string, error) {
			_, err := herdr.Open(dir, args)
			return what + " in neuem Herdr-Pane geöffnet", err
		})
	}
	return m, execInteractive(dir, args, func(err error) tea.Msg {
		return actionDoneMsg{text: what + " beendet", err: err}
	})
}

// execInteractive unterbricht die Oberfläche, startet args in dir und meldet das Ende über done.
// Ohne args startet eine Shell.
func execInteractive(dir string, args []string, done func(error) tea.Msg) tea.Cmd {
	if len(args) == 0 {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		args = []string{shell}
	}
	c := exec.Command(args[0], args[1:]...)
	c.Dir = dir
	return tea.Exec(interruptSafe{c}, done)
}

// newPRD lässt das Template wählen. Vorgewählt ist das Template, das zum Preset passt.
func (m Model) newPRD() (Model, tea.Cmd) {
	p := m.selectedProject()
	if p == nil {
		return m, nil
	}
	preset := ""
	if i := m.configIndex(p.Path); i >= 0 {
		preset = m.cfg.Projects[i].Preset
	}
	m.template = prdtemplate.Index(prdtemplate.Default(preset))
	m.mode = modeTemplate
	return m, nil
}

func (m Model) updateTemplate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(prdtemplate.All)
	switch msg.String() {
	case "esc", "ctrl+c", "q":
		m.mode = modeNormal
		m.fromIssue = nil
	case "left", "h", "k", "up", "shift+tab":
		m.template = (m.template + n - 1) % n
	case "right", "l", "j", "down", "tab":
		m.template = (m.template + 1) % n
	case "enter":
		// Aus einem Issue startet der Chat ohne Fenster „Neues PRD“.
		if m.fromIssue != nil {
			display, desc := issueChat(*m.fromIssue)
			m.fromIssue = nil
			m.mode = modeNormal
			return m.startChat(display, desc)
		}
		m.mode = modeNewPRD
		m.input.Prompt = "> "
		m.input.Placeholder = "z. B. Avatar-Upload im Profil"
		m.input.Width = m.newPRDInputWidth()
		m.input.SetValue("")
		return m, m.input.Focus()
	}
	return m, nil
}

// newPRDInputWidth liefert die Breite des Eingabefelds im Fenster „Neues PRD“. Die -1 lässt Platz
// für den Cursor.
func (m Model) newPRDInputWidth() int {
	w, _ := m.dialogSize()
	return max(1, w-2-2*dialogPadX-ansi.StringWidth(m.input.Prompt)-1)
}

func (m Model) updateNewPRD(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		desc := strings.TrimSpace(m.input.Value())
		if desc == "" {
			return m, nil
		}
		m.mode = modeNormal
		m.input.Blur()
		return m.startChat(desc, desc)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// startChat legt den Chat mit dem gewählten Template an und öffnet sein Fenster. display ist die
// Beschreibung in „Benachrichtigungen“, desc geht in die erste Nachricht.
func (m Model) startChat(display, desc string) (Model, tea.Cmd) {
	p := m.selectedProject()
	t := prdtemplate.All[m.template]
	id, err := chatrun.Create(p.ID, "Neues PRD ("+t.Label+"): "+p.Name(), display, p.Path, time.Now())
	if err == nil {
		err = chatrun.Send(p.ID, id, t.CreateDisplay(desc), t.CreatePrompt(desc))
	}
	var c *chat
	if err == nil {
		c, err = openChat(p.ID, id)
	}
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, loadChatsCmd
	}
	m.chat = c
	m.mode = modeChat
	return m, loadChatsCmd
}

// worktreeRun liefert den Lauf der Auswahl, wenn sein Worktree existiert.
func (m Model) worktreeRun() (*state.Run, bool) {
	r := m.selectedRun()
	if r == nil {
		return nil, false
	}
	if _, err := os.Stat(r.Worktree); err != nil {
		return r, false
	}
	return r, true
}

// lastSession liefert die ID der letzten Claude-Sitzung des Laufs, ohne Sitzung "".
func lastSession(r *state.Run) string {
	session := ""
	for _, s := range r.Steps {
		if s.SessionID != "" {
			session = s.SessionID
		}
	}
	return session
}

// openSession öffnet die letzte Claude-Sitzung des Laufs im Worktree. Claude findet eine
// Sitzung nur im Ordner, in dem sie lief.
func (m Model) openSession() (Model, tea.Cmd) {
	r, ok := m.worktreeRun()
	if !ok {
		m.setMessage("Kein Worktree für dieses PRD, die Sitzung lässt sich nicht öffnen", true)
		return m, nil
	}
	session := lastSession(r)
	if session == "" {
		m.setMessage("Dieser Lauf hat noch keine Claude-Sitzung", true)
		return m, nil
	}
	return m.openInteractive(r.Worktree, []string{m.cfg.Claude.Binary, "--resume", session}, "Claude-Sitzung")
}

func (m Model) openShell() (Model, tea.Cmd) {
	r, ok := m.worktreeRun()
	if !ok {
		m.setMessage("Kein Worktree für dieses PRD", true)
		return m, nil
	}
	return m.openInteractive(r.Worktree, nil, "Shell")
}

// toggleDev startet oder stoppt den Dev-Server im Worktree.
func (m Model) toggleDev() (Model, tea.Cmd) {
	t, d, _, ok := m.target()
	if !ok {
		return m, nil
	}
	r, exists := m.worktreeRun()
	if !exists {
		m.setMessage("Kein Worktree für dieses PRD", true)
		return m, nil
	}
	name := d.Name
	if pid := devserver.PID(t.ID, name); pid != 0 {
		return m, action(func() (string, error) {
			return "Dev-Server gestoppt", devserver.Stop(t.ID, name)
		})
	}
	command := hooks.For(t.Config).Dev
	if len(command) == 0 {
		m.setMessage("Kein Dev-Befehl. Trage hooks.dev in der Konfiguration ein.", true)
		return m, nil
	}
	if !worktree.PortFree(r.Port) {
		m.setMessage(fmt.Sprintf("Port %d ist belegt, der Dev-Server startet nicht", r.Port), true)
		return m, nil
	}
	port, wt := r.Port, r.Worktree
	return m, action(func() (string, error) {
		if _, err := devserver.Start(t.ID, name, wt, command); err != nil {
			return "", err
		}
		return fmt.Sprintf("Dev-Server auf Port %d gestartet. Log: %s", port, devserver.LogPath(t.ID, name)), nil
	})
}
