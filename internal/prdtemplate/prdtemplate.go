// Package prdtemplate beschreibt die Templates für neue PRDs. Ein Template legt fest, wie lazydev
// ein PRD im Chat erstellt und wie der Runner es umsetzt.
//
// Jedes Template bringt seine Prompts eingebettet mit. Sie nutzen $ARGUMENTS als Platzhalter für
// die Beschreibung bzw. den PRD-Pfad.
package prdtemplate

import (
	_ "embed"
	"strings"

	"lazydev/internal/config"
	"lazydev/internal/project"
)

const (
	Nuxt   = "nuxt"
	Simple = "einfach"
)

//go:embed nuxt/create.md
var nuxtCreate string

//go:embed nuxt/implement.md
var nuxtImplement string

//go:embed einfach/create.md
var simpleCreate string

//go:embed einfach/implement.md
var simpleImplement string

type Template struct {
	Name        string
	Label       string
	Description string

	createPrompt, implementPrompt string
}

// All listet die Templates in der Reihenfolge des Pickers.
var All = []Template{
	{
		Name:            Nuxt,
		Label:           "Nuxt-Feature",
		Description:     "mit Datenmodell, Feature-Checkliste und UI-Handoff",
		createPrompt:    nuxtCreate,
		implementPrompt: nuxtImplement,
	},
	{
		Name:            Simple,
		Label:           "Einfach",
		Description:     "ohne Datenmodell, UI-Handoff und Architektur-Doku",
		createPrompt:    simpleCreate,
		implementPrompt: simpleImplement,
	},
}

// Get liefert das Template name. Ein unbekannter oder leerer Name liefert "nuxt", weil ältere PRDs
// keine Template-Angabe tragen.
func Get(name string) Template {
	for _, t := range All {
		if t.Name == name {
			return t
		}
	}
	return All[0]
}

// Index liefert die Position von name in All, sonst 0.
func Index(name string) int {
	for i, t := range All {
		if t.Name == name {
			return i
		}
	}
	return 0
}

// Default wählt das Template, das zum Preset eines Projekts passt.
func Default(preset string) string {
	if preset == config.PresetNuxtDrizzle {
		return Nuxt
	}
	return Simple
}

// Of liest das Template aus dem Freitext der Statuszeile, z. B.
// "> Status: geplant · Template: einfach · Scope: …".
func Of(s project.Status) string {
	for part := range strings.SplitSeq(s.Detail, "·") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(part), "Template:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// CreatePrompt liefert die erste Nachricht des Chats für die Beschreibung desc. Der Chat zeigt
// statt des langen Prompts nur desc.
func (t Template) CreatePrompt(desc string) string {
	return strings.ReplaceAll(t.createPrompt, "$ARGUMENTS", desc)
}

// ImplementPrompt liefert den Prompt für den Schritt implement mit dem PRD-Pfad prd.
func (t Template) ImplementPrompt(prd string) string {
	return strings.ReplaceAll(t.implementPrompt, "$ARGUMENTS", prd)
}
