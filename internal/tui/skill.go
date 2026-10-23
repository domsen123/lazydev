package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/config"
	"lazydev/internal/skill"
)

// The TUI keeps the Claude Code skill lazydev current: at its start it writes the skill like
// "lazydev skill install", so the skill follows every update of the binary. Success stays silent,
// an error becomes a footer note. "skill": false in settings.json switches it off.

// skillMsg reports a failed installation of the skill.
type skillMsg struct{ err error }

// installSkill writes the skill in the background, nil when the settings switch it off. The
// command returns a message only on error.
func installSkill(s config.Settings) tea.Cmd {
	if !s.SkillEnabled() {
		return nil
	}
	return func() tea.Msg {
		if _, err := skill.Install(); err != nil {
			return skillMsg{err: err}
		}
		return nil
	}
}
