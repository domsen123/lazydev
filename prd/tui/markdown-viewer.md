# PRD: Markdown-Viewer als Tab

> Status: geplant · Scope: `alt+v` öffnet die gewählte Markdown-Datei (Tab `Files`) oder das gewählte PRD (Tab
> `PRDs`) als mit Glamour gerendertes Markdown in einem eigenen Center-Tab, der sich bei Änderungen der Datei neu rendert.

## Context

Markdown-Dateien und PRDs lassen sich in lazydev heute nur im Editor öffnen: `l`/`enter` im Tab `Files` oder
`PRDs` startet `$VISUAL`/`$EDITOR`/`vi` als Terminal-Tab (`editorCommand` in `internal/tui/files.go`,
`updateRight` und `updatePRDs` in `internal/tui/keymap.go`). Ein PRD mit Tabellen, Überschriften und langen
Absätzen ist dort als Rohtext schwer zu lesen. Der Nutzer will eine Datei lesen, ohne lazydev zu verlassen.

Nach der Änderung öffnet `alt+v` die Auswahl der rechten Spalte als dritte Art von Center-Tab, neben Terminal-
und Log-Tabs: einen Markdown-Viewer. Er rendert die Datei mit Glamour auf die Tab-Breite, scrollt wie der
Log-Tab, rendert neu, sobald sich die Datei ändert, und öffnet mit `e` den Editor.

Im Repo existiert bereits und wird wiederverwendet:

- `centerTab` (`internal/tui/tabs.go`): Schnittstelle der Center-Tabs (`ID`, `TabTitle`, `Resize`, `View`).
- `logView` (`internal/tui/logview.go`): Vorbild für einen Tab ohne Prozess: Zeilen-Cache nach Breite und Theme,
  `scroll`, `update(key) bool`, `View` mit exakter Größe, `readLogs`/`applyLogs` als Hintergrundlesen pro
  Sekunde, `closeEndedLogs`.
- `openLog` (`internal/tui/main.go`): fokussiert einen offenen Tab oder hängt einen neuen an.
- `inLogView`, `activeLog`, `footerKeys`, `global`, `globalKeys` (`internal/tui/keymap.go`,
  `internal/tui/projects.go`).
- `editorCommand` (`internal/tui/files.go`) und `openTerminal` (`internal/tui/main.go`) für `e`.
- `fileTree.selected` (`internal/tui/files.go`) und `projectEntry.selectedPRD` (`internal/tui/prds.go`). Ein PRD,
  das ein Lauf in seinen Worktree verschoben hat, hat `d.Root` = Worktree, sonst den Hauptordner; der Pfad ist
  `filepath.Join(d.Root, d.Path)` wie in `updatePRDs`.
- `runTickMsg` (`internal/tui/main.go`): sekündlicher Tick, der auch `readLogs` anstößt.
- `updateMouse` und `wheelLines` (`internal/tui/mouse.go`), `termOrigin` und `termSize` (`internal/tui/main.go`).
- `themes` (`internal/tui/theme.go`) mit den Namen `Default` und `Tokyo Night`.

