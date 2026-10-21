package tui

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"lazydev/internal/claude"
	"lazydev/internal/project"
	"lazydev/internal/state"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A log tab shows the complete log of a run in the center column: all steps with a separator each,
// complete texts with line wrap, and the run with its steps in a header. It runs no process. Every
// second it reads what the runner appended to the step logs since the last read.

// logStampWidth is the width of the time column, "15:04:05 ". Continuation lines indent to it.
const logStampWidth = 9

// logTabs replaces characters a terminal would not draw in one cell.
var logTabs = strings.NewReplacer("\t", "    ", "\r", "")

// logView is a center tab with the log of the run of prd in project projectID.
type logView struct {
	id             int
	projectID, prd string
	width, height  int
	// entries are the described lines of each step log, offsets how far each file is read.
	entries map[string][]claude.Entry
	offsets map[string]int64
	// top is the first visible line of the wrapped body. While follow is true, the view sticks to
	// the end.
	top    int
	follow bool
	// header is true while the header shows the run in full.
	header bool
	// ctx is what the header needs from the model, set before each render.
	ctx logContext
	// lines caches the wrapped body for cacheWidth and cacheTheme until new entries arrive.
	lines      []string
	dirty      bool
	cacheWidth int
	cacheTheme int
}

// logContext is the part of the model a log tab shows in its header.
type logContext struct {
	run   *state.Run
	queue int
	dev   bool
	theme int
}

func newLogView(id int, projectID, prd string) *logView {
	return &logView{
		id: id, projectID: projectID, prd: prd,
		entries: map[string][]claude.Entry{}, offsets: map[string]int64{},
		follow: true, header: true, dirty: true,
	}
}

func (lv *logView) ID() int          { return lv.id }
func (lv *logView) TabTitle() string { return "log " + lv.prd }

func (lv *logView) Resize(width, height int) {
	lv.width, lv.height = width, height
}

// logContext collects what the header of lv shows. Log tabs only render in the selected project.
func (m model) logContext(lv *logView) logContext {
	ctx := logContext{queue: m.queuePos(lv.projectID, lv.prd), dev: m.devPIDs[lv.prd] != 0, theme: m.theme}
	if p := m.current(); p != nil {
		ctx.run = p.runs[lv.prd]
	}
	return ctx
}

// logChunk is what one read of a step log found: the entries of the complete lines from offset
// from to offset to. reset is true if the file got shorter and was read from the start.
type logChunk struct {
	from, to int64
	reset    bool
	entries  []claude.Entry
}

// logReadMsg carries the new entries of the log tabs by tab ID and step.
type logReadMsg struct {
	byTab map[int]map[string]logChunk
}

// readLogs reads in the background what is new in the step logs of all open log tabs.
func (m model) readLogs() tea.Cmd {
	type job struct {
		id             int
		projectID, prd string
		offsets        map[string]int64
	}
	var jobs []job
	for _, p := range m.projects {
		for _, tab := range p.tabs {
			if lv, ok := tab.(*logView); ok {
				jobs = append(jobs, job{lv.id, lv.projectID, lv.prd, maps.Clone(lv.offsets)})
			}
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	return func() tea.Msg {
		msg := logReadMsg{byTab: map[int]map[string]logChunk{}}
		for _, j := range jobs {
			msg.byTab[j.id] = readSteps(j.projectID, j.prd, j.offsets)
		}
		return msg
	}
}

// applyLogs hands the read entries to their log tabs.
func (m model) applyLogs(msg logReadMsg) {
	for _, p := range m.projects {
		for _, tab := range p.tabs {
			if lv, ok := tab.(*logView); ok && msg.byTab[lv.id] != nil {
				lv.apply(msg.byTab[lv.id])
			}
		}
	}
}

// readSteps reads every step log of a run from its offset on.
func readSteps(id, prd string, offsets map[string]int64) map[string]logChunk {
	chunks := map[string]logChunk{}
	for _, step := range state.Steps {
		if c, ok := readStep(state.LogPath(id, prd, step), offsets[step]); ok {
			chunks[step] = c
		}
	}
	return chunks
}

// readStep reads the complete lines of a log from offset on. The last line may still be written,
// so everything after the last newline waits for the next read. ok is false if nothing changed.
func readStep(path string, offset int64) (c logChunk, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return c, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return c, false
	}
	c = logChunk{from: offset, to: offset}
	start := offset
	if info.Size() < offset {
		// A discarded and restarted run writes the file anew.
		c.reset, c.to, start = true, 0, 0
	}
	if info.Size() == start {
		return c, c.reset
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return c, c.reset
	}
	data, _ := io.ReadAll(io.LimitReader(f, info.Size()-start))
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return c, c.reset
	}
	for line := range bytes.SplitSeq(data[:end], []byte("\n")) {
		c.entries = append(c.entries, claude.Describe(line)...)
	}
	c.to = start + int64(end) + 1
	return c, true
}

