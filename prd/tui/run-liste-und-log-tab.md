# PRD: Run-Liste im Details-Pane und Log als Center-Tab

> Status: geplant · Scope: Das Details-Pane zeigt bei aktivem PRDs-Tab alle Runs des Projekts als Liste statt eines einzelnen Runs mit gekürztem Live-Log. `l` in der Liste oder `L` im PRDs-Tab öffnet das vollständige Log eines Runs als native, scrollbare Center-Tab-Ansicht mit ein-/ausklappbarem Kopf.

## Context

Heute zeigt der Tab `Details` im Pane unter den Terminals bei aktivem PRDs-Tab genau das in der PRD-Liste
gewählte PRD (`detailsPRD`, `bottomBody` in `internal/tui/main.go`): mit Run die Run-Info, Steps und das
Live-Log als dritte Spalte (`runDetailsView` in `internal/tui/runview.go`), ohne Run `prdDetailsView`
(`internal/tui/prds.go`).

Probleme:

- Laufen mehrere PRDs (bis `runner.MaxParallel` = 3 plus wartende), sieht man immer nur eins. Für die anderen
  muss man den Cursor in der PRD-Liste bewegen.
- Das Log ist dreifach abgeschnitten: `maxDescribeLen = 160` und `truncate` in `internal/claude/describe.go`,
  Claude-Texte und Tool-Argumente nur als erste Zeile (`FirstLine`), und das Pane hat nur
  `max(8, height/4)` Zeilen, von denen das Log eine Spalte neben Info und Steps bekommt.

Ziel: Das Details-Pane wird eine Übersicht aller Runs des Projekts, eine Zeile pro Run. Das Log eines Runs
öffnet sich als eigener Center-Tab mit vollständigem Text, Zeilenumbruch und Scrollen. Run-Details (Branch,
Port, Dev, Fehler, Notiz, Steps) stehen im Kopf dieses Tabs.

Im Repo existiert bereits und wird wiederverwendet:

- `fetchRuns`, `readLog`, `runsMsg`, `runTick` (`internal/tui/runview.go`): Lesen der Run-States jede Sekunde.
- `claude.Describe`, `claude.Entry`, `claude.FirstLine` (`internal/claude/describe.go`, `run.go`).
- `state.All`, `state.LogPath`, `state.Steps`, `Run.Effective`, `Run.Running`, `Run.LastStep`, `Run.Cost`
  (`internal/state/state.go`), `devserver.PID`.
- `startPRD`, `stopPRD`, `discardPRD`, `toggleDev` (`internal/tui/runview.go`).
- Tabellenmuster mit wegfallenden Spalten und Cursor aus `dockerView` (`internal/tui/docker.go`), `pad`.
- Kopf-Inhalt aus `runDetailsView`: Phasentext mit Queue-Position, Steps mit Marke, Dauer und Kosten,
  `duration`, `phaseColor`.
- Muster „Log als Center-Tab“: `l` im Docker-Tab öffnet `docker logs -f` (`updateBottom` in
  `internal/tui/keymap.go`).