`prd/tui/prd-ansicht.md` beschrieb ein älteres Overlay mit Glamour. Es stammt aus einer früheren Codebasis, der
Code existiert nicht mehr. Dieses PRD ersetzt die Idee durch einen Tab.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Taste | `alt+v` als neue globale Aktion `actView`, wirkt überall, auch im Terminal und im Log-Tab. Nach Regel 2 wirkt außerhalb eines Terminals auch `v`, weil keine Spalte `v` selbst nutzt. Im Copy-Mode bleibt `v` die Auswahl, denn Copy-Mode-Tasten laufen vor Regel 2. Während Menü, Git-Cockpit, Frage, Status-Picker, Wizard oder eine Eingabe offen ist, wirkt `alt+v` nicht (die bestehende Reihenfolge in `updateKey` sorgt dafür). |
| Welche Datei | Die Auswahl im aktiven Tab der rechten Spalte des gewählten Projekts, egal welche Spalte den Fokus hat. Tab `Files`: die markierte Zeile. Tab `PRDs`: das markierte PRD, Pfad `filepath.Join(d.Root, d.Path)`, bei einem Lauf also die Datei im Worktree. |
| Fehler beim Öffnen | Ohne Projekt: `Add a folder first: a in the left column`. Tab `Files` mit Ordner oder leerem Baum: `Select a Markdown file in the Files tab.`. Tab `PRDs` auf der Gruppenzeile oder ohne PRDs: `Select a PRD in the PRDs tab.`. Endung nicht `.md` oder `.markdown` (Groß-/Kleinschreibung egal): `<dateiname> is no Markdown file.`. Datei nicht lesbar: `Cannot read <dateiname>: <erste Zeile des Fehlers>`. In allen Fällen öffnet sich kein Tab, der Fehler steht im Footer (`m.err`). `<dateiname>` ist `filepath.Base(path)`. |
| Tab | Ein neuer Center-Tab des gewählten Projekts, angehängt rechts, wird aktiv, die Mitte bekommt den Fokus. Er zählt in der Tab-Zahl des Projekts wie Log-Tabs. Titel: `md <dateiname>`, z. B. `md README.md`. |
| Doppeltes Öffnen | Ist im gewählten Projekt schon ein Viewer-Tab mit demselben absoluten Pfad offen, fokussiert `alt+v` diesen Tab und öffnet keinen neuen. Er liest die Datei dabei nicht neu, das macht die sekündliche Prüfung. |
| Rendern | Mit `github.com/charmbracelet/glamour` (neue Abhängigkeit). Fester Stil nach Theme: `Default` → `styles.DarkStyleConfig`, `Tokyo Night` → `styles.TokyoNightStyleConfig`. Umbruch mit `glamour.WithWordWrap(<Tab-Breite>)`. Keine automatische Stilerkennung. Ein Theme-Wechsel im Menü rendert den Tab neu. |
| Darstellung | Nur das gerenderte Markdown, ohne Kopfzeile. Zeilen breiter als der Tab werden abgeschnitten, kürzere mit Leerzeichen aufgefüllt, so dass `View` genau die Größe aus `Resize` füllt. Ein Fehler von Glamour zeigt stattdessen den Rohtext mit Tabs als vier Leerzeichen, umgebrochen mit `ansi.Wrap` auf die Tab-Breite. |
| Aktualisierung | Jede Sekunde (mit `runTickMsg`) prüft lazydev im Hintergrund für alle Viewer-Tabs aller Projekte Änderungszeit und Größe der Datei. Hat sich eines davon geändert, liest es die Datei neu und der Tab rendert neu. Die Scroll-Position (`top`) bleibt und wird auf das neue Ende begrenzt. Ändert sich nichts, liest lazydev die Datei nicht. |
| Datei verschwindet oder ist nicht lesbar | Findet die Prüfung die Datei nicht mehr oder kann sie sie nicht lesen, schließt sich der Tab von selbst. Der Footer zeigt den Hinweis (`m.note`) `<dateiname> is gone, its viewer closed.` bzw. `Cannot read <dateiname>: <erste Zeile des Fehlers>, its viewer closed.` Mehrere Tabs auf einmal: der Hinweis des letzten gewinnt. |
| PRD aus einem Worktree | Ein Viewer, der beim Öffnen ein PRD aus dem Worktree eines Laufs zeigt (`d.Root` ≠ Projektordner), merkt sich den PRD-Namen. Er schließt sich zusammen mit dem Lauf wie ein Log-Tab: sobald der Lauf weg ist (`X`) oder `merged` ist, ohne Hinweis im Footer. Er wechselt nie auf die Datei im Hauptordner. Viewer aus dem Hauptordner hängen an keinem Lauf. |
| Tasten im Tab | Wie im Log-Tab: `j`/`↓` und `k`/`↑` eine Zeile, `ctrl+d`/`pgdown` und `ctrl+u`/`pgup` die halbe Höhe, `g`/`home` an den Anfang, `G`/`end` ans Ende. Der Viewer folgt dem Ende nicht, er startet oben. `e` öffnet die Datei mit `editorCommand(path)` als neuen Terminal-Tab (`openTerminal`), der Viewer bleibt offen. Andere Tasten gehen an die bestehende Logik (Tab-Wechsel, Regel 2). Keine Taste geht an ein Terminal. |
| Schließen | `alt+w` schließt den Viewer sofort, wie einen Log-Tab. |
| Maus | Mausrad im Inhaltsbereich des aktiven Viewer-Tabs (gleiche Fläche wie der Terminalbereich, `termOrigin`/`termSize`) scrollt um `wheelLines` (3) Zeilen. Ein Linksklick dort fokussiert die Mitte. Sonst wirkt die Maus im Viewer nicht, es gibt keine Auswahl. Overlays blockieren die Maus wie bisher. |
| Copy-Mode | Unverändert: `alt+c` im Viewer zeigt `The active tab is not a terminal.`. |
| `l`/`enter` | Unverändert: öffnen weiterhin den Editor. |
| Footer im Viewer | ` j/k scroll · ctrl+d/u page · g/G top/end · e edit · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+w close · t terminal · alt+g git · m menu · q quit` |
| Footer andernorts | Tab `PRDs`: `v view` nach `l open`. Tab `Files`: `v view` nach `l open`. Terminal: `alt+v view` nach `alt+c copy`. |

