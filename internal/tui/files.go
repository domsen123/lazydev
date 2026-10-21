package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/fsutil"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	rightFiles = iota
	rightPRDs
)

// rightTabs are the tabs of the right column.
var rightTabs = []string{"Files", "PRDs"}

// hiddenEntries never show up in the file tree.
var hiddenEntries = []string{".git"}

// treeRow is one visible line of the file tree.
type treeRow struct {
	path  string
	name  string
	depth int
	dir   bool
}

// fileTree is the file tree of a project. Folders load when they get expanded, and rows are only
// rebuilt on changes, because the view renders far more often than the tree changes.
type fileTree struct {
	root     string
	expanded map[string]bool
	rows     []treeRow
	cursor   int
	// top is the first visible row.
	top int
}

func newFileTree(root string) *fileTree {
	t := &fileTree{root: root, expanded: map[string]bool{}}
	t.rebuild()
	return t
}

// rebuild reads the expanded folders again and keeps the cursor on the same path if it still
// exists.
func (t *fileTree) rebuild() {
	var current string
	if t.cursor < len(t.rows) {
		current = t.rows[t.cursor].path
	}
	t.rows = t.rows[:0]
	t.addDir(t.root, 0)
	if i := slices.IndexFunc(t.rows, func(r treeRow) bool { return r.path == current }); i >= 0 {
		t.cursor = i
	}
	t.cursor = clamp(t.cursor, len(t.rows))
}

// addDir appends the entries of dir: folders first, then files, each sorted by name.
func (t *fileTree) addDir(dir string, depth int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var dirs, files []treeRow
	for _, e := range entries {
		if slices.Contains(hiddenEntries, e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		row := treeRow{path: path, name: e.Name(), depth: depth, dir: fsutil.IsDir(path)}
		if row.dir {
			dirs = append(dirs, row)
		} else {
			files = append(files, row)
		}
	}
	for _, r := range dirs {
		t.rows = append(t.rows, r)
		if t.expanded[r.path] {
			t.addDir(r.path, depth+1)
		}
	}
	t.rows = append(t.rows, files...)
}

func (t *fileTree) move(delta int) {
	t.cursor = clamp(t.cursor+delta, len(t.rows))
}

// selected is the row under the cursor, ok is false for an empty tree.
func (t *fileTree) selected() (treeRow, bool) {
	if t.cursor >= len(t.rows) {
		return treeRow{}, false
	}
	return t.rows[t.cursor], true
}

// toggle expands or collapses the folder under the cursor.
func (t *fileTree) toggle() {
	if r, ok := t.selected(); ok && r.dir {
		t.expanded[r.path] = !t.expanded[r.path]
		t.rebuild()
	}
}

// collapse closes the folder under the cursor, or jumps to the parent folder.
func (t *fileTree) collapse() {
	r, ok := t.selected()
	if !ok {
		return
	}
	if r.dir && t.expanded[r.path] {
		t.toggle()
		return
	}
	parent := filepath.Dir(r.path)
	if i := slices.IndexFunc(t.rows, func(row treeRow) bool { return row.path == parent }); i >= 0 {
		t.cursor = i
	}
}

// view renders height rows of width cells. While focused, the cursor row gets a background.
func (t *fileTree) view(width, height int, focused bool, th theme) string {
	text := lipgloss.NewStyle().Foreground(th.text)
	if len(t.rows) == 0 {
		return text.Render("Empty folder.")
	}
	// Keep the cursor visible.
	if t.cursor < t.top {
		t.top = t.cursor
	}
	if t.cursor >= t.top+height {
		t.top = t.cursor - height + 1
	}
	t.top = max(0, min(t.top, len(t.rows)-height))
	var lines []string
	for i := t.top; i < min(t.top+height, len(t.rows)); i++ {
		r := t.rows[i]
		icon := "  "
		style := text
		if r.dir {
			icon = "▸ "
			if t.expanded[r.path] {
				icon = "▾ "
			}
			style = text.Bold(true).Foreground(th.title)
		}
		line := ansi.Truncate(" "+strings.Repeat("  ", r.depth)+icon+r.name, width, "…")
		line += strings.Repeat(" ", max(0, width-ansi.StringWidth(line)))
		if i == t.cursor && focused {
			style = style.Foreground(th.selectedFg).Background(th.selectedBg)
		}
		lines = append(lines, style.Render(line))
	}
	return strings.Join(lines, "\n")
}

// clamp keeps i within 0..n-1, 0 for n == 0.
func clamp(i, n int) int {
	return max(0, min(i, n-1))
}

// editorCommand opens path in $VISUAL, $EDITOR or vi.
func editorCommand(path string) []string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if args := strings.Fields(os.Getenv(name)); len(args) > 0 {
			return append(args, path)
		}
	}
	return []string{"vi", path}
}
