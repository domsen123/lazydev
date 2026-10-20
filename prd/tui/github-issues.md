# PRD: Box „Issues“ für Projekte mit GitHub-Remote

> Status: umgesetzt (2026-10-20) · Template: einfach · Scope: Zeigt `origin` eines Projekts auf `github.com`, lädt lazydev die offenen Issues über `gh` und zeigt sie in einer Box rechts neben den PRDs. Aus einem Issue startet `n` den PRD-Chat.

## Context

lazydev zeigt pro Projekt die PRDs, aber nicht die Aufgaben, die im Issue-Tracker des Repos warten. Um aus
einem GitHub-Issue ein PRD zu machen, kopierst du heute Titel und Text von Hand in das Fenster „Neues PRD“.

Nach der Änderung prüft lazydev für jedes Projekt, ob `origin` auf `github.com` zeigt. Falls ja, lädt es die
offenen Issues mit `gh issue list` beim Start und danach alle 30 Minuten. Hat das gewählte Projekt offene
Issues, erscheint die Box „Issues“ in der rechten Spalte, unter „Benachrichtigungen“. `Enter` zeigt ein Issue
als Markdown im Fenster, `o` öffnet es im Browser und `n` startet den PRD-Chat mit Titel und Text des Issues.
Den letzten Stand speichert lazydev im Zustandsverzeichnis und zeigt ihn nach dem Start sofort an.

Im Repo existiert bereits und wird wiederverwendet:

- `git.Run` (`internal/git/git.go`): ruft das `git`-Binary auf.
- `state.Dir`, `state.Save` als Vorbild für atomares Schreiben (`internal/state/state.go`).
- Box „Benachrichtigungen“ (`internal/tui/notifications.go`): `showNotifications`, `syncNotifications`,
  `viewNotifications`, `updateNotifications`, Fokus `focusNotifications`, `nextFocus` (`internal/tui/update.go`).
- `layout` mit `notifWidth` (`internal/tui/view.go`).
- PRD-Fenster `prdView`, `openPRDView`, `syncPRDView`, `updatePRDView` (`internal/tui/prdview.go`) und der
  Markdown-Renderer `markdown`.
- `dialog`, `dialogSize`, `box`, `cursorLine`, `scrollWindow`, `padRight`, `clamp` (`internal/tui`).
- `newPRD`, `updateTemplate`, `updateNewPRD` (`internal/tui/interactive.go`), `chatrun.Create`, `chatrun.Send`,
  `openChat`.
