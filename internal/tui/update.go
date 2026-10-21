package tui

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm := next.(Model)
	nm.syncNotifications()
	nm.syncIssues()
	nm.refreshLog()
	nm.syncDetail()
	nm.syncPRDView()
	nm.syncIssueView()
	return nm, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.mode == modeNewPRD {
			m.input.Width = m.newPRDInputWidth()
		}
		return m, nil
	case projectsLoadedMsg:
		m.applyLoaded(msg)
		m.applyChats(msg.chats)
		fetch := m.initIssues()
		if m.watchEnabled && m.watch == nil {
			dirs := make([]string, len(m.projects))
			for i, p := range m.projects {
				dirs[i] = p.Path
			}
			m.watch = newWatcher(dirs)
			m.dockerCh = dockerEvents()
			return m, tea.Batch(fetch, m.watch.next(), m.loadDocker(), waitDockerEvent(m.dockerCh), dockerTick())
		}
		return m, fetch
	case issuesLoadedMsg:
		m.applyIssues(msg)
		return m, nil
	case issuesTickMsg:
		return m, tea.Batch(m.fetchAllIssues(), issuesTick())
	case dockerLoadedMsg:
		m.docker = msg.info
		return m, nil
	case dockerEventMsg:
		return m, tea.Batch(m.loadDocker(), waitDockerEvent(m.dockerCh))
	case dockerTickMsg:
		return m, tea.Batch(m.loadDocker(), dockerTick())
	case databasesLoadedMsg:
		if p := m.selectedProject(); p != nil && p.Path == msg.project {
			m.dockerDBs = msg.dbs
		}
		return m, nil
	case runsLoadedMsg:
		if !sameSignature(m.runs.signature(), msg.runs.signature()) {
			// Eine neue Phase kann die PRD-Liste ändern, etwa nach dem Merge.
			m.runs = msg.runs
			return m, m.loadProjects("")
		}
		m.runs = msg.runs
		return m, nil
	case watchMsg:
		cmds := []tea.Cmd{m.watch.next()}
		switch {
		case msg.prds:
			cmds = append(cmds, m.loadProjects(""))
		case msg.runs:
			cmds = append(cmds, loadRunsCmd)
		}
		if msg.chats {
			cmds = append(cmds, loadChatsCmd)
		}
		// Neue Log-Zeilen liest Update am Ende über refreshLog, die des Chat-Fensters refresh.
		if m.chat != nil {
			m.chat.refresh()
		}
		return m, tea.Batch(cmds...)
	case tickMsg:
		return m, tea.Batch(tick(), loadRunsCmd, loadLimits, loadChatsCmd)
	case chatsLoadedMsg:
		m.applyChats(msg.all)
		return m, nil
	case limitsLoadedMsg:
		m.limits, m.limitsOK = msg.snap, msg.ok
		return m, nil
	case limitsRefreshedMsg:
		m.limitsRefreshing = false
		if msg.err != nil {
			m.setMessage("Limits nicht aktualisiert: "+firstLine(msg.err.Error()), true)
		}
		return m, loadLimits
	case actionDoneMsg:
		if msg.err != nil {
			m.setMessage(firstLine(msg.err.Error()), true)
		} else {
			m.setMessage(msg.text, false)
		}
		return m, tea.Batch(m.loadProjects(""), m.loadDocker())
	case prdEditedMsg:
		return m.handlePRDEdited(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg), nil
	case tea.KeyMsg:
		switch m.mode {
		case modeAddProject:
			return m.updateAddProject(msg)
		case modeConfirmRemove:
			return m.updateConfirmRemove(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeEvent:
			return m.updateEvent(msg)
		case modeBranch:
			return m.updateBranch(msg)
		case modeInit:
			return m.updateInitWizard(msg)
		case modeNewPRD:
			return m.updateNewPRD(msg)
		case modeChat:
			return m.updateChat(msg)
		case modeDocker:
			return m.updateDocker(msg)
		case modeHelp:
			m.mode = modeNormal
			return m, nil
		case modeLimits:
			return m.updateLimits(msg)
		case modePRDView:
			return m.updatePRDView(msg)
		case modeIssueView:
			return m.updateIssueView(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Eine Meldung gilt bis zur nächsten Taste. Aktionen setzen danach ihre eigene.
	m.setMessage("", false)
	if m.focus == focusNotifications {
		if next, cmd, handled := m.updateNotifications(msg); handled {
			return next, cmd
		}
	}
	if m.focus == focusIssues {
		if next, cmd, handled := m.updateIssues(msg); handled {
			return next, cmd
		}
	}
	switch msg.String() {
	case "?":
		m.mode = modeHelp
		return m, nil
	case "u":
		m.mode = modeLimits
		return m, loadLimits
	case "q", "ctrl+c":
		if m.watch != nil {
			m.watch.Close()
		}
		return m, tea.Quit
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "pgdown", "ctrl+d":
		m.moveCursor(m.logPage())
	case "pgup", "ctrl+u":
		m.moveCursor(-m.logPage())
	case "G", "end":
		if m.focus == focusLog {
			m.logFollow = true
			m.detail.GotoBottom()
		}
	case "tab":
		m.focus = m.nextFocus(1, true)
	case "shift+tab":
		m.focus = m.nextFocus(-1, true)
	case "left":
		m.focus = m.nextFocus(-1, false)
	case "right":
		m.focus = m.nextFocus(1, false)
	case "e":
		m.toggleDone()
	case "S":
		m.toggleSort()
	case "enter":
		if m.focus == focusPRDs && m.selectedPRD() == nil && len(m.prdList(m.selectedProject())) > 0 {
			m.toggleDone()
		}
		if m.focus == focusLog && m.logCursor < len(m.logLines) {
			m.mode = modeEvent
			m.event = prettyEvent(m.logLines[m.logCursor].raw)
			m.eventTop = 0
		}
	case "l":
		m.logFull = !m.logFull
		if m.logFull {
			m.focus = focusLog
		} else if m.focus == focusLog {
			m.focus = focusPRDs
		}
	case "n":
		return m.newPRD()
	case "N":
		return m.nextNotification()
	case "v":
		return m.openPRDView()
	case "D":
		return m.openDocker()
	case "o":
		return m.openSession()
	case "w":
		return m.openShell()
	case "p":
		return m.toggleDev()
	case "a":
		m.mode = modeAddProject
		m.input.Prompt = "Projektpfad: "
		m.input.Placeholder = "~/development/…"
		m.input.Width = 0
		m.candidates = nil
		m.input.SetValue("~/")
		m.input.CursorEnd()
		return m, m.input.Focus()
	case "d":
		if m.selectedProject() != nil {
			m.mode = modeConfirmRemove
		}
	case "b":
		return m.openBranchPicker()
	case "i":
		return m.openInitWizard()
	case "s":
		return m.startPRD()
	case "m":
		return m.mergePRD()
	case "x":
		return m.stopPRD()
	case "X":
		return m.discardPRD()
	case "r":
		m.loading = true
		m.setMessage("", false)
		return m, tea.Batch(m.loadProjects(""), m.fetchAllIssues())
	}
	return m, nil
}

func (m Model) updateEvent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.mode = modeNormal
		m.event = ""
	case "j", "down":
		m.eventTop++
	case "k", "up":
		m.eventTop = max(0, m.eventTop-1)
	case "pgdown", "ctrl+d", " ":
		m.eventTop += max(1, m.height-4)
	case "pgup", "ctrl+u":
		m.eventTop = max(0, m.eventTop-max(1, m.height-4))
	}
	return m, nil
}

