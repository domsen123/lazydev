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
	if got := Get("").ImplementPrompt("prd/a/b.md"); got != "/implement-prd prd/a/b.md" {
		t.Errorf("Standard-Template: %q", got)
	}
	if got := Get(Nuxt).CreateDisplay("Avatar"); got != "/create-prd Avatar" {
		t.Errorf("Anzeige Nuxt: %q", got)
	}
	simple := Get(Simple)
	if got := simple.CreateDisplay("Avatar"); got != "Avatar" {
		t.Errorf("Anzeige einfach: %q", got)
	}
	create := simple.CreatePrompt("Avatar")
	impl := simple.ImplementPrompt("prd/a/b.md")
	if !strings.Contains(create, "\nAvatar\n") || !strings.Contains(create, "Template: einfach") ||
		!strings.HasPrefix(impl, "# PRD umsetzen (Template „einfach“): prd/a/b.md\n") ||
		strings.Contains(create+impl, "$ARGUMENTS") {
		t.Errorf("Prompts einfach:\n%s\n---\n%s", create, impl)
	}
}

func TestDefault(t *testing.T) {
	if Default("nuxt-drizzle") != Nuxt || Default("") != Simple {
		t.Error("Default wählt das falsche Template")
	}
}
