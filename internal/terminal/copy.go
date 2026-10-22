package terminal

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Copy mode freezes a terminal tab: it shows a snapshot of the scrollback and the screen, a line
// mark moves through it, and the selection goes to the clipboard as plain text. The program keeps
// running, its output shows again once copy mode ends.

const (
	reverseOn  = "\x1b[7m"
	reverseOff = "\x1b[27m"
)

// copyLine is one line of the snapshot.
type copyLine struct {
	// styled is the line with its ANSI styles, for the view.
	styled string
	// plain is the line without styles and without trailing spaces, for copying. Its columns are
	// the columns of the screen.
	plain string
	// full is true if the line reaches the last column of the tab at the time of the snapshot.
	// x/vt keeps no soft wraps, so the next line counts as its continuation.
	full bool
}

type copyLines []copyLine

// newCopyLine turns a line of the emulator into a copyLine. width is the width of the tab.
func newCopyLine(l uv.Line, width int) copyLine {
	// The scrollback drops trailing empty cells, the screen keeps them, so count up to the last
	// cell with content.
	cells := 0
	for i := len(l) - 1; i >= 0; i-- {
		if c := &l[i]; !c.IsZero() && !c.Equal(&uv.EmptyCell) {
			cells = i + max(1, c.Width)
			break
		}
	}
	return copyLine{styled: l.Render(), plain: strings.TrimRight(l.String(), " "), full: cells >= width}
}

// selectionPoint is a cell of the snapshot: line index and column.
type selectionPoint struct{ line, col int }

func (p selectionPoint) before(o selectionPoint) bool {
	return p.line < o.line || p.line == o.line && p.col < o.col
}

// endOfLine as a column selects up to the end of the line.
const endOfLine = int(^uint(0) >> 1)

// copyMode is the state of an open copy mode.
type copyMode struct {
	lines copyLines
	// top is the first visible line, mark the line of the mark, anchor the other end of a
	// keyboard selection, -1 without one.
	top, mark, anchor int
	// selecting is true while the mouse drags a selection from from to to.
	selecting bool
	from, to  selectionPoint
	// byWheel and byMouse tell how copy mode started: wheel down to the end or releasing the mouse
	// ends it again.
	byWheel, byMouse bool
}

