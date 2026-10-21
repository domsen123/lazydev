package tui

import (
	"path/filepath"

	"slices"

	"lazydev/internal/terminal"

	tea "github.com/charmbracelet/bubbletea"
)

// The key map follows three rules:
//
//  1. Every global action has an alt key that works everywhere, inside a terminal too.
//  2. Outside a terminal, alt is optional: "q" does what "alt+q" does, unless the focused column
//     uses the plain key itself.
//  3. A key means the same in every column: j/k move, l/enter open or go in, h goes back.

// action is a global action.
type action int

const (
	actFocusLeft action = iota
	actFocusRight
	actNextProject
	actPrevProject
	actNextTab
	actPrevTab
	actNewTerminal
	actCloseTab
	actMenu
	actQuit
)

// globalKeys maps alt keys to global actions. alt+1 to alt+9 select a center tab directly.
var globalKeys = map[string]action{
	"alt+h": actFocusLeft, "alt+left": actFocusLeft,
	"alt+l": actFocusRight, "alt+right": actFocusRight,
	"alt+j": actNextProject, "alt+down": actNextProject,
	"alt+k": actPrevProject, "alt+up": actPrevProject,
	"alt+shift+right": actNextTab,
	"alt+shift+left":  actPrevTab,
	"alt+t":           actNewTerminal,
	"alt+w":           actCloseTab,
	"alt+m":           actMenu, "alt+esc": actMenu,
	"alt+q": actQuit, "alt+ctrl+c": actQuit,
}

// updateKey routes a key: open dialogs first, then global alt keys, then the terminal of the
// center column, then the focused column, and finally the plain key as its alt key.
func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err, m.note = "", ""
	switch {
	case m.menu != nil:
		return m.updateMenu(msg)
	case m.question != nil:
		return m.updateQuestion(msg)
	case m.status != nil:
		return m.updateStatus(msg)
	case m.wizardVisible():
		return m.updateWizard(msg)
	case m.adding:
		return m.updateAdd(msg)
	case m.creating:
		return m.updateCreate(msg)
	case m.confirmRemove:
		return m.updateConfirmRemove(msg)
	}
	key := msg.String()
	if next, cmd, ok := m.global(key); ok {
		return next, cmd
	}
	if m.inTerminal() {
		p := m.current()
		p.terms[p.tab].SendKey(msg)
		return m, nil
	}
	var handled bool
	var next tea.Model = m
	var cmd tea.Cmd
	switch m.focus {
	case colLeft:
		next, cmd, handled = m.updateLeft(key)
	case colRight:
		next, cmd, handled = m.updateRight(key)
	case colBottom:
		next, cmd, handled = m.updateBottom(key)
	}
	if handled {
		return next, cmd
	}
	switch key {
	case "tab":
		m.stepFocus(1, true)
		return m, nil
	case "shift+tab":
		m.stepFocus(-1, true)
		return m, nil
	}
	// Rule 2: outside a terminal, alt is optional.
	if next, cmd, ok := m.global("alt+" + key); ok {
		return next, cmd
	}
	return m, nil
}

// wizardVisible is true while the wizard is open and not waiting for its editor.
func (m model) wizardVisible() bool {
	return m.wizard != nil && m.wizard.editor == 0
}

// inTerminal is true while keys go to a terminal: the center column has the focus and a tab.
func (m model) inTerminal() bool {
	p := m.current()
	return m.focus == colCenter && p != nil && len(p.terms) > 0
}

// global runs the global action of key. ok is false if key has none.
func (m model) global(key string) (next tea.Model, cmd tea.Cmd, ok bool) {
	if len(key) == 5 && key[:4] == "alt+" && key[4] >= '1' && key[4] <= '9' {
		m.selectTab(int(key[4] - '1'))
		return m, nil, true
	}
	act, ok := globalKeys[key]
	if !ok {
		return m, nil, false
	}
	p := m.current()
	switch act {
	case actFocusLeft:
		m.stepFocus(-1, false)
	case actFocusRight:
		m.stepFocus(1, false)
	case actNextProject, actPrevProject:
		step := 1
		if act == actPrevProject {
			step = -1
		}
		m.selectProject(m.selected + step)
	case actNextTab, actPrevTab:
		step := 1
		if act == actPrevTab {
			step = -1
		}
		m.stepTab(step)
	case actNewTerminal:
		m.openTerminal(terminal.Shell())
	case actCloseTab:
		if p != nil && len(p.terms) > 0 {
			p.terms[p.tab].Hangup()
		}
	case actMenu:
		m.menu = &menu{}
	case actQuit:
		next, cmd := m.quit()
		return next, cmd, true
	}
	return m, nil, true
}

// stepFocus moves the focus along focusOrder. wrap lets it run from the last pane to the first.
func (m *model) stepFocus(step int, wrap bool) {
	n := len(focusOrder)
	i := slices.Index(focusOrder, m.focus) + step
	if wrap {
		i = (i%n + n) % n
	}
	m.focus = focusOrder[clamp(i, n)]
}

// stepTab switches the tab of the focused column. The left column has no tabs, there it switches
// the center tabs. Switching a center tab focuses its terminal.
func (m *model) stepTab(step int) {
	switch m.focus {
	case colRight:
		n := len(rightTabs)
		m.rightTab = ((m.rightTab+step)%n + n) % n
		return
	case colBottom:
		n := len(bottomTabs)
		m.bottomTab = ((m.bottomTab+step)%n + n) % n
		return
	}
	if p := m.current(); p != nil {
		m.selectTab(p.tab + step)
	}
}

