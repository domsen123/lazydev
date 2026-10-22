package tui

import (
	"path/filepath"

	"slices"

	"lazydev/internal/project"
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
	actGit
	actUpdate
	actCopy
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
	"alt+g": actGit,
	"alt+u": actUpdate,
	"alt+c": actCopy,
	"alt+q": actQuit, "alt+ctrl+c": actQuit,
}

// updateKey routes a key: open dialogs first, then global alt keys, then the terminal of the
// center column, then the focused column, and finally the plain key as its alt key.
func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err, m.note = "", ""
	switch {
	case m.menu != nil:
		return m.updateMenu(msg)
	case m.cockpit != nil:
		return m.updateCockpit(msg)
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
		// In copy mode no key goes to the program.
		if term := m.current().activeTerminal(); term.InCopy() {
			if text, done := term.CopyKey(key); done {
				return m, copyToClipboard(text)
			}
		} else {
			term.SendKey(msg)
		}
		return m, nil
	}
	var handled bool
	var next tea.Model = m
	var cmd tea.Cmd
	switch m.focus {
	case colCenter:
		if m.inLogView() {
			handled = m.current().activeLog().update(key)
		}
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
	// Rule 2: outside a terminal, alt is optional. g skips it: rule 3 makes g "to the top" in every
	// pane, so in a pane without list it must not open the git cockpit. c skips it too, it changes
	// the status in the PRDs tab and must not open copy mode elsewhere.
	if key != "g" && key != "c" {
		if next, cmd, ok := m.global("alt+" + key); ok {
			return next, cmd
		}
	}
	return m, nil
}

// wizardVisible is true while the wizard is open and not waiting for its editor.
func (m model) wizardVisible() bool {
	return m.wizard != nil && m.wizard.editor == 0
}

// inTerminal is true while keys go to a terminal: the center column has the focus and its active
// tab is a terminal.
func (m model) inTerminal() bool {
	p := m.current()
	return m.focus == colCenter && p != nil && p.activeTerminal() != nil
}

// inLogView is true while the center column has the focus and its active tab is a log tab.
func (m model) inLogView() bool {
	p := m.current()
	return m.focus == colCenter && p != nil && p.activeLog() != nil
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
		// A terminal closes once its program ends, a log tab right away.
		switch {
		case p == nil:
		case p.activeTerminal() != nil:
			p.activeTerminal().Hangup()
		case p.activeLog() != nil:
			p.removeTab(p.tab)
		}
	case actMenu:
		m.menu = &menu{}
	case actGit:
		m.openCockpit()
	case actUpdate:
		return m, m.startUpdate(), true
	case actCopy:
		// Copy mode belongs to the tab: alt+c focuses the terminal and opens or leaves it.
		if p == nil || p.activeTerminal() == nil {
			m.err = "The active tab is not a terminal."
			break
		}
		m.focus = colCenter
		if term := p.activeTerminal(); term.InCopy() {
			term.LeaveCopy()
		} else {
			term.EnterCopy(false, false)
		}
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
	case "L":
		// The group row has no run.
		if d := p.selectedPRD(); d != nil {
			if p.runs[d.Name] == nil {
				m.err = d.Label() + " has no run."
				return m, nil, true
			}
			return m, m.openLog(projectIDs(m.projects)[m.selected], d.Name), true
		}
	case "s":
		return m, m.startPRD(selectedName(p)), true
	case "x":
		return m, m.stopPRD(selectedName(p)), true
	case "D":
		return m, m.toggleDev(selectedName(p)), true
	case "X":
		m.discardPRD(selectedName(p))
	case "f":
		m.unblockPRD(selectedName(p))
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
	if key == "i" && m.bottomTab == bottomDetails {
		m.openWizard()
		return m, nil, true
	}
	if m.runListVisible() {
		return m.updateRunList(key)
	}
	if m.bottomTab == bottomDetails {
		return m, nil, false
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

// updateRunList handles the plain keys of the run list. ok is false for keys it does not use.
func (m model) updateRunList(key string) (next tea.Model, cmd tea.Cmd, ok bool) {
	p := m.current()
	names := p.runList()
	switch key {
	case "j", "down":
		p.runCursor = clamp(p.runCursor+1, len(names))
	case "k", "up":
		p.runCursor = clamp(p.runCursor-1, len(names))
	case "g", "home":
		p.runCursor = 0
	case "G", "end":
		p.runCursor = clamp(len(names)-1, len(names))
	case "l", "right", "enter":
		return m, m.openLog(projectIDs(m.projects)[m.selected], p.selectedRun()), true
	case "s":
		return m, m.startPRD(p.selectedRun()), true
	case "x":
		return m, m.stopPRD(p.selectedRun()), true
	case "D":
		return m, m.toggleDev(p.selectedRun()), true
	case "X":
		m.discardPRD(p.selectedRun())
	case "f":
		m.unblockPRD(p.selectedRun())
	default:
		return m, nil, false
	}
	return m, nil, true
}

// selectedName is the name of the PRD under the cursor of the PRD list, empty on the group row.
func selectedName(p *projectEntry) string {
	if d := p.selectedPRD(); d != nil {
		return d.Name
	}
	return ""
}

// unblockKey is " · f unblock" while the run of the PRD called name is blocked, empty otherwise.
func (m model) unblockKey(name string) string {
	if r := m.current().runs[name]; r != nil && r.Effective() == project.PhaseBlocked {
		return " · f unblock"
	}
	return ""
}

// footerKeys is the key help of the focused column.
func (m model) footerKeys() string {
	switch {
	case m.inTerminal() && m.current().activeTerminal().InCopy():
		return " j/k move · ctrl+d/u page · g/G top/end · v select · y copy · esc leave · alt+←/→ pane · alt+shift+←/→ tab · alt+w close · alt+m menu · alt+q quit"
	case m.inTerminal():
		return " alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+t terminal · alt+w close · alt+c copy · alt+g git · alt+m menu · alt+q quit"
	case m.inLogView():
		return " j/k scroll · ctrl+d/u page · g/G top/end · z header · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+w close · t terminal · alt+g git · m menu · q quit"
	case m.focus == colLeft || m.current() == nil:
		return " j/k select · l open · a add · d remove · i edit .lazydev · alt+←/→ pane · alt+shift+←/→ tab · t terminal · alt+g git · m menu · q quit"
	case m.focus == colBottom && m.runListVisible():
		return " j/k move · l log · s start · x stop · X discard · D dev" + m.unblockKey(m.current().selectedRun()) + " · i edit .lazydev · z zoom · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · alt+g git · m menu · q quit"
	case m.focus == colBottom && m.bottomTab == bottomDetails:
		return " i edit .lazydev · z zoom · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · alt+g git · m menu · q quit"
	case m.focus == colBottom && m.bottomTab == bottomDocker:
		return " j/k move · l logs · z zoom · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · alt+g git · m menu · q quit"
	case m.focus == colRight && m.rightTab == rightPRDs:
		return " j/k move · l open · L log · n new · s start · x stop · X discard · D dev" + m.unblockKey(selectedName(m.current())) + " · c status · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+g git · m menu · q quit"
	case m.focus == colRight && m.rightTab == rightFiles:
		return " j/k move · l open · h back · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+g git · m menu · q quit"
	}
	return " alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · alt+g git · m menu · q quit"
}
