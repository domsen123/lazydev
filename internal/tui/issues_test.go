package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/chatrun"
	"lazydev/internal/config"
	"lazydev/internal/github"
	"lazydev/internal/state"
)

const issuesJSON = `[{"number":2,"title":"Alter Fehler","body":"","url":"https://github.com/acme/app/issues/2","createdAt":"2026-09-01T10:00:00Z"},
{"number":3,"title":"Login mit Passkey","body":"  Nutzer melden sich **ohne** Passwort an.\n","url":"https://github.com/acme/app/issues/3","createdAt":"2026-10-01T10:00:00Z"}]`

// fakeGH ersetzt gh durch ein Skript, das seine Argumente in calls protokolliert und den Inhalt
// von out ausgibt. Existiert fail, endet es mit Exit-Code 1.
func fakeGH(t *testing.T, json string) (calls, fail string) {
	t.Helper()
	dir := t.TempDir()
	calls, fail = filepath.Join(dir, "calls"), filepath.Join(dir, "fail")
	out := filepath.Join(dir, "out.json")
	os.WriteFile(out, []byte(json), 0o644)
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + calls + "\necho >> " + calls + "\n" +
		"[ -e " + fail + " ] && { echo 'gh: nicht angemeldet' >&2; echo 'gh auth login' >&2; exit 1; }\n" +
		"cat " + out + "\n"
	bin := filepath.Join(dir, "gh")
	os.WriteFile(bin, []byte(script), 0o755)
	prev := github.Binary
	github.Binary = bin
	t.Cleanup(func() { github.Binary = prev })
	return calls, fail
}

// setupIssues legt das Projekt alpha mit origin auf github.com an, schreibt cache als
// Zwischenspeicher und lädt das Projekt. fetch ist der erste Abruf, den das Laden startet.
func setupIssues(t *testing.T, cache *github.Cache) (m Model, fetch tea.Cmd, calls, fail string) {
	t.Helper()
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:acme/app.git").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if cache != nil {
		if err := github.SaveCache("alpha", cache); err != nil {
			t.Fatal(err)
		}
	}
	calls, fail = fakeGH(t, issuesJSON)
	cfg := &config.Config{MaxParallel: 3, PortBase: 3100, Claude: config.Claude{Binary: "claude"}, Projects: []config.Project{{Path: dir}}}
	m = New(filepath.Join(root, "config.yaml"), cfg)
	m = drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	next, fetch := m.Update(m.Init()())
	return next.(Model), fetch, calls, fail
}

// runAll führt cmd aus und gibt jede Nachricht an das Modell, auch die eines tea.Batch.
func runAll(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runAll(t, m, c)
		}
		return m
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestIssueWidth(t *testing.T) {
	if w := ansi.StringWidth("#"); w != 1 {
		t.Errorf("# ist %d Zellen breit", w)
	}
}