- `action` und `actionDoneMsg` für Befehle im Hintergrund mit Meldung in der Statuszeile.
- `prdtemplate.All`, `Template.CreatePrompt`, `Template.CreateDisplay` (`internal/prdtemplate`).

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Datenquelle | Nur das CLI `gh`. Keine REST-API, kein Token in der Konfiguration. |
| Remote | Nur `origin`. Die URL muss auf den Host `github.com` zeigen. Erkannte Formen: `https://github.com/<owner>/<repo>`, `git@github.com:<owner>/<repo>` und `ssh://git@github.com/<owner>/<repo>`, jeweils mit oder ohne `.git` und abschließenden `/`. Andere Hosts, andere Remotes und Projekte ohne `origin` haben keine Issues. lazydev ruft für sie `gh` nicht auf. |
| Abruf | `gh issue list --repo <owner>/<repo> --state open --limit 30 --json number,title,body,url,createdAt` im Hauptordner des Projekts. |
| Auswahl | Bis zu 30 offene Issues, neueste zuerst nach `createdAt`. |
| Zeitpunkte | Beim ersten Laden eines Projekts (Start der TUI oder neues Projekt mit `a`), alle 30 Minuten für alle Projekte mit GitHub-`origin` und mit `r` für alle sofort. Ein Projekt, dessen Abruf noch läuft, startet keinen zweiten. |
| Zwischenspeicher | `$XDG_STATE_HOME/lazydev/issues/<projekt-id>.json` mit `repo`, `fetchedAt` und `issues`. Beim ersten Laden eines Projekts zeigt lazydev den gespeicherten Stand sofort, wenn `repo` zum aktuellen `origin` passt. Nach jedem erfolgreichen Abruf überschreibt lazydev die Datei atomar. |
| Fehler | `gh` fehlt, ist nicht angemeldet oder endet mit Fehler: Die Statuszeile zeigt `Issues von <projekt> nicht geladen: <erste Zeile der Fehlermeldung>` als Fehler. Der bisherige Stand und die Datei bleiben unverändert. |
| Sichtbarkeit der Box | Nur, wenn das gewählte Projekt einen GitHub-`origin` und mindestens ein offenes Issue hat. |
| Rechte Spalte | Gemeinsame Spalte für „Benachrichtigungen“ und „Issues“, so breit wie die Projektbox: `min(max(24, Breite/4), 36)`. Die PRD-Box wird entsprechend schmaler. „Benachrichtigungen“ steht oben, „Issues“ darunter. Sind beide sichtbar, bekommt „Benachrichtigungen“ `topHeight/2` Zeilen und „Issues“ den Rest. Ist nur eine sichtbar, nimmt sie die volle Höhe. Ohne beide entfällt die Spalte. |
| Inhalt der Box | Titel `Issues · <anzahl>`. Eine Zeile pro Issue: `#<nummer>` in `styleMuted`, Leerzeichen, Titel, gekürzt mit `…`. |
| Indikator am Projekt | ` <anzahl>#` in `styleMuted` hinter dem Projektnamen, nach Docker-Symbol, `<n>●` und `<n>✉`. Ohne offene Issues entfällt er. |
| Fokus | Fünfter Fokus in der Reihenfolge Projekte → PRDs → Benachrichtigungen → Issues → Log. `Tab`, `Shift+Tab`, `←` und `→` überspringen unsichtbare Boxen. Verschwindet die Box bei Fokus darauf, geht der Fokus auf die PRDs. |
| Tasten in der Box | `j` `k` `↑` `↓` wählen. `Enter` öffnet das Fenster „Issue #<nummer>“. `o` öffnet das Issue im Browser. `n` startet den PRD-Chat aus dem Issue. Alle übrigen Tasten wirken wie im Normalmodus. |
| Fenster „Issue #<nummer>“ | Fenster über der TUI wie „PRD: …“. Oben die Zeilen `Titel`, `Erstellt` (`2006-01-02 15:04`, lokale Zeit) und `URL`, beschriftet wie `detailMeta`. Nach einer Leerzeile der Text als Markdown. Ohne Text steht dort `Kein Text` in `styleMuted`. Tasten: `j` `k` scrollen, `PgUp` `PgDn` Seite, `g` `G` Anfang und Ende, `o` Browser, `n` PRD-Chat, `Esc` und `q` schließen. Der Inhalt ändert sich nach dem Öffnen nicht. |
| Browser | `gh issue view <nummer> --repo <owner>/<repo> --web` im Hintergrund über `action`. Meldung `Issue #<nummer> im Browser geöffnet`, bei Fehler die erste Zeile der Fehlermeldung. |
| PRD aus Issue | `n` öffnet „Template wählen“ mit dem Template des Presets vorgewählt. `Enter` startet den Chat sofort, ohne Fenster „Neues PRD“. `Esc` bricht ab und verwirft den Bezug zum Issue. |
| Beschreibung des Chats | Prompt: `#<nummer> <titel>`, eine Leerzeile, dann der Text des Issues ohne führende und abschließende Leerzeichen. Ohne Text nur die erste Zeile. `chatrun.Create` bekommt als Beschreibung nur `#<nummer> <titel>`, damit die Box „Benachrichtigungen“ eine kurze Zeile zeigt. Titel des Chats wie bisher `Neues PRD (<Template>): <Projekt>`. |
| Maus | Klicks und Mausrad über der rechten Spalte bewirken nichts. |
| Fußzeile im Normalmodus | Mit Fokus auf der Box: `j/k wählen · Enter ansehen · o Browser · n PRD` statt der Tasten für Projekte. |
| Fußzeile im Fenster | `j/k scrollen · PgUp/PgDn Seite · g/G Anfang/Ende · o Browser · n PRD · Esc schließen`. |

## Scope

**In Scope**