- `projectDetailsView` (`internal/tui/details.go`).

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Details-Tab bei aktivem PRDs-Tab | Immer die Run-Liste des gewählten Projekts. Das in der PRD-Liste gewählte PRD spielt keine Rolle. |
| Details-Tab bei aktivem Files-Tab | Unverändert: `projectDetailsView`. |
| Leere Run-Liste | Zeigt `projectDetailsView`, wie im Files-Tab. |
| `prdDetailsView` | Entfällt komplett, samt Aufruf. |
| Welche Runs | Nur aktuelles Projekt. Alle Runs mit `Effective()` ungleich `merged`: waiting, setup, implement, review, merge, merge-queue, error, aborted. |
| Reihenfolge der Liste | Nach `StartedAt` aufsteigend, bei gleichem Wert nach PRD-Name. |
| Titel des Panes | `Runs`, solange die Liste sichtbar ist, sonst `Project`. Die Tab-Leiste bleibt `Details`, `Docker`. |
| Spalten einer Zeile | Phasensymbol in Phasenfarbe + PRD-Label, Phase (bei waiting mit ` · #n in queue`), aktueller Step mit Laufzeit, Kosten (`x.xx USD`), Port, Dev (`● :<port>` bei laufendem Dev-Server, sonst `–`), Branch, im Rest der Breite die letzte Logzeile. Kopfzeile mit Spaltentiteln wie im Docker-Tab. |
| Aktueller Step | `Run.LastStep()`: Name und Laufzeit (`duration`, bis jetzt bei laufendem Runner, sonst bis `EndedAt`). Ohne Step `–`. |
| Letzte Logzeile | `FirstLine` des letzten Eintrags des aktuellen Step-Logs, gedimmt. Bei Phase error ersetzt die erste Zeile von `Run.Error` sie in Warnfarbe, sonst bei gesetzter `Run.Note` die Notiz in Warnfarbe. |
| Zu schmal | Spalten fallen von rechts weg wie im Docker-Tab: zuerst letzte Logzeile, dann Branch, Dev, Port, Kosten, Step. PRD und Phase bleiben immer. |
| Tasten in der Run-Liste | `j/k`, `g/G` bewegen, `l`/`enter` öffnen das Log, `s`, `x`, `X`, `D` wirken auf den gewählten Run wie im PRDs-Tab, `i` Wizard, `z` Zoom. |
| Cursor | Eigener Cursor pro Projekt, unabhängig von der PRD-Liste. Keine Synchronisation in beide Richtungen. Verschwindet der Run unter dem Cursor, wird der Cursor geklemmt. |
| `L` im PRDs-Tab | Öffnet das Log des gewählten PRDs. Ohne Run: Fehler `<label> has no run.` In der Gruppenzeile `▸ n done` passiert nichts. |
| Log-Ansicht | Native Bubble-Tea-Ansicht als Center-Tab, kein PTY, kein Prozess. Gehört zum Projekt wie die Terminal-Tabs, zählt in der Tab-Anzahl der Projektliste mit. |
| Tab-Titel | `log <prd-name>`. |
| Schon offen | `l`/`L` auf einen Run, dessen Log-Tab im Projekt offen ist, fokussiert diesen Tab statt einen neuen zu öffnen. |
| Log-Inhalt | Alle Steps in der Reihenfolge `state.Steps`, deren Log-Datei existiert. Vor jedem Step eine Trennzeile `── <step> ──`. |
| Vollständigkeit | Claude-Texte mit allen Zeilen, Tool-Argumente ungekürzt mit allen Zeilen, Runner-Zeilen und stderr ungekürzt. Tool-Fehler bleiben `error: ` + erste Zeile. Keine Tool-Ergebnisse. `maxDescribeLen` und `truncate` entfallen. |
| Darstellung | Jede Zeile `HH:MM:SS ` + Text in der Farbe der Art wie heute (Runner gedimmt, Fehler Warnfarbe, Result OK-Farbe). Subagenten zwei Leerzeichen eingerückt. Text länger als die Breite bricht um. Folgezeilen und Umbrüche sind auf die Textspalte eingerückt (9 Zeichen plus Subagent-Einzug). |
| Kopf | Ausgeklappt beim Öffnen. Zeile 1: Phasensymbol, Phase mit Queue-Position, `branch → base`, `port`, `cost`, `dev`. Zeile 2: Steps `✓ setup 0:12 · ● implement 3:40 1.20 $ · ○ review · ○ merge` mit Marken wie heute in `runDetailsView`. Zeile 3 nur bei Fehler oder Notiz, in Warnfarbe. Darunter eine Trennlinie. |
| Kopf umschalten | `z` in der Log-Ansicht klappt den Kopf ein und aus. Eingeklappt bleibt eine Zeile: Phasensymbol, Phase, aktueller Step mit Laufzeit, Kosten. |
| Scrollen | `j/k` und `↓/↑` eine Zeile, `ctrl+d`/`ctrl+u` und `pgdown`/`pgup` eine halbe Höhe, `g`/`home` Anfang, `G`/`end` Ende. Gezählt wird in umgebrochenen Bildschirmzeilen. |
| Folgen | Steht die Ansicht am Ende, folgt sie neuen Zeilen. Scrollt der Nutzer nach oben, bleibt die Position. `G` oder Scrollen bis zum Ende folgt wieder. |
| Aktualisierung | Mit `runTick` jede Sekunde. Gelesen wird nur, was seit dem letzten Lesen an die Step-Logs angehängt wurde. |
| Run endet mit error oder aborted | Tab bleibt offen und zeigt den Endstand. |
| Run gemergt | Tab schließt sich automatisch, sobald `Effective()` `merged` ist. |
| Run verworfen (`X`) | Tab schließt sich automatisch, sobald der Run-State fehlt. |
| Schließen | `alt+w` schließt einen Log-Tab sofort, ohne SIGHUP. |
| Tasten in der Log-Ansicht | Keine Weitergabe an ein Terminal. Globale `alt`-Tasten gelten, nach Regel 2 auch ohne `alt` (`t`, `m`, `q`), soweit die Ansicht die Taste nicht selbst nutzt. |
| Live-Log im Details-Pane | Entfällt. `m.log`, `m.logKey`, `m.devPID` und `runDetailsView` entfallen. |

