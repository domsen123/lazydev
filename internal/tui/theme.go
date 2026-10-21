package tui

import (
	"slices"

	"lazydev/internal/config"

	"github.com/charmbracelet/lipgloss"
)

// theme holds the colors of the UI. The field comments name the btop theme keys they come from.
type theme struct {
	name string
	// text colors the key help and the menu (main_fg).
	text lipgloss.TerminalColor
	// title colors box titles of unfocused boxes (title).
	title lipgloss.TerminalColor
	// boxes color the outlines of the left, center and right column (cpu_box, proc_box, net_box).
	boxes [3]lipgloss.TerminalColor
	// divider colors separators inside boxes (div_line).
	divider lipgloss.TerminalColor
	// focus colors the focused box and the active tab (hi_fg).
	focus lipgloss.TerminalColor
	// selectedBg and selectedFg color the active tab of an unfocused column and the selected menu
	// row (selected_bg, selected_fg).
	selectedBg, selectedFg lipgloss.TerminalColor
	// onFocus is the text color on a focus colored background, e.g. the active tab (main_bg).
	onFocus lipgloss.TerminalColor
}

// themes lists the color themes in the order of the menu. The first one is the default.
var themes = []theme{
	{
		// btop's built-in Default theme. btop writes grays as "#cc", which means "#cccccc".
		name:       "Default",
		text:       lipgloss.Color("#cccccc"),
		title:      lipgloss.Color("#eeeeee"),
		boxes:      [3]lipgloss.TerminalColor{lipgloss.Color("#556d59"), lipgloss.Color("#805252"), lipgloss.Color("#5c588d")},
		divider:    lipgloss.Color("#303030"),
		focus:      lipgloss.Color("#b54040"),
		selectedBg: lipgloss.Color("#6a2f2f"),
		selectedFg: lipgloss.Color("#eeeeee"),
		onFocus:    lipgloss.Color("#000000"),
	},
	{
		// btop's tokyo-night.theme.
		name:       "Tokyo Night",
		text:       lipgloss.Color("#cfc9c2"),
		title:      lipgloss.Color("#cfc9c2"),
		boxes:      [3]lipgloss.TerminalColor{lipgloss.Color("#565f89"), lipgloss.Color("#565f89"), lipgloss.Color("#565f89")},
		divider:    lipgloss.Color("#565f89"),
		focus:      lipgloss.Color("#7dcfff"),
		selectedBg: lipgloss.Color("#414868"),
		selectedFg: lipgloss.Color("#cfc9c2"),
		onFocus:    lipgloss.Color("#1a1b26"),
	},
}

// loadTheme returns the index of the saved theme, the default theme if nothing is saved or the
// saved name is unknown.
func loadTheme() int {
	name := config.LoadSettings().Theme
	return max(0, slices.IndexFunc(themes, func(t theme) bool { return t.name == name }))
}

// saveTheme remembers the theme by name, so reordering themes keeps the choice.
func saveTheme(i int) error {
	s := config.LoadSettings()
	s.Theme = themes[i].name
	return config.SaveSettings(s)
}