- Neues Paket `internal/github` mit Erkennung des Remotes, Abruf über `gh`, Browser-Aufruf und Zwischenspeicher.
- Box „Issues“, gemeinsame rechte Spalte, Indikator, Fokus, Fenster „Issue #<nummer>“, PRD-Chat aus einem Issue.
- Abruf beim Start, alle 30 Minuten und mit `r`.
- Tests, `README.md` und `SPEC.md`.

**Out of Scope**

- GitHub-REST-API, Tokens in der Konfiguration und GitHub-Enterprise-Hosts.
- Andere Remotes als `origin` und andere Anbieter wie GitLab oder Gitea.
- Geschlossene Issues, Pull Requests, Filter nach Zuweisung, Labels oder Milestones und mehr als 30 Issues.
- Issues schließen, kommentieren, anlegen oder bearbeiten, auch nicht nach dem Merge eines PRDs.
- Eine dauerhafte Verknüpfung zwischen PRD und Issue über die Nummer in der Beschreibung hinaus.
- Box „Issues“ bei Projekten ohne offene Issues und Fehleranzeige in der Box.
- Mausbedienung der rechten Spalte.

## Umsetzung

1. **`internal/state/state.go`.** Ergänze `func IssuesPath(projectID string) string` mit dem Pfad
   `issues/<projekt-id>.json` unter `Dir()`.

2. **Neues Paket `internal/github`, Datei `github.go`.**
   - `var Binary = "gh"`: Tests ersetzen den Wert durch ein Fake-Skript.
   - `func ParseRemote(url string) (repo string, ok bool)`: liefert `<owner>/<repo>` für die Formen aus
     „Remote“, sonst `ok == false`. `.git` und abschließender `/` entfallen. Leere Teile ergeben `ok == false`.
   - `func Repo(dir string) (string, bool)`: `git.Run(dir, "remote", "get-url", "origin")`, Ausgabe getrimmt,
     dann `ParseRemote`. Ein Fehler von `git` ergibt `ok == false`.
   - `type Issue struct` mit den JSON-Feldern `number`, `title`, `body`, `url`, `createdAt` (`time.Time`).
   - `func List(dir, repo string) ([]Issue, error)`: führt den Befehl aus „Abruf“ mit `Binary` in `dir` aus.
     Bei Exit-Code ungleich 0 enthält der Fehler die getrimmte Ausgabe von stderr, sonst den Fehler von
     `exec`. Sortiert das Ergebnis absteigend nach `CreatedAt`.
   - `func OpenWeb(dir, repo string, number int) error`: Befehl aus „Browser“, Fehler wie bei `List`.

3. **`internal/github/cache.go`.**
   - `type Cache struct` mit den JSON-Feldern `repo`, `fetchedAt` (`time.Time`) und `issues` (`[]Issue`).
   - `func LoadCache(projectID string) (*Cache, error)` liest `state.IssuesPath`. Fehlt die Datei, liefert
     sie `nil, nil`.
   - `func SaveCache(projectID string, c *Cache) error` schreibt atomar über eine temporäre Datei im selben
     Ordner und `os.Rename`, wie `state.Save`. Den Ordner legt sie mit `os.MkdirAll` an.

