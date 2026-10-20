# PRD: PRD vollständig im Fenster ansehen

> Status: umgesetzt (2026-10-20) · Template: einfach · Scope: Taste `v` öffnet das gewählte PRD komplett als gerendertes Markdown in einem Overlay-Fenster wie der Chat. Im Fenster öffnet `o` die Datei im Editor.

## Context

Die Detailansicht im unteren Bereich (`viewDetails`, `internal/tui/view.go`) zeigt nur Titel, Metazeilen
und den Freitext der Statuszeile. Den Rest des PRDs, also Context, Produktentscheidungen, Umsetzung und
Erfolgskriterien, siehst du in lazydev nicht. Um ein PRD vor dem Start mit `s` zu prüfen, musst du die
Datei außerhalb von lazydev öffnen. Bei einem laufenden PRD liegt die Datei zudem im Worktree und nicht
im Hauptordner.

Nach der Änderung öffnet `v` das gewählte PRD in einem Fenster über der TUI, so wie `n` das Chat-Fenster
öffnet. Das Fenster zeigt die Metazeilen und darunter die komplette Datei, gerendert mit Glamour. Du
scrollst mit Tastatur oder Mausrad. `o` öffnet die Datei im Editor.

Im Repo existiert bereits und wird wiederverwendet:

- `markdown` und `newRenderer` (`internal/tui/detail.go`): Glamour-Renderer im Stil Tokyo Night mit Cache.
- `detailMeta` (`internal/tui/view.go`): Metazeilen Status, Datei, Git und Erstellt.
- `viewport.Model` aus `github.com/charmbracelet/bubbles/viewport`, wie in `Model.detail`.
- `box()` (`internal/tui/box.go`), `overlay()` und `chatSize()` (`internal/tui/chat.go`).
- `openInteractive` und `interruptSafe` (`internal/tui/interactive.go`, `internal/tui/dockerview.go`):
  Start eines Befehls im Herdr-Pane oder im selben Terminal.
- `herdr.Available()` (`internal/herdr`).
- `addWorktreePRDs` (`internal/tui/runs.go`): Ein PRD, das ein Lauf verschoben hat, liegt unter
  `r.Worktree` + `r.PRD`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Taste zum Öffnen | `v` im Normalmodus, unabhängig vom Fokus. Ohne gewähltes PRD, etwa auf der Gruppenzeile der erledigten PRDs oder ohne Projekt, bewirkt `v` nichts. |
| Fensterart | Overlay wie der Chat: zentriert über der TUI, Größe aus `chatSize()`, also Breite `max(20, min(Breite − 6, 110))` und Höhe `max(10, Höhe − 4)`. |
| Rahmen | Wie das Chat-Fenster zum Zeitpunkt der Umsetzung: fokussierter Rahmen in `colorFocus`. |
| Titel | `PRD: <bereich>/<name>` (`d.Label()`). Passt der Inhalt nicht ins Fenster, folgt ` · <n> %` mit der Scroll-Position, wie in `viewDetails`. |
| Inhalt | Oben die Zeilen aus `detailMeta(d)`, dann eine Leerzeile, dann die komplette Datei ab der ersten Zeile als Markdown, gerendert mit Glamour. Titel und Statuszeile der Datei erscheinen also mit. |
| Quelle der Datei | Gibt es einen Lauf mit `r.PRD != ""` und `r.Phase != project.PhaseMerged`, liest lazydev `filepath.Join(r.Worktree, r.PRD)`. Sonst liest lazydev `filepath.Join(p.Path, d.Path)`. |
| Aktualisierung | Keine. Lazydev liest die Datei einmal beim Öffnen. Dateiereignisse, Ticks und `r` ändern den Inhalt nicht. Ändert sich die Fenstergröße, rendert lazydev den gespeicherten Text in der neuen Breite neu, ohne die Datei neu zu lesen. |
| Datei nicht lesbar | Das Fenster öffnet sich nicht. Die Statuszeile zeigt `PRD nicht lesbar: <erste Zeile des Fehlers>` als Fehler. |
| Tasten im Fenster | `j` `↓` eine Zeile runter, `k` `↑` eine Zeile hoch, `PgDn` `Ctrl+D` `Leertaste` eine Seite runter, `PgUp` `Ctrl+U` eine Seite hoch, `g` `Home` an den Anfang, `G` `End` ans Ende, `o` Editor, `Esc` `q` schließen. Alle anderen Tasten bewirken nichts. |
| Mausrad im Fenster | Scrollt um 1 Zeile pro Ereignis. Klicks bewirken nichts. |
| Editor-Befehl | `$VISUAL`, sonst `$EDITOR`, sonst `vi`. Der Wert wird mit `strings.Fields` in Programm und Argumente zerlegt, danach folgt der absolute Pfad der Datei. |
| Start des Editors | Über `openInteractive` wie `o` und `w`: In Herdr öffnet sich ein neuer Pane, das Fenster bleibt offen und liest nicht neu. Ohne Herdr unterbricht lazydev die Oberfläche bis zum Ende des Editors. Arbeitsordner ist der Worktree oder der Hauptordner, je nach Quelle der Datei. |
| Nach dem Editor (ohne Herdr) | Lazydev liest die Datei neu und behält die Scroll-Position. Danach liest lazydev die Projekte neu ein, weil sich die Statuszeile geändert haben kann. Ist die Datei nicht mehr lesbar, schließt sich das Fenster mit der Fehlermeldung `PRD nicht lesbar: …`. Meldet der Editor einen Fehler, zeigt die Statuszeile `Editor: <erste Zeile des Fehlers>` und lazydev liest die Datei trotzdem neu. |
| Laufender Runner | `o` öffnet den Editor immer, auch wenn ein Runner das PRD gerade bearbeitet. |
| Tastenhilfe der Fußzeile | Im Fenster: `j/k scrollen · PgUp/PgDn Seite · g/G Anfang/Ende · o Editor · Esc schließen`. Im Normalmodus kommt `v ansehen` vor `s starten` hinzu. |

