package tui

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"lazydev/internal/claude"
	"lazydev/internal/devserver"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/runner"
	"lazydev/internal/state"
	"lazydev/internal/usage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The TUI controls no runner directly. It reads the state files of all runs every second,
// because an ending runner causes no event, the last log entry of each run of the selected project
// and what the open log tabs have not read yet. Starting, stopping and discarding run as commands
// in the background.

const (
	// runInterval is how often runs and logs refresh.
	runInterval = time.Second
	// lastTailBytes is how much of the end of a step log the run list reads for the last entry.
	lastTailBytes = 16 << 10
)

// runsMsg carries the runs of all projects by project ID and PRD name, the waiting queue in start
// order as "<id>/<prd>", and the last log entry and dev server of each run of the selected project.
type runsMsg struct {
	byID    map[string]map[string]*state.Run
	waiting []string
	// last and devPIDs map PRD names of the selected project to the last log entry of the current
	// step and the PID of the dev server.
	last     map[string]claude.Entry
	devPIDs  map[string]int
	usage    usage.Snapshot
	hasUsage bool
}

type runTickMsg struct{}

func runTick() tea.Cmd {
	return tea.Tick(runInterval, func(time.Time) tea.Msg { return runTickMsg{} })
}

// fetchRuns reads all runs and, for the runs of the selected project, the last log entry and the
// dev server.
func (m model) fetchRuns() tea.Cmd {
	id := ""
	if m.current() != nil {
		id = projectIDs(m.projects)[m.selected]
	}
	return func() tea.Msg {
		msg := runsMsg{byID: map[string]map[string]*state.Run{}, last: map[string]claude.Entry{}, devPIDs: map[string]int{}}
		msg.usage, msg.hasUsage, _ = usage.Load()
		for _, e := range state.All() {
			if msg.byID[e.ID] == nil {
				msg.byID[e.ID] = map[string]*state.Run{}
			}
			msg.byID[e.ID][e.PRDName] = e.Run
			if e.Run.Phase == project.PhaseWaiting && !e.Run.Running() {
				msg.waiting = append(msg.waiting, e.ID+"/"+e.PRDName)
			}
		}
		for name, r := range msg.byID[id] {
			if r.Effective() == project.PhaseMerged {
				continue
			}
			if s := r.LastStep(); s != nil {
				if entries := readLog(state.LogPath(id, name, s.Name), lastTailBytes); len(entries) > 0 {
					msg.last[name] = entries[len(entries)-1]
				}
			}
			msg.devPIDs[name] = devserver.PID(id, name)
		}
		return msg
	}
}

// readLog describes the last tail bytes of a step log.
func readLog(path string, tail int64) []claude.Entry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	offset := int64(0)
	if info, err := f.Stat(); err == nil && info.Size() > tail {
		offset = info.Size() - tail
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil
	}
	data, _ := io.ReadAll(f)
	lines := bytes.Split(data, []byte("\n"))
	if offset > 0 && len(lines) > 0 {
		// The first line starts somewhere in the middle.
		lines = lines[1:]
	}
	var out []claude.Entry
	for _, l := range lines {
		out = append(out, claude.Describe(l)...)
	}
	return out
}

// runList names the PRDs of the run list: all runs of the project that are not merged, by start
// and then by name.
func (p *projectEntry) runList() []string {
	var names []string
	for name, r := range p.runs {
		if r.Effective() != project.PhaseMerged {
			names = append(names, name)
		}
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(strings.Compare(p.runs[a].StartedAt, p.runs[b].StartedAt), strings.Compare(a, b))
	})
	return names
}

// selectedRun is the PRD name under the cursor of the run list, empty without runs.
func (p *projectEntry) selectedRun() string {
	names := p.runList()
	if len(names) == 0 {
		return ""
	}
	return names[clamp(p.runCursor, len(names))]
}

// prdNamed is the PRD called name, nil if the list does not have it.
func (p *projectEntry) prdNamed(name string) *project.PRD {
	for i := range p.prds {
		if p.prds[i].Name == name {
			return &p.prds[i]
		}
	}
	return nil
}

// labelOf is the label of the PRD called name, the name itself if the list does not have it.
func (p *projectEntry) labelOf(name string) string {
	if d := p.prdNamed(name); d != nil {
		return d.Label()
	}
	return name
}

// question is a yes/no question in the footer. yes runs on y.
type question struct {
	text string
	yes  func(m *model) tea.Cmd
}