## Scope

**In Scope**

- Neue Datei `internal/tui/mdview.go` mit dem Tab-Typ, Öffnen, Rendern, Tasten, Hintergrundprüfung und Schließen.
- Globale Aktion `actView` mit `alt+v`, Footer-Texte, Mausrad im Viewer.
- Abhängigkeit `github.com/charmbracelet/glamour` in `go.mod`/`go.sum`.
- Tests in `internal/tui/mdview_test.go`.
- `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Textsuche, Links folgen, Inhaltsverzeichnis, Bilder.
- Kopieren aus dem Viewer und Copy-Mode für Viewer-Tabs.
- Bearbeiten im Viewer.
- Andere Dateitypen als `.md`/`.markdown`, auch nicht als Rohtext.
- Änderung von `l`/`enter` in den Tabs `Files` und `PRDs`.
- Ein Viewer, der einem PRD vom Worktree in den Hauptordner folgt.
- Eigene Glamour-Stile oder eine Abbildung der Theme-Farben auf Glamour.

## Umsetzung

1. **Abhängigkeit.** `go get github.com/charmbracelet/glamour@latest` im Repo, dann `go mod tidy`. Prüfe danach
   in `go.mod`, dass `github.com/charmbracelet/x/cellbuf` weiterhin auf `v0.0.15` steht und `x/ansi`,
   `lipgloss` und `bubbletea` nicht zurückgestuft wurden. Zieht Glamour eine ältere `cellbuf` herunter, setze
   sie mit `go get github.com/charmbracelet/x/cellbuf@v0.0.15` zurück. `make build` muss danach bauen.

2. **Neue Datei `internal/tui/mdview.go`.** Sie enthält, im Stil von `logview.go` kommentiert:
   - Kommentar am Dateianfang: ein Viewer-Tab zeigt eine Markdown-Datei gerendert, ohne Prozess, und liest sie
     neu, sobald sie sich ändert.
   - Typ `mdView` mit den Feldern `id int`, `path string` (absolut), `run string` (PRD-Name bei einer Datei aus
     einem Worktree, sonst leer), `width, height int`, `text string` (Dateiinhalt), `modTime time.Time`,
     `size int64`, `top int`, `theme int` (vor jedem Render vom Model gesetzt) sowie der Cache `lines []string`,
     `cacheWidth int`, `cacheTheme int`, `dirty bool`.
   - `newMDView(id int, path, run string, text string, info os.FileInfo) *mdView`.
   - `ID()`, `TabTitle()` (`"md " + filepath.Base(path)`), `Resize(width, height int)`.
   - `isMarkdown(path string) bool`: Endung `.md` oder `.markdown`, `strings.ToLower` auf `filepath.Ext`.
   - `mdStyle(theme int) ansi.StyleConfig` (Glamour-Paket `github.com/charmbracelet/glamour/ansi`, Import-Alias
     nötig, weil `x/ansi` schon `ansi` heißt): `Tokyo Night` → `styles.TokyoNightStyleConfig`, sonst
     `styles.DarkStyleConfig`. Entscheide über `themes[theme].name`, ein ungültiger Index gilt als `Default`.
   - `func (v *mdView) body() []string`: gibt den Cache zurück, solange `!dirty`, `cacheWidth == width` und
     `cacheTheme == theme`. Sonst `glamour.NewTermRenderer(glamour.WithStyles(mdStyle(theme)),
     glamour.WithWordWrap(max(1, width)))` und `Render(text)`, Ergebnis ohne abschließende Leerzeilen in Zeilen
     zerlegen. Bei Fehler der Rohtext-Fallback aus den Produktentscheidungen (`logTabs.Replace` wiederverwenden).
   - `maxTop()`, `scroll(delta int)` (begrenzt auf `0..maxTop()`), `update(key string) bool` mit den Tasten
     `j/k/↓/↑`, `ctrl+d/u`, `pgdown/pgup`, `g/home`, `G/end`. `e` behandelt das Model (Schritt 3), nicht
     `update`.
   - `View(bool) string`: `height` Zeilen ab `top`, jede mit `ansi.Truncate` auf `width` gekürzt und mit
     Leerzeichen auf `width` aufgefüllt, wie `logView.View`.
   - `func (m *model) openView()`: ermittelt den Pfad nach „Welche Datei“, prüft in der Reihenfolge der
     Fehlertabelle, liest die Datei mit `os.Stat` und `os.ReadFile`. Bei Erfolg: vorhandenen Viewer mit gleichem
     Pfad im gewählten Projekt fokussieren, sonst `m.nextID++`, `newMDView`, `Resize(m.termSize())`, anhängen,
     aktiv setzen, `m.focus = colCenter`. `run` ist `d.Name`, wenn der Pfad aus dem Tab `PRDs` kommt und
     `d.Root != p.path`, sonst leer.
   - `type mdReadMsg struct{ byTab map[int]mdRead }` mit `mdRead{ text string; modTime time.Time; size int64;
     err error; changed bool }`.
   - `func (m model) readViews() tea.Cmd`: sammelt `id`, `path`, `modTime`, `size` aller Viewer-Tabs aller
     Projekte, `nil` ohne Viewer. Der Befehl ruft für jeden `os.Stat` auf; unverändert → `changed: false`,
     Fehler → `err`, sonst `os.ReadFile` → `text` und `changed: true` (Lesefehler → `err`).
   - `func (m *model) applyViews(msg mdReadMsg)`: für jeden Viewer mit Eintrag: bei `err` den Tab mit
     `p.removeTab(i)` schließen und den Hinweis setzen (`os.IsNotExist(err)` → `<dateiname> is gone, its viewer
     closed.`, sonst `Cannot read <dateiname>: <claude.FirstLine(err.Error())>, its viewer closed.`), dabei
     `m.err` leeren. Bei `changed` `text`, `modTime`, `size` übernehmen, `dirty = true`, `top` danach mit
     `scroll(0)` begrenzen. Rückwärts über die Tabs iterieren, damit `removeTab` die Indizes nicht verschiebt.
   - `func (p *projectEntry) activeView() *mdView` analog zu `activeLog`. Lege sie in
     `internal/tui/projects.go` neben `activeLog`.
   - Ergänze `closeEndedLogs` (`internal/tui/logview.go`) um Viewer mit `run != ""`: schließen, wenn
     `p.runs[run] == nil` oder `Effective() == project.PhaseMerged`. Benenne die Funktion in `closeEndedTabs`
     um und passe den Aufruf in `Update` (`runsMsg`) an.

3. **`internal/tui/keymap.go`.**
   - Neue Aktion `actView` vor `actQuit`, in `globalKeys` `"alt+v": actView`.
   - In `global`: `case actView: m.openView()`.
   - Neue Methode `inMDView()` analog zu `inLogView`.
   - In `updateKey`, Fall `colCenter`: nach dem Log-Zweig ein Zweig für `m.inMDView()`: `e` ruft
     `m.openTerminal(editorCommand(v.path))` auf und gilt als behandelt, sonst `handled = v.update(key)`.
   - `actCloseTab` in `global`: `p.activeView() != nil` schließt den Tab sofort wie ein Log-Tab.
   - `footerKeys`: Fall `m.inMDView()` mit dem Text aus den Produktentscheidungen direkt nach dem Log-Fall.
     `v view` in den Fällen `rightPRDs` und `rightFiles` nach `l open`, `alt+v view` im Terminal-Fall nach
     `alt+c copy`. Der Copy-Mode-Footer bleibt gleich.
   - Kommentar zu Regel 2 in `updateKey` bleibt gültig, `v` braucht keine Ausnahme.

4. **`internal/tui/main.go`.**
   - `runTickMsg`: `m.readViews()` in den `tea.Batch` aufnehmen.
   - Neuer Fall `case mdReadMsg: m.applyViews(msg)`.
   - `centerBody`: vor dem Rendern `v.theme = m.theme` setzen, wenn der aktive Tab ein Viewer ist (analog zu
     `lv.ctx`).
   - `resizeTerminals` prüfen: alle `centerTab` bekommen schon `Resize`, Viewer brauchen nichts Eigenes.

5. **`internal/tui/mouse.go`.** In `updateMouse` vor `term := p.activeTerminal()`: Ist `p.activeView()` nicht
   `nil`, gilt im Inhaltsbereich (`termOrigin`/`termSize`, wie `inside`) Mausrad hoch `scroll(-wheelLines)`,
   runter `scroll(wheelLines)`, Linksklick (Press) `m.focus = colCenter`. Alles andere ignorieren und
   zurückkehren. Kommentar der Funktion um den Viewer ergänzen.

6. **Tests in `internal/tui/mdview_test.go`.** Nutze die Helfer der bestehenden Tests (`press`, `keyRunes`,
   `checkSize`, Modelle aus `logview_test.go`/`mouse_test.go`/`layout_test.go`), isoliert mit
   `t.Setenv("XDG_STATE_HOME", …)`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME`.
   - Öffnen aus `Files`: Projekt mit `README.md` (`# Titel`, `**fett**`, Tabelle, langer Absatz), Tab `Files`,
     Cursor auf der Datei, `alt+v`: aktiver Tab ist ein Viewer, Titel `md README.md`, Fokus `colCenter`, die
     Ansicht enthält `Titel` und `fett`, aber nicht `**`. `checkSize` mit 160×40.
   - `v` ohne `alt` in der rechten Spalte öffnet ebenfalls.
   - Öffnen aus `PRDs`: markiertes PRD öffnet `filepath.Join(d.Root, d.Path)`. Mit einem Lauf, dessen Worktree
     das PRD enthält (`d.Root` = Worktree), zeigt der Viewer den Inhalt aus dem Worktree, und `run` ist gesetzt.
   - Fehler: Ordner → `Select a Markdown file in the Files tab.`, Gruppenzeile → `Select a PRD in the PRDs tab.`,
     `main.go` → `main.go is no Markdown file.`, `NOTES.MD` öffnet, unlesbare Datei (Rechte `0`) →
     beginnt mit `Cannot read`. Jeweils kein neuer Tab.
   - Doppeltes Öffnen: zweimal `alt+v` auf dieselbe Datei, dazwischen Fokus woanders: ein Viewer-Tab, aktiv.
   - Scrollen: lange Datei, `j`, `ctrl+d`, `G`, `g` ändern `top` wie erwartet, `top` bleibt in `0..maxTop()`.
   - `e` öffnet einen Terminal-Tab mit `editorCommand` (mit `t.Setenv("EDITOR", "true")`), der Viewer bleibt.
   - Aktualisierung: Datei ändern (Inhalt und Änderungszeit, z. B. `os.Chtimes`), `readViews()` ausführen und die
     Nachricht an `Update` geben: neuer Inhalt sichtbar, `top` bleibt. Ohne Änderung meldet die Prüfung
     `changed: false`.
   - Datei löschen, Prüfung ausführen: Tab weg, `m.note` = `README.md is gone, its viewer closed.`.
   - Lauf: Viewer mit `run` schließt sich bei `runsMsg` ohne den Lauf und bei Phase `merged`, bleibt bei
     `aborted`. Ein Viewer ohne `run` bleibt bei jedem `runsMsg`.
   - `alt+w` schließt den Viewer sofort.
   - Theme: `mdStyle` liefert für `Tokyo Night` den Tokyo-Night-Stil, sonst Dark. Ein Theme-Wechsel macht den
     Cache ungültig.
   - Maus: Mausrad im Inhaltsbereich scrollt um 3, außerhalb nicht, mit offenem Menü nicht.
   - Footer: im Viewer enthält er `e edit`, im Tab `Files` `v view`.
   - `alt+c` im Viewer: Fehler `The active tab is not a terminal.`.

