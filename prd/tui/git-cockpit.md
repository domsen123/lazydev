# PRD: Git-Cockpit für Fetch, Pull, Push und Commit

> Status: geplant · Scope: Modal `alt+g` (auch im Menü) mit Branch, Änderungen, letzten Commits und den Aktionen Fetch, Pull (`--ff-only`), Push und Commit. Jede Aktion läuft als Terminal-Tab über das interne Kommando `lazydev git`, Pull und Push unter dem Merge-Lock.

## Context

Für Standardaufgaben wie `git push` oder `git pull` im Hauptordner eines Projekts musst du heute einen
Shell-Tab öffnen (`alt+t`) und die Befehle tippen. lazydev zeigt den Stand des Branches bereits in der
linken Spalte (`readGitInfo` in `internal/tui/git.go`, `main ↑2 ↓1`), bietet aber keine Aktion dazu an.
`SPEC.md` nennt „pushing“ sogar als Nicht-Ziel.

Nach der Änderung öffnet `alt+g` ein Modal „Git“ für das ausgewählte Projekt. Das Modal zeigt Branch,
Upstream, `↑n ↓n`, die Zahl geänderter Dateien und die letzten 5 Commits. Es bietet die Aktionen Fetch,
Pull, Push und Commit an. Jede Aktion öffnet einen Terminal-Tab in der Mitte. Darin läuft das neue
interne Kommando `lazydev git <id> <dir> <action> [<message>]`. Abfragen von SSH-Passphrase oder
Zugangsdaten funktionieren deshalb wie in einer Shell. Bei Erfolg schließt sich der Tab, und die
Fußzeile meldet `git <action>: done`. Bei einem Fehler bleibt der Tab offen, bis du `enter` drückst.
Pull und Push halten den Merge-Lock des Projekts, damit sie nie gleichzeitig mit dem Fast-Forward eines
Runners laufen.

Wiederverwendet werden:

- `overlay()` und `box()` (`internal/tui/menu.go`, `internal/tui/main.go`) sowie das Muster des
  Status-Pickers (`internal/tui/status.go`: Struktur am Model, `updateStatus`, `viewStatusPicker`,
  `keys()`).
- `openTerminal()` (`internal/tui/main.go`) und `Terminal.SetLabel()` (`internal/terminal/terminal.go`),
  wie beim Tab `create` in `internal/tui/create.go`.
- `git.Run`, `git.Output`, `git.CurrentBranch` (`internal/git/git.go`).
- `state.TryLock` (`internal/state/lock.go`) mit dem Lock-Namen `<id>.merge`, den auch
  `waitMergeLock` in `internal/runner/merge.go` nutzt.
