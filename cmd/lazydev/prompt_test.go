package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/repo"
)

// capture runs f and returns what it printed on stdout.
func capture(t *testing.T, f func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	ferr := f()
	os.Stdout = old
	w.Close()
	out, _ := io.ReadAll(r)
	if ferr != nil {
		t.Fatal(ferr)
	}
	return string(out)
}

// TestPromptCreateLanguage checks that "lazydev prompt create" builds the prompt in the language of
// the project, with and without --auto.
func TestPromptCreateLanguage(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, tt := range []struct {
		config string
		want   []string
	}{
		{"", []string{"# Write a PRD", "## Rules of lazydev", "The task follows from the conversation with the user so far.", "## Autonomous mode"}},
		{"language: de\n", []string{"# PRD erstellen", "## Regeln von lazydev", "Die Aufgabe ergibt sich aus dem bisherigen Gespräch mit dem Nutzer.", "## Autonomer Modus"}},
	} {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, repo.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(repo.ConfigPath(dir), []byte(tt.config), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := repo.Init(dir); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		got := capture(t, func() error { return promptCreate([]string{"--auto"}) })
		for _, s := range tt.want {
			if !strings.Contains(got, s) {
				t.Errorf("%q: prompt lacks %q:\n%s", tt.config, s, got)
			}
		}
	}
}