func TestIssuesBox(t *testing.T) {
	m, fetch, calls, _ := setupIssues(t, nil)
	before := m.layout()
	if fetch == nil || !m.issues["alpha"].fetching || m.issues["alpha"].repo != "acme/app" {
		t.Fatalf("kein Abruf nach dem Laden: %+v", m.issues["alpha"])
	}
	if before.sideWidth != 0 || strings.Contains(ansi.Strip(m.View()), "Issues") {
		t.Fatalf("Box vor dem Abruf: %+v", before)
	}
	// Ein laufender Abruf startet keinen zweiten.
	if cmd := m.fetchAllIssues(); cmd != nil {
		t.Error("zweiter Abruf während des ersten")
	}

	m = runCmd(t, m, fetch)
	got, _ := os.ReadFile(calls)
	if string(got) != "issue|list|--repo|acme/app|--state|open|--limit|30|--json|number,title,body,url,createdAt|\n" {
		t.Errorf("Aufrufe von gh:\n%s", got)
	}
	l := m.layout()
	if l.sideWidth != l.leftWidth || l.issuesHeight != l.topHeight || l.notifHeight != 0 || l.rightWidth != before.rightWidth-l.leftWidth {
		t.Errorf("Layout mit Box: %+v, ohne: %+v", l, before)
	}
	view := ansi.Strip(m.View())
	newer, older := strings.Index(view, "#3 Login mit Passkey"), strings.Index(view, "#2 Alter Fehler")
	if !strings.Contains(view, "Issues · 2") || newer < 0 || older < newer || !strings.Contains(view, "alpha") || !strings.Contains(view, " 2#") {
		t.Errorf("Box oder Indikator:\n%s", view)
	}
	c, err := github.LoadCache("alpha")
	if err != nil || c == nil || c.Repo != "acme/app" || len(c.Issues) != 2 || c.FetchedAt.IsZero() {
		t.Errorf("Zwischenspeicher: %+v, %v", c, err)
	}

	// Fokus ohne Benachrichtigungen: PRDs, Issues, Log.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusPRDs {
		t.Fatalf("Fokus nach Tab: %v", m.focus)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.focus != focusIssues || !strings.Contains(ansi.Strip(m.helpLine()), "j/k wählen · Enter ansehen · o Browser · n PRD") {
		t.Fatalf("Fokus nach →: %v, %q", m.focus, ansi.Strip(m.helpLine()))
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusLog {
		t.Errorf("Fokus nach Tab auf Issues: %v", m.focus)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyShiftTab}, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != focusPRDs {
		t.Errorf("Fokus nach zweimal Shift+Tab: %v", m.focus)
	}

	// Mit Benachrichtigungen teilen sich beide Boxen die Spalte.
	writeChat(t, "alpha", "a", question("Erste Frage", "2026-10-20T09:00:00+02:00"))
	m = drive(t, m, loadChatsCmd())
	l = m.layout()
	if l.sideWidth != l.leftWidth || l.notifHeight != l.topHeight/2 || l.issuesHeight != l.topHeight-l.topHeight/2 {
		t.Errorf("Layout mit beiden Boxen: %+v", l)
	}
	view = ansi.Strip(m.View())
	if n, i := strings.Index(view, "Benachrichtigungen · 1"), strings.Index(view, "Issues · 2"); n < 0 || i < n {
		t.Errorf("Reihenfolge der Boxen:\n%s", view)
	}
	for _, f := range []focus{focusNotifications, focusIssues, focusLog} {
		m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
		if m.focus != f {
			t.Errorf("Fokus nach Tab: %v statt %v", m.focus, f)
		}
	}
	for _, width := range []int{80, 200} {
		m = drive(t, m, tea.WindowSizeMsg{Width: width, Height: 30})
		checkViewSize(t, m)
	}

	// Verschwindet die Box mit Fokus, geht der Fokus auf die PRDs.
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != focusIssues {
		t.Fatalf("Fokus nach Shift+Tab: %v", m.focus)
	}
	m = drive(t, m, issuesLoadedMsg{projectID: "alpha", name: "alpha", repo: "acme/app", listed: true})
	if m.focus != focusPRDs || m.showIssues() || strings.Contains(ansi.Strip(m.View()), "#") {
		t.Errorf("Fokus ohne Issues: %v", m.focus)
	}
}

func TestIssuesError(t *testing.T) {
	m, fetch, _, fail := setupIssues(t, nil)
	m = runCmd(t, m, fetch)
	before, _ := os.ReadFile(state.IssuesPath("alpha"))

	// Der Tick startet einen Abruf für alle Projekte mit GitHub-origin.
	next, cmd := m.Update(issuesTickMsg{})
	if cmd == nil || !m.issues["alpha"].fetching {
		t.Fatal("kein Abruf nach dem Tick")
	}
	m = drive(t, next.(Model), issuesLoadedMsg{projectID: "alpha", name: "alpha", repo: "acme/app", err: os.ErrPermission})
	if !m.messageErr || m.message != "Issues von alpha nicht geladen: permission denied" || m.issues["alpha"].fetching {
		t.Errorf("Meldung nach dem Tick: %q", m.message)
	}

	os.WriteFile(fail, nil, 0o644)
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = runAll(t, next.(Model), cmd)
	if !m.messageErr || m.message != "Issues von alpha nicht geladen: gh: nicht angemeldet" {
		t.Errorf("Meldung: %q", m.message)
	}
	if len(m.projectIssues("alpha")) != 2 || m.issues["alpha"].fetching {
		t.Errorf("Stand nach dem Fehler: %+v", m.issues["alpha"])
	}
	if after, _ := os.ReadFile(state.IssuesPath("alpha")); string(after) != string(before) {
		t.Errorf("Zwischenspeicher nach dem Fehler:\n%s", after)
	}

	// Ohne gh bleibt die TUI bedienbar.
	github.Binary = filepath.Join(t.TempDir(), "gh")
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = runAll(t, next.(Model), cmd)
	if !strings.HasPrefix(m.message, "Issues von alpha nicht geladen: ") || len(m.projectIssues("alpha")) != 2 {
		t.Errorf("Meldung ohne gh: %q", m.message)
	}
	m = drive(t, m, keys("j")...)
	if m.mode != modeNormal || m.message != "" {
		t.Errorf("nach j: %v %q", m.mode, m.message)
	}
}

func TestIssuesCache(t *testing.T) {
	saved := &github.Cache{Repo: "acme/other", FetchedAt: time.Now(),
		Issues: []github.Issue{{Number: 9, Title: "Fremd", CreatedAt: time.Now()}}}
	m, _, _, _ := setupIssues(t, saved)
	if len(m.projectIssues("alpha")) != 0 {
		t.Errorf("Zwischenspeicher mit anderem repo übernommen: %+v", m.projectIssues("alpha"))
	}

	saved.Repo = "acme/app"
	m, _, _, _ = setupIssues(t, saved)
	if list := m.projectIssues("alpha"); len(list) != 1 || list[0].Title != "Fremd" {
		t.Errorf("Zwischenspeicher nicht übernommen: %+v", list)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Issues · 1") || !strings.Contains(view, " 1#") {
		t.Errorf("Box aus dem Zwischenspeicher:\n%s", view)
	}
}

func TestIssuesWithoutGitHub(t *testing.T) {
	calls, _ := fakeGH(t, issuesJSON)
	m := setupRun(t)
	exec.Command("git", "-C", m.projects[0].Path, "remote", "add", "origin", "https://gitlab.com/acme/app.git").Run()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = runAll(t, next.(Model), cmd)
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Errorf("gh ohne GitHub-origin aufgerufen: %v", err)
	}
	if m.showIssues() || m.layout().sideWidth != 0 || strings.Contains(ansi.Strip(m.View()), "#") {
		t.Errorf("Box ohne GitHub-origin:\n%s", ansi.Strip(m.View()))
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRight}, tea.KeyMsg{Type: tea.KeyRight})
	if m.focus != focusLog {
		t.Errorf("Fokus ohne Boxen: %v", m.focus)
	}
}