// updateLeft handles the plain keys of the project list. ok is false for keys it does not use.
func (m model) updateLeft(key string) (next tea.Model, cmd tea.Cmd, ok bool) {
	switch key {
	case "j", "down":
		m.selectProject(m.selected + 1)
	case "k", "up":
		m.selectProject(m.selected - 1)
	case "g", "home":
		m.selectProject(0)
	case "G", "end":
		m.selectProject(len(m.projects) - 1)
	case "l", "right", "enter":
		// Opening a project means going into its Claude tab.
		if m.current() != nil {
			m.focus = colCenter
		}
	case "a":
		next, cmd := m.startAdd()
		return next, cmd, true
	case "d":
		m.confirmRemove = m.current() != nil
	case "i":
		m.openWizard()
	default:
		return m, nil, false
	}
	return m, nil, true
}

// updateRight handles the plain keys of the right column. ok is false for keys it does not use.
func (m model) updateRight(key string) (next tea.Model, cmd tea.Cmd, ok bool) {
	p := m.current()
	if p == nil {
		return m, nil, false
	}
	if m.rightTab == rightPRDs {
		return m.updatePRDs(key)
	}
	tree := p.files()
	switch key {
	case "j", "down":
		tree.move(1)
	case "k", "up":
		tree.move(-1)
	case "g", "home":
		tree.cursor = 0
	case "G", "end":
		tree.cursor = clamp(len(tree.rows)-1, len(tree.rows))
	case "h", "left":
		tree.collapse()
	case "l", "right", "enter":
		r, found := tree.selected()
		switch {
		case !found:
		case r.dir:
			tree.toggle()
		default:
			// Files open in the editor as a new tab of the center column.
			m.openTerminal(editorCommand(r.path))
		}
	default:
		return m, nil, false
	}
	return m, nil, true
}

// updatePRDs handles the plain keys of the PRD list. ok is false for keys it does not use.
func (m model) updatePRDs(key string) (next tea.Model, cmd tea.Cmd, ok bool) {
	p := m.current()
	rows := p.prdRows()
	switch key {
	case "j", "down":
		p.prdCursor = clamp(p.prdCursor+1, len(rows))
	case "k", "up":
		p.prdCursor = clamp(p.prdCursor-1, len(rows))
	case "g", "home":
		p.prdCursor = 0
	case "G", "end":
		p.prdCursor = clamp(len(rows)-1, len(rows))
	case "l", "right", "enter":
		// The group row folds the done PRDs, a PRD opens in the editor as a center tab.
		if d := p.selectedPRD(); d != nil {
			m.openTerminal(editorCommand(filepath.Join(d.Root, d.Path)))
		} else if len(rows) > 0 {
			p.toggleDone()
		}
	case "s":
		return m, m.startPRD(), true
	case "x":
		return m, m.stopPRD(), true
	case "D":
		return m, m.toggleDev(), true
	case "X":
		m.discardPRD()
	case "c":
		// The run owns the status line until it ends or gets discarded.
		if d := p.selectedPRD(); d != nil && p.runs[d.Name] != nil {
			m.err = d.Label() + " has a run. Discard it first with X."
			return m, nil, true
		}
		m.openStatusPicker()
	case "n":
		next, cmd := m.startCreate()
		return next, cmd, true
	default:
		return m, nil, false
	}
	return m, nil, true
}

// updateBottom handles the plain keys of the pane below the terminals. ok is false for keys it
// does not use.
func (m model) updateBottom(key string) (next tea.Model, cmd tea.Cmd, ok bool) {
	p := m.current()
	if p == nil {
		return m, nil, false
	}
	if key == "z" {
		m.bottomZoom = !m.bottomZoom
		m.resizeTerminals()
		return m, nil, true
	}
	if m.bottomTab == bottomDetails {
		if key != "i" {
			return m, nil, false
		}
		m.openWizard()
		return m, nil, true
	}
	services := m.docker.ProjectServices(p.path)
	switch key {
	case "j", "down":
		p.dockerCursor = clamp(p.dockerCursor+1, len(services))
	case "k", "up":
		p.dockerCursor = clamp(p.dockerCursor-1, len(services))
	case "g", "home":
		p.dockerCursor = 0
	case "G", "end":
		p.dockerCursor = clamp(len(services)-1, len(services))
	case "l", "right", "enter":
		// The logs of a container follow in a new tab of the center column.
		if i := p.dockerCursor; i < len(services) && services[i].Container != nil {
			m.openTerminal([]string{"docker", "logs", "-f", "--tail", "200", services[i].Container.Names})
		} else {
			m.err = "This service has no container."
		}
	default:
		return m, nil, false
	}
	return m, nil, true
}

// footerKeys is the key help of the focused column.
func (m model) footerKeys() string {
	switch {
	case m.inTerminal():
		return " alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+t terminal · alt+w close · alt+m menu · alt+q quit"
	case m.focus == colLeft || m.current() == nil:
		return " j/k select · l open · a add · d remove · i edit .lazydev · alt+←/→ pane · alt+shift+←/→ tab · t terminal · m menu · q quit"
	case m.focus == colBottom && m.bottomTab == bottomDetails:
		return " i edit .lazydev · z zoom · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · m menu · q quit"
	case m.focus == colBottom && m.bottomTab == bottomDocker:
		return " j/k move · l logs · z zoom · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · m menu · q quit"
	case m.focus == colRight && m.rightTab == rightPRDs:
		return " j/k move · l open · n new · s start · x stop · X discard · D dev · c status · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · m menu · q quit"
	case m.focus == colRight && m.rightTab == rightFiles:
		return " j/k move · l open · h back · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · m menu · q quit"
	}
	return " alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · m menu · q quit"
}