7. **Doku.**
   - `SPEC.md`, Abschnitt `### Keys`: Absatz zu `alt+v` analog zum Absatz zu `alt+c` (Taste, Regel 2 gilt für `v`).
     Abschnitt `### Panes`, Punkt **Center**: Viewer-Tabs als dritte Art von Tab nennen und auf einen neuen
     Abschnitt verweisen. Neuer Abschnitt `### Markdown viewer` nach `### Log tab` mit allen
     Produktentscheidungen: Auswahl, Fehlertexte, Titel, doppeltes Öffnen, Glamour-Stil je Theme, Tasten,
     sekündliche Prüfung, Schließen bei verschwundener Datei mit Hinweis, Schließen mit dem Lauf, Maus, Footer.
     Im Abschnitt Mouse den Satz „Mouse events act only in the terminal area of the active terminal tab“ um den
     Viewer ergänzen.
   - `README.md`: Tabelle `## Keys` eine Zeile `` | `alt+v` | Markdown viewer of the selected file or PRD | ``
     nach `alt+c`. Tabelle `## Layout`, Zeile Center: „Terminal tabs, log tabs and Markdown viewers of the
     project“. Kurzer Abschnitt `## Reading Markdown` nach `## Copying from terminal tabs`: was `alt+v` öffnet,
     Tasten, `e`, automatische Aktualisierung, Schließen.
   - `README.de.md`: dieselben Änderungen auf Deutsch.