// apply adds read entries. A chunk read from another offset than the current one is stale, e.g.
// from an overlapping read, and gets dropped, so every entry arrives exactly once.
func (lv *logView) apply(chunks map[string]logChunk) {
	for step, c := range chunks {
		if lv.offsets[step] != c.from {
			continue
		}
		if c.reset {
			lv.entries[step] = nil
		}
		lv.entries[step] = append(lv.entries[step], c.entries...)
		lv.offsets[step] = c.to
		lv.dirty = true
	}
}

func (lv *logView) theme() theme {
	if lv.ctx.theme < 0 || lv.ctx.theme >= len(themes) {
		return themes[0]
	}
	return themes[lv.ctx.theme]
}

// body is the log as wrapped screen lines: per step a separator, then each entry as time and
// text. Continuation lines indent to the text column.
func (lv *logView) body() []string {
	if !lv.dirty && lv.cacheWidth == lv.width && lv.cacheTheme == lv.ctx.theme {
		return lv.lines
	}
	t := lv.theme()
	text := lipgloss.NewStyle().Foreground(t.text)
	muted := text.Faint(true)
	var lines []string
	for _, step := range state.Steps {
		entries := lv.entries[step]
		if len(entries) == 0 {
			continue
		}
		lines = append(lines, muted.Render("── "+step+" ──"))
		for _, e := range entries {
			style := text
			switch e.Kind {
			case claude.KindRunner:
				style = muted
			case claude.KindError:
				style = text.Foreground(t.focus)
			case claude.KindResult:
				style = text.Foreground(colorOK)
			}
			indent := ""
			if e.Sub {
				indent = "  "
			}
			stamp := strings.Repeat(" ", logStampWidth)
			if !e.At.IsZero() {
				stamp = e.At.Local().Format("15:04:05") + " "
			}
			width := max(1, lv.width-logStampWidth-len(indent))
			for part := range strings.SplitSeq(logTabs.Replace(e.Text), "\n") {
				for wrapped := range strings.SplitSeq(ansi.Wrap(part, width, ""), "\n") {
					lines = append(lines, muted.Render(stamp)+style.Render(indent+wrapped))
					stamp = strings.Repeat(" ", logStampWidth)
				}
			}
		}
	}
	lv.lines, lv.dirty, lv.cacheWidth, lv.cacheTheme = lines, false, lv.width, lv.ctx.theme
	return lines
}