// nextFocus liefert den Fokus delta Schritte weiter. wrap springt vom Ende an den Anfang und
// umgekehrt. Unsichtbare Boxen „Benachrichtigungen“ und „Issues“ überspringt nextFocus.
func (m Model) nextFocus(delta int, wrap bool) focus {
	const n = int(focusLog) + 1
	f := int(m.focus)
	for range n {
		next := f + delta
		switch {
		case wrap:
			next = (next + n) % n
		case next < 0 || next >= n:
			return focus(f)
		}
		f = next
		switch {
		case focus(f) == focusNotifications && !m.showNotifications():
		case focus(f) == focusIssues && !m.showIssues():
		default:
			return focus(f)
		}
	}
	return m.focus
}

func (m Model) logPage() int {
	return max(1, m.layout().logRows())
}

func (m *Model) moveCursor(delta int) {
	if m.focus == focusLog && m.selectedRun() == nil {
		if delta > 0 {
			m.detail.ScrollDown(delta)
		} else {
			m.detail.ScrollUp(-delta)
		}
		return
	}
	if m.focus == focusLog {
		m.logCursor = clamp(m.logCursor+delta, len(m.logLines))
		m.logFollow = m.logCursor >= len(m.logLines)-1
		return
	}
	if m.focus == focusNotifications {
		if p := m.selectedProject(); p != nil {
			m.notifCursor = clamp(m.notifCursor+delta, len(m.notifications(p.ID)))
		}
		return
	}
	if m.focus == focusIssues {
		if p := m.selectedProject(); p != nil {
			m.issueCursor = clamp(m.issueCursor+delta, len(m.projectIssues(p.ID)))
		}
		return
	}
	if m.focus == focusProjects {
		next := clamp(m.projCursor+delta, len(m.projects))
		if next != m.projCursor {
			m.projCursor = next
			m.prdCursor = 0
		}
		return
	}
	m.prdCursor = clamp(m.prdCursor+delta, len(m.prdList(m.selectedProject())))
}

