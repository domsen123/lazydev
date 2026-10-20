package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

// markdown rendert den Detail-Text eines PRDs mit Glamour. Ein Renderer gilt für eine Breite,
// das letzte Ergebnis bleibt zwischengespeichert. Model hält einen Zeiger, damit der Cache
// Kopien des Models übersteht.
type markdown struct {
	width    int
	renderer *glamour.TermRenderer
	source   string
	out      string
}

// render liefert text als ANSI-Text mit Zeilenumbruch bei width. Ohne Renderer liefert render
// den Text unverändert.
func (md *markdown) render(text string, width int) string {
	width = max(10, width)
	if md.renderer == nil || md.width != width {
		md.renderer = newRenderer(width)
		md.width = width
		md.source = ""
	}
	if md.renderer == nil {
		return text
	}
	if md.source == text && md.out != "" {
		return md.out
	}
	out, err := md.renderer.Render(text)
	if err != nil {
		return text
	}
	md.source = text
	md.out = strings.Trim(out, "\n")
	return md.out
}

// newRenderer nutzt den Stil Tokyo Night ohne Rand und ohne Leerzeilen um das Dokument. Den Stil gibt
// newRenderer fest vor, weil die automatische Erkennung das Terminal abfragt und Bubble Tea stört.
func newRenderer(width int) *glamour.TermRenderer {
	style := styles.TokyoNightStyleConfig
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = ""
	noMargin := uint(0)
	style.Document.Margin = &noMargin
	r, err := glamour.NewTermRenderer(glamour.WithStyles(style), glamour.WithWordWrap(width))
	if err != nil {
		return nil
	}
	return r
}
