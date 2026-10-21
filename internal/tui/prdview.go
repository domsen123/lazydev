package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/herdr"
	"lazydev/internal/project"
)

// prdView zeigt ein PRD vollständig als Markdown. Den Text liest lazydev einmal beim Öffnen und
// nach dem Editor, Dateiereignisse ändern ihn nicht.
type prdView struct {
	label string
	meta  []string
	// path ist der absolute Pfad der Datei, dir der Worktree oder der Hauptordner.
	path     string
	dir      string
	text     string
	viewport viewport.Model
	// md ist ein eigener Renderer-Cache, damit der Cache der Detailansicht erhalten bleibt.
	md *markdown
}

// prdEditedMsg meldet das Ende des Editors ohne Herdr.
type prdEditedMsg struct {
	path string
	err  error
}

// prdSource liefert Ordner und absoluten Pfad des gewählten PRDs. Ein Lauf, der das PRD in
// seinen Worktree verschoben hat, hat Vorrang vor dem Hauptordner.
func (m Model) prdSource() (dir, path string, ok bool) {
	p, d := m.selectedProject(), m.selectedPRD()
	if p == nil || d == nil {
		return "", "", false
	}
	if r := m.runs.get(p, d); r != nil && r.PRD != "" && r.Phase != project.PhaseMerged {
		return r.Worktree, filepath.Join(r.Worktree, r.PRD), true
	}
	return p.Path, filepath.Join(p.Path, d.Path), true
}

// openPRDView liest das gewählte PRD und öffnet das Fenster.
func (m Model) openPRDView() (Model, tea.Cmd) {
	dir, path, ok := m.prdSource()
	if !ok {
		return m, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		m.setMessage("PRD nicht lesbar: "+firstLine(err.Error()), true)
		return m, nil
	}
	d := m.selectedPRD()
	m.prdView = &prdView{label: d.Label(), meta: detailMeta(d, m.selectedRun()), path: path, dir: dir, text: string(data), md: &markdown{}}
	m.mode = modePRDView
	m.syncPRDView()
	m.prdView.viewport.GotoTop()
	return m, nil
}

// syncPRDView passt Größe und Inhalt des Fensters an. Sie liest die Datei nicht neu.
func (m *Model) syncPRDView() {
	v := m.prdView
	if v == nil {
		return
	}
	w, h := m.dialogSize()
	v.viewport.Width = max(1, w-2-2*dialogPadX)
	v.viewport.Height = max(1, h-2-2*dialogPadY)
	v.viewport.SetContent(strings.Join(v.meta, "\n") + "\n\n" + v.md.render(v.text, v.viewport.Width))
	v.viewport.SetYOffset(v.viewport.YOffset)
}

func (m Model) viewPRDView() string {
	v := m.prdView
	w, h := m.dialogSize()
	title := "PRD: " + v.label
	if v.viewport.TotalLineCount() > v.viewport.VisibleLineCount() {
		title += fmt.Sprintf(" · %d %%", int(v.viewport.ScrollPercent()*100))
	}
	return dialog(title, strings.Split(v.viewport.View(), "\n"), w, h)
}

func (m Model) updatePRDView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vp := &m.prdView.viewport
	switch msg.String() {
	case "esc", "q":
		m.prdView = nil
		m.mode = modeNormal
	case "j", "down":
		vp.ScrollDown(1)
	case "k", "up":
		vp.ScrollUp(1)
	case "pgdown", "ctrl+d", " ":
		vp.ScrollDown(vp.Height)
	case "pgup", "ctrl+u":
		vp.ScrollUp(vp.Height)
	case "g", "home":
		vp.GotoTop()
	case "G", "end":
		vp.GotoBottom()
	case "o":
		return m.openPRDEditor()
	}
	return m, nil
}

// editorCommand liefert $VISUAL, sonst $EDITOR, sonst vi, zerlegt in Programm und Argumente.
func editorCommand() []string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if args := strings.Fields(os.Getenv(name)); len(args) > 0 {
			return args
		}
	}
	return []string{"vi"}
}

// openPRDEditor öffnet die Datei des Fensters im Editor. In Herdr bleibt das Fenster unverändert,
// sonst liest handlePRDEdited die Datei nach dem Editor neu.
func (m Model) openPRDEditor() (Model, tea.Cmd) {
	path, dir := m.prdView.path, m.prdView.dir
	args := append(editorCommand(), path)
	if herdr.Available() {
		return m.openInteractive(dir, args, "Editor")
	}
	return m, execInteractive(dir, args, func(err error) tea.Msg { return prdEditedMsg{path, err} })
}

// handlePRDEdited liest die Datei nach dem Editor neu. Die Scroll-Position bleibt.
func (m Model) handlePRDEdited(msg prdEditedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.setMessage("Editor: "+firstLine(msg.err.Error()), true)
	}
	if m.prdView != nil && m.prdView.path == msg.path {
		data, err := os.ReadFile(msg.path)
		if err != nil {
			m.prdView = nil
			m.mode = modeNormal
			m.setMessage("PRD nicht lesbar: "+firstLine(err.Error()), true)
		} else {
			m.prdView.text = string(data)
		}
	}
	// Der Editor kann die Statuszeile geändert haben.
	return m, m.loadProjects("")
}
