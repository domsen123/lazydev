// Package tui enthält die Bubble-Tea-Oberfläche von lazydev.
package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/chatrun"
	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
	"lazydev/internal/usage"
)

type focus int

const (
	focusProjects focus = iota
	focusPRDs
	// focusNotifications gilt nur, solange die Box „Benachrichtigungen“ sichtbar ist.
	focusNotifications
	// focusIssues gilt nur, solange die Box „Issues“ sichtbar ist.
	focusIssues
	focusLog
)

type mode int

const (
	modeNormal mode = iota
	modeAddProject
	modeConfirmRemove
	// modeConfirm fragt nach einer Aktion aus confirm.
	modeConfirm
	// modeEvent zeigt ein Log-Ereignis vollständig.
	modeEvent
	// modeNewPRD fragt nach der Beschreibung eines neuen PRDs.
	modeNewPRD
	// modeChat zeigt den Chat mit Claude über der TUI.
	modeChat
	// modeDocker zeigt die Container des gewählten Projekts.
	modeDocker
	// modeHelp zeigt alle Tasten.
	modeHelp
	// modeLimits zeigt die Claude-Limits.
	modeLimits
	// modePRDView zeigt ein PRD vollständig im Fenster.
	modePRDView
	// modeIssueView zeigt ein GitHub-Issue vollständig im Fenster.
	modeIssueView
	// modeBranch wählt den Ziel-Branch des gewählten Projekts.
	modeBranch
	// modeInit legt .lazydev/ im gewählten Projekt an.
	modeInit
	// modeStatus wählt den Status des gewählten PRDs.
	modeStatus
)

// confirmation ist eine Rückfrage. yes läuft nach „j“. Mit details zeigt die TUI ein Fenster mit
// diesem Titel und diesen Zeilen, die Frage steht in der Statuszeile.
type confirmation struct {
	question string
	title    string
	details  []string
	yes      func(Model) (Model, tea.Cmd)
}

type Model struct {
	cfgPath string
	cfg     *config.Config

	projects   []project.Project
	projCursor int
	prdCursor  int
	// showDone klappt die erledigten PRDs auf.
	showDone bool
	prdSort  prdSort
	focus    focus
	mode     mode
	loading  bool

	input      textinput.Model
	candidates []string
	branch     *branchPicker
	// prdStatus ist das offene Fenster „Status“, sonst nil.
	prdStatus *statusPicker
	// wizard ist der offene Wizard „.lazydev/ anlegen“, sonst nil.
	wizard *initWizard

	// withoutRepo enthält die Pfade der Projekte ohne .lazydev/config.yaml.
	withoutRepo map[string]bool
	// ignoredShown merkt sich die Projekte, deren ignorierte lokale Einstellungen die Statuszeile
	// schon gemeldet hat. Der Zeiger überlebt die Kopie des Modells.
	ignoredShown map[string]bool

	message    string
	messageErr bool

	runs runs
	// logs liest die Logs aller angesehenen Läufe schrittweise nach.
	logs     logCache
	logKey   runKey
	logLines []logLine
	// logCursor ist die gewählte Log-Zeile. logFollow hält ihn auf der letzten Zeile.
	logCursor int
	logFollow bool
	logFull   bool
	event     string
	eventTop  int

	// detail zeigt den gerenderten Detail-Text des PRDs. detailKey merkt sich das PRD, damit
	// ein Wechsel nach oben springt.
	detail    viewport.Model
	detailKey string
	md        *markdown

	confirm *confirmation
	// returnMode ist der Modus nach einer Rückfrage, etwa die Docker-Ansicht.
	returnMode mode
	chat       *chat
	// chats enthält die Benachrichtigungen aller Projekte. notifCursor ist die gewählte Zeile
	// der Box.
	chats       chats
	notifCursor int
	// prdView ist das offene PRD-Fenster, sonst nil.
	prdView *prdView
	// issues enthält die offenen GitHub-Issues aller Projekte. issueCursor ist die gewählte Zeile
	// der Box.
	issues      issues
	issueCursor int
	// issueView ist das offene Issue-Fenster, sonst nil.
	issueView *issueView

	limits           usage.Snapshot
	limitsOK         bool
	limitsRefreshing bool

	docker       dockerInfo
	dockerCh     <-chan struct{}
	dockerCursor int
	dockerDBs    map[string][]string

	watchEnabled bool
	watch        *watcher

	width, height int
}