// joinText is the plain text from from to to, both cells included, in any order. Lines end with
// "\n" unless they are full, trailing spaces drop, the last line gets no "\n".
func joinText(lines copyLines, from, to selectionPoint) string {
	if to.before(from) {
		from, to = to, from
	}
	var b strings.Builder
	for i := max(0, from.line); i <= to.line && i < len(lines); i++ {
		l := lines[i]
		start, end := 0, ansi.StringWidth(l.plain)
		if i == from.line {
			start = from.col
		}
		if i == to.line && to.col < end {
			end = to.col + 1
		}
		if start < end {
			b.WriteString(ansi.Cut(l.plain, start, end))
		}
		if i < to.line && !l.full {
			b.WriteByte('\n')
		}
	}
	out := strings.Split(b.String(), "\n")
	for i, line := range out {
		out[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(out, "\n")
}

// clamp keeps mark, anchor, view and mouse selection inside the snapshot and the mark visible.
func (c *copyMode) clamp(width, height int) {
	n := len(c.lines)
	c.mark = max(0, min(c.mark, n-1))
	if c.anchor >= 0 {
		c.anchor = min(c.anchor, n-1)
	}
	c.top = max(0, min(c.top, c.mark, n-height))
	if c.mark >= c.top+height {
		c.top = c.mark - height + 1
	}
	c.from, c.to = c.clampPoint(c.from, width), c.clampPoint(c.to, width)
}

func (c *copyMode) clampPoint(p selectionPoint, width int) selectionPoint {
	return selectionPoint{line: max(0, min(p.line, len(c.lines)-1)), col: max(0, min(p.col, width-1))}
}

// move moves the mark by delta lines, the view follows.
func (c *copyMode) move(delta, width, height int) {
	c.mark += delta
	c.clamp(width, height)
}

// scrollBy moves the view by delta lines. The mark moves along and keeps its row in the window.
func (c *copyMode) scrollBy(delta, width, height int) {
	top := max(0, min(c.top+delta, len(c.lines)-height))
	c.mark += top - c.top
	c.top = top
	c.clamp(width, height)
}

// atEnd is true while the view shows the last line of the snapshot.
func (c *copyMode) atEnd(height int) bool {
	return c.top >= len(c.lines)-height
}

// point is the cell of the snapshot under cell x, y of the view.
func (c *copyMode) point(x, y, width, height int) selectionPoint {
	return c.clampPoint(selectionPoint{line: c.top + max(0, min(y, height-1)), col: x}, width)
}

// selection is the text y copies: the mouse selection, the keyboard selection, or the line of the
// mark.
func (c *copyMode) selection() string {
	switch {
	case c.selecting:
		return joinText(c.lines, c.from, c.to)
	case c.anchor >= 0:
		return joinText(c.lines, selectionPoint{min(c.anchor, c.mark), 0}, selectionPoint{max(c.anchor, c.mark), endOfLine})
	}
	return joinText(c.lines, selectionPoint{c.mark, 0}, selectionPoint{c.mark, endOfLine})
}

// view renders the visible lines. The mark and the selection show in reverse video without styles.
func (c *copyMode) view(width, height int) string {
	lo, hi := c.mark, c.mark
	if c.anchor >= 0 {
		lo, hi = min(c.anchor, c.mark), max(c.anchor, c.mark)
	}
	from, to := c.from, c.to
	if to.before(from) {
		from, to = to, from
	}
	out := make([]string, height)
	for row := range height {
		i := c.top + row
		if i >= len(c.lines) {
			continue
		}
		l := c.lines[i]
		switch {
		case c.selecting && i >= from.line && i <= to.line:
			start, end := 0, width
			if i == from.line {
				start = from.col
			}
			if i == to.line {
				end = min(width, to.col+1)
			}
			out[row] = highlight(l.plain, width, start, end)
		case !c.selecting && i >= lo && i <= hi:
			out[row] = highlight(l.plain, width, 0, width)
		default:
			out[row] = ansi.Truncate(l.styled, width, "")
		}
	}
	return strings.Join(out, "\n")
}

// highlight pads plain to width and draws columns start to end in reverse video.
func highlight(plain string, width, start, end int) string {
	s := ansi.Truncate(plain, width, "")
	s += strings.Repeat(" ", max(0, width-ansi.StringWidth(s)))
	if start >= end {
		return s
	}
	return ansi.Cut(s, 0, start) + reverseOn + ansi.Cut(s, start, end) + reverseOff + ansi.Cut(s, end, width)
}

// EnterCopy opens copy mode with a snapshot of the scrollback and the screen. The view shows the
// screen as before, the mark stands on the line of the cursor. byWheel and byMouse tell what opened
// it. In copy mode already, nothing changes.
func (t *Terminal) EnterCopy(byWheel, byMouse bool) {
	if t.copy != nil {
		return
	}
	lines := t.snapshot()
	width, height := t.emu.Width(), t.emu.Height()
	mark := len(lines) - 1
	if pos := t.emu.CursorPosition(); !t.emu.CursorHidden() && pos.Y >= 0 && pos.Y < height {
		mark = len(lines) - height + pos.Y
	}
	t.copy = &copyMode{lines: lines, top: len(lines) - height, mark: mark, anchor: -1, byWheel: byWheel, byMouse: byMouse}
	t.copy.clamp(width, height)
}

// LeaveCopy ends copy mode. The view shows the emulator again, with the output that came meanwhile.
func (t *Terminal) LeaveCopy() { t.copy = nil }

// InCopy is true while copy mode is open.
func (t *Terminal) InCopy() bool { return t.copy != nil }

// MouseSelecting is true while the mouse drags a selection.
func (t *Terminal) MouseSelecting() bool { return t.copy != nil && t.copy.selecting }

// CopyKey handles a key in copy mode. y returns the text to copy and done, and ends copy mode. esc
// and q end it without text. Other keys do nothing.
func (t *Terminal) CopyKey(key string) (text string, done bool) {
	c := t.copy
	if c == nil {
		return "", false
	}
	width, height := t.emu.Width(), t.emu.Height()
	half := max(1, height/2)
	switch key {
	case "j", "down":
		c.move(1, width, height)
	case "k", "up":
		c.move(-1, width, height)
	case "ctrl+d", "pgdown":
		c.move(half, width, height)
	case "ctrl+u", "pgup":
		c.move(-half, width, height)
	case "g", "home":
		c.move(-len(c.lines), width, height)
	case "G", "end":
		c.move(len(c.lines), width, height)
	case "v":
		// v drops a mouse selection, otherwise it sets or drops the anchor.
		switch {
		case c.selecting:
			c.selecting = false
		case c.anchor >= 0:
			c.anchor = -1
		default:
			c.anchor = c.mark
		}
	case "y":
		text := c.selection()
		t.LeaveCopy()
		return text, true
	case "esc", "q":
		t.LeaveCopy()
	}
	return "", false
}

// Scroll moves the view of copy mode by lines, the mark moves along. left is true if copy mode
// ended: the wheel opened it and the view reached the end without a selection.
func (t *Terminal) Scroll(lines int) (left bool) {
	c := t.copy
	if c == nil {
		return false
	}
	width, height := t.emu.Width(), t.emu.Height()
	c.scrollBy(lines, width, height)
	if lines > 0 && c.byWheel && c.atEnd(height) && c.anchor < 0 && !c.selecting {
		t.LeaveCopy()
		return true
	}
	return false
}

// MousePress starts a mouse selection at cell x, y of the terminal area and opens copy mode if it is
// not open. It drops a keyboard selection.
func (t *Terminal) MousePress(x, y int) {
	if t.copy == nil {
		t.EnterCopy(false, true)
	}
	c := t.copy
	p := c.point(x, y, t.emu.Width(), t.emu.Height())
	c.anchor = -1
	c.selecting, c.from, c.to = true, p, p
}

// MouseDrag moves the end of the mouse selection to cell x, y. Above or below the terminal area the
// view scrolls one line in that direction.
func (t *Terminal) MouseDrag(x, y int) {
	c := t.copy
	if c == nil || !c.selecting {
		return
	}
	width, height := t.emu.Width(), t.emu.Height()
	switch {
	case y < 0:
		c.scrollBy(-1, width, height)
	case y >= height:
		c.scrollBy(1, width, height)
	}
	c.to = c.point(x, y, width, height)
}

// MouseRelease ends the mouse selection at cell x, y and returns its text. ok is false if there was
// no selection or it starts and ends on the same cell. Copy mode ends if the mouse opened it.
func (t *Terminal) MouseRelease(x, y int) (text string, ok bool) {
	c := t.copy
	if c == nil || !c.selecting {
		return "", false
	}
	c.to = c.point(x, y, t.emu.Width(), t.emu.Height())
	c.selecting = false
	if ok = c.from != c.to; ok {
		text = joinText(c.lines, c.from, c.to)
	}
	if c.byMouse {
		t.LeaveCopy()
	}
	return text, ok
}