- `fetchGitInfo()` (`internal/tui/git.go`), um die linke Spalte nach einer Aktion sofort zu aktualisieren.
- `projectIDs()` für die Projekt-ID, wie in `updatePRDs` (`internal/tui/keymap.go`).
- Das Muster des internen Kommandos `run` in `cmd/lazydev/main.go`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Aktionen | Fetch, Pull, Push, Commit. Keine weiteren Aktionen. |
| Ausführung | Jede Aktion öffnet einen Terminal-Tab in der Mitte im Hauptordner des ausgewählten Projekts. Der Tab startet `<os.Executable()> git <id> <dir> <action> [<message>]`. Das Modal schließt sich beim Start der Aktion. |
| Tab-Label | Fest `git fetch`, `git pull`, `git push` oder `git commit` über `SetLabel`. |
| Tab-Ende bei Erfolg | `lazydev git` endet mit Exit-Code 0, der Tab schließt sich wie jeder Terminal-Tab. Die Fußzeile zeigt die Notiz `git <action>: done`. |
| Tab-Ende bei Fehler | `lazydev git` schreibt eine Leerzeile und `failed (exit <n>), press enter to close`, wartet auf eine Zeile von stdin und endet dann mit `<n>`. Nach dem Schließen zeigt die Fußzeile den Fehler `git <action> failed (exit <n>)`. |
| Tab vorzeitig geschlossen | `alt+w` beendet den Prozess mit SIGHUP. Exit-Code `-1` aus `ProcessState.ExitCode()`. Die Fußzeile zeigt den Fehler `git <action> stopped`. |
| Fetch | `git fetch`. Kein Lock. |
| Pull | `git pull --ff-only`. Eine Divergenz ist ein Fehler von Git und bleibt im Tab stehen. |
| Push mit Upstream | `git push`. |
| Push ohne Upstream | `git push -u origin <branch>`. Ein fehlendes Remote `origin` ist ein Fehler von Git. |
| Upstream erkennen | `git rev-parse --abbrev-ref --symbolic-full-name @{upstream}`. Ein Fehler bedeutet: kein Upstream. |
| Commit | `git add -A`, danach `git commit -m <message>`. Das nimmt alle Änderungen auf, auch ungetrackte Dateien, `.lazydev/` und `prd/`. Kein Lock. |
| Commit-Nachricht | Eingabefeld im Modal mit dem Prompt `Message: `. `enter` mit leerer oder nur aus Leerzeichen bestehender Nachricht startet nichts und zeigt den Fehler `Enter a commit message.`. Die Nachricht geht unverändert ohne Trailer an Git. |
| Nichts zu committen | Kein eigener Check. Git meldet `nothing to commit`, der Tab bleibt nach den Fehlerregeln offen. |
| Branch | Alle Aktionen wirken auf den Branch, der im Hauptordner ausgecheckt ist. |
| Detached HEAD | Pull und Push schreiben `HEAD is detached, check out a branch first.` und enden mit Exit-Code 1. Fetch und Commit laufen. |
| Merge-Lock | Pull und Push nehmen `state.TryLock(<id> + ".merge")`, bevor Git startet, und geben ihn nach Git frei. Hält ein Runner den Lock, schreibt `lazydev git` `A PRD of <id> merges right now. Try again once the merge is done.` und endet mit Exit-Code 1. Es wartet nicht. |
| Öffnen | `alt+g` global, auch in einem Terminal und im Log-Tab. Zusätzlich der Eintrag „Git“ im Menü (`alt+m`) zwischen „Color theme“ und „Quit“. |
| Taste `g` ohne `alt` | Öffnet das Cockpit nie. `g` bleibt „nach oben“ in Listen. Der Fallback von Regel 2 in `updateKey` überspringt `g`. |
| Ohne Projekt | `alt+g` und der Menüeintrag zeigen den Fehler `Add a folder first: a in the left column`. |
| Ohne Git | Ist der Ordner kein Git-Repository, öffnet sich kein Modal. Fehler: `<name> is no git repository.`, `<name>` wie in `projectEntry.name()`. |
| Andere offene Dialoge | Solange Menü, Frage, Status-Picker, Wizard, Projekt-Eingabe, PRD-Eingabe oder Entfernen-Frage offen sind, öffnet `alt+g` nichts. Diese Dialoge bekommen die Tasten wie bisher zuerst. |
| Modal-Titel | `Git · <name>`. |
| Modal-Breite | `min(m.width − 4, 64)`. |
| Modal-Inhalt | Zeile `Branch    <branch> → <upstream> ↑n ↓n`. Ohne Upstream `<branch> · no upstream`, bei detached HEAD `detached at <7 Zeichen>`. `↑n` und `↓n` stehen nur bei Werten größer 0. Zeile `Changes   clean` oder `Changes   <n> files`, `<n>` zählt die Zeilen von `git status --porcelain` inklusive ungetrackter Dateien. Leerzeile. Die vier Aktionen mit ihrer Taste. Leerzeile. Überschrift `Recent commits` und bis zu 5 Zeilen von `git log --oneline -5`, ohne Commits die Zeile `no commits`. |
| Darstellung der Aktionen | Die ausgewählte Aktion zeigt `> ` und ist in `selectedFg` auf `selectedBg` fett, wie im Status-Picker. Die anderen zeigen `  `. Format `Fetch     f`, `Pull      p`, `Push      P`, `Commit    c`. |
| Stand im Modal | lazydev liest Branch, Upstream, Änderungen und Commits einmal beim Öffnen, synchron. Das Modal aktualisiert sich nicht. |
| Tasten im Modal | `j`/`down` und `k`/`up` bewegen, `enter` führt die Aktion aus, `f` Fetch, `p` Pull, `P` Push, `c` Commit, `esc`, `q`, `ctrl+c` und `alt+g` schließen. |
| Tasten im Eingabefeld | Alle druckbaren Tasten gehen ins Feld, auch `f`, `p`, `q`. `enter` startet den Commit, `esc` kehrt zur Aktionsliste zurück. |
| Fußzeile im Modal | Liste: ` j/k move · enter run · f fetch · p pull · P push · c commit · esc close`. Eingabefeld: ` enter commit · esc back`. Fehler und Notizen haben wie bisher Vorrang. |
| Tastenhilfe außerhalb | Jede Zeile von `footerKeys()` bekommt ` · alt+g git` direkt vor ` · alt+m menu` beziehungsweise ` · m menu`. |
| Aktualisierung danach | Endet ein Git-Tab, liest lazydev die Git-Info aller Projekte sofort mit `fetchGitInfo`. |
| Abgrenzung zu `SPEC.md` | „pushing“ fällt aus den Nicht-Zielen. „pull requests“ bleibt. Der Satz „lazydev commits nothing under `.lazydev/`“ gilt weiter für alles, was lazydev selbst committet. Der Commit des Cockpits nimmt `.lazydev/` auf, weil du ihn auslöst. |

