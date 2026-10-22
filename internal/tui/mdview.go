package tui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazydev/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/x/ansi"
)

// A viewer tab shows a Markdown file rendered with Glamour in the center column. It runs no
// process. Every second it checks the file and reads it again once it changed.

// mdView is a center tab with the rendered Markdown file at path.
type mdView struct {
	id   int
	path string
	// run is the PRD name if the file comes from the worktree of a run, empty otherwise. The tab
	// closes with that run.
	run           string
	width, height int
	// text is the content of the file, modTime and size tell when it changes.
	text    string
	modTime time.Time
	size    int64
	// top is the first visible line of the rendered body.
	top int
	// theme is the theme of the model, set before each render.
	theme int
	// lines caches the rendered body for cacheWidth and cacheTheme until the text changes.
	lines      []string
	cacheWidth int
	cacheTheme int
	dirty      bool
}

func newMDView(id int, path, run, text string, info os.FileInfo) *mdView {
	return &mdView{id: id, path: path, run: run, text: text, modTime: info.ModTime(), size: info.Size(), dirty: true}
}

func (v *mdView) ID() int          { return v.id }
func (v *mdView) TabTitle() string { return "md " + filepath.Base(v.path) }

func (v *mdView) Resize(width, height int) {
	v.width, v.height = width, height
}

// isMarkdown is true for files ending in .md or .markdown, in any case.
func isMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// mdStyle is the fixed Glamour style of a theme. An unknown theme counts as the default one.
func mdStyle(theme int) gansi.StyleConfig {
	if theme >= 0 && theme < len(themes) && themes[theme].name == "Tokyo Night" {
		return styles.TokyoNightStyleConfig
	}
	return styles.DarkStyleConfig
}

// body is the rendered file as screen lines without trailing blank lines. If Glamour fails, it is
// the raw text wrapped to the width.
func (v *mdView) body() []string {
	if !v.dirty && v.cacheWidth == v.width && v.cacheTheme == v.theme {
		return v.lines
	}
	var lines []string
	r, err := glamour.NewTermRenderer(glamour.WithStyles(mdStyle(v.theme)), glamour.WithWordWrap(max(1, v.width)))
	var out string
	if err == nil {
		out, err = r.Render(v.text)
	}
	if err == nil {
		lines = strings.Split(out, "\n")
		for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
			lines = lines[:len(lines)-1]
		}
	} else {
		lines = wrapText(v.text, max(1, v.width))
	}
	v.lines, v.dirty, v.cacheWidth, v.cacheTheme = lines, false, v.width, v.theme
	return lines
}

// maxTop is the top line that shows the end of the body.
func (v *mdView) maxTop() int {
	return max(0, len(v.body())-v.height)
}

// scroll moves the view by delta screen lines within the body.
func (v *mdView) scroll(delta int) {
	v.top = max(0, min(v.top+delta, v.maxTop()))
}

// update handles the keys of the viewer tab. It returns false for keys it does not use.
func (v *mdView) update(key string) bool {
	half := max(1, v.height/2)
	switch key {
	case "j", "down":
		v.scroll(1)
	case "k", "up":
		v.scroll(-1)
	case "ctrl+d", "pgdown":
		v.scroll(half)
	case "ctrl+u", "pgup":
		v.scroll(-half)
	case "g", "home":
		v.top = 0
	case "G", "end":
		v.top = v.maxTop()
	default:
		return false
	}
	return true
}

// View fills exactly the size from Resize with the visible part of the body.
func (v *mdView) View(bool) string {
	body := v.body()
	v.top = max(0, min(v.top, len(body)-v.height))
	lines := make([]string, v.height)
	for i := range lines {
		line := ""
		if v.top+i < len(body) {
			line = ansi.Truncate(body[v.top+i], v.width, "")
		}
		lines[i] = line + strings.Repeat(" ", max(0, v.width-ansi.StringWidth(line)))
	}
	return strings.Join(lines, "\n")
}

