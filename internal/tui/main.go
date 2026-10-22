// Package tui is lazydev's terminal UI in three columns: the projects on the left, terminal tabs of
// the selected project in the center with the details pane below, files and PRDs on the right.
// Claude Code starts in the selected project. The TUI controls no runner directly, it reads their
// state files.
package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/docker"
	"lazydev/internal/fsutil"
	"lazydev/internal/repo"
	"lazydev/internal/terminal"
	"lazydev/internal/usage"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	// sideMin is the minimum width of the side columns.
	sideMin = 20
	// sideRatio splits the width: each side column gets one sideRatio-th of it.
	sideRatio = 5
	// centerChrome is the number of rows in the center column that are not terminal: two borders,
	// the tab bar and the separator.
	centerChrome = 4
	// bottomMin is the minimum height of the pane below the terminals, bottomRatio its share of
	// the height.
	bottomMin   = 8
	bottomRatio = 4
)

// The panes. colLeft, colCenter and colRight index the box colors of a theme, colBottom is the pane
// below the terminals and borrows the center color.
const (
	colLeft = iota
	colCenter
	colRight
	colBottom
)

// focusOrder is the order in which alt+←/→ and tab walk the panes.
var focusOrder = []int{colLeft, colCenter, colRight, colBottom}

const (
	bottomDetails = iota
	bottomDocker
)

// bottomTabs are the tabs of the pane below the terminals.
var bottomTabs = []string{"Details", "Docker"}

// startCommand runs in a project that has no tab yet.
var startCommand = []string{"claude"}

// sender lets the terminal goroutines reach the program, which exists only after the model.
type sender struct{ p *tea.Program }

func (s *sender) send(msg tea.Msg) {
	if s.p != nil {
		s.p.Send(msg)
	}
}

type model struct {
	width, height int
	// focus is the active column.
	focus int
	// projects are the folders of the left column, selected is the active one.
	projects []*projectEntry
	selected int
	nextID   int
	bus      *sender
	err      string
	// theme is the index of the active color theme in themes.
	theme int
	// rightTab is the active tab of the right column, bottomTab the one below the terminals.
	rightTab  int
	bottomTab int
	// docker is the container state of all projects, refreshed on docker events and by a tick.
	docker       docker.Info
	dockerEvents <-chan struct{}
	stopDocker   context.CancelFunc
	// menu is the open esc menu, nil when closed.
	menu *menu
	// wizard edits .lazydev/ of the selected project, nil when closed.
	wizard *wizard
	// status is the open status picker of a PRD, nil when closed.
	status *statusPicker
	// cockpit is the open git cockpit of the selected project, nil when closed.
	cockpit *cockpit
	// gitTabs maps terminal IDs of git cockpit tabs to their action.
	gitTabs map[int]string
	// question is an open yes/no question in the footer, nil without one.
	question *question
	// waiting is the queue of waiting PRDs as "<id>/<prd>". last is the last log entry and devPIDs
	// the dev server of each run of the selected project, by PRD name.
	waiting []string
	last    map[string]claude.Entry
	devPIDs map[string]int
	// usage is the last known state of the Claude limits, from the runners' stream-json.
	usage    usage.Snapshot
	hasUsage bool
	// bottomZoom gives the pane below the terminals most of the height, e.g. for a long run list.
	bottomZoom bool
	// note is a message for the footer that is no error, cleared by the next key.
	note string
	// latest is a release newer than the running version, empty while none is known. updating is
	// true while alt+u installs it, installed is the tag once it is installed.
	latest    string
	updating  bool
	installed string
	// adding is true while the input for a new folder is open.
	adding bool
	input  textinput.Model
	// creating is true while the input for the description of a new PRD is open.
	creating    bool
	createInput textinput.Model
	// confirmRemove is true while the question to remove the selected project is open.
	confirmRemove bool
	// quitting is true while the programs end before lazydev exits.
	quitting bool
}

// quitMsg reports that all programs have ended.
type quitMsg struct{}