func (m Model) updateAddProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "tab":
		completed, candidates := completePath(m.input.Value())
		m.input.SetValue(completed)
		m.input.CursorEnd()
		m.candidates = candidates
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			return m, nil
		}
		added, err := m.cfg.AddProject(path)
		if err == nil {
			err = m.cfg.Save(m.cfgPath)
			if err != nil {
				// Ohne gespeicherte Datei soll die Liste nichts zeigen, was nach einem
				// Neustart fehlt.
				_ = m.cfg.RemoveProject(len(m.cfg.Projects) - 1)
			}
		}
		if err != nil {
			m.setMessage(err.Error(), true)
			return m, nil
		}
		m.mode = modeNormal
		m.input.Blur()
		m.candidates = nil
		if m.watch != nil {
			m.watch.addProject(added.AbsPath())
		}
		m.setMessage("Projekt hinzugefügt: "+added.Path, false)
		m.loading = true
		return m, m.loadProjects(added.AbsPath())
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.candidates = nil
	return m, cmd
}

func (m Model) updateConfirmRemove(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeNormal
	if s := msg.String(); s != "j" && s != "y" {
		m.setMessage("Entfernen abgebrochen", false)
		return m, nil
	}
	p := m.selectedProject()
	if p == nil {
		return m, nil
	}
	idx := m.configIndex(p.Path)
	if idx < 0 {
		m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
		return m, nil
	}
	removed := m.cfg.Projects[idx]
	_ = m.cfg.RemoveProject(idx)
	if err := m.cfg.Save(m.cfgPath); err != nil {
		m.cfg.Projects = slices.Insert(m.cfg.Projects, idx, removed)
		m.setMessage(err.Error(), true)
		return m, nil
	}
	delete(m.issues, p.ID)
	m.setMessage("Projekt entfernt: "+removed.Path+". Die Dateien bleiben erhalten.", false)
	m.loading = true
	return m, m.loadProjects("")
}

func (m Model) configIndex(absPath string) int {
	for i, p := range m.cfg.Projects {
		if p.AbsPath() == absPath {
			return i
		}
	}
	return -1
}

// handleMouse wählt per Klick ein Projekt oder PRD aus und scrollt mit dem Mausrad.
func (m Model) handleMouse(msg tea.MouseMsg) Model {
	// Im PRD-Fenster scrollt nur das Mausrad, um eine Zeile pro Ereignis.
	if m.mode == modePRDView && m.prdView != nil {
		switch msg.Button {
		case tea.MouseButtonWheelDown:
			m.prdView.viewport.ScrollDown(1)
		case tea.MouseButtonWheelUp:
			m.prdView.viewport.ScrollUp(1)
		}
		return m
	}
	if m.mode != modeNormal {
		return m
	}
	l := m.layout()
	inTop := msg.Y > 0 && msg.Y < l.topHeight-1
	onLeft := msg.X < l.leftWidth
	// Klicks und Mausrad über der rechten Spalte bewirken nichts.
	if l.sideWidth > 0 && msg.Y < l.topHeight && msg.X >= l.leftWidth+l.rightWidth {
		return m
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		switch {
		case inTop && onLeft:
			m.focus = focusProjects
		case inTop:
			m.focus = focusPRDs
		case msg.Y >= l.topHeight && msg.Y < l.topHeight+l.detailHeight:
			m.focus = focusLog
		default:
			return m
		}
		delta := 1
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -1
		}
		m.moveCursor(delta)
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress || !inTop {
			return m
		}
		row := msg.Y - 1
		if onLeft {
			m.focus = focusProjects
			start, _ := scrollWindow(m.projCursor, len(m.projects), l.topHeight-2)
			if i := start + row; i < len(m.projects) && i != m.projCursor {
				m.projCursor = i
				m.prdCursor = 0
			}
			return m
		}
		m.focus = focusPRDs
		p := m.selectedProject()
		if p == nil {
			return m
		}
		header := m.prdHeaderRows(p, l)
		n := len(m.prdList(p))
		start, _ := scrollWindow(m.prdCursor, n, m.prdRows(p, l))
		if i := start + row - header; row >= header && row-header < m.prdRows(p, l) && i < n {
			// Ein zweiter Klick auf die Gruppenzeile klappt sie auf oder zu.
			if i == m.prdCursor && m.selectedPRD() == nil {
				m.toggleDone()
				return m
			}
			m.prdCursor = i
		}
	}
	return m
}
