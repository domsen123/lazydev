package prdtemplate

import (
	"strings"
	"testing"

	"lazydev/internal/project"
)

func TestOf(t *testing.T) {
	for line, want := range map[string]string{
		"> Status: geplant · Template: einfach · Scope: TUI":   Simple,
		"> Status: umgesetzt (2026-10-20) · Template: einfach": Simple,
		"> Status: geplant · Scope: Backend-only":              "",
		"> Status: geplant": "",
	} {
		s, _ := project.ParseStatusLine(line)
		if got := Of(s); got != want {
			t.Errorf("Of(%q) = %q, erwartet %q", line, got, want)
		}
	}
}

func TestPrompts(t *testing.T) {
	if got := Get("").Name; got != Nuxt {
		t.Errorf("Standard-Template: %q", got)
	}
	for _, tpl := range All {
		create := tpl.CreatePrompt("Avatar")
		impl := tpl.ImplementPrompt("prd/a/b.md")
		if !strings.Contains(create, "\nAvatar\n") || !strings.Contains(create, "Template: "+tpl.Name) ||
			!strings.HasPrefix(impl, "# PRD umsetzen (Template „"+tpl.Name+"“): prd/a/b.md\n") ||
			strings.Contains(create+impl, "$ARGUMENTS") {
			t.Errorf("Prompts %s:\n%s\n---\n%s", tpl.Name, create, impl)
		}
		// Eingebettete Prompts tragen keine Frontmatter eines Slash-Commands.
		if strings.HasPrefix(create, "---") || strings.HasPrefix(impl, "---") {
			t.Errorf("Prompts %s mit Frontmatter", tpl.Name)
		}
	}
}

func TestDefault(t *testing.T) {
	if Default("nuxt-drizzle") != Nuxt || Default("") != Simple {
		t.Error("Default wählt das falsche Template")
	}
}
