package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/github"
	"lazydev/internal/project"
)

// issueSet ist der Stand eines Projekts. Ohne GitHub-origin ist repo leer.
type issueSet struct {
	repo     string
	issues   []github.Issue
	fetching bool
}

// issues ordnet jeder Projekt-ID ihren Stand zu. Die Zeiger überleben die Kopie des Modells.
type issues map[string]*issueSet

// issuesLoadedMsg meldet das Ende eines Abrufs. listed ist wahr, wenn github.List erfolgreich war.
type issuesLoadedMsg struct {
	projectID, name, repo string
	issues                []github.Issue
	listed                bool
	err                   error
}

// issuesInterval ist der Abstand zwischen zwei Abrufen aller Projekte.
const issuesInterval = 30 * time.Minute

type issuesTickMsg struct{}

func issuesTick() tea.Cmd {
	return tea.Tick(issuesInterval, func(time.Time) tea.Msg { return issuesTickMsg{} })
}

// initIssues legt für jedes neue Projekt einen Stand an und startet den Abruf.
func (m *Model) initIssues() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.projects {
		p := &m.projects[i]
		if _, ok := m.issues[p.ID]; ok {
			continue
		}
		m.syncIssueSet(p)
		cmds = append(cmds, m.fetchIssues(p))
	}
	return tea.Batch(cmds...)
}

// syncIssueSet liest origin von p und legt den Stand bei Bedarf an. Ändert sich das Repo,
// verwirft syncIssueSet die Issues und übernimmt den passenden Zwischenspeicher.
func (m *Model) syncIssueSet(p *project.Project) {
	repo, _ := github.Repo(p.Path)
	set := m.issues[p.ID]
	if set == nil {
		set = &issueSet{}
		m.issues[p.ID] = set
	}
	if set.repo == repo {
		return
	}
	set.repo, set.issues = repo, nil
	if repo == "" {
		return
	}
	if c, err := github.LoadCache(p.ID); err == nil && c != nil && c.Repo == repo {
		set.issues = c.Issues
	}
}

// fetchIssues lädt die Issues von p im Hintergrund. Ohne GitHub-origin oder bei laufendem Abruf
// liefert fetchIssues nil.
func (m *Model) fetchIssues(p *project.Project) tea.Cmd {
	set := m.issues[p.ID]
	if set == nil || set.repo == "" || set.fetching {
		return nil
	}
	set.fetching = true
	id, name, dir, repo := p.ID, p.Name(), p.Path, set.repo
	return func() tea.Msg {
		list, err := github.List(dir, repo)
		if err != nil {
			return issuesLoadedMsg{projectID: id, name: name, repo: repo, err: err}
		}
		// Die geladenen Issues gelten auch, wenn der Zwischenspeicher nicht schreibbar ist.
		err = github.SaveCache(id, &github.Cache{Repo: repo, FetchedAt: time.Now(), Issues: list})
		return issuesLoadedMsg{projectID: id, name: name, repo: repo, issues: list, listed: true, err: err}
	}
}

// fetchAllIssues liest origin aller Projekte neu und startet die Abrufe.
func (m *Model) fetchAllIssues() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.projects {
		p := &m.projects[i]
		m.syncIssueSet(p)
		cmds = append(cmds, m.fetchIssues(p))
	}
	return tea.Batch(cmds...)
}

// applyIssues übernimmt einen Abruf. Ein Fehler lässt den bisherigen Stand stehen.
func (m *Model) applyIssues(msg issuesLoadedMsg) {
	if set := m.issues[msg.projectID]; set != nil {
		set.fetching = false
		if msg.listed && set.repo == msg.repo {
			set.issues = msg.issues
		}
	}
	if msg.err != nil {
		m.setMessage("Issues von "+msg.name+" nicht geladen: "+firstLine(msg.err.Error()), true)
	}
}

