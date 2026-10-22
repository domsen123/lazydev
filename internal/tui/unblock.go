package tui

import (
	"lazydev/internal/claude"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
)

// A blocked run waits for a decision of the user. f opens Claude Code interactively in the
// worktree of the run with the unblock prompt: Claude asks the questions under "## Blocker" and
// works the answers into the PRD. s continues the run afterwards.

// unblockPRD opens the unblock dialog for the run of the PRD called name.
func (m *model) unblockPRD(name string) {
	p := m.current()
	if name == "" || p == nil {
		return
	}
	r := p.runs[name]
	if r == nil || r.Effective() != project.PhaseBlocked {
		m.err = p.labelOf(name) + " is not blocked."
		return
	}
	// Like create, a missing unblock.md comes back first.
	if !m.initLazydev(p) {
		return
	}
	prompt, err := prompts.Build(p.path, r.Worktree, p.lazydev.Config, "unblock", prompts.Vars{Arguments: r.PRD, Base: r.Base, Branch: r.Branch})
	if err != nil {
		m.err = err.Error()
		return
	}
	agents, err := claude.WriteAgents()
	if err != nil {
		m.err = err.Error()
		return
	}
	m.openClaudeTab("unblock", r.Worktree, prompt, "--agents", agents)
}

// blockedInWorktree is true while the status line of the PRD of the run of name in its worktree
// says blockiert.
func (p *projectEntry) blockedInWorktree(name string) bool {
	r := p.runs[name]
	return r != nil && project.BlockedIn(r.Worktree, r.PRD)
}
