package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// wheelLines is how far one step of the mouse wheel scrolls.
const wheelLines = 3

// updateMouse handles the mouse in the terminal area of the active terminal tab: the wheel scrolls
// copy mode, dragging selects and copies on release. Mouse events never go to the program. With a
// dialog or input open, and outside the terminal area, lazydev ignores them, except moving and
// releasing during a selection. Unlike keys, they keep the footer error and note. In the same area
// of an active viewer tab, the wheel scrolls it and a left click focuses the center, nothing else.
func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.menu != nil || m.cockpit != nil || m.question != nil || m.status != nil || m.wizardVisible() ||
		m.adding || m.creating || m.confirmRemove {
		return m, nil
	}
	p := m.current()
	if p == nil {
		return m, nil
	}
	ox, oy := m.termOrigin()
	w, h := m.termSize()
	x, y := msg.X-ox, msg.Y-oy
	inside := x >= 0 && x < w && y >= 0 && y < h
	if v := p.activeView(); v != nil {
		switch {
		case !inside:
		case msg.Button == tea.MouseButtonWheelUp:
			v.scroll(-wheelLines)
		case msg.Button == tea.MouseButtonWheelDown:
			v.scroll(wheelLines)
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			m.focus = colCenter
		}
		return m, nil
	}
	term := p.activeTerminal()
	if term == nil {
		return m, nil
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		if inside {
			term.EnterCopy(true, false)
			term.Scroll(-wheelLines)
		}
	case msg.Button == tea.MouseButtonWheelDown:
		if inside {
			term.Scroll(wheelLines)
		}
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		if inside {
			m.focus = colCenter
			term.MousePress(x, y)
		}
	case msg.Action == tea.MouseActionMotion:
		term.MouseDrag(x, y)
	case msg.Action == tea.MouseActionRelease:
		if text, ok := term.MouseRelease(x, y); ok {
			return m, copyToClipboard(text)
		}
	}
	return m, nil
}
