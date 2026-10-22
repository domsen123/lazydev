package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/project"
	"lazydev/internal/state"
	"lazydev/internal/terminal"
)

var (
	altV = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v"), Alt: true}
	altW = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w"), Alt: true}
)

const readme = "# Titel\n\nSome **fett** text.\n\n| Name | Value |\n|---|---|\n| one | 1 |\n\n"

// longMarkdown has n numbered paragraphs, one screen line each.
func longMarkdown(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "Paragraph %d\n\n", i)
	}
	return b.String()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// viewModel is a model with one project that has a README.md, the right column on the Files tab
// and the focus on the left.
func viewModel(t *testing.T) (model, *projectEntry) {
	t.Helper()
	isolateTUI(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), readme+strings.Repeat("A long paragraph with many words. ", 30)+"\n")
	p := &projectEntry{path: dir}
	m := model{width: 160, height: 40, focus: colLeft, rightTab: rightFiles, projects: []*projectEntry{p}, bus: &sender{}, gitTabs: map[int]string{}}
	return m, p
}

// selectFile puts the cursor of the file tree on the row called name.
func selectFile(t *testing.T, p *projectEntry, name string) {
	t.Helper()
	tree := p.files()
	tree.rebuild()
	for i, r := range tree.rows {
		if r.name == name {
			tree.cursor = i
			return
		}
	}
	t.Fatalf("no row %q", name)
}

func countViews(p *projectEntry) int {
	n := 0
	for _, tab := range p.tabs {
		if _, ok := tab.(*mdView); ok {
			n++
		}
	}
	return n
}

// openLong opens a viewer on a file with 100 paragraphs and returns it.
func openLong(t *testing.T) (model, *projectEntry, *mdView) {
	t.Helper()
	m, p := viewModel(t)
	writeFile(t, filepath.Join(p.path, "LONG.md"), longMarkdown(100))
	selectFile(t, p, "LONG.md")
	m = press(t, m, altV)
	v := p.activeView()
	if v == nil {
		t.Fatalf("no viewer, err %q", m.err)
	}
	return m, p, v
}

