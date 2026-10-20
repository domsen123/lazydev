package tui

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/project"
	"lazydev/internal/runner"
	"lazydev/internal/state"
)

// target liefert Projekt, PRD und Lauf der Auswahl. ok ist false ohne gewähltes PRD.
func (m Model) target() (t runner.Target, d *project.PRD, r *state.Run, ok bool) {
	p, d := m.selectedProject(), m.selectedPRD()
	if p == nil || d == nil {
		return t, nil, nil, false
	}
	t, err := runner.FindProject(m.cfg, p.ID)
	if err != nil {
		return t, nil, nil, false
	}
	return t, d, m.runs.get(p, d), true
}

// startBlocked nennt den Grund, warum s für das PRD nichts startet. Ohne Grund liefert sie "".
// Die Fußzeile zeigt s nur ohne Grund.
func startBlocked(d *project.PRD, r *state.Run) string {
	switch {
	case r == nil && d.Status.Kind == project.StatusImplemented:
		return d.Label() + " ist schon umgesetzt"
	case r == nil:
		return ""
	case r.Running():
		return d.Label() + " läuft schon"
	case runner.NextStep(r) == "":
		return d.Label() + " ist schon gemergt"
	}
	return ""
}

// mergeBlocked nennt den Grund, warum m nichts mergt. Der Merge läuft nur bei Phase Merge-Queue
// oder nach einem Fehler im Merge.
func mergeBlocked(d *project.PRD, r *state.Run) string {
	if r == nil || r.Running() || runner.NextStep(r) != runner.StepMerge {
		return d.Label() + " ist nicht bereit für den Merge"
	}
	return ""
}

// stopBlocked nennt den Grund, warum x keinen Runner abbricht.
func stopBlocked(d *project.PRD, r *state.Run) string {
	if r == nil || (!r.Running() && r.Phase != project.PhaseWaiting) {
		return "Für " + d.Label() + " läuft kein Runner"
	}
	return ""
}

// discardBlocked nennt den Grund, warum X nichts verwirft.
func discardBlocked(d *project.PRD, r *state.Run) string {
	switch {
	case r == nil && d.Branch != project.FeatureBranchPrefix+d.Name:
		return "Für " + d.Label() + " gibt es nichts zu verwerfen"
	case r != nil && r.Running():
		return d.Label() + " läuft noch. Brich den Runner zuerst mit x ab."
	}
	return ""
}

func (m Model) ask(question string, yes func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
	m.mode = modeConfirm
	m.confirm = &confirmation{question: question, yes: yes}
	return m, nil
}

// startPRD fragt nach und startet die Umsetzung, nach Fehler oder Abbruch ab dem gescheiterten Schritt.
func (m Model) startPRD() (Model, tea.Cmd) {
	t, d, r, ok := m.target()
	if !ok {
		return m, nil
	}
	if reason := startBlocked(d, r); reason != "" {
		m.setMessage(reason, true)
		return m, nil
	}
	question := fmt.Sprintf("Umsetzung von %s starten? (j/n)", d.Label())
	if r != nil {
		question = fmt.Sprintf("%s ab Schritt %s fortsetzen? (j/n)", d.Label(), runner.NextStep(r))
	}
	name := d.Name
	return m.ask(question, func(m Model) (Model, tea.Cmd) {
		return m, action(func() (string, error) {
			started, err := runner.Start(m.cfg, t, name)
			if err != nil {
				return "", err
			}
			if !started {
				return fmt.Sprintf("maxParallel (%d) erreicht, %s wartet", m.cfg.MaxParallel, name), nil
			}
			return name + " gestartet", nil
		})
	})
}

// mergePRD gibt den Merge frei, ohne Rückfrage. Er läuft nur bei Phase Merge-Queue oder nach
// einem Fehler im Merge.
func (m Model) mergePRD() (Model, tea.Cmd) {
	t, d, r, ok := m.target()
	if !ok {
		return m, nil
	}
	if reason := mergeBlocked(d, r); reason != "" {
		m.setMessage(reason, true)
		return m, nil
	}
	name := d.Name
	return m, action(func() (string, error) {
		if _, err := runner.Merge(m.cfg, t, name); err != nil {
			return "", err
		}
		return "Merge von " + name + " angestoßen", nil
	})
}

func (m Model) stopPRD() (Model, tea.Cmd) {
	t, d, r, ok := m.target()
	if !ok {
		return m, nil
	}
	if reason := stopBlocked(d, r); reason != "" {
		m.setMessage(reason, true)
		return m, nil
	}
	name := d.Name
	return m.ask(fmt.Sprintf("Runner von %s abbrechen? Der Worktree bleibt. (j/n)", d.Label()), func(m Model) (Model, tea.Cmd) {
		return m, action(func() (string, error) {
			return "Abbruch von " + name + " angefordert", runner.Stop(t, name)
		})
	})
}

// discardPRD fragt zweimal nach, weil Teardown ungemergte Commits löscht.
func (m Model) discardPRD() (Model, tea.Cmd) {
	t, d, r, ok := m.target()
	if !ok {
		return m, nil
	}
	if reason := discardBlocked(d, r); reason != "" {
		m.setMessage(reason, true)
		return m, nil
	}
	name, label := d.Name, d.Label()
	return m.ask(fmt.Sprintf("%s verwerfen? Datenbanken, Worktree und Branch werden gelöscht. (j/n)", label), func(m Model) (Model, tea.Cmd) {
		return m.ask(fmt.Sprintf("Wirklich? Ungemergte Commits auf feat/%s gehen verloren. (j/n)", name), func(m Model) (Model, tea.Cmd) {
			m.setMessage("Verwerfe "+label+" …", false)
			return m, action(func() (string, error) {
				return label + " verworfen, das PRD liegt wieder im Hauptordner", runner.Teardown(t, name, io.Discard)
			})
		})
	})
}

func action(run func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		text, err := run()
		return actionDoneMsg{text: text, err: err}
	}
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.confirm
	m.mode = m.returnMode
	m.returnMode = modeNormal
	m.confirm = nil
	if s := msg.String(); c == nil || (s != "j" && s != "y") {
		m.setMessage("Abgebrochen", false)
		return m, nil
	}
	return c.yes(m)
}