## Technische Hinweise

- Alle UI-Texte, Fehler, Kommentare und Doku auf Englisch, außer `README.de.md`. Commits nach Conventional
  Commits, z. B. `feat(tui): open markdown viewer tab with alt+v`.
- Die TUI liest nur. Datei-I/O der sekündlichen Prüfung läuft als `tea.Cmd` im Hintergrund wie `readLogs`. Nur
  das Öffnen liest synchron, damit Fehler sofort im Footer stehen.
- Keine Glamour-Autostyle-Funktionen (`WithAutoStyle`, `WithStandardStyle("auto")`), sie fragen das Terminal ab
  und stören Bubble Tea. Nur `WithStyles` mit festem Stil.
- Glamour ist langsam im Vergleich zum Zeichnen. `View` läuft bei jeder Nachricht, deshalb rendert nur `body()`
  und nur bei geänderter Breite, geändertem Theme oder neuem Text (`dirty`).
- `x/cellbuf` ist auf `v0.0.15` gepinnt (`CLAUDE.md`), ältere Versionen bauen nicht gegen das neuere `x/ansi`.
  Nach `go get` prüfen.
- Glamour setzt einen eigenen Rand (`Document.Margin`). Die Zeilen können trotzdem breiter als der Tab werden,
  deshalb kürzt `View` jede Zeile mit `ansi.Truncate`.