func TestMDViewOpensFromFiles(t *testing.T) {
	m, p := viewModel(t)
	selectFile(t, p, "README.md")
	m = press(t, m, altV)
	v := p.activeView()
	if v == nil {
		t.Fatalf("no viewer, err %q", m.err)
	}
	if v.TabTitle() != "md README.md" || m.focus != colCenter || v.run != "" {
		t.Errorf("title %q, focus %d, run %q", v.TabTitle(), m.focus, v.run)
	}
	plain := ansi.Strip(v.View(true))
	for _, want := range []string{"Titel", "fett", "Name", "one"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	for _, raw := range []string{"**", "# Titel", "|---"} {
		if strings.Contains(plain, raw) {
			t.Errorf("view shows raw %q:\n%s", raw, plain)
		}
	}
	w, h := m.termSize()
	checkSize(t, v.View(true), w, h)
	checkSize(t, m.View(), 160, 40)
}

func TestMDViewPlainV(t *testing.T) {
	m, p := viewModel(t)
	selectFile(t, p, "README.md")
	m.focus = colRight
	m = press(t, m, keyRunes("v"))
	if p.activeView() == nil || m.focus != colCenter {
		t.Fatalf("v opened no viewer, err %q", m.err)
	}
}

func TestMDViewOpensPRD(t *testing.T) {
	m, p := viewModel(t)
	m.rightTab = rightPRDs
	writeFile(t, filepath.Join(p.path, "prd", "checkout.md"), "# Main\n")
	p.prds = []project.PRD{{Path: "prd/checkout.md", Root: p.path, Name: "checkout"}}
	m = press(t, m, altV)
	v := p.activeView()
	if v == nil || v.path != filepath.Join(p.path, "prd", "checkout.md") || v.run != "" {
		t.Fatalf("viewer %+v, err %q", v, m.err)
	}

	worktree := t.TempDir()
	writeFile(t, filepath.Join(worktree, "prd", "checkout.md"), "# Worktree\n")
	p.prds = []project.PRD{{Path: "prd/checkout.md", Root: worktree, Name: "checkout"}}
	m = press(t, m, altV)
	v = p.activeView()
	if v == nil || v.path != filepath.Join(worktree, "prd", "checkout.md") || v.run != "checkout" {
		t.Fatalf("viewer %+v, err %q", v, m.err)
	}
	if plain := ansi.Strip(v.View(true)); !strings.Contains(plain, "Worktree") {
		t.Errorf("view:\n%s", plain)
	}
	if n := countViews(p); n != 2 {
		t.Errorf("viewers: %d", n)
	}
}

func TestMDViewErrors(t *testing.T) {
	m, p := viewModel(t)
	writeFile(t, filepath.Join(p.path, "docs", "a.md"), "a")
	writeFile(t, filepath.Join(p.path, "main.go"), "package main")
	writeFile(t, filepath.Join(p.path, "NOTES.MD"), "# Notes")
	secret := filepath.Join(p.path, "secret.md")
	writeFile(t, secret, "# Secret")
	if err := os.Chmod(secret, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(secret, 0o644) })

	check := func(name, want string) {
		t.Helper()
		selectFile(t, p, name)
		n := len(p.tabs)
		m = press(t, m, altV)
		if m.err != want || len(p.tabs) != n {
			t.Errorf("%s: err %q, tabs %d → %d", name, m.err, n, len(p.tabs))
		}
	}
	check("docs", "Select a Markdown file in the Files tab.")
	check("main.go", "main.go is no Markdown file.")
	if os.Geteuid() != 0 {
		selectFile(t, p, "secret.md")
		m = press(t, m, altV)
		if !strings.HasPrefix(m.err, "Cannot read secret.md: ") || len(p.tabs) != 0 {
			t.Errorf("secret.md: err %q, tabs %d", m.err, len(p.tabs))
		}
	}
	selectFile(t, p, "NOTES.MD")
	m = press(t, m, altV)
	if v := p.activeView(); v == nil || v.TabTitle() != "md NOTES.MD" {
		t.Errorf("NOTES.MD opened no viewer, err %q", m.err)
	}

	// The group row of the done PRDs.
	m.rightTab = rightPRDs
	p.prds = []project.PRD{{Path: "prd/old.md", Root: p.path, Name: "old", Status: project.Status{Kind: project.StatusImplemented}}}
	p.prdCursor = 0
	n := len(p.tabs)
	m = press(t, m, altV)
	if m.err != "Select a PRD in the PRDs tab." || len(p.tabs) != n {
		t.Errorf("group row: err %q, tabs %d", m.err, len(p.tabs))
	}

	m.projects = nil
	m = press(t, m, altV)
	if m.err != "Add a folder first: a in the left column" {
		t.Errorf("without project: err %q", m.err)
	}
}

func TestMDViewFocusesExisting(t *testing.T) {
	m, p := viewModel(t)
	selectFile(t, p, "README.md")
	m = press(t, m, altV)
	p.tabs = append(p.tabs, newLogView(99, "p", "checkout"))
	p.tab = len(p.tabs) - 1
	m.focus = colRight
	m = press(t, m, altV)
	if n := countViews(p); n != 1 {
		t.Fatalf("viewers: %d", n)
	}
	if p.activeView() == nil || m.focus != colCenter {
		t.Errorf("active tab %v, focus %d", p.tabs[p.tab], m.focus)
	}
}

