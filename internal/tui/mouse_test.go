package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/terminal"
)

var altC = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c"), Alt: true}

// termModel is cockpitModel with a terminal tab that prints XYZ and keeps running.
func termModel(t *testing.T) (model, *terminal.Terminal) {
	t.Helper()
	m := cockpitModel(t)
	w, h := m.termSize()
	term, err := terminal.Start(1, w, h, m.projects[0].path, []string{"sh", "-c", "printf XYZ; sleep 30"}, func(tea.Msg) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		term.Hangup()
		<-term.Done()
		term.Release()
	})
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(term.View(false), "XYZ") {
		if time.Now().After(deadline) {
			t.Fatal("no output")
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.projects[0].tabs = append(m.projects[0].tabs, term)
	return m, term
}

// captureClipboard sends OSC 52 into the returned buffer instead of stdout and skips the local
// clipboard.
func captureClipboard(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	oldOut, oldLocal := clipboardOut, writeLocal
	clipboardOut, writeLocal = &out, func(string) error { return nil }
	t.Cleanup(func() { clipboardOut, writeLocal = oldOut, oldLocal })
	return &out
}

func mouse(t *testing.T, m model, x, y int, button tea.MouseButton, action tea.MouseAction) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Button: button, Action: action})
	return next.(model), cmd
}

func TestTermOrigin(t *testing.T) {
	m, _ := termModel(t)
	x, y := m.termOrigin()
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	if got := ansi.Cut(lines[y], x, x+3); got != "XYZ" {
		t.Errorf("cells at the origin: %q in line %q", got, lines[y])
	}
	if got := ansi.Cut(lines[y], x-1, x); got != "│" {
		t.Errorf("cell left of the origin: %q", got)
	}
}

func TestMouseOutsideOrWithMenu(t *testing.T) {
	m, term := termModel(t)
	x, y := m.termOrigin()
	w, h := m.termSize()
	for _, p := range [][2]int{{x - 1, y}, {x, y - 1}, {x + w, y}, {x, y + h}} {
		m, _ = mouse(t, m, p[0], p[1], tea.MouseButtonLeft, tea.MouseActionPress)
		m, _ = mouse(t, m, p[0], p[1], tea.MouseButtonWheelUp, tea.MouseActionPress)
		if term.InCopy() || m.focus != colLeft {
			t.Fatalf("mouse at %v: copy mode %v, focus %d", p, term.InCopy(), m.focus)
		}
	}
	m.menu = &menu{}
	m, _ = mouse(t, m, x, y, tea.MouseButtonLeft, tea.MouseActionPress)
	m, _ = mouse(t, m, x, y, tea.MouseButtonWheelUp, tea.MouseActionPress)
	if term.InCopy() || m.focus != colLeft {
		t.Fatalf("mouse with menu: copy mode %v, focus %d", term.InCopy(), m.focus)
	}

	// Without menu the same press focuses the center and selects.
	m.menu = nil
	m, _ = mouse(t, m, x, y, tea.MouseButtonLeft, tea.MouseActionPress)
	if !term.MouseSelecting() || m.focus != colCenter {
		t.Fatalf("press: selecting %v, focus %d", term.MouseSelecting(), m.focus)
	}
	out := captureClipboard(t)
	m, _ = mouse(t, m, x+2, y, tea.MouseButtonLeft, tea.MouseActionMotion)
	m, cmd := mouse(t, m, x+2, y, tea.MouseButtonLeft, tea.MouseActionRelease)
	if cmd == nil || term.InCopy() {
		t.Fatalf("release: cmd %v, copy mode %v", cmd != nil, term.InCopy())
	}
	next, _ := m.Update(cmd())
	m = next.(model)
	if m.note != "Copied 1 line." || out.String() != ansi.SetSystemClipboard("XYZ") {
		t.Errorf("note %q, err %q, OSC 52 %q", m.note, m.err, out.String())
	}
}

func TestCopyWithoutTerminal(t *testing.T) {
	m := press(t, cockpitModel(t), altC)
	if m.err != "The active tab is not a terminal." {
		t.Errorf("err %q", m.err)
	}
}

func TestCopyKeysInTUI(t *testing.T) {
	m, term := termModel(t)
	// c has no meaning in the project list and must not become alt+c.
	m = press(t, m, keyRunes("c"))
	if term.InCopy() || m.err != "" {
		t.Fatalf("c: copy mode %v, err %q", term.InCopy(), m.err)
	}
	m = press(t, m, altC)
	if !term.InCopy() || m.focus != colCenter {
		t.Fatalf("alt+c: copy mode %v, focus %d", term.InCopy(), m.focus)
	}
	plain := ansi.Strip(m.View())
	for _, want := range []string{"sh [copy]", "v select · y copy · esc leave"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	m = press(t, m, altC)
	if term.InCopy() {
		t.Fatal("alt+c in copy mode did not leave it")
	}
	m = press(t, m, altC)
	next, cmd := m.Update(keyRunes("y"))
	m = next.(model)
	if cmd == nil || term.InCopy() {
		t.Fatalf("y: cmd %v, copy mode %v", cmd != nil, term.InCopy())
	}
	out := captureClipboard(t)
	next, _ = m.Update(cmd())
	if m := next.(model); m.note != "Copied 1 line." || !strings.Contains(out.String(), ansi.SetSystemClipboard("XYZ")) {
		t.Errorf("note %q, err %q, OSC 52 %q", m.note, m.err, out.String())
	}
}

func TestCopyNothing(t *testing.T) {
	next, _ := cockpitModel(t).Update(copyToClipboard("")())
	if m := next.(model); m.err != "Nothing to copy." {
		t.Errorf("err %q", m.err)
	}
}