func newModel(bus *sender) model {
	in := textinput.New()
	in.Prompt = " Add folder: "
	create := textinput.New()
	create.Prompt = " New PRD, what is it about? "
	create.Placeholder = "empty: Claude asks"
	m := model{bus: bus, input: in, createInput: create, focus: colCenter, theme: loadTheme(), gitTabs: map[int]string{}}
	ctx, cancel := context.WithCancel(context.Background())
	m.dockerEvents, m.stopDocker = docker.Events(ctx), cancel
	projects, err := loadProjects()
	if err != nil {
		m.err = err.Error()
	}
	m.projects = projects
	if len(projects) == 0 {
		m.focus = colLeft
	}
	m.readLazydev()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchGitInfo(m.projects), fetchPRDs(m.projects), gitTick(), m.fetchRuns(), runTick(),
		loadDocker(m.projects, nil), waitDockerEvent(m.dockerEvents), dockerTick(), checkUpdate())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		started := m.width > 0
		m.width, m.height = msg.Width, msg.Height
		m.resizeTerminals()
		// Terminals need the real size, so the first one starts with the first size message.
		if !started {
			m.ensureTerminal()
		}
	case terminal.OutputMsg:
		// Nothing to update, the view reads the emulator.
	case terminal.ExitMsg:
		action, ok := m.gitTabs[msg.ID]
		if !ok {
			m.removeTerminal(msg.ID)
			break
		}
		delete(m.gitTabs, msg.ID)
		switch msg.Code {
		case 0:
			m.note = "git " + action + ": done"
		case -1:
			m.err = "git " + action + " stopped"
		default:
			m.err = fmt.Sprintf("git %s failed (exit %d)", action, msg.Code)
		}
		m.removeTerminal(msg.ID)
		return m, fetchGitInfo(m.projects)
	case quitMsg:
		return m, tea.Quit
	case discordTestMsg:
		if msg.err != nil {
			m.err = "discord: " + msg.err.Error()
		} else {
			m.note = "discord: test sent"
		}
	case updateMsg:
		m.latest = msg.latest
		if m.err == "" && m.note == "" {
			m.note = updateNote(msg.latest)
		}
	case installMsg:
		m.updating = false
		if msg.err != nil {
			m.note, m.err = "", "update: "+msg.err.Error()
			break
		}
		m.latest, m.installed = "", msg.tag
		m.err, m.note = "", installedNote(msg.tag)
	case gitInfoMsg:
		for _, p := range m.projects {
			if p.path == msg.path {
				p.git = msg.info
			}
		}
	case gitTickMsg:
		// New or deleted files show up in the tree with the same rhythm as the git info.
		if p := m.current(); p != nil && p.tree != nil {
			p.tree.rebuild()
		}
		m.readLazydev()
		return m, tea.Batch(fetchGitInfo(m.projects), fetchPRDs(m.projects), gitTick())
	case prdsMsg:
		for _, p := range m.projects {
			if p.path != msg.path {
				continue
			}
			// The cursor stays on the same PRD, even if others come or go.
			selected := ""
			if d := p.selectedPRD(); d != nil {
				selected = d.Path
			}
			p.prds, p.prdErr = msg.prds, msg.err
			if selected != "" {
				p.selectPRDPath(selected)
			}
		}
	case actionMsg:
		m.err, m.note = "", msg.text
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		return m, tea.Batch(fetchPRDs(m.projects), m.fetchRuns())
	case runTickMsg:
		return m, tea.Batch(m.fetchRuns(), m.readLogs(), runTick())
	case runsMsg:
		ids := projectIDs(m.projects)
		for i, p := range m.projects {
			p.runs = msg.byID[ids[i]]
			p.closeEndedLogs()
		}
		m.waiting, m.last, m.devPIDs = msg.waiting, msg.last, msg.devPIDs
		m.usage, m.hasUsage = msg.usage, msg.hasUsage
	case logReadMsg:
		m.applyLogs(msg)
	case dockerLoadedMsg:
		m.docker = msg.info
	case dockerEventMsg:
		return m, tea.Batch(loadDocker(m.projects, m.docker.Services), waitDockerEvent(m.dockerEvents))
	case dockerTickMsg:
		return m, tea.Batch(loadDocker(m.projects, m.docker.Services), dockerTick())
	case tea.KeyMsg:
		if m.quitting {
			return m, nil
		}
		return m.updateKey(msg)
	case tea.MouseMsg:
		if m.quitting {
			return m, nil
		}
		return m.updateMouse(msg)
	case copiedMsg:
		m.err, m.note = "", ""
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.note = copiedNote(msg.lines)
		}
	}
	return m, nil
}