func (m model) updateQuestion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	q := m.question
	m.question = nil
	if s := msg.String(); s == "y" || s == "j" {
		return m, q.yes(&m)
	}
	return m, nil
}

// selectedTarget is the selected project as runner target.
func (m model) selectedTarget() runner.Target {
	return runner.Target{ID: projectIDs(m.projects)[m.selected], Dir: m.current().path}
}

// startPRD starts or continues the PRD called name. Untrusted hooks or allowed tools need a yes
// first.
func (m *model) startPRD(name string) tea.Cmd {
	p := m.current()
	d, r := p.prdNamed(name), p.runs[name]
	if name == "" || (d == nil && r == nil) {
		return nil
	}
	label := p.labelOf(name)
	switch {
	case r != nil && (r.Running() || r.Phase == project.PhaseWaiting):
		m.err = label + " runs already."
		return nil
	case r == nil && d.Phase().Done():
		m.err = label + " is " + string(d.Phase()) + "."
		return nil
	case r != nil && r.NextStep() == "":
		m.err = "Nothing left to do for " + label + "."
		return nil
	case p.blockedInWorktree(name):
		m.err = label + " is blocked. Resolve it with f first."
		return nil
	}
	t := m.selectedTarget()
	start := func(*model) tea.Cmd {
		return func() tea.Msg {
			started, err := runner.Start(t, name)
			switch {
			case err != nil:
				return actionMsg{err: err}
			case !started:
				return actionMsg{text: fmt.Sprintf("%s waits, %d PRDs run already.", name, runner.MaxParallel)}
			}
			return actionMsg{text: "Started " + name + "."}
		}
	}
	verb := "Start"
	if r != nil {
		verb = "Continue"
	}
	info := repo.Read(t.Dir, t.ID)
	if info.Trust != repo.TrustPending {
		m.question = &question{text: fmt.Sprintf("%s %s? (y/n)", verb, label), yes: start}
		return nil
	}
	m.question = &question{
		text: fmt.Sprintf("%s/config.yaml sets %s. Trust it and %s %s? (y/n)", repo.Dir, trustSummary(info.Config), strings.ToLower(verb), label),
		yes: func(m *model) tea.Cmd {
			if err := state.Trust(t.ID, info.Config.TrustHash()); err != nil {
				m.err = err.Error()
				return nil
			}
			m.readLazydev()
			return start(m)
		},
	}
	return nil
}

// trustSummary names what config.yaml lets run: the hook scripts and the number of allowed tools.
func trustSummary(r repo.Config) string {
	var parts []string
	for _, h := range []struct{ name, script string }{{"setup", r.Hooks.Setup}, {"teardown", r.Hooks.Teardown}, {"check", r.Hooks.Check}, {"dev", r.Hooks.Dev}} {
		if h.script != "" {
			parts = append(parts, h.name+" "+h.script)
		}
	}
	if n := len(r.AllowedTools); n > 0 {
		parts = append(parts, fmt.Sprintf("%d allowed tools", n))
	}
	return strings.Join(parts, ", ")
}

// stopPRD stops the runner of the PRD called name, a waiting PRD becomes aborted.
func (m *model) stopPRD(name string) tea.Cmd {
	if name == "" || m.current().runs[name] == nil {
		return nil
	}
	id := m.selectedTarget().ID
	return func() tea.Msg {
		if err := runner.Stop(id, name); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: "Stopping " + name + "."}
	}
}

// discardPRD asks to throw away the run of the PRD called name with worktree and branch.
func (m *model) discardPRD(name string) {
	p := m.current()
	if name == "" || p.runs[name] == nil {
		return
	}
	t := m.selectedTarget()
	m.question = &question{
		text: fmt.Sprintf("Discard the run of %s with worktree, branch and its commits? (y/n)", p.labelOf(name)),
		yes: func(*model) tea.Cmd {
			return func() tea.Msg {
				if err := runner.Discard(t, name); err != nil {
					return actionMsg{err: err}
				}
				return actionMsg{text: "Discarded the run of " + name + "."}
			}
		},
	}
}

// toggleDev starts the dev server in the worktree of the PRD called name, or stops it.
func (m *model) toggleDev(name string) tea.Cmd {
	p := m.current()
	if name == "" {
		return nil
	}
	if p.runs[name] == nil {
		m.err = p.labelOf(name) + " has no worktree yet. Start it with s."
		return nil
	}
	t := m.selectedTarget()
	return func() tea.Msg {
		if devserver.PID(t.ID, name) != 0 {
			if err := devserver.Stop(t.ID, name); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "Stopped the dev server of " + name + "."}
		}
		pid, err := runner.StartDev(t, name)
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: fmt.Sprintf("Started the dev server of %s (PID %d), log: %s", name, pid, state.DevLogPath(t.ID, name))}
	}
}

