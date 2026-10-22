package terminal

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// emuTerminal is a terminal without PTY and program. Output goes into the emulator with write.
func emuTerminal(t *testing.T, width, height int) *Terminal {
	t.Helper()
	term := &Terminal{name: "sh", emu: vt.NewSafeEmulator(width, height)}
	t.Cleanup(func() { _ = term.emu.Close() })
	return term
}

func (t *Terminal) write(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = t.emu.Write([]byte(s))
}

func TestCopyJoinsWrappedLine(t *testing.T) {
	term := emuTerminal(t, 10, 5)
	long := "abcdefghijklmnopqrstuvwxy"
	term.write(long)
	lines := term.snapshot()
	if !lines[0].full || !lines[1].full || lines[2].full {
		t.Fatalf("full: %v %v %v", lines[0].full, lines[1].full, lines[2].full)
	}
	if got := joinText(lines, selectionPoint{0, 0}, selectionPoint{2, endOfLine}); got != long {
		t.Errorf("text %q, want %q", got, long)
	}
}

func TestCopyShortLines(t *testing.T) {
	term := emuTerminal(t, 10, 5)
	term.write("a   \r\nb  ")
	got := joinText(term.snapshot(), selectionPoint{0, 0}, selectionPoint{1, endOfLine})
	if got != "a\nb" {
		t.Errorf("text %q", got)
	}
}

func TestCopyMouseSelection(t *testing.T) {
	lines := copyLines{{plain: "first line"}, {plain: "middle"}, {plain: "last line"}}
	from, to := selectionPoint{0, 6}, selectionPoint{2, 3}
	want := "line\nmiddle\nlast"
	if got := joinText(lines, from, to); got != want {
		t.Errorf("text %q, want %q", got, want)
	}
	if got := joinText(lines, to, from); got != want {
		t.Errorf("swapped: text %q, want %q", got, want)
	}
}

func TestCopyWideCharacters(t *testing.T) {
	term := emuTerminal(t, 10, 3)
	term.write("日本 ok")
	lines := term.snapshot()
	if got := joinText(lines, selectionPoint{0, 0}, selectionPoint{0, endOfLine}); got != "日本 ok" {
		t.Errorf("text %q", got)
	}
	// Columns count cells: 日本 takes columns 0 to 3.
	if got := joinText(lines, selectionPoint{0, 2}, selectionPoint{0, 3}); got != "本" {
		t.Errorf("cut %q", got)
	}
}

func TestCopyKeys(t *testing.T) {
	term := emuTerminal(t, 10, 4)
	term.write("1\r\n2\r\n3\r\n4\r\n5\r\n6\r\n7")
	term.EnterCopy(false, false)
	if !term.InCopy() || term.TabTitle() != "sh [copy]" {
		t.Fatalf("copy mode: %v, title %q", term.InCopy(), term.TabTitle())
	}
	c := term.copy
	// Scrollback 1 to 3, screen 4 to 7, the cursor stands on 7.
	if len(c.lines) != 7 || c.mark != 6 || c.top != 3 {
		t.Fatalf("lines %d, mark %d, top %d", len(c.lines), c.mark, c.top)
	}
	term.CopyKey("g")
	term.CopyKey("k")
	if c.mark != 0 || c.top != 0 {
		t.Errorf("k at the start: mark %d, top %d", c.mark, c.top)
	}
	term.CopyKey("G")
	if c.mark != 6 || c.top != 3 {
		t.Errorf("G: mark %d, top %d", c.mark, c.top)
	}
	for _, k := range []string{"v", "k", "k"} {
		term.CopyKey(k)
	}
	if !strings.Contains(term.View(false), reverseOn+"5") {
		t.Errorf("selection not in reverse video: %q", term.View(false))
	}
	text, done := term.CopyKey("y")
	if text != "5\n6\n7" || !done || term.InCopy() {
		t.Errorf("y: %q, done %v, in copy %v", text, done, term.InCopy())
	}

	term.EnterCopy(false, false)
	text, done = term.CopyKey("x")
	if text != "" || done || !term.InCopy() {
		t.Errorf("x: %q, done %v, in copy %v", text, done, term.InCopy())
	}
	text, done = term.CopyKey("esc")
	if text != "" || done || term.InCopy() {
		t.Errorf("esc: %q, done %v, in copy %v", text, done, term.InCopy())
	}
	if term.TabTitle() != "sh" {
		t.Errorf("title after esc: %q", term.TabTitle())
	}
}

func TestCopySnapshot(t *testing.T) {
	term := emuTerminal(t, 10, 3)
	term.write("before")
	term.EnterCopy(false, false)
	view := term.View(false)
	term.write("\r\nafter")
	if got := term.View(false); got != view {
		t.Errorf("view changed in copy mode: %q", got)
	}
	if strings.Contains(ansi.Strip(view), "after") {
		t.Errorf("view shows new output: %q", view)
	}
	term.LeaveCopy()
	if got := ansi.Strip(term.View(false)); !strings.Contains(got, "after") {
		t.Errorf("view after leaving lacks the output: %q", got)
	}
}

func TestCopyScrollByWheel(t *testing.T) {
	term := emuTerminal(t, 10, 3)
	term.write("1\r\n2\r\n3\r\n4\r\n5\r\n6")
	term.EnterCopy(true, false)
	if term.Scroll(-3) || term.copy.top != 0 || term.copy.mark != 2 {
		t.Fatalf("scroll up: top %d, mark %d", term.copy.top, term.copy.mark)
	}
	if !term.Scroll(3) || term.InCopy() {
		t.Error("wheel down to the end did not end copy mode")
	}

	term.EnterCopy(false, false)
	term.Scroll(-3)
	if term.Scroll(3) || !term.InCopy() {
		t.Error("copy mode without byWheel ended at the end")
	}
}

func TestCopyMouse(t *testing.T) {
	term := emuTerminal(t, 10, 3)
	term.write("one\r\ntwo\r\nthree")
	term.MousePress(1, 0)
	if !term.InCopy() || !term.MouseSelecting() {
		t.Fatal("press opened no copy mode")
	}
	term.MouseDrag(2, 2)
	if text, ok := term.MouseRelease(2, 2); !ok || text != "ne\ntwo\nthr" {
		t.Errorf("release: %q, %v", text, ok)
	}
	if term.InCopy() {
		t.Error("copy mode opened by the mouse stays after release")
	}

	// A click copies nothing, and copy mode opened by alt+c stays.
	term.EnterCopy(false, false)
	term.MousePress(1, 1)
	if text, ok := term.MouseRelease(1, 1); ok || text != "" {
		t.Errorf("click: %q, %v", text, ok)
	}
	if !term.InCopy() {
		t.Error("copy mode ended after release")
	}
}
