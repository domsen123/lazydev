package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Farben aus dem btop-Theme „tokyo-night“. Den Hintergrund lässt lazydev beim Terminal.
var (
	colorFocus    = lipgloss.Color("#7dcfff") // hi_fg
	colorBorder   = lipgloss.Color("#565f89") // box, div_line
	colorMuted    = lipgloss.Color("#565f89") // inactive_fg
	colorTitle    = lipgloss.Color("#cfc9c2") // title, main_fg
	colorSelected = lipgloss.Color("#414868") // selected_bg
	colorError    = lipgloss.Color("#f7768e") // *_end
	colorWarn     = lipgloss.Color("#e0af68") // *_mid
	colorOK       = lipgloss.Color("#9ece6a") // *_start

	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
	styleError    = lipgloss.NewStyle().Foreground(colorError)
	styleWarn     = lipgloss.NewStyle().Foreground(colorWarn)
	styleOK       = lipgloss.NewStyle().Foreground(colorOK)
	styleSelected = lipgloss.NewStyle().Bold(true)
	styleKey      = lipgloss.NewStyle().Bold(true).Foreground(colorFocus)
)

// selMark am Zeilenanfang lässt box die Zeile über die volle Breite mit colorSelected hinterlegen.
const selMark = "\x00"

// box zeichnet einen Rahmen mit runden Ecken und dem Titel in der oberen Kante, wie btop.
// width und height gelten inklusive Rahmen. Zu lange Zeilen kürzt box mit "…".
func box(title string, lines []string, width, height int, focused bool) string {
	if width < 4 || height < 2 {
		return ""
	}
	border := lipgloss.NewStyle().Foreground(colorBorder)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorTitle)
	if focused {
		border = border.Foreground(colorFocus)
		titleStyle = titleStyle.Foreground(colorFocus)
	}
	inner := width - 2

	// Titel in Klammern wie bei btop: ╭─┐ Titel ┌──╮
	title = ansi.Truncate(" "+printable(title)+" ", max(0, inner-3), "…")
	top := border.Render("╭─┐") + titleStyle.Render(title) +
		border.Render("┌"+strings.Repeat("─", max(0, inner-3-ansi.StringWidth(title)))+"╮")
	if inner < 4 {
		top = border.Render("╭" + strings.Repeat("─", inner) + "╮")
	}

	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for i := 0; i < height-2; i++ {
		line, selected := "", false
		if i < len(lines) {
			line, selected = strings.CutPrefix(lines[i], selMark)
			line = ansi.Truncate(printable(line), inner, "…")
		}
		line += strings.Repeat(" ", max(0, inner-ansi.StringWidth(line)))
		if selected {
			line = highlight(line)
		}
		b.WriteString(border.Render("│") + line + border.Render("│"))
		b.WriteByte('\n')
	}
	b.WriteString(border.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}

// Innenabstand der Fenster über der TUI: Spalten links und rechts, Zeilen oben und unten.
const (
	dialogPadX = 2
	dialogPadY = 1
)

// dialog zeichnet ein Fenster mit box und Innenabstand. width und height gelten inklusive Rahmen.
// Die Innenbreite für lines ist width - 2 - 2*dialogPadX. Zu lange Zeilen kürzt dialog mit "…",
// überzählige Zeilen schneidet es ab.
func dialog(title string, lines []string, width, height int) string {
	inner := max(0, width-2-2*dialogPadX)
	rows := max(0, height-2-2*dialogPadY)
	pad := strings.Repeat(" ", dialogPadX)
	padded := make([]string, 0, height-2)
	for range dialogPadY {
		padded = append(padded, "")
	}
	for i, line := range lines {
		if i >= rows {
			break
		}
		line = ansi.Truncate(printable(line), inner, "…")
		padded = append(padded, pad+padRight(line, inner)+pad)
	}
	for len(padded) < height-2 {
		padded = append(padded, "")
	}
	return box(title, padded, width, height, true)
}

// highlight hinterlegt s mit colorSelected. Jeder Reset innerhalb von s beendet den Hintergrund,
// deshalb setzt highlight ihn nach jedem Reset neu.
func highlight(s string) string {
	on, _, _ := strings.Cut(lipgloss.NewStyle().Background(colorSelected).Render(" "), " ")
	if on == "" {
		return s
	}
	return on + strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+on) + "\x1b[0m"
}

// printable entfernt Formatzeichen wie den weichen Trennstrich U+00AD und Wagenrückläufe und ersetzt
// Tabs durch Leerzeichen. ansi.StringWidth und das Terminal zählen solche Zeichen unterschiedlich. Die
// Zeile bricht dann um und verschiebt die ganze Oberfläche. Escape-Sequenzen bleiben erhalten.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r == '\r', unicode.Is(unicode.Cf, r):
			return -1
		}
		return r
	}, s)
}

// scrollWindow liefert den Ausschnitt [start, end), der cursor bei rows Zeilen sichtbar hält.
func scrollWindow(cursor, total, rows int) (start, end int) {
	if rows <= 0 || total <= rows {
		return 0, total
	}
	start = max(0, min(cursor-rows/2, total-rows))
	return start, start + rows
}

// padRight füllt s mit Leerzeichen auf w Zellen auf oder kürzt es.
func padRight(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