func TestMDViewScroll(t *testing.T) {
	m, _, v := openLong(t)
	if v.top != 0 || v.maxTop() == 0 {
		t.Fatalf("top %d, maxTop %d", v.top, v.maxTop())
	}
	m = press(t, m, keyRunes("j"))
	if v.top != 1 {
		t.Errorf("j: top %d", v.top)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if v.top != 1+v.height/2 {
		t.Errorf("ctrl+d: top %d", v.top)
	}
	m = press(t, m, keyRunes("G"))
	if v.top != v.maxTop() {
		t.Errorf("G: top %d, maxTop %d", v.top, v.maxTop())
	}
	m = press(t, m, keyRunes("j"))
	if v.top != v.maxTop() {
		t.Errorf("j at the end: top %d", v.top)
	}
	if plain := ansi.Strip(v.View(true)); !strings.Contains(plain, "Paragraph 99") {
		t.Errorf("end not visible:\n%s", plain)
	}
	m = press(t, m, keyRunes("g"), keyRunes("k"))
	if v.top != 0 {
		t.Errorf("g: top %d", v.top)
	}
}

func TestMDViewEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	t.Setenv("VISUAL", "")
	m, p, v := openLong(t)
	m = press(t, m, keyRunes("e"))
	var term *terminal.Terminal
	for _, tab := range p.tabs {
		if tt, ok := tab.(*terminal.Terminal); ok {
			term = tt
		}
	}
	if term == nil {
		t.Fatalf("e opened no terminal, err %q", m.err)
	}
	t.Cleanup(func() {
		term.Hangup()
		<-term.Done()
		term.Release()
	})
	if countViews(p) != 1 || p.tabs[0] != v || m.focus != colCenter {
		t.Errorf("tabs %v, focus %d", p.tabs, m.focus)
	}
}

func TestMDViewRefresh(t *testing.T) {
	m, p, v := openLong(t)
	m = press(t, m, keyRunes("j"), keyRunes("j"))
	cmd := m.readViews()
	if cmd == nil {
		t.Fatal("no check")
	}
	msg := cmd().(mdReadMsg)
	if r := msg.byTab[v.id]; r.changed || r.err != nil {
		t.Fatalf("unchanged file: %+v", r)
	}
	path := filepath.Join(p.path, "LONG.md")
	writeFile(t, path, "Changed first\n\n"+longMarkdown(100))
	later := time.Now().Add(time.Hour)
	os.Chtimes(path, later, later)
	next, _ := m.Update(m.readViews()())
	m = next.(model)
	if v.top != 2 {
		t.Errorf("top %d", v.top)
	}
	if !strings.Contains(v.text, "Changed first") {
		t.Errorf("text not read again")
	}
	if !strings.Contains(ansi.Strip(strings.Join(v.body(), "\n")), "Changed first") {
		t.Errorf("body not rendered again")
	}

	// A shorter file keeps top within the new end.
	writeFile(t, path, "# Short\n")
	os.Chtimes(path, later.Add(time.Hour), later.Add(time.Hour))
	next, _ = m.Update(m.readViews()())
	m = next.(model)
	if v.top != v.maxTop() {
		t.Errorf("top %d, maxTop %d", v.top, v.maxTop())
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	next, _ = m.Update(m.readViews()())
	m = next.(model)
	if countViews(p) != 0 || m.note != "LONG.md is gone, its viewer closed." {
		t.Errorf("viewers %d, note %q", countViews(p), m.note)
	}
	if m.readViews() != nil {
		t.Error("check without viewers")
	}
}

func TestMDViewGoneNote(t *testing.T) {
	m, p := viewModel(t)
	selectFile(t, p, "README.md")
	m = press(t, m, altV)
	os.Remove(filepath.Join(p.path, "README.md"))
	next, _ := m.Update(m.readViews()())
	m = next.(model)
	if countViews(p) != 0 || m.note != "README.md is gone, its viewer closed." {
		t.Errorf("viewers %d, note %q", countViews(p), m.note)
	}
}

func TestMDViewClosesWithRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		runs map[string]*state.Run
	}{
		{"merged", map[string]*state.Run{"checkout": {Phase: project.PhaseMerged}}},
		{"discarded", map[string]*state.Run{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, p := viewModel(t)
			id := projectIDs(m.projects)[0]
			selectFile(t, p, "README.md")
			m = press(t, m, altV)
			worktree := t.TempDir()
			writeFile(t, filepath.Join(worktree, "prd", "checkout.md"), "# Worktree\n")
			p.prds = []project.PRD{{Path: "prd/checkout.md", Root: worktree, Name: "checkout"}}
			m.rightTab = rightPRDs
			m = press(t, m, altV)
			if countViews(p) != 2 || p.activeView().run != "checkout" {
				t.Fatalf("viewers %d, err %q", countViews(p), m.err)
			}
			m.Update(runsMsg{byID: map[string]map[string]*state.Run{id: {"checkout": {Phase: project.PhaseAborted}}}})
			if countViews(p) != 2 {
				t.Fatal("an aborted run closed its viewer")
			}
			m.Update(runsMsg{byID: map[string]map[string]*state.Run{id: tc.runs}})
			if countViews(p) != 1 || p.tabs[0].(*mdView).run != "" {
				t.Errorf("viewers %d", countViews(p))
			}
		})
	}
}