## Scope

**In Scope**

- Run-Liste im Tab `Details` mit Cursor und Tasten.
- Neue Log-Ansicht als Center-Tab-Art neben Terminals.
- `L` im PRDs-Tab.
- Vollständige Texte in `claude.Describe`.
- Entfernen von `prdDetailsView`, `runDetailsView` und dem Live-Log im Details-Pane.
- Tests, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Runs anderer Projekte in der Liste.
- Tool-Ergebnisse (Output von Bash, Read usw.) im Log.
- Suche oder Filter im Log.
- Scrollback für Terminal-Tabs.
- Änderungen an Runner, Log-Format oder Run-State.
- Speichern des Kopf-Zustands oder offener Log-Tabs über einen Neustart.

## Umsetzung

1. **`internal/claude/describe.go`:** `maxDescribeLen` und `truncate` entfernen, `entry` übernimmt den Text
   unverändert. Bei `assistant`/`text` den ganzen Text mit `strings.TrimSpace` statt `FirstLine`, leere Texte
   ergeben keinen Eintrag. `toolArg` gibt den ganzen Wert mit `strings.TrimSpace` zurück. Tool-Fehler bleiben
   bei `FirstLine`. `Entry.Text` darf damit Zeilenumbrüche enthalten, Kommentar an `Entry` ergänzen.
   `internal/claude/describe_test.go`: den Kürzungstest (Zeilen um `maxDescribeLen`) ersetzen durch Tests,
   dass ein Text mit 300 Zeichen und ein mehrzeiliger Claude-Text sowie ein mehrzeiliges `command`
   vollständig ankommen und ein Tool-Fehler nur seine erste Zeile behält.