## Scope

**In Scope**

- Neuer Modus `modePRDView` mit Fenster, Tasten, Mausrad und Editor-Aufruf.
- Neue Datei `internal/tui/prdview.go` mit Zustand, Laden, Rendern und Tasten der Ansicht.
- Aufteilung von `openInteractive`, damit der Editor ohne Herdr eine eigene Nachricht liefert.
- Einträge in Hilfe-Overlay, Tastenhilfe, `README.md` und `SPEC.md`.
- Tests in `internal/tui/prdview_test.go`.

**Out of Scope**

- Automatisches Neuladen bei Dateiänderungen.
- Textsuche, Kopieren in die Zwischenablage, Links oder Inhaltsverzeichnis.
- Bearbeiten im Fenster selbst.
- Änderungen an der Detailansicht im unteren Bereich.
- Öffnen per `Enter` auf einer PRD-Zeile.
- Änderungen an `dialog()` oder am Dimmen aus dem PRD `prd/tui/overlay-fenster.md`.

## Umsetzung

1. **Modus in `internal/tui/model.go`.** Ergänze nach `modeLimits` die Konstante `modePRDView` mit dem
   Kommentar „modePRDView zeigt ein PRD vollständig im Fenster.“. Ergänze in `Model` das Feld
   `prdView *prdView` mit dem Kommentar „prdView ist das offene PRD-Fenster, sonst nil.“.

2. **Neue Datei `internal/tui/prdview.go`.** Sie enthält:
   - Typ `prdView` mit den Feldern `label string` (für den Titel), `meta []string` (aus `detailMeta`),
     `path string` (absoluter Dateipfad), `dir string` (Worktree oder Hauptordner), `text string`
     (Dateiinhalt), `viewport viewport.Model` und `md *markdown` (eigener Renderer-Cache, damit der Cache
     der Detailansicht erhalten bleibt).
   - `func (m Model) prdSource() (dir, path string, ok bool)`: liefert Ordner und absoluten Pfad nach der
     Regel „Quelle der Datei“ aus den Produktentscheidungen. `ok` ist false ohne Projekt oder PRD.
   - `func (m Model) openPRDView() (Model, tea.Cmd)`: ermittelt die Quelle, liest die Datei mit
     `os.ReadFile`, setzt bei Fehler die Meldung `PRD nicht lesbar: <firstLine(err)>` und bleibt im
     Normalmodus. Bei Erfolg legt sie `m.prdView` an, setzt `m.mode = modePRDView` und springt an den
     Anfang.
   - `func (m *Model) syncPRDView()`: bei `m.prdView == nil` ohne Wirkung. Sonst gilt `w, h := m.chatSize()`,
     `viewport.Width = max(1, w-4)` und `viewport.Height = max(1, h-2)`. Inhalt ist
     `strings.Join(meta, "\n") + "\n\n" + md.render(text, viewport.Width)`. Danach `SetContent` und
     `SetYOffset(YOffset)`, damit die Position erhalten bleibt und begrenzt wird.
   - `func (m Model) viewPRDView() string`: Zeilen des Viewports mit `" "` davor, Titel nach der Regel
     „Titel“, Rückgabe `box(title, lines, w, h, true)`. Existiert bei der Umsetzung in
     `internal/tui/box.go` die Funktion `dialog`, nutze stattdessen `dialog(title, lines, w, h)` ohne
     führendes Leerzeichen und setze in `syncPRDView` `Width = w-2-2*dialogPadX` und
     `Height = h-2-2*dialogPadY`.
   - `func (m Model) updatePRDView(msg tea.KeyMsg) (tea.Model, tea.Cmd)`: Tasten nach der Tabelle. `Esc`
     und `q` setzen `m.prdView = nil` und `m.mode = modeNormal`. Seitenweise blättert um
     `viewport.Height`.
   - `func editorCommand() []string`: `$VISUAL`, sonst `$EDITOR`, sonst `vi`, zerlegt mit `strings.Fields`.
   - `type prdEditedMsg struct{ path string; err error }`.
   - `func (m Model) openPRDEditor() (Model, tea.Cmd)`: Argumente sind `editorCommand()` plus
     `m.prdView.path`. In Herdr ruft sie `m.openInteractive(m.prdView.dir, args, "Editor")` auf. Sonst gibt
     sie `execInteractive(dir, args, func(err error) tea.Msg { return prdEditedMsg{path, err} })` zurück.
   - `func (m Model) handlePRDEdited(msg prdEditedMsg) (Model, tea.Cmd)`: setzt bei `msg.err != nil` die
     Meldung `Editor: <firstLine(err)>`. Ist das Fenster noch offen und `m.prdView.path == msg.path`, liest
     sie die Datei neu und ersetzt `text`. Die Scroll-Position bleibt. Bei Lesefehler schließt sie das
     Fenster und setzt `PRD nicht lesbar: …`. Rückgabe immer mit `m.loadProjects("")`.