## Scope

**In Scope**

- Neues Paket `internal/gitcmd` mit den vier Aktionen, Lock und Fehlerausgabe.
- Internes Kommando `lazydev git` in `cmd/lazydev/main.go`.
- Exit-Code in `terminal.ExitMsg`.
- Modal `Git` in `internal/tui/gitcockpit.go` mit Eingabefeld für die Commit-Nachricht.
- Taste `alt+g`, Menüeintrag „Git“, Fußzeilen-Hilfe.
- Notiz oder Fehler nach dem Ende eines Git-Tabs und sofortige Aktualisierung der Git-Info.
- Tests für `internal/gitcmd`, `internal/terminal` und `internal/tui`.
- `SPEC.md`, `README.md` und `README.de.md`.

**Out of Scope**

- Branch wechseln, `stash`, Staging einzelner Dateien, Diff-Ansicht.
- Force-Push, Pull mit Rebase oder Merge, Auswahl des Remotes.
- Git-Aktionen in Worktrees von PRDs.
- Aktualisierung des Modals, während es offen ist.
- Eigene Prüfung auf „nothing to commit“ oder auf fehlende Remotes vor dem Start.
- Ein CLI-Kommando für Nutzer: `lazydev git` ist intern und erscheint nicht in `lazydev help`.

## Umsetzung

1. **Exit-Code in `internal/terminal/terminal.go`.** Erweitere `ExitMsg` zu
   `type ExitMsg struct{ ID, Code int }` und passe den Kommentar an: `Code` ist der Exit-Code des
   Programms, `-1` nach einem Signal. Setze in der Output-Goroutine nach `cmd.Wait()`
   `code := cmd.ProcessState.ExitCode()` und sende `ExitMsg{id, code}`. Ist `cmd.ProcessState` nil,
   ist `code` `-1`. Passe `internal/terminal/terminal_test.go` an: Der bestehende Test prüft
   `ExitMsg{ID: 7, Code: 0}`. Ergänze einen Test, der `sh -c "exit 3"` startet und `Code: 3` erwartet.
   Alle anderen Stellen, die `ExitMsg` erzeugen oder vergleichen, zieht der Compiler nach.

