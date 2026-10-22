package tui

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/claude"
	"lazydev/internal/prompts"
	"lazydev/internal/repo"
)

// The create step writes a new PRD together with the user. Claude Code runs interactively as a
// center tab, so the user answers its questions right there. The prompt is the one of every step:
// header, knowledge, .lazydev/create.md and the contract.

// startCreate opens the input for the description of a new PRD below the columns.
func (m model) startCreate() (tea.Model, tea.Cmd) {
	m.creating = true
	m.createInput.SetValue("")
	return m, m.createInput.Focus()
}

// updateCreate handles the input of a new PRD. Enter starts Claude, Esc cancels.
func (m model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.creating = false
		m.createInput.Blur()
		return m, nil
	case "enter":
		m.creating = false
		m.createInput.Blur()
		m.createPRD(strings.TrimSpace(m.createInput.Value()))
		return m, nil
	}
	var cmd tea.Cmd
	m.createInput, cmd = m.createInput.Update(msg)
	return m, cmd
}

// createPRD starts Claude Code with the create prompt in the main folder of the selected project.
// description fills $ARGUMENTS, Claude asks for the rest.
func (m *model) createPRD(description string) {
	p := m.current()
	if p == nil {
		return
	}
	if !m.initLazydev(p) {
		return
	}
	arguments := description
	if arguments == "" {
		arguments = "Neues PRD. Frag mich, worum es geht."
	}
	prompt, err := prompts.Build(p.path, p.path, p.lazydev.Config, "create", prompts.Vars{Arguments: arguments, Base: p.lazydev.Config.Base(), Branch: p.git.branch})
	if err != nil {
		m.err = err.Error()
		return
	}
	m.openClaudeTab("create", p.path, prompt)
}

// initLazydev runs .lazydev/ init in the main folder of p, so a deleted .lazydev/ comes back,
// and reads it again. It is false with the error in the footer.
func (m *model) initLazydev(p *projectEntry) bool {
	if err := repo.Init(p.path); err != nil {
		m.err = err.Error()
		return false
	}
	m.readLazydev()
	if p.lazydev.Err != nil {
		m.err = p.lazydev.Err.Error()
		return false
	}
	return true
}

// openClaudeTab opens Claude Code interactively with args in dir as a center tab of the selected
// project with the fixed label.
func (m *model) openClaudeTab(label, dir string, args ...string) {
	p := m.current()
	if p == nil {
		return
	}
	n := len(p.tabs)
	m.openTerminalIn(dir, append(slices.Clone(claude.Command), args...))
	if t := p.activeTerminal(); t != nil && len(p.tabs) > n {
		t.SetLabel(label)
	}
}
