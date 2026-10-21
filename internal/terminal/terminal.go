// Package terminal runs programs in a PTY and renders their screen with the x/vt emulator.
package terminal

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// closeTimeout is how long a program gets to end after SIGHUP before it is killed.
const closeTimeout = 2 * time.Second

// Terminal is a program running in a PTY. The emulator turns the PTY output into a screen of cells,
// and everything written to the emulator's input (keys, replies to queries) goes back to the PTY.
type Terminal struct {
	id int
	// name is the program name, the tab title until the program sets one.
	name string
	// label is a fixed tab title, e.g. the workflow step. It wins over the program's title.
	label string
	// mu guards label and title. The output goroutine sets the title.
	mu    sync.Mutex
	title string
	emu   *vt.SafeEmulator
	pty   *os.File
	cmd   *exec.Cmd
	// done is closed once the program has ended.
	done chan struct{}
}

// OutputMsg reports new output of terminal ID, so the view gets redrawn.
type OutputMsg struct{ ID int }

// ExitMsg reports that the program of terminal ID has ended. Code is the exit code of the program,
// -1 after a signal.
type ExitMsg struct{ ID, Code int }

// Shell is the user's shell, /bin/sh without $SHELL.
func Shell() []string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return []string{shell}
	}
	return []string{"/bin/sh"}
}

// Start runs args in dir, in a PTY of the given size. The tab title is the program name until the
// program sets one. send delivers OutputMsg and ExitMsg to the Bubble Tea program.
func Start(id, width, height int, dir string, args []string, send func(tea.Msg)) (*Terminal, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	f, err := pty.StartWithSize(cmd, winsize(width, height))
	if err != nil {
		return nil, err
	}
	t := &Terminal{id: id, name: filepath.Base(args[0]), emu: vt.NewSafeEmulator(width, height), pty: f, cmd: cmd,
		done: make(chan struct{})}

	// PTY output → emulator, without title sequences.
	go func() {
		titles := titleFilter{onTitle: t.setTitle}
		buf := make([]byte, 32*1024)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				_, _ = t.emu.Write(titles.filter(buf[:n]))
				send(OutputMsg{id})
			}
			if err != nil {
				break
			}
		}
		_ = cmd.Wait()
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		// done comes before send: send blocks while Update runs, and closeAll waits in Update.
		close(t.done)
		send(ExitMsg{id, code})
	}()
	// Emulator input → PTY. The emulator's input is a pipe, so it must be drained constantly.
	go func() { _, _ = io.Copy(f, t.emu) }()
	return t, nil
}

// ID is the number the terminal was started with.
func (t *Terminal) ID() int { return t.id }

// Done is closed once the program has ended.
func (t *Terminal) Done() <-chan struct{} { return t.done }

// SetLabel sets a fixed tab title, e.g. the workflow step. It wins over the program's title.
func (t *Terminal) SetLabel(label string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.label = label
}

func (t *Terminal) setTitle(title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.title = title
}

// TabTitle is the fixed label, otherwise the title the program set, otherwise the program name.
func (t *Terminal) TabTitle() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case t.label != "":
		return t.label
	case t.title == "":
		return t.name
	}
	return t.title
}

func winsize(width, height int) *pty.Winsize {
	return &pty.Winsize{Cols: uint16(max(1, width)), Rows: uint16(max(1, height))}
}

// Resize changes the size of the screen and the PTY.
func (t *Terminal) Resize(width, height int) {
	if width < 1 || height < 1 || (width == t.emu.Width() && height == t.emu.Height()) {
		return
	}
	t.emu.Resize(width, height)
	_ = pty.Setsize(t.pty, winsize(width, height))
}

// Hangup asks the program to end with SIGHUP, like a terminal window that gets closed. Killing it
// right away would keep programs from cleaning up: Claude Code then believes its fullscreen
// renderer crashed and falls back to the classic one. A program still running after closeTimeout
// gets killed. The output goroutine reports ExitMsg once it has ended.
func (t *Terminal) Hangup() {
	if t.cmd.Process == nil {
		return
	}
	_ = t.cmd.Process.Signal(syscall.SIGHUP)
	go func() {
		select {
		case <-t.done:
		case <-time.After(closeTimeout):
			_ = t.cmd.Process.Kill()
		}
	}()
}

// Release frees the PTY and the emulator of an ended program.
func (t *Terminal) Release() {
	_ = t.pty.Close()
	_ = t.emu.Close()
}

// View renders the screen. With showCursor, the cell under the cursor is drawn in reverse video.
func (t *Terminal) View(showCursor bool) string {
	lines := strings.Split(t.emu.Render(), "\n")
	pos := t.emu.CursorPosition()
	if !showCursor || t.emu.CursorHidden() || pos.Y < 0 || pos.Y >= len(lines) {
		return strings.Join(lines, "\n")
	}
	line := lines[pos.Y]
	char := ansi.Strip(ansi.Cut(line, pos.X, pos.X+1))
	if char == "" {
		char = " "
	}
	left := ansi.Truncate(line, pos.X, "")
	if pad := pos.X - ansi.StringWidth(left); pad > 0 {
		left += strings.Repeat(" ", pad)
	}
	lines[pos.Y] = left + "\x1b[7m" + char + "\x1b[27m" + ansi.TruncateLeft(line, pos.X+1, "")
	return strings.Join(lines, "\n")
}