2. **Paket `internal/gitcmd`.** Lege `internal/gitcmd/gitcmd.go` mit Paketkommentar an
   („Package gitcmd runs the actions of the git cockpit in a terminal tab.“). Inhalt:
   - Konstanten der Aktionen `Fetch = "fetch"`, `Pull = "pull"`, `Push = "push"`, `Commit = "commit"`.
   - `func Run(id, dir, action, message string, in io.Reader, out io.Writer) int`. Sie führt die Aktion
     aus und gibt den Exit-Code zurück. Jeder Git-Aufruf ist ein `exec.Command("git", …)` mit
     `Dir = dir`, `Stdin = in`, `Stdout = out`, `Stderr = out`. Vor jedem Aufruf schreibt sie
     `$ git <args>` und einen Zeilenumbruch nach `out`. Der Exit-Code eines fehlgeschlagenen Aufrufs ist
     `ExitCode()` des `*exec.ExitError`, bei anderen Fehlern 1 mit der Fehlermeldung in `out`.
   - Fetch: `git fetch`.
   - Pull: Branch mit `git.CurrentBranch(dir)`, leer → `HEAD is detached, check out a branch first.`
     und 1. Dann Lock (unten), dann `git pull --ff-only`.
   - Push: Branch wie bei Pull, dann Lock, dann `git.Run(dir, "rev-parse", "--abbrev-ref",
     "--symbolic-full-name", "@{upstream}")`. Ohne Fehler `git push`, sonst `git push -u origin <branch>`.
   - Commit: `strings.TrimSpace(message)` leer → `Enter a commit message.` und 1. Sonst `git add -A`.
     Bei Erfolg `git commit -m <message>` mit der unveränderten Nachricht.
   - Lock für Pull und Push: `unlock, err := state.TryLock(id + ".merge")`. Bei `err` die Meldung und 1.
     Bei `unlock == nil` `A PRD of <id> merges right now. Try again once the merge is done.` und 1.
     Sonst `defer unlock()`.
   - Unbekannte Aktion: `unknown git action: <action>` und 1.
   - `func Main(id, dir, action, message string) int`. Sie ruft
     `Run(id, dir, action, message, os.Stdin, os.Stdout)` auf. Bei Code ungleich 0 schreibt sie eine
     Leerzeile und `failed (exit <n>), press enter to close`, liest mit `bufio.NewReader(os.Stdin)` eine
     Zeile und gibt den Code zurück.

3. **Tests `internal/gitcmd/gitcmd_test.go`.** Isoliere mit `t.Setenv("XDG_STATE_HOME", t.TempDir())`
   und setze `GIT_AUTHOR_*` und `GIT_COMMITTER_*`. Baue pro Test ein Bare-Repo als `origin` und einen
   Klon in `t.TempDir()`. Rufe `Run` mit `in = nil` und `out = &bytes.Buffer{}` auf. Tests:
   - Commit nimmt eine geänderte und eine ungetrackte Datei auf, Code 0, `git status --porcelain` leer
     danach, die Nachricht steht unverändert in `git log -1 --format=%B`.
   - Commit mit leerer Nachricht: Code 1, Ausgabe enthält `Enter a commit message.`, kein neuer Commit.
   - Commit ohne Änderungen: Code ungleich 0.
   - Push ohne Upstream: Ausgabe enthält `$ git push -u origin main`, danach hat der Branch einen Upstream.
   - Push mit Upstream: Ausgabe enthält `$ git push` ohne `-u`.
   - Pull holt einen Commit, den ein zweiter Klon gepusht hat, Code 0.
   - Pull bei Divergenz: Code ungleich 0, kein Merge-Commit.
   - Pull und Push bei detached HEAD: Code 1 und die Meldung.
   - Pull und Push bei gehaltenem Lock: Der Test nimmt `state.Lock(id + ".merge")`, Code 1, Ausgabe
     enthält `merges right now`, kein Git-Aufruf in der Ausgabe.
   - Fetch bei gehaltenem Lock: Code 0.
   - Unbekannte Aktion: Code 1.