2. **Center-Tabs (`internal/tui/projects.go`, `internal/tui/main.go`, `internal/tui/keymap.go`):**
   `projectEntry.terms []*terminal.Terminal` wird `tabs []centerTab`. `centerTab` ist ein Interface in
   `internal/tui/tabs.go` mit `ID() int`, `TabTitle() string`, `Resize(width, height int)`,
   `View(focused bool) string`. `*terminal.Terminal` erfüllt es unverändert. Alle Stellen, die `p.terms`
   nutzen, umstellen:
   - `inTerminal` ist nur wahr, wenn der aktive Tab ein `*terminal.Terminal` ist. Ein neues `inLogView` ist
     wahr bei Fokus Mitte und aktivem Log-Tab. `updateKey` reicht Tasten bei `inLogView` nach den globalen
     `alt`-Tasten an die Log-Ansicht. Nutzt sie die Taste nicht, greift wie bei den anderen Panes Regel 2.
   - `actCloseTab`: Terminal wie bisher `Hangup`, Log-Tab sofort entfernen (gleiche Index-Logik wie
     `removeTerminal`, ausgelagert in eine gemeinsame Funktion).
   - `quit`, `updateConfirmRemove`, `removeTerminal`, `resizeTerminals`, `ensureTerminal`, `leftBody`
     (Tab-Anzahl), `centerBody`: Terminals über Typprüfung, Log-Tabs ohne Hangup.
   - `openTerminal` hängt weiter ein Terminal an. Neu `openLog(id, prd string)`: sucht im aktuellen Projekt
     einen Log-Tab desselben Runs und fokussiert ihn, sonst legt es einen an, macht ihn aktiv und setzt
     `m.focus = colCenter`.

3. **Neue Datei `internal/tui/logview.go`:** Typ `logView` mit Projekt-ID, PRD-Name, eigener ID aus
   `m.nextID`, Größe, Einträgen pro Step, Lese-Offset pro Step-Datei, Scroll-Position, Folgen-Flag und
   Kopf-Flag (Start ausgeklappt).
   - Lesen: Funktion, die für jeden Step aus `state.Steps` ab dem gespeicherten Offset liest, nur
     vollständige Zeilen (bis zum letzten `\n`) verarbeitet, sie mit `claude.Describe` beschreibt und den
     Offset erhöht. Läuft als `tea.Cmd` im Hintergrund und liefert ein Message mit den neuen Einträgen pro
     Log-Tab-ID.
   - Rendering: Kopf wie in den Produktentscheidungen, Inhalt aus `runDetailsView` (Phasentext, Steps,
     `duration`, `phaseColor`) übernehmen. Trennzeile `── <step> ──` vor jedem Step mit Einträgen. Umbruch
     mit `ansi.Wrap(text, width-9-Einzug, "")`, Wörter länger als die Breite brechen hart um.
   - Tasten wie in den Produktentscheidungen, `z` schaltet den Kopf.
   - `View(focused bool)` füllt genau die Größe aus `Resize`, keine Zeile breiter als die Breite.

4. **`internal/tui/runview.go`:**
   - `fetchRuns`: statt des Logs eines Runs für das gewählte Projekt pro Run der Liste den letzten Eintrag
     des Logs von `LastStep()` (Tail von 16 KiB, `readLog` bekommt die Tail-Größe als Parameter) und
     `devserver.PID`. `runsMsg` bekommt `last map[string]claude.Entry` und `devPIDs map[string]int`, beide nach
     PRD-Name; `logKey`, `log`, `devPID` entfallen, ebenso `logEntries` und `logTailBytes` in der heutigen Form.
   - Bei `runTickMsg` zusätzlich das Lesen aller offenen Log-Tabs aller Projekte anstoßen.
   - Bei `runsMsg`: Log-Tabs schließen, deren Run fehlt oder `Effective()` `merged` ist (alle Projekte).
   - `runDetailsView` entfernen. Neu `runListView(p, width, height, focused, t)` nach dem Muster von
     `dockerView`: Kopfzeile, Spalten wie in den Produktentscheidungen, Cursor `p.runCursor` (neues Feld in
     `projectEntry`) mit Hintergrund bei Fokus, Fenster hält den Cursor sichtbar.
   - Hilfsfunktion `(p *projectEntry) runList() []string`: PRD-Namen der Liste in der festgelegten Filterung
     und Reihenfolge.
   - `startPRD`, `stopPRD`, `discardPRD`, `toggleDev` nehmen den PRD-Namen als Parameter statt
     `selectedPRD()`. Die PRD-Daten (Label, Phase) holt eine Hilfsfunktion aus `p.prds` über den Namen. Aufrufe
     im PRDs-Tab übergeben den Namen des gewählten PRDs, die Run-Liste den Namen unter `p.runCursor`.

