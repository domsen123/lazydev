package terminal

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type collector struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (c *collector) send(msg tea.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, msg)
}

func (c *collector) has(want tea.Msg) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if m == want {
			return true
		}
	}
	return false
}

func TestStart(t *testing.T) {
	var c collector
	term, err := Start(7, 40, 5, t.TempDir(), []string{"sh", "-c", `printf '\033]0;my title\007hi'`}, c.send)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Release()
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("program did not end")
	}
	// ExitMsg follows right after Done.
	deadline := time.Now().Add(5 * time.Second)
	for !c.has(ExitMsg{ID: 7, Code: 0}) {
		if time.Now().After(deadline) {
			t.Fatal("no ExitMsg")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !c.has(OutputMsg{ID: 7}) {
		t.Error("no OutputMsg")
	}
	if term.ID() != 7 {
		t.Errorf("ID: %d", term.ID())
	}
	view := term.View(false)
	if !strings.Contains(view, "hi") || strings.Contains(view, "my title") {
		t.Errorf("view: %q", view)
	}
	if got := term.TabTitle(); got != "my title" {
		t.Errorf("tab title: %q", got)
	}
	term.SetLabel("implement")
	if got := term.TabTitle(); got != "implement" {
		t.Errorf("tab title with label: %q", got)
	}
	if lines := strings.Split(term.View(true), "\n"); len(lines) != 5 {
		t.Errorf("view lines: %d", len(lines))
	}
}

func TestStartTabTitleFallsBackToName(t *testing.T) {
	var c collector
	term, err := Start(1, 20, 3, t.TempDir(), []string{"/bin/sh", "-c", "exit 0"}, c.send)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Release()
	<-term.Done()
	if got := term.TabTitle(); got != "sh" {
		t.Errorf("tab title: %q", got)
	}
}

func TestStartExitCode(t *testing.T) {
	var c collector
	term, err := Start(4, 20, 3, t.TempDir(), []string{"sh", "-c", "exit 3"}, c.send)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Release()
	<-term.Done()
	deadline := time.Now().Add(5 * time.Second)
	for !c.has(ExitMsg{ID: 4, Code: 3}) {
		if time.Now().After(deadline) {
			t.Fatal("no ExitMsg with code 3")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestHangup(t *testing.T) {
	var c collector
	term, err := Start(2, 20, 3, t.TempDir(), []string{"sleep", "30"}, c.send)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Release()
	term.Hangup()
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("program survived Hangup")
	}
}

func TestStartMissingProgram(t *testing.T) {
	if _, err := Start(3, 20, 3, t.TempDir(), []string{"/nonexistent/program"}, func(tea.Msg) {}); err == nil {
		t.Error("no error")
	}
}

func TestShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	if got := Shell(); len(got) != 1 || got[0] != "/bin/zsh" {
		t.Errorf("Shell: %v", got)
	}
	t.Setenv("SHELL", "")
	if got := Shell(); got[0] != "/bin/sh" {
		t.Errorf("Shell without $SHELL: %v", got)
	}
}
