package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	// tabMaxLabel caps the label of a single tab, number included.
	tabMaxLabel = 24
	// tabMinLabel is how far labels shrink before the bar starts to scroll. It keeps the number
	// and a few characters of the title.
	tabMinLabel = 6
	// tabPadding is the space left and right of a label.
	tabPadding = 1
)

// tabLayout decides which tabs the bar shows and how long their labels may be.
type tabLayout struct {
	// first and last are the visible tabs, last included.
	first, last int
	// limit is the maximum label width.
	limit int
}

// layoutTabs fits labels into width. Labels first get capped at tabMaxLabel, then all of them
// shrink evenly down to tabMinLabel, and if that is still too wide, the bar shows a window around
// the active tab with room for the "‹n" and "n›" markers.
func layoutTabs(widths []int, active, width int) tabLayout {
	n := len(widths)
	for limit := tabMaxLabel; limit >= tabMinLabel; limit-- {
		if barWidth(widths, 0, n-1, limit) <= width {
			return tabLayout{first: 0, last: n - 1, limit: limit}
		}
	}
	// Grow a window around the active tab, alternating right and left.
	l := tabLayout{first: active, last: active, limit: tabMinLabel}
	fits := func(first, last int) bool {
		return barWidth(widths, first, last, tabMinLabel)+markerWidth(first, last, n) <= width
	}
	for grown := true; grown; {
		grown = false
		if l.last+1 < n && fits(l.first, l.last+1) {
			l.last++
			grown = true
		}
		if l.first > 0 && fits(l.first-1, l.last) {
			l.first--
			grown = true
		}
	}
	return l
}

// barWidth is the width of tabs first to last with labels capped at limit, one space between tabs.
func barWidth(widths []int, first, last, limit int) int {
	w := 0
	for i := first; i <= last; i++ {
		w += min(widths[i], limit) + 2*tabPadding + 1
	}
	return w - 1
}

// markerWidth is the room the "‹n " and " n›" markers need for a window first..last of n tabs.
func markerWidth(first, last, n int) int {
	w := 0
	if first > 0 {
		w += len(fmt.Sprint(first)) + 2
	}
	if last < n-1 {
		w += len(fmt.Sprint(n-1-last)) + 2
	}
	return w
}

// renderTabs draws the tab bar. style returns the style of tab i.
func renderTabs(labels []string, active, width int, style func(i int) lipgloss.Style, muted lipgloss.Style) string {
	widths := make([]int, len(labels))
	for i, l := range labels {
		widths[i] = ansi.StringWidth(l)
	}
	l := layoutTabs(widths, active, width)
	var parts []string
	if l.first > 0 {
		parts = append(parts, muted.Render(fmt.Sprintf("‹%d", l.first)))
	}
	for i := l.first; i <= l.last; i++ {
		label := ansi.Truncate(labels[i], l.limit, "…")
		parts = append(parts, style(i).Padding(0, tabPadding).Render(label))
	}
	if rest := len(labels) - 1 - l.last; rest > 0 {
		parts = append(parts, muted.Render(fmt.Sprintf("%d›", rest)))
	}
	return ansi.Truncate(strings.Join(parts, " "), width, "…")
}