4. **Internes Kommando in `cmd/lazydev/main.go`.** Ergänze im `switch cmd` vor `case "trust"`:
   `case "git":` mit Kommentar `// "lazydev git <id> <dir> <action> [<message>]" runs an action of the
   git cockpit in a terminal tab.` Bei `len(rest)` ungleich 4 oder 5 `usageError()`. Sonst
   `os.Exit(gitcmd.Main(rest[0], rest[1], rest[2], message))`, `message` ist `rest[3]` bei 5 Argumenten,
   sonst leer. Der Text `usage` ändert sich nicht.

5. **Modal `internal/tui/gitcockpit.go`.** Lege die Datei mit einem Kommentarblock oben an, wie in
   `internal/tui/status.go`. Inhalt:
   - `const cockpitWidth = 64`.
   - Aktionen in fester Reihenfolge Fetch, Pull, Push, Commit mit Label und Taste.
   - `type cockpit struct` mit `dir`, `id`, `name`, `branch`, `upstream`, `ahead`, `behind`, `changes`,
     `commits []string`, `cursor`, `committing bool` und `input textinput.Model`.
   - `func (m *model) openCockpit()`: ohne Projekt der Fehler `Add a folder first: a in the left column`.
     `git.Run(p.path, "rev-parse", "--git-dir")` mit Fehler → `<name> is no git repository.`. Sonst
     Branch über `git.CurrentBranch`, bei leerem Ergebnis `detached at <git rev-parse --short=7 HEAD>`.
     Upstream über `git rev-parse --abbrev-ref --symbolic-full-name @{upstream}`. `ahead` und `behind`
     aus `p.git` (`gitInfo`). `changes` = `len(git.Output(p.path, "status", "--porcelain"))`. `commits`
     = `git.Output(p.path, "log", "--oneline", "-5")`. Die ID kommt aus `projectIDs(m.projects)[m.selected]`.
   - `func (m model) updateCockpit(msg tea.KeyMsg) (tea.Model, tea.Cmd)` nach den Tasten der
     Produktentscheidungen. Im Eingabefeld leitet sie alle Tasten außer `enter` und `esc` an
     `input.Update` weiter. `c` und `enter` auf Commit setzen `committing = true`, leeren das Feld und
     fokussieren es.
   - `func (m *model) runGitAction(action, message string)`: schließt das Modal und ruft `openTerminal`
     mit `[exe, "git", id, dir, action]` auf, bei Commit zusätzlich `message`. `exe` kommt aus
     `os.Executable()`, ein Fehler davon wird zu `m.err`. Danach setzt sie auf dem neuen Tab
     (`p.tabs[p.tab]` als `*terminal.Terminal`) `SetLabel("git " + action)` und trägt die Terminal-ID in
     `m.gitTabs[id] = action` ein. Schlägt `openTerminal` fehl, bleibt `m.err` stehen, kein Eintrag.
   - `func (m model) viewCockpit() string` nach Modal-Inhalt und Darstellung der Produktentscheidungen,
     mit `box("Git · "+name, …, width, len(lines)+2, t.focus, true, t)`. Zeilen mit 1 Leerzeichen
     Einrückung, wie `viewStatusPicker`. Im Eingabefeld ersetzt die Zeile `Message: <Feld>` die Aktionen.
   - `func (c *cockpit) keys() string` mit den Fußzeilen-Texten.

