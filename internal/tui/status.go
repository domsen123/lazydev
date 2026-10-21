package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazydev/internal/project"
)

// The status picker sets the status of a PRD without run. It changes the value before the first
// "·" only. A tracked PRD gets committed in the main folder, an untracked one only changes.

// statusPickerWidth is the outer width of the picker box.
const statusPickerWidth = 46

// statusPicker is the open picker. current is the index of the current value, -1 for another one.
type statusPicker struct {
	dir, base  string
	rel, label string
	tracked    bool
	current    int
	cursor     int
	// confirm is true while the question to commit is open.
	confirm bool
}

// actionMsg reports the end of an action in the background, like a status change or a start.
type actionMsg struct {
	text string
	err  error
}

// openStatusPicker opens the picker for the selected PRD, with its current value selected.
func (m *model) openStatusPicker() {
	p := m.current()
	d := p.selectedPRD()
	if d == nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(p.path, d.Path))
	if err != nil {
		m.err = err.Error()
		return
	}
	if _, ok := project.SetStatus(string(data), project.ValuePlanned); !ok {
		m.err = d.Label() + " has no status line."
		return
	}
	s := &statusPicker{dir: p.path, base: p.lazydev.Config.Base(), rel: d.Path, label: d.Label(), tracked: d.Tracked, current: -1}
	for i, c := range project.Choices {
		if c.Kind == d.Status.Kind {
			s.current, s.cursor = i, i
		}
	}
	m.status = s
}

// updateStatus handles all keys while the picker is open.
func (m model) updateStatus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.status
	key := msg.String()
	if s.confirm {
		m.status = nil
		if key == "y" || key == "j" {
			return m, changeStatus(s)
		}
		return m, nil
	}
	switch key {
	case "esc", "ctrl+c", "q":
		m.status = nil
	case "j", "down":
		s.cursor = clamp(s.cursor+1, len(project.Choices))
	case "k", "up":
		s.cursor = clamp(s.cursor-1, len(project.Choices))
	case "enter":
		// The current value changes nothing, not even the date of merged or discarded.
		if s.cursor == s.current {
			m.status = nil
			return m, nil
		}
		if s.tracked {
			s.confirm = true
			return m, nil
		}
		m.status = nil
		return m, changeStatus(s)
	}
	return m, nil
}

// changeStatus writes the status in the background.
func changeStatus(s *statusPicker) tea.Cmd {
	return func() tea.Msg {
		c := project.Choices[s.cursor]
		committed, err := project.ChangeStatus(s.dir, s.base, s.rel, c, time.Now())
		if err != nil {
			return actionMsg{err: err}
		}
		text := fmt.Sprintf("Status of %s: %s", s.label, c.Label)
		if committed {
			text += ", committed"
		}
		return actionMsg{text: text}
	}
}

// viewStatusPicker renders the picker as a box for the overlay.
func (m model) viewStatusPicker() string {
	s := m.status
	t := themes[m.theme]
	text := lipgloss.NewStyle().Foreground(t.text)
	inner := min(statusPickerWidth, m.width-4) - 4
	lines := []string{""}
	for i, c := range project.Choices {
		label := c.Label
		if i == s.current {
			label += " (current)"
		}
		if i == s.cursor {
			lines = append(lines, text.Bold(true).Foreground(t.selectedFg).Background(t.selectedBg).Render("> "+pad(label, inner-2)))
		} else {
			lines = append(lines, text.Render("  "+pad(label, inner-2)))
		}
	}
	if s.confirm {
		lines = append(lines, "", text.Bold(true).Render(pad("Commit "+s.rel+"? (y/n)", inner)))
	}
	lines = append(lines, "")
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return box("Status · "+s.label, strings.Join(lines, "\n"), inner+4, len(lines)+2, t.focus, true, t)
}

// keys is the key help of the picker.
func (s *statusPicker) keys() string {
	if s.confirm {
		return " y set and commit · n cancel"
	}
	return " j/k status · enter set · esc cancel"
}