type runsLoadedMsg struct{ runs runs }

// actionDoneMsg meldet das Ende einer Aktion wie Start oder Verwerfen.
type actionDoneMsg struct {
	text string
	err  error
}

type projectsLoadedMsg struct {
	projects []project.Project
	runs     runs
	chats    []chatrun.Entry
	// withoutRepo enthält die Pfade der Projekte ohne .lazydev/config.yaml.
	withoutRepo map[string]bool
	// ignored nennt je Projektpfad die lokalen Felder, die .lazydev/config.yaml ersetzt.
	ignored map[string][]string
	// selectPath wählt nach dem Laden dieses Projekt aus, sonst bleibt die Auswahl.
	selectPath string
}

func New(cfgPath string, cfg *config.Config) Model {
	in := textinput.New()
	in.Prompt = "Projektpfad: "
	in.Placeholder = "~/development/…"
	return Model{cfgPath: cfgPath, cfg: cfg, input: in, loading: true, runs: runs{}, logs: logCache{}, logFollow: true,
		md: &markdown{}, chats: chats{}, issues: issues{}, ignoredShown: map[string]bool{}}
}

// WithWatch schaltet Dateiüberwachung und regelmäßiges Neueinlesen ein. Tests lassen beides aus.
func (m Model) WithWatch() Model {
	m.watchEnabled = true
	return m
}

func (m Model) Init() tea.Cmd {
	if m.watchEnabled {
		return tea.Batch(m.loadProjects(""), tick(), issuesTick(), loadLimits)
	}
	return m.loadProjects("")
}

// loadProjects liest alle Projekte parallel, weil jedes Projekt mehrere git-Aufrufe braucht.
func (m Model) loadProjects(selectPath string) tea.Cmd {
	cfgProjects := slices.Clone(m.cfg.Projects)
	return func() tea.Msg {
		paths := make([]string, len(cfgProjects))
		for i, p := range cfgProjects {
			paths[i] = p.AbsPath()
		}
		ids := project.AssignIDs(paths)
		projects := make([]project.Project, len(paths))
		hasRepo := make([]bool, len(paths))
		ignored := make([][]string, len(paths))
		var wg sync.WaitGroup
		for i := range paths {
			wg.Add(1)
			go func() {
				defer wg.Done()
				repo, ign, err := config.Effective(cfgProjects[i])
				projects[i] = project.Load(ids[i], paths[i], repo.Base())
				if err != nil {
					projects[i].Err = errors.Join(err, projects[i].Err)
				}
				hasRepo[i] = config.RepoExists(paths[i])
				ignored[i] = ign
			}()
		}
		wg.Wait()
		rs := loadRuns()
		msg := projectsLoadedMsg{runs: rs, withoutRepo: map[string]bool{}, ignored: map[string][]string{}, selectPath: selectPath}
		for i := range projects {
			addWorktreePRDs(&projects[i], rs)
			if !hasRepo[i] {
				msg.withoutRepo[paths[i]] = true
			}
			if len(ignored[i]) > 0 {
				msg.ignored[paths[i]] = ignored[i]
			}
		}
		msg.projects = projects
		msg.chats, _ = chatrun.All()
		return msg
	}
}

func (m Model) selectedProject() *project.Project {
	if m.projCursor < 0 || m.projCursor >= len(m.projects) {
		return nil
	}
	return &m.projects[m.projCursor]
}

