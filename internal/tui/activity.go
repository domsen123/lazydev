package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	"lazydev/internal/activity"
	"lazydev/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// activityState is what the activity log needs between events: the terminal tab the keys go to
// and the phase of every run the last poll saw. The model shares one pointer, so the value
// receivers of Update see the same state.
type activityState struct {
	// inTerm is true while keys go to a terminal tab; termTab names it, termAt is the entry,
	// termLast the last key and termActive the active time, idle gaps left out.
	inTerm     bool
	termTab    string
	termAt     time.Time
	termLast   time.Time
	termActive time.Duration
	termKeys   int
	// phases maps "<id>/<prd>" to the phase of the run the last poll saw.
	phases map[string]string
}

// paneNames are the names of the panes in the log.
var paneNames = map[int]string{colLeft: "projects", colCenter: "center", colRight: "explorer", colBottom: "details"}

func newActivityState() *activityState {
	activity.Log(activity.Event{Kind: activity.KindSession, Text: "start", Version: version.Version})
	return &activityState{phases: map[string]string{}}
}

// dialogName names the open overlay, in the order updateKey routes keys. Empty without one.
func (m model) dialogName() string {
	switch {
	case m.menu != nil:
		if m.menu.editing != "" {
			return "menu-input"
		}
		return "menu"
	case m.cockpit != nil:
		return "git"
	case m.question != nil:
		return "question"
	case m.status != nil:
		return "status"
	case m.wizardVisible():
		return "wizard"
	case m.adding:
		return "add"
	case m.creating:
		return "create"
	case m.confirmRemove:
		return "remove"
	}
	return ""
}

// activityEvent is an event of kind with the place the user is at: pane, tab, dialog, project
// and the selected PRD with its phase.
func (m model) activityEvent(kind string) activity.Event {
	e := activity.Event{Kind: kind, Pane: paneNames[m.focus], Dialog: m.dialogName()}
	p := m.current()
	if p == nil {
		return e
	}
	e.Project = p.name()
	switch m.focus {
	case colCenter:
		e.Tab = m.centerTabName()
	case colRight:
		e.Tab = strings.ToLower(rightTabs[m.rightTab])
	case colBottom:
		e.Tab = strings.ToLower(bottomTabs[m.bottomTab])
	}
	if d := p.selectedPRD(); d != nil {
		e.PRD = d.Name
		e.Phase = string(d.Phase())
		if r := p.runs[d.Name]; r != nil {
			e.Phase = string(r.Effective())
		}
	}
	return e
}

// centerTabName is "terminal:<program>", "log", "md" or empty without a tab.
func (m model) centerTabName() string {
	p := m.current()
	if p == nil {
		return ""
	}
	switch {
	case p.activeTerminal() != nil:
		return "terminal:" + p.activeTerminal().TabTitle()
	case p.activeLog() != nil:
		return "log"
	case p.activeView() != nil:
		return "md"
	}
	return ""
}

// isGlobalKey is true for keys global runs: alt keys of actions and alt+1 to alt+9.
func isGlobalKey(key string) bool {
	if _, ok := globalKeys[key]; ok {
		return true
	}
	return len(key) == 5 && key[:4] == "alt+" && key[4] >= '1' && key[4] <= '9'
}

// trackKey logs a key before updateKey routes it. Keys that a terminal tab gets are not logged
// one by one: the tab gets an "enter" event and a "leave" event with their count and active
// time. Single characters typed into a dialog's input stay out of the log too.
func (m model) trackKey(key string) {
	a := m.act
	if a == nil {
		return
	}
	dialog := m.dialogName()
	if dialog == "" && m.inTerminal() && !isGlobalKey(key) {
		tab := m.centerTabName()
		now := time.Now()
		if !a.inTerm || a.termTab != tab {
			a.leaveTerminal()
			a.inTerm, a.termTab, a.termAt, a.termLast = true, tab, now, now
			activity.Log(activity.Event{Kind: activity.KindTerminal, Text: "enter", Tab: tab, Project: m.current().name()})
		}
		if gap := now.Sub(a.termLast); gap < activity.Idle {
			a.termActive += gap
		}
		a.termLast = now
		a.termKeys++
		return
	}
	a.leaveTerminal()
	if dialog != "" && utf8.RuneCountInString(key) == 1 {
		return
	}
	e := m.activityEvent(activity.KindKey)
	e.Key = key
	activity.Log(e)
}

// leaveTerminal ends the terminal span, if one is open.
func (a *activityState) leaveTerminal() {
	if a == nil || !a.inTerm {
		return
	}
	activity.Log(activity.Event{
		Kind: activity.KindTerminal, Text: "leave", Tab: a.termTab,
		Count: a.termKeys, Seconds: a.termActive.Seconds(),
	})
	a.inTerm, a.termTab, a.termKeys, a.termActive = false, "", 0, 0
}

// trackMouse logs a press, not the wheel and not motion.
func (m model) trackMouse(msg tea.MouseMsg) {
	if m.act == nil || msg.Action != tea.MouseActionPress {
		return
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown ||
		msg.Button == tea.MouseButtonWheelLeft || msg.Button == tea.MouseButtonWheelRight {
		return
	}
	m.act.leaveTerminal()
	e := m.activityEvent(activity.KindMouse)
	e.Key = msg.String()
	activity.Log(e)
}

// trackAction logs the footer text of an action.
func (m model) trackAction(text string, err error) {
	if m.act == nil {
		return
	}
	e := activity.Event{Kind: activity.KindAction, Text: text}
	if err != nil {
		e.Err = err.Error()
	}
	if p := m.current(); p != nil {
		e.Project = p.name()
	}
	activity.Log(e)
}

// trackRuns logs every phase change the poll shows, compared with the poll before. Runs that
// are gone are forgotten.
func (m model) trackRuns() {
	a := m.act
	if a == nil {
		return
	}
	seen := map[string]bool{}
	ids := projectIDs(m.projects)
	for i, p := range m.projects {
		for name, r := range p.runs {
			key := ids[i] + "/" + name
			seen[key] = true
			phase := string(r.Effective())
			prev, known := a.phases[key]
			if known && prev == phase {
				continue
			}
			a.phases[key] = phase
			activity.Log(activity.Event{Kind: activity.KindRun, Project: p.name(), PRD: name, Phase: phase, From: prev})
		}
	}
	for key := range a.phases {
		if !seen[key] {
			delete(a.phases, key)
		}
	}
}

// trackEnd closes the terminal span and the session.
func (m model) trackEnd() {
	if m.act == nil {
		return
	}
	m.act.leaveTerminal()
	activity.Log(activity.Event{Kind: activity.KindSession, Text: "end"})
}