5. **`internal/tui/main.go`:** `bottomBody`: bei `bottomDetails` und aktivem PRDs-Tab `runListView`, wenn
   `runList()` nicht leer ist, sonst `projectDetailsView`. Aufruf von `prdDetailsView` und `runDetailsView`
   entfernen. `bottomTitle` gibt `Runs` bzw. `Project` zurück. `detailsPRD` entfernen, es hat danach keinen Nutzer mehr.
   Felder `log`, `logKey`, `devPID` aus `model` entfernen, dafür `last` und `devPIDs` aufnehmen.

6. **`internal/tui/prds.go`:** `prdDetailsView` entfernen. `project.IsRunInfo` bleibt, `project.RunInfo`
   nutzt es.

7. **`internal/tui/keymap.go`:**
   - `updateBottom` bei `bottomDetails` mit sichtbarer Run-Liste: `j/k`, `↓/↑`, `g/G`, `home/end` bewegen
     `p.runCursor`, `l`/`→`/`enter` ruft `openLog`, `s`, `x`, `X`, `D` die Aktionen mit dem Namen unter dem
     Cursor. `i` und `z` wie bisher.
   - `updatePRDs`: `L` ruft `openLog` für das gewählte PRD, ohne Run Fehler `<label> has no run.`
   - `footerKeys`: Run-Liste
     `" j/k move · l log · s start · x stop · X discard · D dev · i edit .lazydev · z zoom · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · t terminal · m menu · q quit"`,
     PRDs-Tab ergänzt `L log` nach `l open`, Log-Ansicht
     `" j/k scroll · ctrl+d/u page · g/G top/end · z header · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+w close · t terminal · m menu · q quit"`.

8. **Tests `internal/tui/`:**
   - `layout_test.go` `TestViewSmoke`: Projekt mit zwei Runs direkt in `p.runs` (Phasen implement und
     error, ohne PID): Pane-Titel `Runs`, beide PRD-Namen sichtbar,
     `checkSize` hält. Ohne Runs zeigt Details `base` aus `projectDetailsView`.
   - Neuer Test `TestRunList`: Filterung (merged fehlt, error und aborted sind drin) und Reihenfolge nach
     `StartedAt`.
   - Neuer Test `TestLogViewWrap`: langer Eintrag bricht um, keine Zeile breiter als die Breite, Höhe exakt.
   - Neuer Test `TestLogViewFollow`: am Ende folgt die Ansicht neuen Einträgen, nach `k` nicht mehr, nach
     `G` wieder.
   - Neuer Test `TestLogViewReadsIncrementally`: Step-Log mit unvollständiger letzter Zeile, zweites Lesen
     nach Anhängen liefert jeden Eintrag genau einmal, Trennzeilen pro Step.
   - Neuer Test `TestOpenLogFocusesExisting`: zweites `openLog` auf denselben Run erzeugt keinen zweiten Tab.
   - Neuer Test `TestLogTabClosesOnMergeAndDiscard`: `runsMsg` mit merged Run bzw. ohne Run entfernt den Tab.

9. **`SPEC.md`:**
   - UI → Panes → Details pane: Beschreibung der Run-Liste (Filter, Reihenfolge, Spalten, Tasten, leere Liste,
     Files-Tab) statt PRD- und Run-Details. Satz zu `prdDetailsView`-Inhalten entfernen.
   - UI → Panes → Center: Log-Tabs als zweite Tab-Art (Titel, kein Prozess, `alt+w`, automatisches
     Schließen, Fokus auf vorhandenen Tab).
   - Abschnitt `### Live log` ersetzen durch `### Log tab`: Inhalt pro Step mit Trenner, vollständige Texte,
     Umbruch, Kopf mit `z`, Scroll-Tasten, Folgen, inkrementelles Lesen jede Sekunde. Die 160-Zeichen-Grenze
     und das Lesen der letzten 256 KiB entfallen.
   - Explorer bzw. Keys: `L` im PRDs-Tab.

