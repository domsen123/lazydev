// Package prdtemplate beschreibt die Templates für neue PRDs. Ein Template legt fest, wie lazydev
// ein PRD im Chat erstellt und wie der Runner es umsetzt.
//
// Das Template "nuxt" ruft die globalen Commands /create-prd und /implement-prd auf. Das Template
// "einfach" bringt seine Prompts selbst mit, die globalen Commands bleiben unverändert.
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

//go:embed einfach/create.md
var simpleCreate string

//go:embed einfach/implement.md
var simpleImplement string

type Template struct {
	Name        string
	Label       string
	Description string
	// createCommand und implementCommand sind globale Slash-Commands. Ohne sie nutzt das Template
	// seine eingebetteten Prompts.
	createCommand, implementCommand string
	createPrompt, implementPrompt   string
}

// All listet die Templates in der Reihenfolge des Pickers.
var All = []Template{
	{
		Name:             Nuxt,
		Label:            "Nuxt-Feature",
		Description:      "/create-prd mit Datenmodell, Feature-Checkliste und UI-Handoff",
		createCommand:    "/create-prd",
		implementCommand: "/implement-prd",
	},
	{
		Name:            Simple,
		Label:           "Einfach",
		Description:     "ohne Datenmodell, UI-Handoff und Architektur-Doku",
		createPrompt:    simpleCreate,
		implementPrompt: simpleImplement,
	},
}

// Get liefert das Template name. Ein unbekannter oder leerer Name liefert "nuxt", weil PRDs aus
// dem globalen /create-prd keine Template-Angabe tragen.
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

// CreatePrompt liefert die erste Nachricht des Chats für die Beschreibung desc.
func (t Template) CreatePrompt(desc string) string {
	if t.createCommand != "" {
		return t.createCommand + " " + desc
	}
	return strings.ReplaceAll(t.createPrompt, "$ARGUMENTS", desc)
}

// CreateDisplay liefert die erste Nachricht so, wie der Chat sie anzeigt. Ein eingebetteter Prompt
// ist zu lang dafür, der Chat zeigt dann nur die Beschreibung.
func (t Template) CreateDisplay(desc string) string {
	if t.createCommand != "" {
		return t.CreatePrompt(desc)
	}
	return desc
}

// ImplementPrompt liefert den Prompt für den Schritt implement mit dem PRD-Pfad prd.
func (t Template) ImplementPrompt(prd string) string {
	if t.implementCommand != "" {
		return t.implementCommand + " " + prd
	}
	return strings.ReplaceAll(t.implementPrompt, "$ARGUMENTS", prd)
}
