package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
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
)

// The TUI controls no runner directly. It reads the state files of all runs every second,
// because an ending runner causes no event, and the log of the run the details tab shows. Starting,
// stopping and discarding run as commands in the background.

const (
	// runInterval is how often runs and the shown log refresh.
	runInterval = time.Second
	// logTailBytes is how much of the end of a step log the details tab reads.
	logTailBytes = 256 << 10
	// logEntries is how many described lines of a log the details tab keeps.
	logEntries = 500
)

// runsMsg carries the runs of all projects by project ID and PRD name, the waiting queue in start
// order as "<id>/<prd>", and the log of the run the details tab shows.
type runsMsg struct {
	byID    map[string]map[string]*state.Run
	waiting []string
	logKey  string
	log     []claude.Entry
	// devPID is the dev server of the run the details tab shows, 0 without one.
	devPID   int
	usage    usage.Snapshot
	hasUsage bool
}

type runTickMsg struct{}

func runTick() tea.Cmd {
	return tea.Tick(runInterval, func(time.Time) tea.Msg { return runTickMsg{} })
}

// fetchRuns reads all runs and the log of the run the details tab shows.
func (m model) fetchRuns() tea.Cmd {
	var id, name string
	if d := m.detailsPRD(); d != nil {
		id, name = projectIDs(m.projects)[m.selected], d.Name
	}
	return func() tea.Msg {
		msg := runsMsg{byID: map[string]map[string]*state.Run{}}
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
		if r := msg.byID[id][name]; r != nil && r.LastStep() != nil {
			msg.logKey = id + "/" + name
			msg.log = readLog(state.LogPath(id, name, r.LastStep().Name))
			msg.devPID = devserver.PID(id, name)
		}
		return msg
	}
}

// readLog describes the end of a step log.
func readLog(path string) []claude.Entry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	offset := int64(0)
	if info, err := f.Stat(); err == nil && info.Size() > logTailBytes {
		offset = info.Size() - logTailBytes
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
	return out[max(0, len(out)-logEntries):]
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

// startPRD starts or continues the selected PRD. Untrusted hooks or allowed tools need a yes first.
func (m *model) startPRD() tea.Cmd {
	p, d := m.current(), m.current().selectedPRD()
	if d == nil {
		return nil
	}
	r := p.runs[d.Name]
	switch {
	case r != nil && (r.Running() || r.Phase == project.PhaseWaiting):
		m.err = d.Label() + " runs already."
		return nil
	case r == nil && d.Phase().Done():
		m.err = d.Label() + " is " + string(d.Phase()) + "."
		return nil
	case r != nil && r.NextStep() == "":
		m.err = "Nothing left to do for " + d.Label() + "."
		return nil
	}
	t, name := m.selectedTarget(), d.Name
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
		m.question = &question{text: fmt.Sprintf("%s %s? (y/n)", verb, d.Label()), yes: start}
		return nil
	}
	m.question = &question{
		text: fmt.Sprintf("%s/config.yaml sets %s. Trust it and %s %s? (y/n)", repo.Dir, trustSummary(info.Config), strings.ToLower(verb), d.Label()),
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

// stopPRD stops the runner of the selected PRD, a waiting PRD becomes aborted.
func (m *model) stopPRD() tea.Cmd {
	p, d := m.current(), m.current().selectedPRD()
	if d == nil || p.runs[d.Name] == nil {
		return nil
	}
	id, name := m.selectedTarget().ID, d.Name
	return func() tea.Msg {
		if err := runner.Stop(id, name); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: "Stopping " + name + "."}
	}
}

// discardPRD asks to throw away the run of the selected PRD with worktree and branch.
func (m *model) discardPRD() {
	p, d := m.current(), m.current().selectedPRD()
	if d == nil || p.runs[d.Name] == nil {
		return
	}
	t, name := m.selectedTarget(), d.Name
	m.question = &question{
		text: fmt.Sprintf("Discard the run of %s with worktree, branch and its commits? (y/n)", d.Label()),
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

// toggleDev starts the dev server in the worktree of the selected PRD, or stops it.
func (m *model) toggleDev() tea.Cmd {
	p, d := m.current(), m.current().selectedPRD()
	if d == nil {
		return nil
	}
	if p.runs[d.Name] == nil {
		m.err = d.Label() + " has no worktree yet. Start it with s."
		return nil
	}
	t, name := m.selectedTarget(), d.Name
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

// runDetailsView renders a PRD with a run: the run and its steps on the left, the live log of the
// current step on the right.
func (m model) runDetailsView(d *project.PRD, r *state.Run, width, height int, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	muted := text.Faint(true)
	warn := text.Foreground(t.focus)
	ok := text.Foreground(colorOK)
	field := func(label string, value string) string {
		return muted.Render(pad(label, 7)) + value
	}

	ph := r.Effective()
	symbol := text
	if c := phaseColor(ph, t); c != nil {
		symbol = symbol.Foreground(c)
	}
	phaseText := symbol.Render(ph.Symbol()) + text.Render(" "+string(ph))
	if ph == project.PhaseWaiting {
		for i, key := range m.waiting {
			if key == projectIDs(m.projects)[m.selected]+"/"+d.Name {
				phaseText += muted.Render(fmt.Sprintf(" · #%d in queue", i+1))
			}
		}
	}
	info := column{title: d.Label(), max: 34, rows: []string{
		phaseText,
		field("branch", text.Render(r.Branch+" → "+r.Base)),
		field("port", text.Render(fmt.Sprint(r.Port))),
		field("cost", text.Render(fmt.Sprintf("%.2f USD", r.Cost()))),
	}}
	if m.devPID != 0 && m.logKey == projectIDs(m.projects)[m.selected]+"/"+d.Name {
		info.rows = append(info.rows, field("dev", ok.Render("●")+text.Render(fmt.Sprintf(" :%d", r.Port))))
	}
	if r.Error != nil && ph == project.PhaseError {
		info.rows = append(info.rows, warn.Render(claude.FirstLine(*r.Error)))
	}
	if r.Note != "" {
		info.rows = append(info.rows, warn.Render(r.Note))
	}

	steps := column{title: "Steps", max: 30}
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
			mark, style = "✗", warn
			extra = duration(s.StartedAt, s.EndedAt)
		default:
			mark, style = "✓", ok
			extra = duration(s.StartedAt, s.EndedAt)
		}
		if s != nil && s.CostUSD != nil {
			extra += fmt.Sprintf(" %.2f $", *s.CostUSD)
		}
		steps.rows = append(steps.rows, style.Render(mark+" ")+text.Render(pad(name, 10))+muted.Render(extra))
	}

	logTitle := "Log"
	if s := r.LastStep(); s != nil {
		logTitle += " · " + s.Name
	}
	logCol := column{title: logTitle, max: width}
	entries := m.log
	if m.logKey != projectIDs(m.projects)[m.selected]+"/"+d.Name {
		entries = nil
	}
	for _, e := range entries[max(0, len(entries)-(height-1)):] {
		style := text
		switch e.Kind {
		case claude.KindRunner:
			style = muted
		case claude.KindError:
			style = warn
		case claude.KindResult:
			style = ok
		}
		indent := ""
		if e.Sub {
			indent = "  "
		}
		stamp := "        "
		if !e.At.IsZero() {
			stamp = e.At.Local().Format("15:04:05")
		}
		logCol.rows = append(logCol.rows, muted.Render(stamp+" ")+style.Render(indent+e.Text))
	}
	if len(logCol.rows) == 0 {
		logCol.rows = []string{muted.Render("No log yet.")}
	}
	return renderColumns([]column{info, steps, logCol}, width, height, t)
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