// headerLines shows the run: unfolded phase, branch, port, cost and dev, then the steps, then an
// error or note, and a separator. Folded, one line with phase, current step and cost.
func (lv *logView) headerLines(t theme) []string {
	text := lipgloss.NewStyle().Foreground(t.text)
	muted := text.Faint(true)
	warn := text.Foreground(t.focus)
	dot := muted.Render(" · ")
	r := lv.ctx.run
	if r == nil {
		return []string{muted.Render(" No run.")}
	}
	ph := r.Effective()
	cost := text.Render(fmt.Sprintf("%.2f USD", r.Cost()))
	if !lv.header {
		return []string{" " + phaseText(ph, 0, t) + dot + text.Render(currentStep(r)) + dot + cost}
	}
	field := func(label, value string) string { return muted.Render(label+" ") + value }
	dev := muted.Render("–")
	if lv.ctx.dev {
		dev = text.Foreground(colorOK).Render("●") + text.Render(fmt.Sprintf(" :%d", r.Port))
	}
	lines := []string{
		" " + phaseText(ph, lv.ctx.queue, t) + dot +
			field("branch", text.Render(r.Branch+" → "+r.Base)) + dot +
			field("port", text.Render(fmt.Sprint(r.Port))) + dot +
			field("cost", cost) + dot +
			field("dev", dev),
		" " + strings.Join(stepMarks(r, t), dot),
	}
	switch {
	case ph == project.PhaseError && r.Error != nil:
		lines = append(lines, " "+warn.Render(claude.FirstLine(*r.Error)))
	case r.Note != "":
		lines = append(lines, " "+warn.Render(r.Note))
	}
	return append(lines, lipgloss.NewStyle().Foreground(t.divider).Render(strings.Repeat("─", max(0, lv.width))))
}

// bodyHeight is the number of body lines below the header.
func (lv *logView) bodyHeight() int {
	return max(0, lv.height-len(lv.headerLines(lv.theme())))
}

// maxTop is the top line that shows the end of the body.
func (lv *logView) maxTop() int {
	return max(0, len(lv.body())-lv.bodyHeight())
}

// scroll moves the view by delta screen lines. Reaching the end follows again.
func (lv *logView) scroll(delta int) {
	end := lv.maxTop()
	if lv.follow {
		lv.top = end
	}
	lv.top = max(0, min(lv.top+delta, end))
	lv.follow = lv.top >= end
}

// update handles the keys of the log tab. It returns false for keys it does not use.
func (lv *logView) update(key string) bool {
	half := max(1, lv.bodyHeight()/2)
	switch key {
	case "j", "down":
		lv.scroll(1)
	case "k", "up":
		lv.scroll(-1)
	case "ctrl+d", "pgdown":
		lv.scroll(half)
	case "ctrl+u", "pgup":
		lv.scroll(-half)
	case "g", "home":
		lv.scroll(-len(lv.body()) - 1)
	case "G", "end":
		lv.follow = true
	case "z":
		lv.header = !lv.header
	default:
		return false
	}
	return true
}

// View fills exactly the size from Resize: the header, then the visible part of the body.
func (lv *logView) View(bool) string {
	t := lv.theme()
	head := lv.headerLines(t)
	head = head[:min(len(head), lv.height)]
	body := lv.body()
	rows := max(0, lv.height-len(head))
	end := max(0, len(body)-rows)
	if lv.follow {
		lv.top = end
	}
	lv.top = max(0, min(lv.top, end))
	lines := head
	for i := range rows {
		line := ""
		if lv.top+i < len(body) {
			line = body[lv.top+i]
		}
		lines = append(lines, line)
	}
	for i, l := range lines {
		l = ansi.Truncate(l, lv.width, "")
		lines[i] = l + strings.Repeat(" ", max(0, lv.width-ansi.StringWidth(l)))
	}
	return strings.Join(lines, "\n")
}

// closeEndedLogs closes the log tabs whose run is gone or merged.
func (p *projectEntry) closeEndedLogs() {
	for i := len(p.tabs) - 1; i >= 0; i-- {
		lv, ok := p.tabs[i].(*logView)
		if !ok {
			continue
		}
		if r := p.runs[lv.prd]; r == nil || r.Effective() == project.PhaseMerged {
			p.removeTab(i)
		}
	}
}