10. **`README.md` und `README.de.md`** (gleicher Inhalt): Get started Schritt 4 auf Run-Liste und Log-Tab
    (`l`) umstellen. Layout-Tabelle „Below center“/„Unter der Mitte“: `Details`: Runs des Projekts, ohne Runs
    `.lazydev/`-Setup. Key-Tabelle: `L` (PRDs), `l` (Runs) für Log als Tab, `z` zusätzlich „Kopf im Log-Tab“,
    `s`/`x`/`X`/`D` gelten auch in der Run-Liste.

## Technische Hinweise

- Alle UI-Texte, Kommentare und Fehler auf Englisch. Commits nach Conventional Commits, z. B.
  `feat(tui): list runs in details pane and open logs as tabs`.
- Verhalten ändern heißt `SPEC.md`, `README.md` und `README.de.md` mitziehen.
- Die TUI liest nur Dateien. Kein neuer Schreibzugriff auf Run-State oder Logs.
- Step-Logs werden pro Step-Name angehängt (`O_APPEND` in `internal/runner/log.go`). Ein fortgesetzter Step
  schreibt in dieselbe Datei, der Offset bleibt gültig. Wird eine Datei kürzer als der Offset (Run verworfen
  und neu gestartet), Offset auf 0 und Einträge dieses Steps neu lesen.
- Die letzte Zeile eines Logs kann während des Schreibens unvollständig sein. Nur bis zum letzten `\n`
  lesen, den Rest beim nächsten Tick.
- `claude.Describe` hat nur `readLog` als Nutzer in der TUI. `claude.FirstLine` bleibt, der Runner nutzt es.
- Tastenregeln aus `SPEC.md`: `z` heißt „mehr Platz für Inhalt“, `l`/`enter` öffnen, `j/k` bewegen.
- `terminal.ExitMsg` betrifft nur Terminals. Log-Tabs bekommen IDs aus demselben Zähler `m.nextID`, damit
  IDs eindeutig bleiben.
- `renderTabs` und `layoutTabs` bleiben unverändert, sie arbeiten mit Labels.
- Tests isolieren mit `t.Setenv("XDG_STATE_HOME", t.TempDir())` (siehe `isolateTUI`).
- `View` jedes Panes muss exakt Breite und Höhe einhalten, `checkSize` in `layout_test.go` prüft das.

## Erfolgskriterien

- `make test` (`go vet ./...` und `go test ./...`) ist grün, inklusive der neuen Tests aus Schritt 1 und 8.
- `make build` erzeugt `bin/lazydev`.
- Mit drei laufenden und einem wartenden PRD zeigt der Tab `Details` bei aktivem PRDs-Tab vier Zeilen mit
  Phase, Step, Kosten, Port, Dev, Branch und letzter Logzeile, der Wartende mit `#1 in queue`.
- Ein Run mit Phase error steht in der Liste mit seiner Fehlermeldung, ein gemergter Run nicht.
- `l` auf einem Run öffnet einen Center-Tab `log <prd>` mit allen Steps, Trennern, vollständigen
  mehrzeiligen Claude-Texten und umgebrochenen Zeilen. Ein zweites `l` öffnet keinen weiteren Tab.
- `L` im PRDs-Tab auf einem PRD ohne Run zeigt `<label> has no run.`
- Im Log-Tab scrollen `j/k`, `ctrl+d/u`, `g/G`, `z` klappt den Kopf um, am Ende folgt die Ansicht live.
- Nach dem Merge oder `X` schließt sich der Log-Tab des Runs selbst.
- `grep -rn "maxDescribeLen\|prdDetailsView\|runDetailsView" internal` findet nichts.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben Run-Liste, Log-Tab und `L`.
