package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/version"
)

// updateTimeout bounds the check for a newer release.
const updateTimeout = 10 * time.Second

// updateMsg reports a release newer than the running version.
type updateMsg struct{ latest string }

// checkUpdate asks gh once for the newest release. Builds without a release version check nothing,
// errors stay silent.
func checkUpdate() tea.Cmd {
	if _, _, _, ok := version.Release(version.Version); !ok {
		return nil
	}
	current := version.Version
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
		defer cancel()
		latest, err := version.Latest(ctx)
		if err != nil || !version.Newer(latest, current) {
			return nil
		}
		return updateMsg{latest: latest}
	}
}

// updateNote is the footer note about a newer release.
func updateNote(latest string) string {
	return "lazydev " + latest + " available, you run " + version.Version + ". Run install.sh again to update."
}

// versionLine is the line of the General tab with the running version and a newer release.
func (m model) versionLine() string {
	line := "lazydev " + version.Version
	if m.latest != "" {
		line += " · " + m.latest + " available, run install.sh again."
	}
	return line
}