6. **Model und Routing.**
   - `internal/tui/main.go`: Feld `cockpit *cockpit` mit Kommentar nach `status`. Feld
     `gitTabs map[int]string` mit Kommentar („gitTabs maps terminal IDs of git cockpit tabs to their
     action.“), in `newModel` initialisiert. In `Update` bei `terminal.ExitMsg`: Steht `msg.ID` in
     `m.gitTabs`, entferne den Eintrag und setze bei `Code == 0` `m.note = "git <action>: done"`, bei
     `Code == -1` `m.err = "git <action> stopped"`, sonst `m.err = "git <action> failed (exit <n>)"`.
     Danach `removeTerminal` wie bisher und `return m, fetchGitInfo(m.projects)`. In `View` das Overlay
     `viewCockpit()` nach dem Status-Picker und vor dem Menü. In `footer` ein Fall
     `m.cockpit != nil && m.menu == nil` mit `m.cockpit.keys()` direkt nach dem Fall des Status-Pickers.
     In `centerBody` gilt `m.cockpit == nil` zusätzlich als Bedingung für den Fokus des Tabs, wie
     `m.status == nil`.
   - `internal/tui/keymap.go`: Neue Aktion `actGit`, `"alt+g": actGit` in `globalKeys`, in `global`
     `case actGit: m.openCockpit()`. In `updateKey` der Fall `case m.cockpit != nil: return m.updateCockpit(msg)`
     direkt nach dem Menü. Der Fallback „Rule 2“ am Ende von `updateKey` läuft nur bei `key != "g"`, mit
     Kommentar, warum. `footerKeys()` bekommt in jeder Zeile ` · alt+g git` nach den Produktentscheidungen.
   - `internal/tui/menu.go`: Konstante `menuGit` zwischen `menuTheme` und `menuQuit`. `viewMenu` zeigt
     zwischen dem Theme und „Quit“ die Zeile `row(menuGit, "Git")` mit Leerzeile davor. `enter` und `space`
     auf `menuGit` setzen `m.menu = nil` und rufen `m.openCockpit()` auf. Passe den Typkommentar von
     `menu` an.

7. **Tests `internal/tui/gitcockpit_test.go`.** Isoliere mit `XDG_STATE_HOME`, `XDG_DATA_HOME`,
   `XDG_CONFIG_HOME` und `GIT_*` wie in den bestehenden Tests. Baue das Model nach dem Muster von
   `TestViewSmoke` in `internal/tui/layout_test.go` mit einem Projekt in einem temporären Git-Repo. Tests:
   - `alt+g` öffnet das Cockpit, der View enthält `Git · `, `Branch`, `Changes`, `Recent commits` und
     die vier Aktionen.
   - `esc`, `q` und `alt+g` schließen es.
   - `j`, `k` bewegen den Cursor und klemmen an den Enden.
   - `c` öffnet das Eingabefeld, `f` landet darin als Text und startet kein Fetch, `esc` kehrt zur Liste
     zurück.
   - `enter` mit leerer Nachricht: Fehler `Enter a commit message.`, das Cockpit bleibt offen.
   - Ordner ohne Git: Fehler `<name> is no git repository.`, kein Cockpit.
   - Plain `g` im Details-Pane ohne Runs öffnet kein Cockpit.
   - `terminal.ExitMsg` mit einer ID aus `gitTabs`: Code 0 setzt die Notiz, Code 2 den Fehler
     `git push failed (exit 2)`, Code -1 `git push stopped`. Der Eintrag verschwindet aus `gitTabs`.
   - Menü: Cursor auf „Git“ und `enter` öffnen das Cockpit.

8. **`SPEC.md`.**
   - Abschnitt „Goal“: „pushing“ aus den Nicht-Zielen streichen.
   - Abschnitt `.lazydev/`: Der Satz „lazydev commits nothing under `.lazydev/`.“ wird zu „lazydev commits
     nothing under `.lazydev/` by itself. The commit of the git cockpit takes all changes, `.lazydev/`
     included.“
   - Abschnitt „UI“, nach „### Log tab“: neuer Abschnitt `### Git cockpit (alt+g)` mit Öffnen, Inhalt,
     Tasten, den vier Aktionen mit ihren Git-Befehlen, Tab-Label, Verhalten bei Erfolg und Fehler,
     Merge-Lock, detached HEAD und allen Meldungen aus den Produktentscheidungen.
   - Abschnitt „Processes“: `lazydev git <id> <dir> <action> [<message>]` läuft im Terminal-Tab, Pull und
     Push nehmen `locks/<id>.merge` ohne Warten.
   - Abschnitt „CLI“: „`run` is internal“ wird zu „`run` and `git` are internal“.

