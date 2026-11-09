package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/config"
	"lazydev/internal/mod"
	"lazydev/internal/skill"
)

// The TUI keeps the Claude Code skill lazydev current: at its start it writes the skill like
// "lazydev skill install", so the skill follows every update of the binary. Success stays silent,
// an error becomes a footer note. "skill": false in settings.json switches it off.

// skillMsg reports a failed installation of the skill.
type skillMsg struct{ err error }

// installSkill writes the skill in the background unless the settings switch it off, and updates an
// installed mod (see package mod) either way. The command returns a message only on error.
func installSkill(s config.Settings) tea.Cmd {
	return func() tea.Msg {
		if s.SkillEnabled() {
			if _, err := skill.Install(); err != nil {
				return skillMsg{err: err}
			}
		}
		// An installed mod follows the binary like the skill; a missing one stays missing.
		if _, err := mod.Update(); err != nil {
			return skillMsg{err: fmt.Errorf("mod %s: %w", mod.Name, err)}
		}
		return nil
	}
}