// current is the selected project, nil without projects.
func (m model) current() *projectEntry {
	if m.selected < 0 || m.selected >= len(m.projects) {
		return nil
	}
	return m.projects[m.selected]
}

// startAdd opens the input for a new folder below the columns.
func (m model) startAdd() (tea.Model, tea.Cmd) {
	m.adding = true
	m.input.SetValue("~/")
	m.input.CursorEnd()
	return m, m.input.Focus()
}

// updateAdd handles the folder input. Tab completes, Enter adds, Esc cancels.
func (m model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.adding = false
		m.input.Blur()
		return m, nil
	case "tab":
		m.input.SetValue(config.CompletePath(m.input.Value()))
		m.input.CursorEnd()
		return m, nil
	case "enter":
		if err := m.addProject(m.input.Value()); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.adding = false
		m.input.Blur()
		return m, tea.Batch(fetchGitInfo(m.projects), fetchPRDs(m.projects), loadDocker(m.projects, m.docker.Services))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// addProject adds a folder, selects it and focuses its Claude tab. A folder that is already in the
// list only gets selected.
func (m *model) addProject(input string) error {
	path, err := config.ExpandPath(strings.TrimSpace(input))
	if err != nil {
		return err
	}
	if !fsutil.IsDir(path) {
		return fmt.Errorf("not a folder: %s", config.DisplayPath(path))
	}
	// Every project gets .lazydev/, an existing one only its missing files.
	if err := repo.Init(path); err != nil {
		return err
	}
	if i := slices.IndexFunc(m.projects, func(p *projectEntry) bool { return p.path == path }); i >= 0 {
		m.selectProject(i)
		return nil
	}
	projects := append(slices.Clone(m.projects), &projectEntry{path: path})
	if err := saveProjects(projects); err != nil {
		return err
	}
	m.projects = projects
	m.selectProject(len(projects) - 1)
	m.focus = colCenter
	return nil
}

// updateConfirmRemove answers the question to remove the selected project.
func (m model) updateConfirmRemove(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirmRemove = false
	if s := msg.String(); s != "y" && s != "j" {
		return m, nil
	}
	p := m.current()
	if p == nil {
		return m, nil
	}
	projects := slices.Delete(slices.Clone(m.projects), m.selected, m.selected+1)
	if err := saveProjects(projects); err != nil {
		m.err = err.Error()
		return m, nil
	}
	// The project is gone from the list, so its exitMsgs find nothing. Release here instead.
	for _, t := range p.terminals() {
		t.Hangup()
		go func() {
			<-t.Done()
			t.Release()
		}()
	}
	m.projects = projects
	m.selectProject(min(m.selected, len(projects)-1))
	return m, nil
}

// selectProject selects project i and starts Claude there if it has no tab yet. The focus stays
// where it is, so browsing the list does not jump into the terminal.
func (m *model) selectProject(i int) {
	n := len(m.projects)
	if n == 0 {
		m.selected = 0
		return
	}
	m.selected = (i%n + n) % n
	m.readLazydev()
	m.ensureTerminal()
}

// readLazydev reads the lazydev setup of the selected project. The files are few and small, so
// this runs right in Update.
func (m *model) readLazydev() {
	if p := m.current(); p != nil {
		p.lazydev = repo.Read(p.path, projectIDs(m.projects)[m.selected])
	}
}

// ensureTerminal starts Claude in the selected project if it has no terminal yet. Before the first
// size message the terminal size is unknown, so it waits for that.
func (m *model) ensureTerminal() {
	if p := m.current(); p != nil && len(p.terminals()) == 0 && m.width > 0 {
		focus := m.focus
		m.openTerminal(startCommand)
		m.focus = focus
	}
}

// openTerminal adds a tab running args in the selected project and focuses it.
func (m *model) openTerminal(args []string) {
	m.openTerminalIn("", args)
}

// openTerminalIn is openTerminal with dir as working directory, e.g. the worktree of a run. An
// empty dir is the main folder. The tab belongs to the selected project.
func (m *model) openTerminalIn(dir string, args []string) {
	p := m.current()
	if p == nil {
		m.err = "Add a folder first: a in the left column"
		return
	}
	if dir == "" {
		dir = p.path
	}
	w, h := m.termSize()
	m.nextID++
	t, err := terminal.Start(m.nextID, w, h, dir, args, m.bus.send)
	if err != nil {
		m.err = err.Error()
		return
	}
	p.tabs = append(p.tabs, t)
	p.tab = len(p.tabs) - 1
	m.focus = colCenter
}

// openLog focuses the log tab of the run of prd in the selected project with ID id, or opens one.
// The returned command reads the log right away instead of with the next tick.
func (m *model) openLog(id, prd string) tea.Cmd {
	p := m.current()
	if p == nil {
		return nil
	}
	m.focus = colCenter
	for i, tab := range p.tabs {
		if lv, ok := tab.(*logView); ok && lv.prd == prd {
			p.tab = i
			return nil
		}
	}
	m.nextID++
	lv := newLogView(m.nextID, id, prd)
	lv.Resize(m.termSize())
	p.tabs = append(p.tabs, lv)
	p.tab = len(p.tabs) - 1
	return m.readLogs()
}

// removeTerminal drops the tab of a terminal whose program has ended.
func (m *model) removeTerminal(id int) {
	for _, p := range m.projects {
		for i, tab := range p.tabs {
			t, ok := tab.(*terminal.Terminal)
			if !ok || t.ID() != id {
				continue
			}
			t.Release()
			p.removeTab(i)
			// The wizard waits for its prompt editor and comes back when it ends.
			if w := m.wizard; w != nil && w.editor == id {
				w.editor = 0
				w.refreshPrompts()
				m.readLazydev()
			}
			return
		}
	}
}

func (m *model) selectTab(i int) {
	if p := m.current(); p != nil && len(p.tabs) > 0 {
		n := len(p.tabs)
		p.tab = (i%n + n) % n
		m.focus = colCenter
	}
}

// quit hangs up every program and exits once all have ended, so they can clean up first. The
// waiting runs as a command: the terminals keep sending output while their programs shut down,
// and that needs a free event loop. hangup kills stragglers after closeTimeout.
func (m model) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	m.menu = nil
	m.stopDocker()
	var terms []*terminal.Terminal
	for _, p := range m.projects {
		terms = append(terms, p.terminals()...)
	}
	for _, t := range terms {
		t.Hangup()
	}
	return m, func() tea.Msg {
		for _, t := range terms {
			<-t.Done()
		}
		return quitMsg{}
	}
}

