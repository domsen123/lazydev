package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/docker"
)

func plainStyle(int) lipgloss.Style { return lipgloss.NewStyle() }

func TestLayoutTabsFits(t *testing.T) {
	l := layoutTabs([]int{5, 7, 3}, 1, 100)
	if l != (tabLayout{first: 0, last: 2, limit: tabMaxLabel}) {
		t.Errorf("layout: %+v", l)
	}
}

func TestLayoutTabsShrinks(t *testing.T) {
	// Three tabs at limit L need 3*(L+3)-1 cells, so 50 cells allow L = 14.
	l := layoutTabs([]int{30, 30, 30}, 0, 50)
	if l != (tabLayout{first: 0, last: 2, limit: 14}) {
		t.Errorf("layout: %+v", l)
	}
	if w := barWidth([]int{30, 30, 30}, 0, 2, l.limit); w > 50 {
		t.Errorf("bar width %d", w)
	}
}

func TestLayoutTabsScrolls(t *testing.T) {
	widths := make([]int, 10)
	for i := range widths {
		widths[i] = 20
	}
	l := layoutTabs(widths, 5, 40)
	if l != (tabLayout{first: 4, last: 6, limit: tabMinLabel}) {
		t.Errorf("layout: %+v", l)
	}
	if w := barWidth(widths, l.first, l.last, l.limit) + markerWidth(l.first, l.last, len(widths)); w > 40 {
		t.Errorf("window width %d", w)
	}
	// At the start, the window only grows to the right.
	if l := layoutTabs(widths, 0, 40); l.first != 0 || l.last < 1 {
		t.Errorf("layout at start: %+v", l)
	}
	// At the end, it only grows to the left.
	if l := layoutTabs(widths, 9, 40); l.last != 9 || l.first > 8 {
		t.Errorf("layout at end: %+v", l)
	}
}

func TestMarkerWidth(t *testing.T) {
	for _, tc := range []struct{ first, last, n, want int }{
		{0, 9, 10, 0},
		{3, 9, 10, 3},
		{0, 5, 10, 3},
		{12, 13, 20, 4 + 3},
	} {
		if got := markerWidth(tc.first, tc.last, tc.n); got != tc.want {
			t.Errorf("markerWidth(%d, %d, %d) = %d, want %d", tc.first, tc.last, tc.n, got, tc.want)
		}
	}
}

func TestRenderTabs(t *testing.T) {
	got := ansi.Strip(renderTabs([]string{"Files", "PRDs"}, 0, 40, plainStyle, lipgloss.NewStyle()))
	if got != " Files   PRDs " {
		t.Errorf("fitting bar: %q", got)
	}

	labels := make([]string, 10)
	for i := range labels {
		labels[i] = fmt.Sprintf("%d long-title-%d", i+1, i)
	}
	bar := renderTabs(labels, 5, 40, plainStyle, lipgloss.NewStyle())
	plain := ansi.Strip(bar)
	if !strings.HasPrefix(plain, "‹4 ") || !strings.HasSuffix(plain, " 3›") {
		t.Errorf("scrolling bar: %q", plain)
	}
	if !strings.Contains(plain, "6 lon…") || strings.Contains(plain, "1 lon") || strings.Contains(plain, "8 lon") {
		t.Errorf("visible tabs: %q", plain)
	}
	if w := ansi.StringWidth(bar); w > 40 {
		t.Errorf("bar width %d: %q", w, plain)
	}
}

func TestColumns(t *testing.T) {
	for _, tc := range []struct{ width, side, center int }{
		{160, 32, 96},
		{100, 20, 60},
		{50, 15, 20},
		{10, 0, 10},
	} {
		side, center := columns(tc.width)
		if side != tc.side || center != tc.center {
			t.Errorf("columns(%d) = %d, %d, want %d, %d", tc.width, side, center, tc.side, tc.center)
		}
		if 2*side+center != tc.width {
			t.Errorf("columns(%d) do not add up", tc.width)
		}
	}
}

func TestRows(t *testing.T) {
	for _, tc := range []struct {
		height      int
		zoom        bool
		top, bottom int
	}{
		{39, false, 30, 9},
		{39, true, 13, 26},
		{100, false, 75, 25},
		{10, false, 5, 5},
		{10, true, 4, 6},
	} {
		top, bottom := model{bottomZoom: tc.zoom}.rows(tc.height)
		if top != tc.top || bottom != tc.bottom {
			t.Errorf("rows(%d, zoom %v) = %d, %d, want %d, %d", tc.height, tc.zoom, top, bottom, tc.top, tc.bottom)
		}
	}
}