// openView opens the selection of the active tab of the right column in a viewer tab: the marked
// file of the Files tab or the marked PRD. A viewer of the same file gets the focus instead.
func (m *model) openView() {
	p := m.current()
	if p == nil {
		m.err = "Add a folder first: a in the left column"
		return
	}
	var path, run string
	if m.rightTab == rightPRDs {
		d := p.selectedPRD()
		if d == nil {
			m.err = "Select a PRD in the PRDs tab."
			return
		}
		path = filepath.Join(d.Root, d.Path)
		if d.Root != p.path {
			run = d.Name
		}
	} else {
		r, found := p.files().selected()
		if !found || r.dir {
			m.err = "Select a Markdown file in the Files tab."
			return
		}
		path = r.path
	}
	name := filepath.Base(path)
	if !isMarkdown(path) {
		m.err = name + " is no Markdown file."
		return
	}
	info, err := os.Stat(path)
	var data []byte
	if err == nil {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		m.err = "Cannot read " + name + ": " + claude.FirstLine(err.Error())
		return
	}
	m.focus = colCenter
	for i, tab := range p.tabs {
		if v, ok := tab.(*mdView); ok && v.path == path {
			p.tab = i
			return
		}
	}
	m.nextID++
	v := newMDView(m.nextID, path, run, string(data), info)
	v.Resize(m.termSize())
	p.tabs = append(p.tabs, v)
	p.tab = len(p.tabs) - 1
}

// mdRead is what one check of a viewer's file found. changed is false if modification time and
// size are the same, then text is empty.
type mdRead struct {
	text    string
	modTime time.Time
	size    int64
	err     error
	changed bool
}

// mdReadMsg carries the checks of the viewer tabs by tab ID.
type mdReadMsg struct {
	byTab map[int]mdRead
}

// readViews checks in the background the files of all viewer tabs and reads those that changed.
func (m model) readViews() tea.Cmd {
	type job struct {
		id      int
		path    string
		modTime time.Time
		size    int64
	}
	var jobs []job
	for _, p := range m.projects {
		for _, tab := range p.tabs {
			if v, ok := tab.(*mdView); ok {
				jobs = append(jobs, job{v.id, v.path, v.modTime, v.size})
			}
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	return func() tea.Msg {
		msg := mdReadMsg{byTab: map[int]mdRead{}}
		for _, j := range jobs {
			info, err := os.Stat(j.path)
			switch {
			case err != nil:
				msg.byTab[j.id] = mdRead{err: err}
			case info.ModTime().Equal(j.modTime) && info.Size() == j.size:
				msg.byTab[j.id] = mdRead{}
			default:
				data, err := os.ReadFile(j.path)
				msg.byTab[j.id] = mdRead{text: string(data), modTime: info.ModTime(), size: info.Size(), err: err, changed: err == nil}
			}
		}
		return msg
	}
}

// applyViews hands the checks to their viewer tabs. A file that is gone or unreadable closes its
// tab with a note.
func (m *model) applyViews(msg mdReadMsg) {
	for _, p := range m.projects {
		for i := len(p.tabs) - 1; i >= 0; i-- {
			v, ok := p.tabs[i].(*mdView)
			if !ok {
				continue
			}
			r, ok := msg.byTab[v.id]
			if !ok {
				continue
			}
			name := filepath.Base(v.path)
			switch {
			case r.err != nil:
				p.removeTab(i)
				m.err = ""
				if os.IsNotExist(r.err) {
					m.note = name + " is gone, its viewer closed."
				} else {
					m.note = "Cannot read " + name + ": " + claude.FirstLine(r.err.Error()) + ", its viewer closed."
				}
			case r.changed:
				v.text, v.modTime, v.size, v.dirty = r.text, r.modTime, r.size, true
				v.scroll(0)
			}
		}
	}
}
