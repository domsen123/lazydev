package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// The right column lists the PRDs of the selected project with their phase. Done PRDs fold into
// one group row.

// colorOK marks running containers, present files and merged PRDs.
var colorOK = lipgloss.Color("#5faf5f")

// colorBlocked marks blocked runs, they wait for a decision of the user.
var colorBlocked = lipgloss.Color("#d7af00")

// phaseColor is the color of a phase symbol, nil for the text color.
func phaseColor(p project.Phase, t theme) lipgloss.TerminalColor {
	switch p {
	case project.PhaseMerged:
		return colorOK
	case project.PhaseError, project.PhaseAborted:
		return t.focus
	case project.PhaseBlocked:
		return colorBlocked
	case project.PhaseSetup, project.PhaseImplement, project.PhaseReview, project.PhaseMerge, project.PhaseMergeQueue:
		return t.title
	}
	return nil
}

// phaseOf is the phase of d: the one of its run, otherwise the one of its status line.
func (p *projectEntry) phaseOf(d *project.PRD) project.Phase {
	if r := p.runs[d.Name]; r != nil {
		return r.Effective()
	}
	return d.Phase()
}

// prdsMsg carries the PRDs of the project at path.
type prdsMsg struct {
	path string
	prds []project.PRD
	err  error
}

// fetchPRDs reads the PRDs of all projects in the background, one message per project. PRDs that
// a run moved into its worktree come from there.
func fetchPRDs(projects []*projectEntry) tea.Cmd {
	ids := projectIDs(projects)
	cmds := make([]tea.Cmd, len(projects))
	for i, p := range projects {
		path, id := p.path, ids[i]
		cmds[i] = func() tea.Msg {
			var moved []project.Moved
			for name, r := range state.ProjectRuns(id) {
				moved = append(moved, project.Moved{Name: name, Worktree: r.Worktree, Path: r.PRD, Branch: r.Branch})
			}
			prds, err := project.ReadAll(path, repo.Read(path, id).Config.Base(), moved)
			return prdsMsg{path: path, prds: prds, err: err}
		}
	}
	return tea.Batch(cmds...)
}

// prdRow is a row of the PRD list. Without a prd it is the group row of the done PRDs.
type prdRow struct {
	prd  *project.PRD
	done int
	// cost is the sum of all done PRDs.
	cost float64
}

// prdRows lists the open PRDs, the group row of the done ones and, if unfolded, the done PRDs.
func (p *projectEntry) prdRows() []prdRow {
	var open, done []*project.PRD
	for i := range p.prds {
		if d := &p.prds[i]; p.phaseOf(d).Done() {
			done = append(done, d)
		} else {
			open = append(open, d)
		}
	}
	// The newest PRD comes first, the label breaks ties.
	newest := func(a, b *project.PRD) int {
		if c := b.ModTime.Compare(a.ModTime); c != 0 {
			return c
		}
		return strings.Compare(a.Label(), b.Label())
	}
	slices.SortStableFunc(open, newest)
	slices.SortStableFunc(done, newest)
	var rows []prdRow
	for _, d := range open {
		rows = append(rows, prdRow{prd: d})
	}
	if len(done) > 0 {
		group := prdRow{done: len(done)}
		for _, d := range done {
			group.cost += d.Cost
		}
		rows = append(rows, group)
	}
	if p.showDone {
		for _, d := range done {
			rows = append(rows, prdRow{prd: d})
		}
	}
	return rows
}

// selectedPRD is the PRD under the cursor, nil on the group row or without PRDs.
func (p *projectEntry) selectedPRD() *project.PRD {
	rows := p.prdRows()
	if p.prdCursor >= len(rows) {
		return nil
	}
	return rows[p.prdCursor].prd
}

// selectPRDPath puts the cursor on the PRD at path. If it is gone, the cursor stays in range.
func (p *projectEntry) selectPRDPath(path string) {
	rows := p.prdRows()
	if i := slices.IndexFunc(rows, func(r prdRow) bool { return r.prd != nil && r.prd.Path == path }); i >= 0 {
		p.prdCursor = i
		return
	}
	p.prdCursor = clamp(p.prdCursor, len(rows))
}

// toggleDone folds the done PRDs in or out. The selected PRD stays selected, or the cursor goes to
// the group row if it disappears.
func (p *projectEntry) toggleDone() {
	path := ""
	if d := p.selectedPRD(); d != nil {
		path = d.Path
	}
	p.showDone = !p.showDone
	rows := p.prdRows()
	if i := slices.IndexFunc(rows, func(r prdRow) bool { return path != "" && r.prd != nil && r.prd.Path == path }); i >= 0 {
		p.prdCursor = i
		return
	}
	p.prdCursor = max(0, slices.IndexFunc(rows, func(r prdRow) bool { return r.prd == nil }))
}

// prdListView renders the PRD list in two lines per PRD, like the project list: label, then phase
// and branch. The group row takes one line. While focused, the cursor row gets a background.
func (p *projectEntry) prdListView(width, height int, focused bool, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	if p.prdErr != nil {
		msg, _, _ := strings.Cut(p.prdErr.Error(), "\n")
		return text.Foreground(t.focus).Render(ansi.Truncate(msg, width, "…"))
	}
	rows := p.prdRows()
	if len(rows) == 0 {
		return text.Faint(true).Render("No PRDs under " + project.Dir + "/.")
	}
	p.prdCursor = clamp(p.prdCursor, len(rows))
	inner := max(1, width-1)
	fill := func(s string) string {
		s = ansi.Truncate(s, inner, "…")
		return s + strings.Repeat(" ", max(0, inner-ansi.StringWidth(s)))
	}
	// Render all rows, then show a window that keeps the cursor row visible.
	var lines []string
	cursorTop, cursorBottom := 0, 0
	for i, r := range rows {
		selected := i == p.prdCursor
		if selected {
			cursorTop = len(lines)
		}
		if r.prd == nil {
			arrow := "▸"
			if p.showDone {
				arrow = "▾"
			}
			label := fmt.Sprintf("%s %d done", arrow, r.done)
			if r.cost > 0 {
				label += fmt.Sprintf(" · %.2f USD", r.cost)
			}
			style := text.Faint(true)
			if selected && focused {
				style = text.Foreground(t.selectedFg).Background(t.selectedBg)
			}
			lines = append(lines, style.Render(" "+fill(label)))
		} else {
			d := r.prd
			ph := p.phaseOf(d)
			second := string(ph)
			if d.Branch != "" {
				second += " · " + d.Branch
			}
			nameStyle, infoStyle := text.Bold(true), text.Faint(true)
			if selected {
				nameStyle, infoStyle = nameStyle.Foreground(t.focus), text
				if focused {
					nameStyle = nameStyle.Background(t.selectedBg)
					infoStyle = infoStyle.Foreground(t.selectedFg).Background(t.selectedBg)
				}
			}
			symbol := nameStyle
			if c := phaseColor(ph, t); c != nil {
				symbol = symbol.Foreground(c)
			}
			lines = append(lines,
				nameStyle.Render(" ")+symbol.Render(ph.Symbol())+nameStyle.Render(" "+pad(d.Label(), inner-2)),
				infoStyle.Render(" "+fill("  "+second)))
		}
		if selected {
			cursorBottom = len(lines)
		}
	}
	top := 0
	if cursorBottom > height {
		top = cursorBottom - height
	}
	top = max(0, min(top, cursorTop, len(lines)-height))
	return strings.Join(lines[top:min(len(lines), top+height)], "\n")
}