9. **`README.md` und `README.de.md`.** Gleicher Inhalt in beiden Sprachen:
   - Tabelle „Keys“: neue Zeile `alt+g` | git cockpit: fetch, pull, push, commit. Die Zeile von `alt+m`
     nennt „menu (color theme, git)“.
   - Neuer Abschnitt „Git cockpit“ nach „Runs“: Aktionen mit Git-Befehlen, Tab schließt bei Erfolg und
     bleibt bei Fehler offen, Pull und Push warten nicht auf einen laufenden Merge, Commit nimmt alle
     Änderungen auf.

## Technische Hinweise

- Alle UI-Strings, Fehler, Kommentare und die Doku sind Englisch. Nur `README.de.md` ist Deutsch.
- Prüfe `SPEC.md`, `README.md` und `README.de.md` zusammen mit dem Code, `SPEC.md` ist verbindlich.
- `lazydev git` läuft im PTY des Tabs. `Stdin` von Git ist das PTY, deshalb funktionieren Abfragen von
  `ssh` und Credential-Helpern. Setze kein `GIT_TERMINAL_PROMPT=0` und kein SSH `BatchMode`.
- `exec.Cmd` mit `Stdin` als `io.Reader`, der keine `*os.File` ist, kopiert den Reader in einer
  Goroutine. Deshalb wartet `Main` und nicht `Run` auf `enter`. Tests übergeben `in = nil`.
- `state` und `git` dürfen `gitcmd` nicht importieren. `gitcmd` importiert `state` und `git`, nicht
  umgekehrt.
- `os.Executable()` reicht für den Tab. Die Kopie nach `$XDG_STATE_HOME/lazydev/bin/` aus
  `runner.spawn` braucht es nicht: Der Prozess ist kurz und endet vor einem neuen Build.
- Der Runner hält `locks/<id>.merge` während des ganzen Merge-Schritts, also auch während er auf einen
  sauberen Hauptordner wartet. In dieser Zeit melden Pull und Push den Lock-Fehler. Commit nimmt keinen
  Lock, damit du einen schmutzigen Hauptordner für den wartenden Merge aufräumen kannst.
- Ein Terminal-Tab schließt sich heute bei jedem Ende seines Programms (`removeTerminal`). Das Verhalten
  bei Fehler entsteht allein durch das Warten in `gitcmd.Main`, `removeTerminal` ändert sich nicht.
- Regel 2 in `updateKey` prüft für unbenutzte Tasten `"alt+" + key`. Ohne die Ausnahme für `g` öffnet
  plain `g` in Panes ohne Liste das Cockpit. Das widerspricht Regel 3, `g` heißt überall „nach oben“.
- Commits, die lazydev selbst macht, nutzen `git commit` ohne Trailer. Das gilt auch für den Commit des
  Cockpits.
- Git-Tests setzen `GIT_AUTHOR_*` und `GIT_COMMITTER_*`. Setze im Bare-Repo und im Klon `main` als
  Branch, z. B. mit `git init -b main`, damit die erwarteten Befehle `origin main` enthalten.

## Erfolgskriterien

- `make test` läuft ohne Fehler, inklusive `go vet ./...`.
- `go test ./internal/gitcmd ./internal/terminal ./internal/tui` enthält die Tests aus den Schritten 1,
  3 und 7 und alle sind grün.
- `make build` erzeugt `bin/lazydev`.
- `bin/lazydev git <id> <dir> push` in einem Repo ohne Upstream führt `git push -u origin <branch>` aus.
- In der TUI öffnet `alt+g` das Modal aus jedem Pane, auch aus einem Terminal-Tab. Plain `g` öffnet es
  nie.
- Ein erfolgreicher Push schließt seinen Tab, und die Fußzeile zeigt `git push: done`.
- Ein Pull bei Divergenz lässt den Tab mit `failed (exit <n>), press enter to close` offen. Nach `enter`
  zeigt die Fußzeile `git pull failed (exit <n>)`.
- Während ein Runner im Merge-Schritt steht, enden Pull und Push mit der Lock-Meldung.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben das Cockpit, `SPEC.md` nennt „pushing“ nicht
  mehr als Nicht-Ziel.
