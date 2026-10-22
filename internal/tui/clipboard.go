package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// copiedMsg reports the end of a copy to the clipboard: the number of lines, or the error.
type copiedMsg struct {
	lines int
	err   error
}

// clipboardOut receives the OSC 52 sequence, the terminal lazydev runs in. writeLocal writes the
// local clipboard. Tests replace both.
var (
	clipboardOut io.Writer = os.Stdout
	writeLocal             = clipboard.WriteAll
)

// copyToClipboard puts text into the clipboard: always through OSC 52 to the terminal of lazydev,
// also into the local clipboard outside SSH sessions. Its error is ignored, OSC 52 is written.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return copiedMsg{err: errors.New("Nothing to copy.")}
		}
		// One write keeps the sequence from mixing with a frame of the renderer.
		if _, err := clipboardOut.Write([]byte(ansi.SetSystemClipboard(text))); err != nil {
			return copiedMsg{err: fmt.Errorf("Copy failed: %w", err)}
		}
		if !clipboard.Unsupported && os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == "" {
			_ = writeLocal(text)
		}
		return copiedMsg{lines: strings.Count(text, "\n") + 1}
	}
}

// copiedNote is the footer note after copying n lines.
func copiedNote(n int) string {
	if n == 1 {
		return "Copied 1 line."
	}
	return fmt.Sprintf("Copied %d lines.", n)
}