func TestOverlay(t *testing.T) {
	bg := strings.Repeat("..........\n", 4) + ".........."
	got := strings.Split(overlay(bg, "ab\ncd", 10, 5), "\n")
	want := []string{"..........", "....ab....", "....cd....", "..........", ".........."}
	if len(got) != len(want) {
		t.Fatalf("lines: %q", got)
	}
	for i := range want {
		if ansi.Strip(got[i]) != want[i] {
			t.Errorf("line %d: %q, want %q", i, ansi.Strip(got[i]), want[i])
		}
	}
	// A foreground taller than the background is cut.
	if lines := strings.Split(overlay("ab\ncd", "1\n2\n3\n4", 2, 2), "\n"); len(lines) != 2 {
		t.Errorf("tall foreground: %q", lines)
	}
	// Short background lines get padded up to the foreground.
	if got := ansi.Strip(overlay("x", "ab", 6, 1)); got != "x ab" {
		t.Errorf("short line: %q", got)
	}
}

func TestRenderColumns(t *testing.T) {
	th := themes[0]
	rows := func(n, w int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = strings.Repeat(fmt.Sprint(i), w)
		}
		return out
	}
	cols := []column{
		{title: "Alpha", max: 20, rows: rows(5, 20)},
		{title: "Beta", max: 10, rows: rows(1, 10)},
		{title: "Gamma", max: 10, rows: rows(1, 10)},
	}
	got := ansi.Strip(renderColumns(cols, 30, 3, th))
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("lines: %q", lines)
	}
	if !strings.HasPrefix(lines[0], " Alpha") || !strings.Contains(lines[0], "Beta") || strings.Contains(got, "Gamma") {
		t.Errorf("columns: %q", lines)
	}
	// Two rows fit below the title: the first row and the fold.
	if !strings.Contains(lines[1], strings.Repeat("0", 20)) || !strings.Contains(lines[2], "+4 more") {
		t.Errorf("fold: %q", lines)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 30 {
			t.Errorf("line %d too wide: %d", i, w)
		}
	}
}

func TestRenderColumnsKeepsInput(t *testing.T) {
	cols := []column{{title: "A", max: 20, rows: []string{"one", "two", "three", "four"}}}
	renderColumns(cols, 40, 3, themes[0])
	if cols[0].rows[1] != "two" {
		t.Errorf("renderColumns changed its input, row 1 is %q", cols[0].rows[1])
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
	} {
		t.Setenv(k, v)
	}
	dir := filepath.Join(t.TempDir(), "shop")
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# shop\n"), 0o644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	return dir
}

func isolateTUI(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

func checkSize(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Errorf("view has %d lines, want %d", len(lines), height)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > width {
			t.Errorf("line %d is %d wide: %q", i, w, ansi.Strip(l))
		}
	}
}

func TestViewSmoke(t *testing.T) {
	isolateTUI(t)
	dir := newGitRepo(t)
	p := &projectEntry{path: dir}
	p.git = readGitInfo(dir)
	m := model{width: 160, height: 40, focus: colLeft, projects: []*projectEntry{p}}
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"Projects", "Explorer", "Details", "Docker", "Files", "PRDs", "shop", "main", "README.md", "src"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	checkSize(t, view, 160, 40)

	m.bottomTab = bottomDocker
	plain = ansi.Strip(m.View())
	if !strings.Contains(plain, "No containers and no compose file.") {
		t.Errorf("docker tab:\n%s", plain)
	}
	m.docker = docker.Info{
		Services:   map[string][]string{dir: {"db"}},
		Containers: []docker.Container{{Names: "shop-db-1", State: "running", Status: "Up 1 hour", Image: "postgres:16", Ports: "0.0.0.0:5435->5432/tcp", Labels: "com.docker.compose.project.working_dir=" + dir + ",com.docker.compose.service=db"}},
	}
	plain = ansi.Strip(m.View())
	for _, want := range []string{"db", "Up 1 hour", ":5435", "postgres:16"} {
		if !strings.Contains(plain, want) {
			t.Errorf("docker tab lacks %q:\n%s", want, plain)
		}
	}

	m.bottomZoom, m.focus, m.rightTab = true, colRight, rightPRDs
	checkSize(t, m.View(), 160, 40)
}

func TestViewWithoutProjects(t *testing.T) {
	isolateTUI(t)
	if got := (model{}).View(); got != "" {
		t.Errorf("view without size: %q", got)
	}
	m := model{width: 80, height: 24, focus: colLeft}
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"Projects", "Explorer", "No folders yet.", "No project selected."} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	checkSize(t, view, 80, 24)
}

func TestViewNarrow(t *testing.T) {
	isolateTUI(t)
	dir := newGitRepo(t)
	m := model{width: 30, height: 12, focus: colCenter, projects: []*projectEntry{{path: dir}}}
	// A tiny terminal must not panic.
	m.View()
	m.width, m.height = 5, 3
	m.View()
}