// queuePos is the 1-based position of the PRD called name of project id in the waiting queue, 0
// if it does not wait.
func (m model) queuePos(id, name string) int {
	return slices.Index(m.waiting, id+"/"+name) + 1
}

// phaseText is the phase symbol in its color and the phase, with the queue position while waiting.
func phaseText(ph project.Phase, queue int, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	symbol := text
	if c := phaseColor(ph, t); c != nil {
		symbol = symbol.Foreground(c)
	}
	s := symbol.Render(ph.Symbol()) + text.Render(" "+string(ph))
	if ph == project.PhaseWaiting && queue > 0 {
		s += text.Faint(true).Render(fmt.Sprintf(" · #%d in queue", queue))
	}
	return s
}

// stepEnd is when step s of r ended, now while it runs.
func stepEnd(r *state.Run, s *state.Step) string {
	if s.EndedAt == "" && r.Running() {
		return state.Now()
	}
	return s.EndedAt
}

// currentStep is the last step of r with its duration, "–" without one.
func currentStep(r *state.Run) string {
	s := r.LastStep()
	if s == nil {
		return "–"
	}
	return strings.TrimSpace(s.Name + " " + duration(s.StartedAt, stepEnd(r, s)))
}

// stepMarks renders every step of a run with its mark, duration and cost: ✓ done, ● running,
// ✗ failed, ○ not started.
func stepMarks(r *state.Run, t theme) []string {
	text := lipgloss.NewStyle().Foreground(t.text)
	muted := text.Faint(true)
	var marks []string
	for _, name := range state.Steps {
		var s *state.Step
		for i := range r.Steps {
			if r.Steps[i].Name == name {
				s = &r.Steps[i]
			}
		}
		mark, style, extra := "○", muted, ""
		switch {
		case s == nil:
		case s.ExitCode == nil && r.Running():
			mark, style = "●", text.Foreground(t.title)
			extra = duration(s.StartedAt, state.Now())
		case s.ExitCode == nil || !s.OK:
			mark, style = "✗", text.Foreground(t.focus)
			extra = duration(s.StartedAt, s.EndedAt)
		default:
			mark, style = "✓", text.Foreground(colorOK)
			extra = duration(s.StartedAt, s.EndedAt)
		}
		if s != nil && s.CostUSD != nil {
			extra += fmt.Sprintf(" %.2f $", *s.CostUSD)
		}
		if extra != "" {
			extra = " " + strings.TrimSpace(extra)
		}
		marks = append(marks, style.Render(mark+" ")+text.Render(name)+muted.Render(extra))
	}
	return marks
}

// runCell is a cell of the run list with its own style outside the cursor row.
type runCell struct {
	text  string
	style lipgloss.Style
}

