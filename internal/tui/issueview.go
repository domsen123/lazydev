package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/github"
)

// issueView zeigt ein GitHub-Issue als Markdown. Der Inhalt ändert sich nach dem Öffnen nicht.
type issueView struct {
	issue    github.Issue
	meta     []string
	viewport viewport.Model
	// md ist ein eigener Renderer-Cache, damit der Cache der Detailansicht erhalten bleibt.
	md *markdown
}

// openIssueView öffnet das Fenster „Issue #<nummer>“.
func (m Model) openIssueView(issue github.Issue) (Model, tea.Cmd) {
	meta := []string{
		metaLabel("Titel") + issue.Title,
		metaLabel("Erstellt") + issue.CreatedAt.Local().Format("2006-01-02 15:04"),
		metaLabel("URL") + issue.URL,
	}
	m.issueView = &issueView{issue: issue, meta: meta, md: &markdown{}}
	m.mode = modeIssueView
	m.syncIssueView()
	m.issueView.viewport.GotoTop()
	return m, nil
}

// syncIssueView passt Größe und Inhalt des Fensters an.
func (m *Model) syncIssueView() {
	v := m.issueView
	if v == nil {
		return
	}
	w, h := m.dialogSize()
	v.viewport.Width = max(1, w-2-2*dialogPadX)
	v.viewport.Height = max(1, h-2-2*dialogPadY)
	body := styleMuted.Render("Kein Text")
	if strings.TrimSpace(v.issue.Body) != "" {
		body = v.md.render(v.issue.Body, v.viewport.Width)
	}
	v.viewport.SetContent(strings.Join(v.meta, "\n") + "\n\n" + body)
	v.viewport.SetYOffset(v.viewport.YOffset)
}

func (m Model) viewIssueView() string {
	v := m.issueView
	w, h := m.dialogSize()
	title := fmt.Sprintf("Issue #%d", v.issue.Number)
	if v.viewport.TotalLineCount() > v.viewport.VisibleLineCount() {
		title += fmt.Sprintf(" · %d %%", int(v.viewport.ScrollPercent()*100))
	}
	return dialog(title, strings.Split(v.viewport.View(), "\n"), w, h)
}

func (m Model) updateIssueView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vp := &m.issueView.viewport
	switch msg.String() {
	case "esc", "q":
		m.issueView = nil
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
		return m.openIssueWeb(m.issueView.issue)
	case "n":
		issue := m.issueView.issue
		m.issueView = nil
		m.mode = modeNormal
		return m.prdFromIssue(issue)
	}
	return m, nil
}