// columns splits width into three columns. On narrow terminals the side columns shrink first.
func columns(width int) (side, center int) {
	side = max(sideMin, width/sideRatio)
	if width-2*side < sideMin {
		side = max(0, (width-sideMin)/2)
	}
	return side, width - 2*side
}

// rows splits height into the terminal box and the pane below it. On low terminals the pane
// below shrinks first. Zoomed, the pane below gets two thirds.
func (m model) rows(height int) (top, bottom int) {
	bottom = min(max(bottomMin, height/bottomRatio), height/2)
	if m.bottomZoom {
		bottom = max(bottom, height*2/3)
	}
	return height - bottom, bottom
}

// resizeTerminals gives every center tab the current size of the terminal area.
func (m model) resizeTerminals() {
	w, h := m.termSize()
	for _, p := range m.projects {
		for _, t := range p.tabs {
			t.Resize(w, h)
		}
	}
}

// termSize is the size of the terminal area inside the center column.
func (m model) termSize() (width, height int) {
	_, center := columns(m.width)
	top, _ := m.rows(m.height - 1)
	return max(1, center-2), max(1, top-centerChrome)
}

// termOrigin is the screen cell of the top left corner of the terminal area: below the border, the
// tab bar and the separator of the center column.
func (m model) termOrigin() (x, y int) {
	side, _ := columns(m.width)
	return side + 1, 3
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	// The last line is reserved for the key help.
	height := m.height - 1
	t := themes[m.theme]
	side, center := columns(m.width)
	centerTitle := "Center"
	if p := m.current(); p != nil {
		centerTitle = config.DisplayPath(p.path)
	}
	leftContent, activeLine := m.leftBody(side-2, height-2, t)
	left := box("Projects", leftContent, side, height, t.boxes[colLeft], m.focus == colLeft, t)
	left = markBorder(left, activeLine, t.focus)
	top, bottom := m.rows(height)
	middle := lipgloss.JoinVertical(lipgloss.Left,
		box(centerTitle, m.centerBody(center-2, t), center, top, t.boxes[colCenter], m.focus == colCenter, t),
		box(m.bottomTitle(), m.bottomBody(center-2, bottom-2, t), center, bottom, t.boxes[colCenter], m.focus == colBottom, t))
	right := box("Explorer", m.rightBody(side-2, height-2, t), side, height, t.boxes[colRight], m.focus == colRight, t)

	view := lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right), m.footer(t))
	if m.wizardVisible() {
		view = overlay(view, m.viewWizard(), m.width, m.height)
	}
	if m.status != nil {
		view = overlay(view, m.viewStatusPicker(), m.width, m.height)
	}
	if m.cockpit != nil {
		view = overlay(view, m.viewCockpit(), m.width, m.height)
	}
	if m.menu != nil {
		view = overlay(view, m.viewMenu(), m.width, m.height)
	}
	return view
}