func TestMDViewCloseKey(t *testing.T) {
	m, p := viewModel(t)
	selectFile(t, p, "README.md")
	m = press(t, m, altV, altW)
	if len(p.tabs) != 0 {
		t.Errorf("tabs %d", len(p.tabs))
	}
}

func TestMDViewTheme(t *testing.T) {
	tokyo := slices.IndexFunc(themes, func(th theme) bool { return th.name == "Tokyo Night" })
	if !reflect.DeepEqual(mdStyle(tokyo), styles.TokyoNightStyleConfig) {
		t.Error("Tokyo Night has not the Tokyo Night style")
	}
	for _, i := range []int{0, -1, len(themes)} {
		if !reflect.DeepEqual(mdStyle(i), styles.DarkStyleConfig) {
			t.Errorf("theme %d has not the dark style", i)
		}
	}
	_, _, v := openLong(t)
	v.theme = 0
	dark := strings.Join(v.body(), "\n")
	v.theme = tokyo
	if v.body(); v.cacheTheme != tokyo || strings.Join(v.lines, "\n") == dark {
		t.Errorf("theme change kept the cache")
	}
}

func TestMDViewMouse(t *testing.T) {
	m, _, v := openLong(t)
	x, y := m.termOrigin()
	w, h := m.termSize()
	m, _ = mouse(t, m, x, y, tea.MouseButtonWheelDown, tea.MouseActionPress)
	if v.top != wheelLines {
		t.Fatalf("wheel down: top %d", v.top)
	}
	m, _ = mouse(t, m, x+w, y, tea.MouseButtonWheelDown, tea.MouseActionPress)
	m, _ = mouse(t, m, x, y+h, tea.MouseButtonWheelDown, tea.MouseActionPress)
	if v.top != wheelLines {
		t.Fatalf("wheel outside: top %d", v.top)
	}
	m.menu = &menu{}
	m, _ = mouse(t, m, x, y, tea.MouseButtonWheelDown, tea.MouseActionPress)
	if v.top != wheelLines {
		t.Fatalf("wheel with menu: top %d", v.top)
	}
	m.menu = nil
	m, _ = mouse(t, m, x, y, tea.MouseButtonWheelUp, tea.MouseActionPress)
	if v.top != 0 {
		t.Fatalf("wheel up: top %d", v.top)
	}
	m.focus = colLeft
	m, _ = mouse(t, m, x, y, tea.MouseButtonLeft, tea.MouseActionPress)
	if m.focus != colCenter {
		t.Errorf("click: focus %d", m.focus)
	}
}

func TestMDViewFooterAndCopy(t *testing.T) {
	m, p := viewModel(t)
	selectFile(t, p, "README.md")
	m.focus = colRight
	if keys := m.footerKeys(); !strings.Contains(keys, "l open · v view") {
		t.Errorf("Files footer: %q", keys)
	}
	m = press(t, m, altV)
	if keys := m.footerKeys(); !strings.Contains(keys, "e edit") {
		t.Errorf("viewer footer: %q", keys)
	}
	m = press(t, m, altC)
	if m.err != "The active tab is not a terminal." {
		t.Errorf("alt+c: err %q", m.err)
	}
}