// projectIssues liefert die offenen Issues eines Projekts mit GitHub-origin.
func (m Model) projectIssues(projectID string) []github.Issue {
	set := m.issues[projectID]
	if set == nil || set.repo == "" {
		return nil
	}
	return set.issues
}

// showIssues meldet, ob das gewählte Projekt offene Issues hat.
func (m Model) showIssues() bool {
	p := m.selectedProject()
	return p != nil && len(m.projectIssues(p.ID)) > 0
}

// syncIssues begrenzt den Cursor der Box. Verschwindet die Box mit Fokus, geht der Fokus auf
// die PRDs.
func (m *Model) syncIssues() {
	if p := m.selectedProject(); p != nil {
		m.issueCursor = clamp(m.issueCursor, len(m.projectIssues(p.ID)))
	}
	if m.focus == focusIssues && !m.showIssues() {
		m.focus = focusPRDs
	}
}

// viewIssues zeigt eine Zeile pro Issue: Nummer und Titel.
func (m Model) viewIssues(l layout) string {
	focused := m.focus == focusIssues
	list := m.projectIssues(m.selectedProject().ID)
	var lines []string
	start, end := scrollWindow(m.issueCursor, len(list), l.issuesHeight-2)
	for i := start; i < end; i++ {
		text := styleMuted.Render(fmt.Sprintf("#%d", list[i].Number)) + " " + list[i].Title
		lines = append(lines, cursorLine(i == m.issueCursor, focused, text))
	}
	title := fmt.Sprintf("Issues · %d", len(list))
	return box(title, lines, l.sideWidth, l.issuesHeight, focused)
}

// selectedIssue liefert das Issue unter dem Cursor der Box.
func (m Model) selectedIssue() (github.Issue, bool) {
	p := m.selectedProject()
	if p == nil {
		return github.Issue{}, false
	}
	list := m.projectIssues(p.ID)
	if m.issueCursor < 0 || m.issueCursor >= len(list) {
		return github.Issue{}, false
	}
	return list[m.issueCursor], true
}

// updateIssues bedient die Box. handled ist false für Tasten, die wie im Normalmodus wirken.
func (m Model) updateIssues(msg tea.KeyMsg) (next Model, cmd tea.Cmd, handled bool) {
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "enter", "o", "n":
		issue, ok := m.selectedIssue()
		if !ok {
			return m, nil, true
		}
		switch msg.String() {
		case "enter":
			next, cmd = m.openIssueView(issue)
		case "o":
			next, cmd = m.openIssueWeb(issue)
		default:
			next, cmd = m.prdFromIssue(issue)
		}
		return next, cmd, true
	default:
		return m, nil, false
	}
	return m, nil, true
}

// openIssueWeb öffnet das Issue des gewählten Projekts im Browser.
func (m Model) openIssueWeb(issue github.Issue) (Model, tea.Cmd) {
	// Das Issue-Fenster bleibt offen, während ein Neuladen das Projekt oder origin ändern kann.
	p := m.selectedProject()
	if p == nil || m.issues[p.ID] == nil || m.issues[p.ID].repo == "" {
		return m, nil
	}
	dir, repo := p.Path, m.issues[p.ID].repo
	return m, action(func() (string, error) {
		return fmt.Sprintf("Issue #%d im Browser geöffnet", issue.Number), github.OpenWeb(dir, repo, issue.Number)
	})
}

// prdFromIssue startet den Chat zum PRD direkt mit dem Issue.
func (m Model) prdFromIssue(issue github.Issue) (Model, tea.Cmd) {
	display, desc := issueChat(issue)
	return m.startChat(display, desc)
}

// issueChat liefert die kurze Beschreibung und die Beschreibung mit Text für den Chat aus issue.
func issueChat(issue github.Issue) (display, desc string) {
	display = fmt.Sprintf("#%d %s", issue.Number, issue.Title)
	desc = display
	if body := strings.TrimSpace(issue.Body); body != "" {
		desc += "\n\n" + body
	}
	return display, desc
}
