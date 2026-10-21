package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/terminal"
)

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

var (
	altG     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g"), Alt: true}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
)

// press sends keys to m one after the other.
func press(t *testing.T, m model, keys ...tea.KeyMsg) model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(model)
	}
	return m
}

// cockpitModel is a model with one project in a git repository, like TestViewSmoke.
func cockpitModel(t *testing.T) model {
	t.Helper()
	isolateTUI(t)
	dir := newGitRepo(t)
	p := &projectEntry{path: dir}
	p.git = readGitInfo(dir)
	return model{width: 160, height: 40, focus: colLeft, projects: []*projectEntry{p}, gitTabs: map[int]string{}}
}

func TestCockpitOpens(t *testing.T) {
	m := press(t, cockpitModel(t), altG)
	if m.cockpit == nil {
		t.Fatalf("alt+g opened no cockpit, err %q", m.err)
	}
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"Git · shop", "Branch    main · no upstream", "Changes   clean", "Recent commits", "init",
		"Fetch     f", "Pull      p", "Push      P", "Commit    c", "j/k move · enter run"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	checkSize(t, view, 160, 40)
}

func TestCockpitCountsChanges(t *testing.T) {
	m := cockpitModel(t)
	dir := m.projects[0].path
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644)
	m = press(t, m, altG)
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Changes   2 files") {
		t.Errorf("view:\n%s", plain)
	}
}

func TestCockpitCloses(t *testing.T) {
	for _, k := range []tea.KeyMsg{keyEsc, keyRunes("q"), altG} {
		m := press(t, cockpitModel(t), altG, k)
		if m.cockpit != nil {
			t.Errorf("%s left the cockpit open", k)
		}
	}
}

func TestCockpitCursor(t *testing.T) {
	m := press(t, cockpitModel(t), altG, keyRunes("k"))
	if m.cockpit.cursor != 0 {
		t.Errorf("cursor after k at the top: %d", m.cockpit.cursor)
	}
	m = press(t, m, keyRunes("j"), keyRunes("j"), keyRunes("j"), keyRunes("j"), keyRunes("j"))
	if m.cockpit.cursor != len(cockpitActions)-1 {
		t.Errorf("cursor after j past the end: %d", m.cockpit.cursor)
	}
	m = press(t, m, keyRunes("k"))
	if m.cockpit.cursor != len(cockpitActions)-2 {
		t.Errorf("cursor after k: %d", m.cockpit.cursor)
	}
}

func TestCockpitCommitInput(t *testing.T) {
	m := press(t, cockpitModel(t), altG, keyRunes("c"))
	if m.cockpit == nil || !m.cockpit.committing {
		t.Fatal("c opened no input")
	}
	m = press(t, m, keyRunes("f"), keyRunes("q"))
	if m.cockpit == nil || m.cockpit.input.Value() != "fq" {
		t.Fatalf("keys did not go into the input: %+v", m.cockpit)
	}
	if n := len(m.projects[0].tabs); n != 0 {
		t.Errorf("f started an action: %d tabs", n)
	}
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Message: fq") || strings.Contains(plain, "Fetch     f") || !strings.Contains(plain, "enter commit · esc back") {
		t.Errorf("view with input:\n%s", plain)
	}
	m = press(t, m, keyEsc)
	if m.cockpit == nil || m.cockpit.committing {
		t.Errorf("esc did not go back to the list: %+v", m.cockpit)
	}
}

func TestCockpitEmptyMessage(t *testing.T) {
	m := press(t, cockpitModel(t), altG, keyRunes("c"), keyRunes(" "), keyEnter)
	if m.err != "Enter a commit message." {
		t.Errorf("err: %q", m.err)
	}
	if m.cockpit == nil || !m.cockpit.committing {
		t.Error("cockpit closed")
	}
	if n := len(m.projects[0].tabs); n != 0 {
		t.Errorf("tabs: %d", n)
	}
}

func TestCockpitWithoutGit(t *testing.T) {
	isolateTUI(t)
	dir := filepath.Join(t.TempDir(), "plain")
	os.MkdirAll(dir, 0o755)
	m := model{width: 160, height: 40, focus: colLeft, projects: []*projectEntry{{path: dir}}, gitTabs: map[int]string{}}
	m = press(t, m, altG)
	if m.cockpit != nil || m.err != "plain is no git repository." {
		t.Errorf("cockpit %v, err %q", m.cockpit, m.err)
	}
}

func TestCockpitWithoutProject(t *testing.T) {
	isolateTUI(t)
	m := press(t, model{width: 160, height: 40, focus: colLeft, gitTabs: map[int]string{}}, altG)
	if m.cockpit != nil || m.err != "Add a folder first: a in the left column" {
		t.Errorf("cockpit %v, err %q", m.cockpit, m.err)
	}
}

func TestPlainGOpensNoCockpit(t *testing.T) {
	m := cockpitModel(t)
	m.focus, m.bottomTab = colBottom, bottomDetails
	m = press(t, m, keyRunes("g"))
	if m.cockpit != nil {
		t.Error("plain g opened the cockpit")
	}
}

func TestGitTabExit(t *testing.T) {
	for _, tc := range []struct {
		code      int
		note, err string
	}{
		{0, "git push: done", ""},
		{2, "", "git push failed (exit 2)"},
		{-1, "", "git push stopped"},
	} {
		m := cockpitModel(t)
		m.gitTabs[5] = "push"
		next, cmd := m.Update(terminal.ExitMsg{ID: 5, Code: tc.code})
		m = next.(model)
		if m.note != tc.note || m.err != tc.err {
			t.Errorf("code %d: note %q, err %q", tc.code, m.note, m.err)
		}
		if _, ok := m.gitTabs[5]; ok {
			t.Errorf("code %d: tab still in gitTabs", tc.code)
		}
		if cmd == nil {
			t.Errorf("code %d: no git info refresh", tc.code)
		}
	}
}

func TestMenuOpensCockpit(t *testing.T) {
	m := cockpitModel(t)
	m.menu = &menu{}
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Git") {
		t.Errorf("menu lacks Git:\n%s", plain)
	}
	// General, Discord, Git.
	m = press(t, m, keyRunes("j"), keyRunes("j"), keyEnter)
	if m.menu != nil || m.cockpit == nil {
		t.Errorf("menu %v, cockpit %v, err %q", m.menu, m.cockpit, m.err)
	}
}
