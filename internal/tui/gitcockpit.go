package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazydev/internal/git"
	"lazydev/internal/gitcmd"
	"lazydev/internal/terminal"
)

// The git cockpit shows the state of the main folder of the selected project and runs fetch, pull,
// push and commit. Each action runs "lazydev git" as a center tab, so prompts for passphrases or
// credentials work like in a shell. The tab closes on success and waits for enter after a failure.

// cockpitWidth is the outer width of the cockpit box.
const cockpitWidth = 64

// cockpitActions are the actions in the order of the list, with their label and key.
var cockpitActions = []struct{ action, label, key string }{
	{gitcmd.Fetch, "Fetch", "f"},
	{gitcmd.Pull, "Pull", "p"},
	{gitcmd.Push, "Push", "P"},
	{gitcmd.Commit, "Commit", "c"},
}

// cockpit is the open git cockpit. It reads the repository once when it opens.
type cockpit struct {
	dir, id, name string
	// branch is the checked out branch, or "detached at <commit>" with detached set.
	branch   string
	detached bool
	// upstream is empty for a branch without upstream.
	upstream      string
	ahead, behind int
	changes       int
	commits       []string
	cursor        int
	// committing is true while the input for the commit message is open.
	committing bool
	input      textinput.Model
}

// openCockpit opens the cockpit for the main folder of the selected project.
func (m *model) openCockpit() {
	p := m.current()
	if p == nil {
		m.err = "Add a folder first: a in the left column"
		return
	}
	if _, err := git.Run(p.path, "rev-parse", "--git-dir"); err != nil {
		m.err = p.name() + " is no git repository."
		return
	}
	in := textinput.New()
	in.Prompt = "Message: "
	c := &cockpit{dir: p.path, id: projectIDs(m.projects)[m.selected], name: p.name(), input: in,
		branch: git.CurrentBranch(p.path), ahead: p.git.ahead, behind: p.git.behind}
	if c.branch == "" {
		short, _ := git.Run(p.path, "rev-parse", "--short=7", "HEAD")
		c.branch, c.detached = "detached at "+short, true
	}
	c.upstream = git.Upstream(p.path)
	c.changes = len(git.Output(p.path, "status", "--porcelain"))
	c.commits = git.Output(p.path, "log", "--oneline", "-5")
	m.cockpit = c
}

// updateCockpit handles all keys while the cockpit is open.
func (m model) updateCockpit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.cockpit
	key := msg.String()
	if c.committing {
		switch key {
		case "esc":
			c.committing = false
			c.input.Blur()
			return m, nil
		case "enter":
			message := c.input.Value()
			if strings.TrimSpace(message) == "" {
				m.err = "Enter a commit message."
				return m, nil
			}
			m.runGitAction(gitcmd.Commit, message)
			return m, nil
		}
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		return m, cmd
	}
	switch key {
	case "esc", "q", "ctrl+c", "alt+g":
		m.cockpit = nil
	case "j", "down":
		c.cursor = clamp(c.cursor+1, len(cockpitActions))
	case "k", "up":
		c.cursor = clamp(c.cursor-1, len(cockpitActions))
	case "enter":
		return m.startCockpitAction(cockpitActions[c.cursor].action)
	default:
		for _, a := range cockpitActions {
			if key == a.key {
				return m.startCockpitAction(a.action)
			}
		}
	}
	return m, nil
}

// startCockpitAction runs action, a commit opens the input for its message first.
func (m model) startCockpitAction(action string) (tea.Model, tea.Cmd) {
	if action != gitcmd.Commit {
		m.runGitAction(action, "")
		return m, nil
	}
	c := m.cockpit
	c.committing = true
	c.input.SetValue("")
	return m, c.input.Focus()
}

// runGitAction closes the cockpit and runs "lazydev git" for action as a center tab of the selected
// project.
func (m *model) runGitAction(action, message string) {
	c := m.cockpit
	m.cockpit = nil
	exe, err := os.Executable()
	if err != nil {
		m.err = err.Error()
		return
	}
	args := []string{exe, "git", c.id, c.dir, action}
	if action == gitcmd.Commit {
		args = append(args, message)
	}
	p := m.current()
	n := len(p.tabs)
	m.openTerminal(args)
	if len(p.tabs) == n {
		return
	}
	t := p.tabs[p.tab].(*terminal.Terminal)
	t.SetLabel("git " + action)
	m.gitTabs[t.ID()] = action
}

// viewCockpit renders the cockpit as a box for the overlay.
func (m model) viewCockpit() string {
	c := m.cockpit
	t := themes[m.theme]
	text := lipgloss.NewStyle().Foreground(t.text)
	width := min(m.width-4, cockpitWidth)
	inner := width - 4
	branch := c.branch
	switch {
	case c.detached:
	case c.upstream == "":
		branch += " · no upstream"
	default:
		branch += " → " + c.upstream
		if c.ahead > 0 {
			branch += " ↑" + strconv.Itoa(c.ahead)
		}
		if c.behind > 0 {
			branch += " ↓" + strconv.Itoa(c.behind)
		}
	}
	changes := "clean"
	if c.changes > 0 {
		changes = fmt.Sprintf("%d files", c.changes)
	}
	lines := []string{"", text.Render(pad("Branch    "+branch, inner)), text.Render(pad("Changes   "+changes, inner)), ""}
	if c.committing {
		in := c.input
		in.PromptStyle = text.Bold(true)
		in.TextStyle = text
		in.Width = max(1, inner-lipgloss.Width(in.Prompt)-1)
		lines = append(lines, in.View())
	} else {
		for i, a := range cockpitActions {
			label := fmt.Sprintf("%-10s%s", a.label, a.key)
			if i == c.cursor {
				lines = append(lines, text.Bold(true).Foreground(t.selectedFg).Background(t.selectedBg).Render("> "+pad(label, inner-2)))
			} else {
				lines = append(lines, text.Render("  "+pad(label, inner-2)))
			}
		}
	}
	lines = append(lines, "", text.Bold(true).Render(pad("Recent commits", inner)))
	if len(c.commits) == 0 {
		lines = append(lines, text.Faint(true).Render(pad("no commits", inner)))
	}
	for _, commit := range c.commits {
		lines = append(lines, text.Render(pad(commit, inner)))
	}
	lines = append(lines, "")
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return box("Git · "+c.name, strings.Join(lines, "\n"), width, len(lines)+2, t.focus, true, t)
}

// keys is the key help of the cockpit.
func (c *cockpit) keys() string {
	if c.committing {
		return " enter commit · esc back"
	}
	return " j/k move · enter run · f fetch · p pull · P push · c commit · esc close"
}
