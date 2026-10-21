package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/repo"
)

// The details tab shows the lazydev setup of the selected project while the right column shows
// its files, and the selected PRD while it shows the PRDs.

// column is one section of the details tab: a title and one value per row.
type column struct {
	title string
	rows  []string
	// max caps the width, the rest goes to the columns further right.
	max int
}

// detailsGap is the space between two columns of the details tab.
const detailsGap = 3

// projectDetailsView renders the lazydev setup as columns side by side: project, prompts, hooks,
// knowledge and allowed tools. Columns that do not fit drop from the right, rows that do not fit
// fold into "+n more".
func projectDetailsView(info repo.Info, width, height int, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	muted := text.Faint(true)
	warn := text.Foreground(t.focus)
	ok := text.Foreground(colorOK)

	switch {
	case info.Err != nil:
		msg, _, _ := strings.Cut(info.Err.Error(), "\n")
		return warn.Render(ansi.Truncate(" "+repo.Dir+"/"+msg, width, "…"))
	case !info.Initialized:
		return " " + warn.Render(repo.Dir+"/ missing") + muted.Render(", press i to set it up")
	}
	// field is a row of a label and a value, labels of one column share a width.
	field := func(label string, w int, value string) string {
		return muted.Render(pad(label, w)) + value
	}

	project := column{title: "Project", max: 28, rows: []string{field("base", 7, text.Render(info.Config.Base()))}}
	switch info.Trust {
	case repo.TrustNone:
		project.rows = append(project.rows, field("trust", 7, muted.Render("–")))
	case repo.TrustConfirmed:
		project.rows = append(project.rows, field("trust", 7, ok.Render("✓ trusted")))
	case repo.TrustPending:
		// Without trust lazydev refuses to start runs.
		project.rows = append(project.rows, field("trust", 7, warn.Render("✗ pending")),
			muted.Render("s on a PRD asks"))
	}

	prompts := column{title: "Prompts", max: 20}
	for i, step := range repo.Steps {
		if info.Prompts[i] {
			prompts.rows = append(prompts.rows, ok.Render("✓ ")+text.Render(step))
		} else {
			prompts.rows = append(prompts.rows, warn.Render("✗ "+step))
		}
	}

	h := info.Config.Hooks
	hooks := column{title: "Hooks", max: 36}
	for _, hook := range []struct{ name, script string }{{"setup", h.Setup}, {"teardown", h.Teardown}, {"check", h.Check}, {"dev", h.Dev}} {
		value := muted.Render("–")
		if hook.script != "" {
			value = text.Render(hook.script)
		}
		hooks.rows = append(hooks.rows, field(hook.name, 10, value))
	}

	// Knowledge lists each file once with the steps that read it.
	knowledge := column{title: "Knowledge", max: 48}
	var files []string
	readers := map[string][]string{}
	for _, k := range repo.KnowledgeKeys {
		for _, f := range info.Config.Knowledge[k] {
			if !slices.Contains(files, f) {
				files = append(files, f)
			}
			readers[f] = append(readers[f], k)
		}
	}
	fileW := 0
	for _, f := range files {
		fileW = max(fileW, ansi.StringWidth(f)+2)
	}
	for _, f := range files {
		knowledge.rows = append(knowledge.rows, text.Render(pad(f, fileW))+muted.Render(strings.Join(readers[f], " ")))
	}
	if len(files) == 0 {
		knowledge.rows = []string{muted.Render("–")}
	}

	tools := column{title: "Allowed tools", max: 40}
	for _, tool := range info.Config.AllowedTools {
		tools.rows = append(tools.rows, text.Render(tool))
	}
	if len(tools.rows) == 0 {
		tools.rows = []string{muted.Render("–")}
	}

	return renderColumns([]column{project, prompts, hooks, knowledge, tools}, width, height, t)
}

// renderColumns lays out columns left to right with their titles in the first row.
func renderColumns(cols []column, width, height int, t theme) string {
	title := lipgloss.NewStyle().Foreground(t.title).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.text).Faint(true)
	rows := max(0, height-1)
	var blocks []string
	left := width - 1
	for i, c := range cols {
		if len(c.rows) > rows && rows > 0 {
			rest := len(c.rows) - rows + 1
			c.rows = append(slices.Clone(c.rows[:rows-1]), muted.Render(fmt.Sprintf("+%d more", rest)))
		}
		w := ansi.StringWidth(c.title)
		for _, r := range c.rows {
			w = max(w, ansi.StringWidth(r))
		}
		w = min(w, c.max)
		if i > 0 {
			w += detailsGap
		}
		if left < 8 {
			break
		}
		w = min(w, left)
		left -= w
		lines := []string{title.Render(c.title)}
		lines = append(lines, c.rows...)
		for j := range lines {
			if i > 0 {
				lines[j] = strings.Repeat(" ", detailsGap) + lines[j]
			}
			lines[j] = pad(lines[j], w)
		}
		blocks = append(blocks, strings.Join(lines[:min(len(lines), height)], "\n"))
	}
	view := lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
	lines := strings.Split(view, "\n")
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return strings.Join(lines, "\n")
}