3. **`internal/tui/interactive.go`.** Ziehe den Zweig ohne Herdr aus `openInteractive` in die neue Funktion
   `execInteractive(dir string, args []string, done func(error) tea.Msg) tea.Cmd`. Sie ergänzt bei leeren
   `args` die Shell wie bisher und startet `tea.Exec(interruptSafe{c}, done)`. `openInteractive` ruft sie
   mit `actionDoneMsg{text: what + " beendet", err: err}` auf. Das Verhalten von `o` und `w` im Normalmodus
   bleibt gleich.

4. **`internal/tui/update.go`.**
   - In `Update` nach `nm.syncDetail()` den Aufruf `nm.syncPRDView()` ergänzen.
   - In `update` einen Fall `case prdEditedMsg: return m.handlePRDEdited(msg)` ergänzen.
   - Im `tea.KeyMsg`-Switch `case modePRDView: return m.updatePRDView(msg)` ergänzen.
   - In `updateNormal` den Fall `case "v": return m.openPRDView()` ergänzen.
   - In `handleMouse`: Ist `m.mode == modePRDView` und `m.prdView != nil`, scrollt das Mausrad den
     Viewport um 1 Zeile. Andere Mausereignisse bewirken nichts. Die Prüfung steht vor
     `if m.mode != modeNormal`.

5. **`internal/tui/view.go`.**
   - In `View()` nach dem Overlay für `modeLimits`: `if m.mode == modePRDView && m.prdView != nil` gibt
     `overlay(view, m.viewPRDView(), m.width)` zurück. Existiert bei der Umsetzung eine Overlay-Auswahl nach
     Modus, wie sie `prd/tui/overlay-fenster.md` beschreibt, ergänze `modePRDView` dort.
   - In `helpLine` einen Fall `modePRDView` mit den Tasten aus „Tastenhilfe der Fußzeile“ ergänzen. Im
     `default`-Zweig `{"v", "ansehen"}` vor `{"s", "starten"}` einfügen.
   - In `helpSections`, Abschnitt „PRDs“, nach dem Eintrag `n` den Eintrag
     `{"v", "PRD vollständig ansehen, darin o für den Editor"}` einfügen.

6. **Tests in `internal/tui/prdview_test.go`.** Nutze `newTestProject`, `drive`, `keys` und `runCmd` aus
   den bestehenden Tests. Setze `HERDR_ENV` mit `t.Setenv("HERDR_ENV", "")` leer, außer im Herdr-Test.
   - `v` öffnet das Fenster: Die Ansicht enthält `PRD: auth/token`, `Status    geplant` und Text aus einem
     Abschnitt weit unten in der Datei, nachdem `G` gedrückt wurde. Markdown erscheint gerendert, z. B.
     ohne `**`.
   - `Esc` und `q` schließen das Fenster, der Modus ist danach `modeNormal`.
   - Ein PRD mit Lauf: Lege mit `state` einen Lauf mit `Worktree` und `PRD` an. `v` zeigt den Inhalt der
     Datei im Worktree, nicht im Hauptordner.
   - Keine Aktualisierung: Nach dem Öffnen die Datei ändern und `runsLoadedMsg` sowie
     `projectsLoadedMsg` senden. Der alte Inhalt bleibt sichtbar.
   - Datei nicht lesbar: Datei nach dem Laden löschen, `v` drücken. Der Modus bleibt `modeNormal`, die
     Meldung beginnt mit `PRD nicht lesbar:`.
   - `prdEditedMsg`: Datei ändern, ein Stück scrollen, `prdEditedMsg{path: …}` senden. Der neue Inhalt
     erscheint, `YOffset` bleibt gleich. Mit gelöschter Datei schließt sich das Fenster.
   - `editorCommand`: `VISUAL` vor `EDITOR` vor `vi`, Argumente in `EDITOR="code -w"` werden zerlegt.
   - Herdr: Mit `fakeHerdr` ruft `o` im Fenster herdr mit dem Dateipfad auf, und das Fenster bleibt offen.
   - Erweitere `TestViewFitsTerminal` (`internal/tui/model_test.go`) um einen Fall mit offenem Fenster, wenn
     der Test Modi durchläuft. Sonst prüft ein eigener Test, dass keine Zeile breiter als das Terminal ist.

