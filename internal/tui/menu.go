package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// menuWidth is the outer width of the menu box.
const menuWidth = 39

const (
	menuTheme = iota
	menuQuit
	menuItems
)

// menu is the btop style menu that esc opens. Changing the theme applies it right away.
type menu struct {
	cursor int
}

// updateMenu handles all keys while the menu is open.
func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "alt+m":
		m.menu = nil
	case "up", "k":
		m.menu.cursor = (m.menu.cursor + menuItems - 1) % menuItems
	case "down", "j", "tab":
		m.menu.cursor = (m.menu.cursor + 1) % menuItems
	case "left", "h":
		if m.menu.cursor == menuTheme {
			m.setTheme(m.theme - 1)
		}
	case "right", "l":
		if m.menu.cursor == menuTheme {
			m.setTheme(m.theme + 1)
		}
	case "enter", " ":
		switch m.menu.cursor {
		case menuTheme:
			m.setTheme(m.theme + 1)
		case menuQuit:
			return m.quit()
		}
	}
	return m, nil
}

// setTheme switches to theme i, wrapping around, and saves the choice for the next start.
func (m *model) setTheme(i int) {
	n := len(themes)
	m.theme = (i%n + n) % n
	if err := saveTheme(m.theme); err != nil {
		m.err = err.Error()
	}
}

// viewMenu renders the menu box.
func (m model) viewMenu() string {
	t := themes[m.theme]
	inner := menuWidth - 2
	row := func(i int, text string) string {
		style := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).Foreground(t.text)
		if i == m.menu.cursor {
			style = style.Bold(true).Foreground(t.selectedFg).Background(t.selectedBg)
		}
		return style.Render(text)
	}
	hint := lipgloss.NewStyle().Foreground(t.text).Width(inner).Align(lipgloss.Center)
	lines := []string{
		"",
		row(menuTheme, "Color theme"),
		row(menuTheme, "‹ "+t.name+" ›"),
		"",
		row(menuQuit, "Quit"),
		"",
		hint.Render("↑/↓ select · ←/→ change · esc close"),
	}
	return box("Menu", strings.Join(lines, "\n"), menuWidth, len(lines)+2, t.focus, true, t)
}

// overlay draws fg centered on top of bg. Both are multi line strings with ANSI styles.
func overlay(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	fgWidth := 0
	for _, l := range fgLines {
		fgWidth = max(fgWidth, ansi.StringWidth(l))
	}
	x := max(0, (width-fgWidth)/2)
	y := max(0, (height-len(fgLines))/2)
	for i, l := range fgLines {
		if y+i >= len(bgLines) {
			break
		}
		line := bgLines[y+i]
		left := ansi.Truncate(line, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		bgLines[y+i] = left + "\x1b[0m" + l + "\x1b[0m" + ansi.TruncateLeft(line, x+ansi.StringWidth(l), "")
	}
	return strings.Join(bgLines, "\n")
}