// footer is the last line: the folder input, a question, an error or the key help.
func (m model) footer(t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	switch {
	case m.adding:
		m.input.PromptStyle = text.Bold(true)
		m.input.TextStyle = text
		m.input.Width = max(1, m.width-lipgloss.Width(m.input.Prompt)-1)
		line := m.input.View()
		if m.err != "" {
			line = ansi.Truncate(line, m.width/2, "…") + text.Foreground(t.focus).Render("  "+m.err)
		}
		return ansi.Truncate(line, m.width, "…")
	case m.question != nil:
		return text.Bold(true).Render(ansi.Truncate(" "+m.question.text, m.width, "…"))
	case m.creating:
		m.createInput.PromptStyle = text.Bold(true)
		m.createInput.TextStyle = text
		m.createInput.PlaceholderStyle = text.Faint(true)
		m.createInput.Width = max(1, m.width-lipgloss.Width(m.createInput.Prompt)-1)
		return ansi.Truncate(m.createInput.View(), m.width, "…")
	case m.confirmRemove:
		q := fmt.Sprintf(" Remove %s from the list? Its terminals close, the folder stays. (y/n)", m.current().name())
		return text.Bold(true).Render(ansi.Truncate(q, m.width, "…"))
	case m.quitting:
		return text.Render(" Closing terminals …")
	case m.err != "":
		return text.Foreground(t.focus).Render(ansi.Truncate(" "+m.err, m.width, "…"))
	case m.note != "" && m.updateNoteShown():
		return text.Foreground(t.update).Render(ansi.Truncate(" "+m.note, m.width, "…"))
	case m.note != "":
		return text.Render(ansi.Truncate(" "+m.note, m.width, "…"))
	case m.status != nil && m.menu == nil:
		return text.Render(ansi.Truncate(m.status.keys(), m.width, "…"))
	case m.cockpit != nil && m.menu == nil:
		return text.Render(ansi.Truncate(m.cockpit.keys(), m.width, "…"))
	case m.wizardVisible() && m.menu == nil:
		return text.Render(ansi.Truncate(m.wizard.keys(), m.width, "…"))
	}
	// The Claude limits stand at the right end, the key help gets the rest.
	// A newer release stays visible in front of the key help.
	limits := m.usageView(t)
	prefix := ""
	if m.latest != "" {
		prefix = " alt+u update " + m.latest + " ·"
	}
	keys := ansi.Truncate(prefix+m.footerKeys(), max(0, m.width-ansi.StringWidth(limits)-1), "…")
	if prefix != "" && strings.HasPrefix(keys, prefix) {
		return text.Foreground(t.update).Render(prefix) + text.Render(keys[len(prefix):]) + strings.Repeat(" ", max(1, m.width-ansi.StringWidth(keys)-ansi.StringWidth(limits))) + limits
	}
	return text.Render(keys) + strings.Repeat(" ", max(1, m.width-ansi.StringWidth(keys)-ansi.StringWidth(limits))) + limits
}

