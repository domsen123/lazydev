package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazydev/internal/project"
	"lazydev/internal/repo"
)

// TestUnblockGerman checks that f in a German project opens the dialog with the German contract
// and the German agents.
func TestUnblockGerman(t *testing.T) {
	m, p, _ := blockedModel(t)
	if err := os.MkdirAll(filepath.Join(p.path, repo.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo.ConfigPath(p.path), []byte("language: de\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := fakeClaude(t)
	m = press(t, m, keyRunes("f"))
	hangupAll(t, p)
	if m.err != "" {
		t.Fatalf("err: %q", m.err)
	}
	var got string
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(got, "--agents") {
		if time.Now().After(deadline) {
			t.Fatalf("claude output: %q", got)
		}
		time.Sleep(10 * time.Millisecond)
		data, _ := os.ReadFile(out)
		got = string(data)
	}
	for _, s := range []string{"## Regeln von lazydev", "Kläre die Blocker", "Du committest"} {
		if !strings.Contains(got, s) {
			t.Errorf("German dialog lacks %q:\n%s", s, got)
		}
	}
}

// TestStatusPickerWritesProjectLanguage checks that c writes the value in the language of the
// project, whatever language the PRD had.
func TestStatusPickerWritesProjectLanguage(t *testing.T) {
	for _, tt := range []struct{ config, want string }{
		{"", "> Status: implemented (" + time.Now().Format("2006-01-02") + ") · Scope: a"},
		{"language: de\n", "> Status: umgesetzt (" + time.Now().Format("2006-01-02") + ") · Scope: a"},
	} {
		m := cockpitModel(t)
		p := m.projects[0]
		if err := os.MkdirAll(filepath.Join(p.path, repo.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(repo.ConfigPath(p.path), []byte(tt.config), 0o644)
		rel := "prd/a/x.md"
		os.MkdirAll(filepath.Join(p.path, "prd/a"), 0o755)
		os.WriteFile(filepath.Join(p.path, rel), []byte("# PRD: X\n\n> Status: geplant · Scope: a\n"), 0o644)
		m.readLazydev()
		d, err := project.Read(p.path, filepath.Join(p.path, rel))
		if err != nil {
			t.Fatal(err)
		}
		p.prds = []project.PRD{d}
		m.focus, m.rightTab = colRight, rightPRDs
		m.openStatusPicker()
		if m.status == nil {
			t.Fatalf("picker did not open: %s", m.err)
		}
		m.status.cursor = 1
		if msg := changeStatus(m.status)().(actionMsg); msg.err != nil {
			t.Fatal(msg.err)
		}
		if got, _ := os.ReadFile(filepath.Join(p.path, rel)); !strings.Contains(string(got), tt.want) {
			t.Errorf("%q: PRD = %q", tt.config, got)
		}
	}
}