func (m Model) selectedPRD() *project.PRD {
	rows := m.prdList(m.selectedProject())
	if m.prdCursor < 0 || m.prdCursor >= len(rows) {
		return nil
	}
	return rows[m.prdCursor].prd
}

func (m *Model) setMessage(text string, isErr bool) {
	m.message = text
	m.messageErr = isErr
}

func (m *Model) applyLoaded(msg projectsLoadedMsg) {
	prev, prevPRD := "", ""
	if p := m.selectedProject(); p != nil {
		prev = p.Path
	}
	if d := m.selectedPRD(); d != nil {
		prevPRD = d.Name
	}
	if msg.selectPath != "" {
		prev = msg.selectPath
	}
	m.projects = msg.projects
	m.runs = msg.runs
	m.withoutRepo = msg.withoutRepo
	m.loading = false
	m.showIgnored(msg.ignored)
	m.projCursor = clamp(m.projCursor, len(m.projects))
	for i, p := range m.projects {
		if p.Path == prev {
			m.projCursor = i
			break
		}
	}
	if p := m.selectedProject(); p != nil && p.Path == prev {
		// Ändert sich die Liste, etwa nach einem Merge, bleibt das gewählte PRD gewählt.
		m.selectPRD(prevPRD)
	} else {
		m.prdCursor = clamp(m.prdCursor, len(m.prdList(p)))
	}
}

// showIgnored meldet einmal je Projekt die lokalen Felder, die .lazydev/config.yaml ersetzt.
func (m *Model) showIgnored(ignored map[string][]string) {
	var notes []string
	for _, p := range m.projects {
		fields, ok := ignored[p.Path]
		if !ok || m.ignoredShown[p.Path] {
			continue
		}
		m.ignoredShown[p.Path] = true
		notes = append(notes, fmt.Sprintf("%s: lokale Einstellungen ignoriert (%s), es gilt %s/config.yaml",
			p.Name(), strings.Join(fields, ", "), config.RepoDir))
	}
	if len(notes) > 0 {
		m.setMessage(strings.Join(notes, ". "), false)
	}
}

func clamp(i, n int) int {
	return max(0, min(i, n-1))
}

func (m Model) selectedRun() *state.Run {
	return m.runs.get(m.selectedProject(), m.selectedPRD())
}

func loadRunsCmd() tea.Msg {
	return runsLoadedMsg{runs: loadRuns()}
}

// syncDetail passt Größe und Inhalt der Detailansicht an das gewählte PRD an.
func (m *Model) syncDetail() {
	d := m.selectedPRD()
	if d == nil || m.selectedRun() != nil || m.md == nil {
		m.detailKey = ""
		return
	}
	l := m.layout()
	m.detail.Width = max(1, l.width-4)
	m.detail.Height = max(1, l.detailHeight-2-len(m.detailHeader(d)))
	content := strings.Join(detailMeta(d, m.selectedRun()), "\n")
	if d.Status.Detail != "" {
		content += "\n\n" + m.md.render(d.Status.Detail, m.detail.Width)
	}
	m.detail.SetContent(content)
	if key := m.selectedProject().ID + "/" + d.Path; key != m.detailKey {
		m.detailKey = key
		m.detail.GotoTop()
	}
	m.detail.SetYOffset(m.detail.YOffset)
}

// refreshLog liest das Log des gewählten PRDs nach. Bei einem anderen PRD springt die Ansicht
// ans Ende.
func (m *Model) refreshLog() {
	p, d := m.selectedProject(), m.selectedPRD()
	r := m.runs.get(p, d)
	if r == nil {
		m.logLines = nil
		m.logKey = runKey{}
		return
	}
	key := runKey{p.ID, d.Name}
	if key != m.logKey {
		m.logKey = key
		m.logFollow = true
	}
	m.logLines = m.logs.lines(key.projectID, key.prdName, r)
	if m.logFollow {
		m.logCursor = len(m.logLines) - 1
	}
	m.logCursor = clamp(m.logCursor, len(m.logLines))
}
