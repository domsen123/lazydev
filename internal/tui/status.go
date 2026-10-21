package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

// statusChoices sind die Werte im Fenster „Status“, in dieser Reihenfolge.
var statusChoices = []struct {
	kind  project.StatusKind
	value string
}{
	{project.StatusPlanned, project.StatusValuePlanned},
	{project.StatusImplemented, project.StatusValueImplemented},
	{project.StatusDiscarded, project.StatusValueDiscarded},
}

// statusPicker wählt den Status eines PRDs ohne Lauf-Zustand. path ist der absolute Projektpfad,
// base der Ziel-Branch. current ist der Index des aktuellen Werts, -1 bei einem anderen Wert.
type statusPicker struct {
	path, base string
	rel, label string
	tracked    bool
	current    int
	cursor     int
}

// statusBlocked nennt den Grund, warum c den Status nicht ändert. Die Fußzeile zeigt c nur ohne Grund.
func statusBlocked(d *project.PRD, r *state.Run) string {
	if r != nil {
		return d.Label() + " hat einen Lauf von lazydev. Verwirf ihn zuerst mit X."
	}
	return ""
}

// openStatusPicker zeigt das Fenster „Status“ für das gewählte PRD. Vorgewählt ist der aktuelle Wert.
func (m Model) openStatusPicker() (Model, tea.Cmd) {
	p, d := m.selectedProject(), m.selectedPRD()
	if p == nil || d == nil {
		return m, nil
	}
	if reason := statusBlocked(d, m.runs.get(p, d)); reason != "" {
		m.setMessage(reason, true)
		return m, nil
	}
	data, err := os.ReadFile(filepath.Join(p.Path, d.Path))
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	if _, ok := project.SetStatus(string(data), project.StatusValuePlanned); !ok {
		m.setMessage(d.Label()+" hat keine Statuszeile", true)
		return m, nil
	}
	s := &statusPicker{path: p.Path, base: p.Base, rel: d.Path, label: d.Label(), tracked: d.Tracked, current: -1}
	for i, c := range statusChoices {
		if c.kind == d.Status.Kind {
			s.current, s.cursor = i, i
		}
	}
	m.prdStatus = s
	m.mode = modeStatus
	return m, nil
}

// lines zeigt die drei Werte, den aktuellen mit „(aktuell)“.
func (s *statusPicker) lines() []string {
	var lines []string
	for i, c := range statusChoices {
		label := c.value
		if i == s.current {
			label += " (aktuell)"
		}
		if i == s.cursor {
			lines = append(lines, styleSelected.Render("> "+label))
		} else {
			lines = append(lines, styleMuted.Render("  "+label))
		}
	}
	return lines
}

// value liefert den gewählten Statuswert. umgesetzt und verworfen bekommen das heutige Datum.
func (s *statusPicker) value(now time.Time) string {
	c := statusChoices[s.cursor]
	if c.kind == project.StatusPlanned {
		return c.value
	}
	return fmt.Sprintf("%s (%s)", c.value, now.Format("2006-01-02"))
}

func (m Model) updateStatus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.prdStatus
	switch msg.String() {
	case "esc", "ctrl+c", "q":
		m.mode = modeNormal
		m.prdStatus = nil
	case "j", "down":
		s.cursor = clamp(s.cursor+1, len(statusChoices))
	case "k", "up":
		s.cursor = clamp(s.cursor-1, len(statusChoices))
	case "enter":
		m.mode = modeNormal
		m.prdStatus = nil
		// Der aktuelle Wert ändert nichts, auch nicht das Datum von umgesetzt oder verworfen.
		if s.cursor == s.current {
			return m, nil
		}
		return m.changeStatus(s, s.value(time.Now()))
	}
	return m, nil
}

// changeStatus schreibt den Statuswert. Ein getracktes PRD committet lazydev nach Rückfrage im
// Hauptordner, ein ungetracktes ändert es ohne Rückfrage und ohne Commit.
func (m Model) changeStatus(s *statusPicker, value string) (Model, tea.Cmd) {
	apply := func(m Model) (Model, tea.Cmd) {
		return m, action(func() (string, error) {
			committed, err := project.ChangeStatus(s.path, s.base, s.rel, value)
			if err != nil {
				return "", err
			}
			text := fmt.Sprintf("Status von %s: %s", s.label, value)
			if committed {
				text += ", committet"
			}
			return text, nil
		})
	}
	if !s.tracked {
		return apply(m)
	}
	return m.ask(fmt.Sprintf("Status von %s auf %s setzen und committen? (j/n)", s.label, value), apply)
}

// viewStatusPicker zeigt das Fenster „Status“.
func (m Model) viewStatusPicker() string {
	s := m.prdStatus
	lines := s.lines()
	w, _ := m.dialogSize()
	return dialog("Status · "+s.label, lines, w, len(lines)+4)
}