- `model` wird per Wert kopiert, Tabs sind Zeiger (`*mdView`) wie `*logView`, Änderungen in `applyViews`
  wirken deshalb direkt.
- `p.removeTab` verschiebt `p.tab`; beim Schließen mehrerer Tabs rückwärts iterieren wie `closeEndedLogs`.
- `readViews` liest über alle Projekte, nicht nur das gewählte, damit ein Viewer im Hintergrund-Projekt beim
  Zurückwechseln aktuell ist und eine gelöschte Datei auch dort den Tab schließt.
- Änderungszeit plus Größe genügen als Änderungserkennung. Schreibt ein Editor atomar (neue Datei, rename),
  ändert sich die Änderungszeit, `os.Stat` folgt dem Pfad.
- Tests: kein echter Editor, `EDITOR=true`. Terminals in Tests wie in `mouse_test.go` aufräumen.

## Erfolgskriterien

- `make test` läuft ohne Fehler (`go vet ./...` und `go test ./...`).
- `make build` baut `bin/lazydev`, `go.mod` führt `github.com/charmbracelet/glamour` und
  `github.com/charmbracelet/x/cellbuf v0.0.15`.
- `go test ./internal/tui -run 'MDView|View'` läuft ohne Fehler.
- `alt+v` auf einer `.md`-Datei im Tab `Files` öffnet einen Tab `md <dateiname>` mit gerendertem Markdown (keine
  `**`, `#` oder `|`-Rohzeichen für Fett, Überschriften und Tabellen), auch aus einem Terminal heraus.
- `alt+v` im Tab `PRDs` zeigt das markierte PRD, bei einem Lauf aus dem Worktree.
- Nicht-Markdown, Ordner, Gruppenzeile und unlesbare Dateien öffnen keinen Tab und zeigen die festgelegten
  Fehlertexte.
- Ein zweites `alt+v` auf dieselbe Datei fokussiert den vorhandenen Tab.
- Speichern der Datei im Editor zeigt den neuen Inhalt innerhalb von etwa einer Sekunde an derselben Position.
- Löschen der Datei schließt den Tab mit dem Hinweis `<dateiname> is gone, its viewer closed.`.
- Ein Viewer auf ein PRD im Worktree schließt sich mit dem Merge oder `X` des Laufs.
- `j/k`, `ctrl+d/u`, `g/G` und das Mausrad scrollen, `e` öffnet den Editor als Tab, `alt+w` schließt den Viewer.
- Footer, `SPEC.md`, `README.md` und `README.de.md` nennen `alt+v`.