// runListView renders the runs of p as a table of height rows: PRD, phase, step, cost, port, dev,
// branch and the last log line. Columns that do not fit into width drop from the right, PRD and
// phase stay. While focused, the cursor row gets a background.
func (m model) runListView(p *projectEntry, width, height int, focused bool, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	names := p.runList()
	p.runCursor = clamp(p.runCursor, len(names))
	id := projectIDs(m.projects)[m.selected]

	// Plain phase texts measure the phase column, the styled ones go into the rows.
	nameW, phaseW, branchW := 3, 5, 6
	phases := make([]string, len(names))
	for i, name := range names {
		r := p.runs[name]
		phases[i] = ansi.Strip(phaseText(r.Effective(), m.queuePos(id, name), t))
		nameW = max(nameW, ansi.StringWidth(p.labelOf(name)))
		phaseW = max(phaseW, ansi.StringWidth(phases[i]))
		branchW = max(branchW, ansi.StringWidth(r.Branch))
	}
	nameW, phaseW, branchW = min(nameW, 28), min(phaseW, 24), min(branchW, 32)

	// The optional columns in the order they drop from the right, the log line takes the rest.
	titles := []string{"Step", "Cost", "Port", "Dev", "Branch"}
	widths := []int{16, 9, 5, 7, branchW}
	const logMin = 10
	// A row is " ", the symbol and per cell a space and the cell.
	used := 2 + 1 + nameW + 1 + phaseW
	visible := 0
	for _, w := range widths {
		if used+1+w > width {
			break
		}
		used += 1 + w
		visible++
	}
	logW := 0
	if visible == len(widths) && width-used-1 >= logMin {
		logW = width - used - 1
	}

	// row joins the cells after the symbol column, each in its style, or all in base on the
	// cursor row.
	row := func(cells []runCell, base lipgloss.Style, plain bool) string {
		ws := append([]int{nameW, phaseW}, widths[:visible]...)
		if logW > 0 {
			ws = append(ws, logW)
		}
		var b strings.Builder
		for i, c := range cells[:len(ws)] {
			style := c.style
			if plain {
				style = base
			}
			b.WriteString(base.Render(" ") + style.Render(pad(c.text, ws[i])))
		}
		return b.String()
	}
	muted := text.Faint(true)
	header := []runCell{{"PRD", muted}, {"Phase", muted}}
	for _, title := range titles {
		header = append(header, runCell{title, muted})
	}
	header = append(header, runCell{"Log", muted})
	lines := []string{ansi.Truncate(muted.Render("  ")+row(header, muted, false), width, "")}

	// Keep the cursor visible below the header.
	rows := max(1, height-1)
	top := max(0, min(p.runCursor-rows+1, len(names)-rows))
	for i := top; i < min(top+rows, len(names)); i++ {
		name := names[i]
		r := p.runs[name]
		ph := r.Effective()
		base := text
		selected := i == p.runCursor && focused
		if selected {
			base = base.Foreground(t.selectedFg).Background(t.selectedBg)
		}
		symbol := base
		if c := phaseColor(ph, t); c != nil && !selected {
			symbol = symbol.Foreground(c)
		}
		cost, port, dev := fmt.Sprintf("%.2f USD", r.Cost()), "–", runCell{"–", muted}
		if r.Port != 0 {
			port = fmt.Sprint(r.Port)
		}
		if m.devPIDs[name] != 0 {
			dev = runCell{fmt.Sprintf("● :%d", r.Port), text.Foreground(colorOK)}
		}
		last := runCell{claude.FirstLine(m.last[name].Text), muted}
		switch {
		case (ph == project.PhaseError || ph == project.PhaseBlocked) && r.Error != nil:
			last = runCell{claude.FirstLine(*r.Error), text.Foreground(t.focus)}
		case r.Note != "":
			last = runCell{r.Note, text.Foreground(t.focus)}
		}
		phaseCell := runCell{phases[i], text}
		if ph == project.PhaseWaiting {
			phaseCell.style = muted
		}
		cells := []runCell{
			{p.labelOf(name), text.Bold(true)}, phaseCell,
			{currentStep(r), text}, {cost, text}, {port, text}, dev, {r.Branch, text}, last,
		}
		line := base.Render(" ") + symbol.Render(ph.Symbol()) + row(cells, base, selected)
		line += base.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(line))))
		lines = append(lines, ansi.Truncate(line, width, ""))
	}
	return strings.Join(lines, "\n")
}

// usageWindows names the limit windows in the footer.
var usageWindows = map[string]string{"five_hour": "5h", "seven_day": "7d"}

// usageView shows the Claude limits like "5h 11% · 7d 37%". A window past its reset counts as
// empty. 80% and more get the focus color.
func (m model) usageView(t theme) string {
	if !m.hasUsage {
		return ""
	}
	text := lipgloss.NewStyle().Foreground(t.text)
	var parts []string
	for _, w := range m.usage.Windows {
		name, ok := usageWindows[w.Name]
		if !ok {
			continue
		}
		used := w.Utilization
		if time.Now().After(w.ResetsAt) {
			used = 0
		}
		style := text.Faint(true)
		if used >= 0.8 {
			style = text.Foreground(t.focus)
		}
		parts = append(parts, style.Render(fmt.Sprintf("%s %.0f%%", name, used*100)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, text.Faint(true).Render(" · ")) + " "
}

// duration formats the time between two RFC 3339 stamps as "1:02" or "1:02:03".
func duration(from, to string) string {
	a, err1 := time.Parse(time.RFC3339, from)
	b, err2 := time.Parse(time.RFC3339, to)
	if err1 != nil || err2 != nil {
		return ""
	}
	d := b.Sub(a).Round(time.Second)
	h, mnt, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, mnt, s)
	}
	return fmt.Sprintf("%d:%02d", mnt, s)
}