// leftBody lists the projects in two lines each: name and number of open tabs, then branch and
// commits ahead of and behind the upstream. The active project gets its name in the focus color
// and, while the column has the focus, a background. activeLine is the content line where the
// active project starts, -1 if it is not visible. markBorder draws its bar on the box border.
func (m model) leftBody(width, height int, t theme) (content string, activeLine int) {
	text := lipgloss.NewStyle().Foreground(t.text)
	if len(m.projects) == 0 {
		return text.Render("No folders yet.\nPress a to add one."), -1
	}
	rows := max(1, height/2)
	start := max(0, min(m.selected-rows/2, len(m.projects)-rows))
	inner := max(1, width-1)
	activeLine = -1
	var lines []string
	for i := start; i < min(start+rows, len(m.projects)); i++ {
		p := m.projects[i]
		count := ""
		if n := len(p.tabs); n > 0 {
			count = fmt.Sprint(n)
		}
		name := ansi.Truncate(p.name(), max(1, inner-2-len(count)), "…")
		first := name + strings.Repeat(" ", max(1, inner-ansi.StringWidth(name)-len(count)-1)) + count + " "
		second := ansi.Truncate("  "+p.git.summary(), inner, "…")
		second += strings.Repeat(" ", max(0, inner-ansi.StringWidth(second)))

		nameStyle, gitStyle := text.Bold(true), text.Faint(true)
		if i == m.selected {
			activeLine = len(lines)
			nameStyle = nameStyle.Foreground(t.focus)
			gitStyle = text
			if m.focus == colLeft {
				nameStyle = nameStyle.Background(t.selectedBg)
				gitStyle = gitStyle.Foreground(t.selectedFg).Background(t.selectedBg)
			}
		}
		// The leading space belongs to the styled part, so the background fills the whole row.
		lines = append(lines, nameStyle.Render(" "+first), gitStyle.Render(" "+second))
	}
	return strings.Join(lines, "\n"), activeLine
}

// markBorder replaces the left border of content lines line and line+1 of a box with a bar in
// color, so the active project stands out right on the edge of the column.
func markBorder(box string, line int, color lipgloss.TerminalColor) string {
	if line < 0 {
		return box
	}
	lines := strings.Split(box, "\n")
	bar := lipgloss.NewStyle().Foreground(color).Render("┃")
	// Line 0 of the box is the top border.
	for i := line + 1; i <= line+2 && i < len(lines)-1; i++ {
		lines[i] = bar + ansi.TruncateLeft(lines[i], 1, "")
	}
	return strings.Join(lines, "\n")
}

// tabStyle returns the style of tab i in a bar whose active tab is active: the focus color while
// the pane has the focus, the selection color otherwise.
func tabStyle(active int, focused bool, t theme) func(i int) lipgloss.Style {
	text := lipgloss.NewStyle().Foreground(t.text)
	return func(i int) lipgloss.Style {
		switch {
		case i != active:
			return text
		case focused:
			return text.Bold(true).Foreground(t.onFocus).Background(t.focus)
		}
		return text.Bold(true).Foreground(t.selectedFg).Background(t.selectedBg)
	}
}

// rightBody puts the tab bar of the right column above the content of the active tab. width and
// height are the inner size of the column.
func (m model) rightBody(width, height int, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	bar := renderTabs(rightTabs, m.rightTab, width, tabStyle(m.rightTab, m.focus == colRight, t), text)
	separator := lipgloss.NewStyle().Foreground(t.divider).Render(strings.Repeat("─", max(0, width)))
	var content string
	switch p := m.current(); {
	case p == nil:
		content = text.Faint(true).Render("No project selected.")
	case m.rightTab == rightPRDs:
		content = p.prdListView(width, max(1, height-2), m.focus == colRight, t)
	default:
		content = p.files().view(width, max(1, height-2), m.focus == colRight, t)
	}
	return bar + "\n" + separator + "\n" + content
}