func TestIssueView(t *testing.T) {
	m, fetch, calls, _ := setupIssues(t, nil)
	m = runCmd(t, m, fetch)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRight}, tea.KeyMsg{Type: tea.KeyRight})
	if m.focus != focusIssues {
		t.Fatalf("Fokus: %v", m.focus)
	}

	// Das zweite Issue hat keinen Text.
	m = drive(t, m, keys("j")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeIssueView || m.issueView == nil {
		t.Fatalf("Enter öffnet kein Fenster: %v", m.mode)
	}
	view := ansi.Strip(m.View())
	for _, want := range []string{"Issue #2", "Titel     Alter Fehler", "Erstellt  " + time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC).Local().Format("2006-01-02 15:04"),
		"URL       https://github.com/acme/app/issues/2", "Kein Text"} {
		if !strings.Contains(view, want) {
			t.Errorf("Fenster ohne %q:\n%s", want, view)
		}
	}
	if !strings.Contains(ansi.Strip(m.helpLine()), "j/k scrollen · PgUp/PgDn Seite · g/G Anfang/Ende · o Browser · n PRD · Esc schließen") {
		t.Errorf("Fußzeile: %q", ansi.Strip(m.helpLine()))
	}
	checkViewSize(t, m)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal || m.issueView != nil {
		t.Errorf("nach Esc: %v", m.mode)
	}

	m = drive(t, m, keys("k")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Issue #3") || !strings.Contains(view, "Passwort an.") || strings.Contains(view, "Kein Text") {
		t.Errorf("Fenster mit Text:\n%s", view)
	}

	// o öffnet das Issue im Browser, im Fenster wie in der Box.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = runCmd(t, next.(Model), cmd)
	if m.messageErr || m.message != "Issue #3 im Browser geöffnet" {
		t.Errorf("Meldung nach o: %q", m.message)
	}
	got, _ := os.ReadFile(calls)
	if !strings.HasSuffix(string(got), "\nissue|view|3|--repo|acme/app|--web|\n") {
		t.Errorf("Aufrufe von gh:\n%s", got)
	}
	m = drive(t, m, keys("q")...)
	if m.mode != modeNormal {
		t.Errorf("nach q: %v", m.mode)
	}
}

func TestPRDFromIssue(t *testing.T) {
	m, fetch, _, _ := setupIssues(t, nil)
	bin, argsPath := fakeClaude(t)
	m.cfg.Claude.Binary = bin
	inlineRounds(t, m.cfg)
	m = runCmd(t, m, fetch)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyRight}, tea.KeyMsg{Type: tea.KeyRight})

	// n in der Box startet den Chat direkt, ohne Fenster „Neues PRD“.
	m = drive(t, m, keys("n")...)
	if m.mode != modeChat || m.chat == nil {
		t.Fatalf("kein Chat nach n: %v %q", m.mode, m.message)
	}
	m = waitRound(t, m)
	all, _ := chatrun.All()
	if len(all) != 1 || all[0].Chat.Description != "#3 Login mit Passkey" || all[0].Chat.Title != "Neues PRD: alpha" {
		t.Fatalf("Chat: %+v", all[0].Chat)
	}
	got, _ := os.ReadFile(argsPath)
	if !strings.Contains(string(got), "-p|# lazydev create: #3 Login mit Passkey\n") ||
		!strings.Contains(string(got), "#3 Login mit Passkey\n\nNutzer melden sich **ohne** Passwort an.") {
		t.Errorf("Prompt ohne Text des Issues:\n%s", got)
	}
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = drive(t, m, loadChatsCmd())
	if view := ansi.Strip(m.View()); !strings.Contains(view, "? #3 Login mit Passkey") {
		t.Errorf("Benachrichtigung:\n%s", view)
	}

	// n im Fenster schließt es und startet den Chat. Der Fokus bleibt auf den Issues.
	if m.focus != focusIssues {
		t.Fatalf("Fokus: %v", m.focus)
	}
	m = drive(t, m, keys("j")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = drive(t, m, keys("n")...)
	if m.mode != modeChat || m.issueView != nil || m.chat == nil || m.chat.state.Description != "#2 Alter Fehler" {
		t.Fatalf("n im Fenster: %v %q", m.mode, m.message)
	}
	display, desc := issueChat(github.Issue{Number: 2, Title: "Alter Fehler"})
	if display != "#2 Alter Fehler" || desc != display {
		t.Errorf("Beschreibung ohne Text: %q, %q", display, desc)
	}
}