4. **Neue Datei `internal/tui/issues.go`.**
   - `type issueSet struct { repo string; issues []github.Issue; fetching bool }` und
     `type issues map[string]*issueSet` (Projekt-ID → Stand). Ein Projekt ohne GitHub-`origin` hat einen
     Eintrag mit leerem `repo`.
   - `type issuesLoadedMsg struct { projectID, name, repo string; issues []github.Issue; listed bool; err error }`.
     `listed` ist wahr, wenn `github.List` erfolgreich war.
   - `const issuesInterval = 30 * time.Minute`, `type issuesTickMsg struct{}` und
     `func issuesTick() tea.Cmd` mit `tea.Tick`.
   - `func (m *Model) initIssues() tea.Cmd`: für jedes Projekt ohne Eintrag `github.Repo` aufrufen, Eintrag
     anlegen, bei passendem `repo` den Stand aus `github.LoadCache` übernehmen und den Abruf starten. Liefert
     die Abrufe als `tea.Batch`.
   - `func (m *Model) fetchIssues(p *project.Project) tea.Cmd`: ohne `repo` oder bei `fetching` nil. Sonst
     `fetching = true` und ein Befehl, der `github.List` aufruft, bei Erfolg `github.SaveCache` mit
     `fetchedAt = time.Now()` schreibt und `issuesLoadedMsg` liefert. Ein Fehler von `SaveCache` geht als
     `err` in die Nachricht, die geladenen Issues gelten trotzdem.
   - `func (m *Model) fetchAllIssues() tea.Cmd`: `github.Repo` für jedes Projekt neu lesen und `repo` im
     Eintrag aktualisieren, danach `fetchIssues` für alle.
   - `func (m *Model) applyIssues(msg issuesLoadedMsg)`: `fetching = false`. Bei `listed` den Stand übernehmen.
     Bei `err != nil` `m.setMessage("Issues von "+name+" nicht geladen: "+firstLine(…), true)`.
   - `func (m Model) projectIssues(projectID string) []github.Issue`, `func (m Model) showIssues() bool`,
     `func (m *Model) syncIssues()` analog zu `syncNotifications` mit `issueCursor`.
   - `func (m Model) viewIssues(l layout) string` nach „Inhalt der Box“.
   - `func (m Model) selectedIssue() (github.Issue, bool)` und
     `func (m Model) updateIssues(msg tea.KeyMsg) (next Model, cmd tea.Cmd, handled bool)` nach „Tasten in
     der Box“.
   - `func (m Model) openIssueWeb(issue github.Issue) (Model, tea.Cmd)` über `action`.
   - `func (m Model) prdFromIssue(issue github.Issue) (Model, tea.Cmd)`: setzt `m.fromIssue = &issue` und
     ruft `m.newPRD()` auf.

5. **Neue Datei `internal/tui/issueview.go`.**
   - `type issueView struct` mit `issue github.Issue`, `meta []string`, `viewport viewport.Model` und
     `md *markdown`.
   - `func (m Model) openIssueView(issue github.Issue) (Model, tea.Cmd)` setzt `modeIssueView`.
   - `func (m *Model) syncIssueView()` analog zu `syncPRDView`.
   - `func (m Model) viewIssueView() string` mit `dialog("Issue #<nummer>", …)`.
   - `func (m Model) updateIssueView(msg tea.KeyMsg) (tea.Model, tea.Cmd)` nach „Fenster „Issue #<nummer>““.
     `n` schließt das Fenster und ruft `prdFromIssue` auf.

6. **`internal/tui/interactive.go`.**
   - Ziehe den Start des Chats aus `updateNewPRD` in `func (m Model) startChat(display, desc string) (Model, tea.Cmd)`:
     `display` geht an `chatrun.Create`, `desc` an `CreateDisplay` und `CreatePrompt`. `updateNewPRD` ruft
     `startChat(desc, desc)` auf.
   - `updateTemplate`: Bei `Enter` mit `m.fromIssue != nil` die Beschreibung nach „Beschreibung des Chats“
     bilden, `m.fromIssue = nil` setzen und `startChat("#<nummer> <titel>", beschreibung)` aufrufen. Bei
     `Esc`, `Ctrl+C` und `q` `m.fromIssue = nil` setzen.

7. **`internal/tui/model.go`.**
   - Konstante `focusIssues` zwischen `focusNotifications` und `focusLog`.
   - Konstante `modeIssueView` mit Kommentar.
   - Felder `issues issues`, `issueCursor int`, `issueView *issueView` und `fromIssue *github.Issue`.
   - `Init` startet bei `watchEnabled` zusätzlich `issuesTick()`.

8. **`internal/tui/view.go`.**
   - `layout`: Ersetze `notifWidth` durch `sideWidth` (Breite der rechten Spalte, 0 ohne Spalte) und ergänze
     `notifHeight` und `issuesHeight` nach „Rechte Spalte“. Passe alle Stellen an, die `notifWidth` nutzen.
   - `viewNotifications` und `viewIssues` nutzen `l.sideWidth` und ihre eigene Höhe. `scrollWindow` in
     `viewNotifications` rechnet mit `l.notifHeight-2`.
   - `View` stapelt beide Boxen mit `lipgloss.JoinVertical` zur rechten Spalte und setzt sie mit
     `JoinHorizontal` rechts an. Ergänze im `switch` den Fall `modeIssueView` mit `viewIssueView`.
   - `viewProjects` ergänzt den Indikator nach „Indikator am Projekt“.
   - `helpLine`: Fälle `modeIssueView` und `focusIssues` nach den Fußzeilen in den Produktentscheidungen.
   - `helpSections`: In „Navigation“ den Fokus-Text um „Issues“ ergänzen. Neuer Abschnitt „Issues“:
     `{"Enter", "Issue im Fenster ansehen"}`, `{"o", "Issue im Browser öffnen"}`,
     `{"n", "PRD-Chat aus dem Issue starten"}`.