7. **Doku.**
   - `README.md`, Tabelle „Tasten“: nach `n` die Zeile
     `` | `v` | PRD vollständig als Markdown im Fenster ansehen. Darin öffnet `o` die Datei im Editor. | ``.
   - `SPEC.md`, Abschnitt „Tasten“: dieselbe Zeile nach `n`. Ergänze im Abschnitt „Log-Darstellung“ nach
     dem Punkt zur Detailansicht einen Punkt zu `v`: Fenster wie der Chat, Metazeilen und komplette Datei,
     Quelle Worktree oder Hauptordner, keine Aktualisierung bei offenem Fenster, Tasten, Editor über
     `$VISUAL`, `$EDITOR` oder `vi`, Neueinlesen nach dem Editor ohne Herdr.

## Technische Hinweise

- Alle Texte sind deutsch: UI-Strings, Fehlermeldungen, Kommentare. Kommentare im Stil der umliegenden
  Dateien, kurz und im Präsens.
- `Update` ruft nach jeder Nachricht `refreshLog` und `syncDetail` auf. `syncPRDView` darf die Datei nicht
  lesen, sonst bricht die Entscheidung „keine Aktualisierung“. Sie arbeitet nur mit `prdView.text`.
- Nutze für das Fenster einen eigenen `*markdown`. `Model.md` cacht nur einen Text. Teilt das Fenster den
  Cache mit der Detailansicht, rendert Glamour bei jeder Nachricht beide Texte neu.
- `Model` wird per Wert kopiert. `prdView` ist deshalb ein Zeiger, wie `chat`.
- `newRenderer` gibt den Stil fest vor. Rufe keine automatische Stilerkennung auf, sie fragt das Terminal
  ab und stört Bubble Tea.
- `o` hat im Normalmodus eine andere Bedeutung (Claude-Sitzung). Im Fenster gilt nur der Editor.
- Das PRD `prd/tui/overlay-fenster.md` läuft parallel und ändert `overlay()`, `View()` und
  `chat.render`. Nutze für das neue Fenster dieselben Funktionen wie das Chat-Fenster auf dem Stand von
  `main` bei der Umsetzung. Die Regeln dazu stehen in Schritt 2 und Schritt 5.
- `tea.Exec` lässt sich in Tests nicht ausführen. Teste den Editor ohne Herdr über `prdEditedMsg` und
  `editorCommand`, nicht über einen echten Editor.
- Tests isolieren sich über `t.Setenv("XDG_STATE_HOME", t.TempDir())` und `XDG_DATA_HOME`.

## Erfolgskriterien

- `make test` läuft ohne Fehler, also `go vet ./...` und `go test ./...`.
- `go test ./internal/tui -run 'PRDView|EditorCommand'` läuft ohne Fehler.
- `v` auf einem PRD öffnet ein zentriertes Fenster mit Titel `PRD: <bereich>/<name>`, Metazeilen und der
  kompletten Datei als gerendertes Markdown.
- Bei einem laufenden PRD zeigt das Fenster die Datei aus dem Worktree.
- `j` `k`, `PgUp` `PgDn`, `g` `G` und das Mausrad scrollen. Der Titel zeigt die Position in Prozent, wenn
  der Inhalt nicht passt.
- Ändert sich die Datei bei offenem Fenster, bleibt der Inhalt gleich.
- `o` im Fenster öffnet `$VISUAL`, `$EDITOR` oder `vi` mit der Datei. Ohne Herdr zeigt das Fenster danach
  den neuen Inhalt an derselben Position.
- `Esc` und `q` schließen das Fenster. Eine nicht lesbare Datei öffnet kein Fenster und zeigt
  `PRD nicht lesbar: …`.
- Hilfe-Overlay, Tastenhilfe, `README.md` und `SPEC.md` nennen `v`.
