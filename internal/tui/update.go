package tui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/version"
)

// updateTimeout bounds the check for a newer release, installTimeout its download.
const (
	updateTimeout  = 10 * time.Second
	installTimeout = 2 * time.Minute
)

// executable is the binary the update replaces. Tests set a temporary file.
var executable = os.Executable

// updateMsg reports a release newer than the running version.
type updateMsg struct{ latest string }

// installMsg reports the end of an update to tag.
type installMsg struct {
	tag string
	err error
}

// checkUpdate asks GitHub once for the newest release. Builds without a release version check
// nothing, errors stay silent.
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

// startUpdate installs the newer release over the running binary in the background.
func (m *model) startUpdate() tea.Cmd {
	switch {
	case m.updating:
		m.note = "Updating lazydev to " + m.latest + " …"
		return nil
	case m.installed != "":
		m.note = installedNote(m.installed)
		return nil
	case m.latest == "":
		m.note = "No newer release of lazydev known."
		return nil
	}
	m.updating = true
	m.note = "Updating lazydev to " + m.latest + " …"
	tag := m.latest
	return func() tea.Msg {
		path, err := executable()
		if err == nil {
			// make install and install.sh may link the binary, the update replaces the target.
			path, err = filepath.EvalSymlinks(path)
		}
		if err != nil {
			return installMsg{tag: tag, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
		defer cancel()
		return installMsg{tag: tag, err: version.Install(ctx, tag, path)}
	}
}

// updateNote is the footer note about a newer release.
func updateNote(latest string) string {
	return "lazydev " + latest + " available, you run " + version.Version + ". alt+u updates."
}

// installedNote is the footer note after an update.
func installedNote(tag string) string {
	return "lazydev " + tag + " installed. Restart lazydev to use it."
}

// updateNoteShown is true while the footer note is about a newer or installed release.
func (m model) updateNoteShown() bool {
	return (m.latest != "" && m.note == updateNote(m.latest)) || (m.installed != "" && m.note == installedNote(m.installed))
}

// versionLine is the line of the General tab with the running version and a newer release.
func (m model) versionLine() string {
	line := "lazydev " + version.Version
	switch {
	case m.installed != "":
		line += " · " + m.installed + " installed, restart lazydev."
	case m.latest != "":
		line += " · " + m.latest + " available, alt+u updates."
	}
	return line
}