// runListVisible is true while the details tab shows the run list: the right column shows the
// PRDs and the project has runs. It follows the tab, not the focus.
func (m model) runListVisible() bool {
	p := m.current()
	return p != nil && m.bottomTab == bottomDetails && m.rightTab == rightPRDs && len(p.runList()) > 0
}

// bottomTitle names what the details tab shows.
func (m model) bottomTitle() string {
	if m.runListVisible() {
		return "Runs"
	}
	return "Project"
}

// bottomBody puts the tab bar of the pane below the terminals above the content of the active tab.
// width and height are the inner size of the pane.
func (m model) bottomBody(width, height int, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	bar := renderTabs(bottomTabs, m.bottomTab, width, tabStyle(m.bottomTab, m.focus == colBottom, t), text)
	separator := lipgloss.NewStyle().Foreground(t.divider).Render(strings.Repeat("─", max(0, width)))
	var content string
	switch p := m.current(); {
	case p == nil:
		content = text.Faint(true).Render("No project selected.")
	case m.runListVisible():
		content = m.runListView(p, width, max(1, height-2), m.focus == colBottom, t)
	case m.bottomTab == bottomDetails:
		content = projectDetailsView(p.lazydev, width, max(1, height-2), t)
	default:
		content = dockerView(m.docker, p, width, max(1, height-2), m.focus == colBottom, t)
	}
	return bar + "\n" + separator + "\n" + content
}

// centerBody puts the tab bar and a separator above the active tab. width is the inner width of the
// column.
func (m model) centerBody(width int, t theme) string {
	separator := lipgloss.NewStyle().Foreground(t.divider).Render(strings.Repeat("─", max(0, width)))
	text := lipgloss.NewStyle().Foreground(t.text)
	p := m.current()
	if p == nil {
		return "\n" + separator + "\n" + text.Render("Add a folder in the left column, Claude Code then opens in it.")
	}
	if len(p.tabs) == 0 {
		return "\n" + separator + "\n" + text.Render("No terminal open. Press alt+t for a shell, or select the project again for Claude Code.")
	}
	labels := make([]string, len(p.tabs))
	for i, tab := range p.tabs {
		labels[i] = fmt.Sprintf("%d %s", i+1, tab.TabTitle())
	}
	if lv := p.activeLog(); lv != nil {
		lv.ctx = m.logContext(lv)
	}
	bar := renderTabs(labels, p.tab, width, tabStyle(p.tab, m.focus == colCenter, t), text)
	return bar + "\n" + separator + "\n" + p.tabs[p.tab].View(m.focus == colCenter && m.menu == nil && m.status == nil && m.cockpit == nil && !m.wizardVisible())
}

// box draws a box with rounded corners and the title in the top border. outline is the border
// color while the box has no focus.
func box(title, body string, width, height int, outline lipgloss.TerminalColor, focused bool, t theme) string {
	if width < 4 || height < 3 {
		return strings.Repeat(" ", max(0, width))
	}
	color, titleColor := outline, t.title
	if focused {
		color, titleColor = t.focus, t.focus
	}
	border := lipgloss.NewStyle().Foreground(color)
	label := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(ansi.Truncate(title, max(1, width-6), "…"))
	fill := max(0, width-5-lipgloss.Width(label))
	top := border.Render("╭─ ") + label + border.Render(" "+strings.Repeat("─", fill)+"╮")
	rest := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderTop(false).BorderForeground(color).
		Width(width - 2).Height(height - 2).MaxHeight(height - 1).
		Render(body)
	return top + "\n" + rest
}

// Run starts the TUI.
func Run() error {
	bus := &sender{}
	p := tea.NewProgram(newModel(bus), tea.WithAltScreen(), tea.WithMouseCellMotion())
	bus.p = p
	_, err := p.Run()
	return err
}