9. **`internal/tui/update.go`.**
   - `projectsLoadedMsg`: nach `applyChats` `m.initIssues()` als Befehl zurückgeben.
   - Neue Fälle `issuesLoadedMsg` (`applyIssues`) und `issuesTickMsg` (`fetchAllIssues` und `issuesTick()`).
   - `modeIssueView` in der Tastenverteilung an `updateIssueView`.
   - `updateNormal`: Bei Fokus auf der Box gehen die Tasten zuerst an `updateIssues`, wie bei
     `focusNotifications`. `r` startet zusätzlich `fetchAllIssues`.
   - `nextFocus` überspringt `focusIssues`, wenn `showIssues()` falsch ist. `syncIssues` läuft dort, wo
     `syncNotifications` läuft.
   - `handleMouse`: Die Prüfung auf die rechte Spalte nutzt `l.sideWidth`.
   - `tea.WindowSizeMsg` ruft `syncIssueView` wie `syncPRDView` auf.

10. **Tests.**
    - `internal/github/github_test.go`: `ParseRemote` tabellarisch mit allen drei Formen, mit und ohne
      `.git`, mit `/` am Ende, sowie `gitlab.com`, `github.example.com`, leerer Owner und lokaler Pfad als
      Negativfälle. `Repo` gegen ein temporäres Git-Repo mit und ohne `origin`. `List` mit einem Fake-`gh`
      als Shell-Skript über `Binary`: prüft die Argumente, die Sortierung nach `createdAt` und die Fehlermeldung
      mit stderr bei Exit-Code 1. `SaveCache` und `LoadCache` im Rundlauf, `LoadCache` ohne Datei.
    - `internal/tui/issues_test.go`: Box nur bei offenen Issues sichtbar. Beide Boxen teilen sich die Höhe nach
      „Rechte Spalte“, `sideWidth` gleich `leftWidth`. Fokus-Reihenfolge mit und ohne sichtbare Boxen.
      Indikator ` 2#` am Projekt, `ansi.StringWidth("#") == 1`. Fehler im `issuesLoadedMsg` lässt den alten
      Stand stehen und zeigt die Meldung. Zwischenspeicher mit anderem `repo` wird ignoriert. `Enter` öffnet
      das Fenster mit `Kein Text` bei leerem Text. `n` in der Box, dann `Enter` im Template-Fenster startet
      einen Chat, dessen Beschreibung `#<nummer> <titel>` ist und dessen Prompt den Text enthält
      (Fake-`claude` und `chatrun.SetSpawn` wie in `internal/tui/interactive_test.go`). `Esc` im
      Template-Fenster setzt `fromIssue` zurück. `checkViewSize` mit beiden Boxen bei 80 und 200 Spalten.
    - Bestehende Tests in `internal/tui/notifications_test.go`, die `notifWidth` prüfen, auf `sideWidth`
      umstellen.

11. **Doku.**
    - `SPEC.md`, „Layout“: rechte Spalte mit „Benachrichtigungen“ und „Issues“, Aufteilung der Höhe, Indikator
      `#`. „Tasten“: Zeile für `Enter` / `o` / `n` in „Issues“ und den Fokus-Text. Neuer Abschnitt
      „GitHub-Issues“ unter „Bedienung“ mit Remote-Erkennung, Abruf, Zeitpunkten, Fehlern, Fenster und PRD aus
      Issue. „Datenhaltung“: Zeile für den Zwischenspeicher. „Fenster über der TUI“: „Issue #<nummer>“ ergänzen.
    - `README.md`, „Voraussetzungen“: `gh` (angemeldet) für die Box „Issues“. „Erste Schritte“: Absatz zur Box.
      „Tasten“: `Enter` / `o` / `n` in „Issues“ und der Fokus-Text. „Dateien“: Zeile für den Zwischenspeicher.

