package tui

import (
	"testing"

	"lazydev/internal/config"
)

// TestSyncProjects checks that the TUI takes projects that the CLI added or removed, keeps the
// entries of the others and stays on the selected project.
func TestSyncProjects(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, b := &projectEntry{path: "/p/a"}, &projectEntry{path: "/p/b"}
	m := model{projects: []*projectEntry{a, b}, selected: 1}
	if err := config.SaveProjects([]string{"/p/b", "/p/c"}); err != nil {
		t.Fatal(err)
	}
	m.syncProjects()
	if len(m.projects) != 2 || m.projects[0] != b || m.projects[1].path != "/p/c" {
		t.Fatalf("projects = %v", projectPaths(m.projects))
	}
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0 (still /p/b)", m.selected)
	}
	if _, err := config.AddProject("/p/d"); err != nil {
		t.Fatal(err)
	}
	m.syncProjects()
	if got := projectPaths(m.projects); len(got) != 3 || got[2] != "/p/d" {
		t.Errorf("after AddProject: %v", got)
	}
}