## Technische Hinweise

- Alle Texte sind deutsch: UI-Strings, Fehlermeldungen, Kommentare. Kommentare kurz und im Präsens wie in den
  umliegenden Dateien.
- Git läuft immer über `git.Run`, nicht über eine Go-Bibliothek. Für GitHub gilt dasselbe mit `gh`.
- Die TUI liest sonst nur Dateien. `github.SaveCache` ist eine weitere schreibende Ausnahme, neben
  `chatrun.Create`, `Send`, `Stop` und `Remove`.
- `gh` und `git` laufen nie im `Update`-Pfad, sondern in `tea.Cmd`. Ausnahme: `github.Repo` in `initIssues`
  und `fetchAllIssues`. Der Aufruf ist lokal und kurz, wie `git` beim Laden der Projekte.
- Der 2-Sekunden-Tick (`tickMsg`) ruft `gh` nicht auf. Nur `issuesTickMsg` alle 30 Minuten, `r` und das erste
  Laden eines Projekts.
- `Init` startet `issuesTick()` nur bei `watchEnabled`, wie `tick()`. Tests senden `issuesLoadedMsg` und
  `issuesTickMsg` direkt.
- Bestehende Tests nutzen temporäre Repos ohne `origin`. Sie rufen `gh` deshalb nicht auf.
- `Model` wird per Wert kopiert. `issues` ist eine Map mit Zeigern, `fetching` überlebt so die Kopie.
  `issueView` bleibt ein Zeiger wie `prdView`.
- `gh issue list` liefert ohne `--repo` die Issues des Repos im aktuellen Ordner. lazydev übergibt `--repo`
  trotzdem, damit `gh` nicht ein anderes Remote als `origin` wählt.
- Der Text eines Issues kann HTML-Kommentare und sehr lange Zeilen enthalten. Der Renderer `markdown`
  bricht nach der Breite des Viewports um.
- Prüfe die Breite von `#` und `✉` im Indikator mit `ansi.StringWidth` (siehe Commit `ba0179a`).
- Eine laufende TUI startet Chat-Runden mit dem installierten Binary. Teste den Chat aus einem Issue nach
  `make install`.
- Tests isolieren sich über `t.Setenv("XDG_STATE_HOME", t.TempDir())` und `XDG_DATA_HOME`. Git-Tests setzen
  `GIT_AUTHOR_*` und `GIT_COMMITTER_*`.

## Erfolgskriterien

- `make test` läuft ohne Fehler, also `go vet ./...` und `go test ./...`.
- `go test ./internal/github ./internal/tui` läuft ohne Fehler.
- In einem Projekt mit `origin` auf `github.com` und offenen Issues zeigt die TUI nach dem Start die Box
  „Issues · <anzahl>“ mit höchstens 30 Zeilen, neueste oben, und das Projekt zeigt ` <anzahl>#`.
- Ein Projekt ohne `origin` oder mit einem anderen Host zeigt weder Box noch Indikator, und lazydev ruft
  `gh` dafür nicht auf.
- Nach `q` und Neustart erscheint der letzte Stand sofort aus `issues/<projekt-id>.json`.
- Ohne `gh` im `PATH` zeigt die Statuszeile `Issues von <projekt> nicht geladen: …`. Die TUI bleibt bedienbar.
- `Enter` in der Box zeigt das Issue im Fenster. `o` öffnet es im Browser.
- `n` in der Box und `Enter` im Fenster „Template wählen“ starten einen Chat. Dessen erste Nachricht enthält
  `#<nummer> <titel>` und den Text des Issues. Bei einer Rückfrage zeigt „Benachrichtigungen“ die Zeile
  `#<nummer> <titel>`.
- Mit Benachrichtigungen und Issues teilen sich beide Boxen die rechte Spalte. Ohne beide hat die PRD-Box
  dieselbe Breite wie heute.
- `README.md` und `SPEC.md` beschreiben Box, Indikator, Tasten, Fenster, Abruf und Zwischenspeicher.
