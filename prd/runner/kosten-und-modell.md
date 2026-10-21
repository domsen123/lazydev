# PRD: Kosten und Modell im PRD festhalten

> Status: umgesetzt (2026-10-21) · Template: einfach · Scope: Vor dem Fast-Forward schreibt lazydev die Gesamtkosten des Laufs und das Modell der Umsetzung in die Statuszeile des PRDs und committet das im Worktree. Die TUI zeigt Kosten und Modell aus der Statuszeile, wenn es keinen Lauf-Zustand gibt, und eine neue Spalte „Modell“.

## Context

Die Kosten eines PRDs stehen heute nur im Lauf-Zustand unter
`$XDG_STATE_HOME/lazydev/runs/<projekt-id>/<prd>.json` (`state.Run.Cost()`). Die Spalte „Kosten“ der
PRD-Liste und die Summe im Titel der PRD-Box lesen nur von dort. Wer ein PRD auf einem anderen Rechner
umgesetzt hat oder dessen Zustand gelöscht wurde, sieht keine Kosten. Mit welchem Modell ein PRD
umgesetzt wurde, hält lazydev nirgends fest.

Nach der Änderung trägt jedes gemergte PRD Kosten und Modell in seiner Statuszeile. Die Werte kommen mit
dem Merge ins Repo und sind auf jedem Rechner und für jedes Teammitglied sichtbar:

```
> Status: umgesetzt (2026-10-21) · Kosten: 4.12 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: …
```

Im Repo existiert bereits und wird wiederverwendet:

- `claude.Event`, `claude.Summary.Add` (`internal/claude/stream.go`): Das Ereignis `system`/`init` trägt
  das Feld `model`, z. B. `claude-opus-5-5`.
- `runClaude` und `OnChange` (`internal/runner/claude.go`): schreiben Sitzung und Kosten laufend in den
  letzten Schritt.
- `state.Step`, `state.Run.Cost()` (`internal/state/state.go`).
- `merger.run` (`internal/runner/merge.go`): Rebase, Prüfung, `waitMainClean`, Fast-Forward.
- `movePRD` (`internal/runner/runner.go`): Vorbild für einen Commit von lazydev ohne Claude und ohne
  Trailer.
- `project.ParseStatusLine`, `project.ParseHeader`, `project.ResetStatus` (`internal/project/status.go`):
  Statuszeile mit Wert und Freitext nach dem ersten `·`.
- `prdtemplate.Of` (`internal/prdtemplate/prdtemplate.go`): Vorbild für das Lesen eines Felds
  `Name: Wert` aus dem Freitext.
- PRD-Liste, Gruppenzeile und Titel der PRD-Box (`internal/tui/view.go`), `detailMeta`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Ort im PRD | Freitext der Statuszeile, als Felder `Kosten: <betrag> USD` und `Modell: <modell-id>`, direkt nach dem Statuswert und vor allen anderen Teilen des Freitexts. Getrennt mit ` · ` wie die übrigen Teile. |
| Betrag | `state.Run.Cost()` zum Zeitpunkt des Schreibens, also alle Schritte des Laufs inklusive gescheiterter Versuche und eines `/merge-prd`. Zwei Nachkommastellen mit Punkt, z. B. `4.12 USD`. |
| Modell | Das Feld `model` aus dem ersten `system`/`init`-Ereignis des letzten erfolgreichen Schritts `implement`. Volle ID, z. B. `claude-opus-5-5`. Fehlt es, entfällt das Feld `Modell:`. |
| Modell im Zustand | `state.Step` bekommt `Model string` (`json:"model,omitempty"`). `runClaude` setzt es aus dem ersten `init`-Ereignis des Schritts. `claude.Event` bekommt `Model` (`json:"model"`), `claude.Summary` bekommt `Model`, gesetzt mit der `SessionID` aus demselben Ereignis. |
| Zeitpunkt | Im Merge-Schritt nach bestandener Prüfung und vor `waitMainClean`. So zählt ein `/merge-prd` mit, und der Commit kommt mit dem Fast-Forward in den Ziel-Branch. |
| Commit | lazydev ändert die Statuszeile des PRDs im Worktree und committet sie selbst mit `git commit -m "docs(prd): record cost of <prd-name>" -- <prd-pfad>`, ohne Claude und ohne Trailer, wie `movePRD`. |
| Wiederholung | Der Schritt ist wiederholbar. Stehen `Kosten:` und `Modell:` schon in der Statuszeile, ersetzt lazydev die Werte an ihrer Stelle. Ändert sich dadurch nichts, entsteht kein Commit. |
| PRD im Ziel-Branch schon getrackt | Gleiches Verfahren. Die Datei liegt im Worktree unter `r.PRD`. |
| PRD nicht lesbar oder ohne Statuszeile | Der Merge scheitert mit `Kosten nicht im PRD vermerkt: <fehler>`. |
| Log | `Kosten im PRD vermerkt: 4.12 USD · claude-opus-5-5` und, bei einem Commit, `Commit: docs(prd): record cost of <prd-name>`. |
| Lesen | Neue Funktion `project.RunInfo(s Status) (cost float64, hasCost bool, model string)` liest beide Felder aus dem Freitext. Ein Betrag, der sich nicht als Zahl lesen lässt, gilt als nicht vorhanden. |
| PRD-Feld | `project.PRD` bekommt `CostUSD float64`, `HasCost bool` und `Model string`, gefüllt beim Einlesen aus der Statuszeile. |
| Kosten in der PRD-Liste | Gibt es einen Lauf-Zustand mit `r.Cost() > 0`, zeigt die Spalte diesen Wert wie heute. Sonst den Wert aus dem PRD. Gleiches Format wie heute (`4.12 $`). |
| Summe in Gruppenzeile und Titel | Summe über alle PRDs des Projekts mit derselben Regel: Lauf-Zustand, sonst Wert aus dem PRD. Kein PRD zählt doppelt. |
| Spalte „Modell“ | Neue Spalte zwischen „Branch“ und „Kosten“, Breite 9, Überschrift `Modell`. Wert: Modell des letzten erfolgreichen `implement` im Lauf-Zustand, sonst aus dem PRD, sonst leer. |
| Kurzform | Die Spalte zeigt eine Kurzform: `claude-` am Anfang entfällt, der erste Teil wird großgeschrieben, die restlichen Teile werden mit `.` verbunden. `claude-opus-5-5` → `Opus 5.5`, `claude-haiku-5-5` → `Haiku 5.5`. Eine ID ohne `claude-` am Anfang erscheint unverändert, gekürzt auf die Spaltenbreite mit `…`. |
| Detailansicht | `detailMeta` bekommt nach „Erstellt“ die Zeilen `Kosten` (`4.12 USD`) und `Modell` (volle ID), jeweils nur, wenn ein Wert vorhanden ist. Quelle wie in der Liste. |
| Chat-Kosten | Die Kosten des Chats, in dem das PRD entstand, zählen nicht mit. lazydev kennt die Zuordnung von Chat zu PRD nicht sicher. |

## Scope

**In Scope**

- Modell aus dem `init`-Ereignis in `claude.Event`, `claude.Summary` und `state.Step`.
- Schreiben und Committen von `Kosten:` und `Modell:` im Merge-Schritt.
- Lesen der Felder in `internal/project` und Anzeige in PRD-Liste, Gruppenzeile, Titel und Details.
- Tests, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Kosten pro Schritt oder pro Modell im PRD.
- Kosten des Chats beim Erstellen.
- Nachträgliches Eintragen für schon gemergte PRDs.
- Kosten von verworfenen Läufen (`X`).

## Umsetzung

1. **Modell aus dem Stream** (`internal/claude/stream.go`): `Event.Model` mit `json:"model"`,
   `Summary.Model`. In `Summary.Add` setzt der Fall `init` neben `SessionID` auch `Model`, solange
   `Model` leer ist. Test in `internal/claude/stream_test.go`.
2. **Modell im Zustand** (`internal/state/state.go`, `internal/runner/claude.go`): `Step.Model`.
   `OnChange` in `runClaude` setzt `last.Model = s.Model`, wenn `s.Model` nicht leer ist. Neue Methode
   `func (r *Run) ImplementModel() string` liefert das Modell des letzten Schritts `implement` mit `OK`.
3. **Felder lesen** (`internal/project/status.go`, `project.go`): `RunInfo` nach „Lesen“. `Load` füllt
   `PRD.CostUSD`, `PRD.HasCost`, `PRD.Model`. Tests in `internal/project/status_test.go`.
4. **Felder schreiben** (`internal/project/status.go`): `SetRunInfo(content string, cost float64, model string) string`
   setzt beide Felder in der ersten Statuszeile nach „Ort im PRD“ und „Wiederholung“. Ohne Statuszeile
   liefert sie den Inhalt unverändert, der Aufrufer meldet den Fehler. Tests für: Freitext ohne Felder,
   mit Feldern, mit `Template:` und `Scope:`, ohne Freitext, ohne Modell.
5. **Merge** (`internal/runner/merge.go`): neue Methode `merger.recordRunInfo()` nach der Prüfung und vor
   `waitMainClean`. Sie liest `r.PRD` im Worktree, ruft `SetRunInfo` mit `r.Cost()` und
   `r.ImplementModel()`, schreibt die Datei, committet nach „Commit“ und loggt nach „Log“.
6. **TUI** (`internal/tui/view.go`, `runs.go`).
   - Hilfsfunktion `prdCost(d, r) (float64, bool)` und `prdModel(d, r) string` nach den Regeln aus
     „Kosten in der PRD-Liste“ und „Spalte Modell“. Liste, Gruppenzeile und Titel nutzen `prdCost`.
   - Konstante `colModel = 9`, Spalte in Kopfzeile und Zeilen, `colPRD` um `colModel + 1` kleiner.
   - `shortModel(id string) string` nach „Kurzform“.
   - `detailMeta` nach „Detailansicht“. `detailMeta` braucht dafür den Lauf, Signatur entsprechend
     anpassen, auch in `internal/tui/prdview.go`.
7. **Tests.**
   - `internal/runner`: Der Fake-`claude` gibt im `init` ein `model` aus. Nach `TestMergeFastForward`
     steht im PRD auf `main` `Kosten: 0.80 USD · Modell: <modell>` in der Statuszeile. Die letzten Commits
     auf `main` sind `docs(prd): record cost of new-token`, `feat: review`, `docs(prd): add new-token`,
     `init`. Ein zweiter Merge-Versuch nach einem Fehler erzeugt keinen zweiten Commit mit gleichen Werten.
   - `internal/tui`: Spalte `Modell` mit Kurzform, Kosten aus dem PRD ohne Lauf-Zustand, Summe ohne
     Doppelzählung, Zeilen in `detailMeta`.
   - Bestehende Tests mit festen Zeilen der PRD-Liste an die neue Spalte anpassen.
8. **Doku.** `SPEC.md`: „PRD-Format“ (Felder `Kosten:` und `Modell:`), Merge-Ablauf (neuer Schritt vor dem
   Fast-Forward), Layout der PRD-Liste, Lauf-Zustand (`model` pro Schritt). `README.md` und `README.de.md`:
   ein Satz zu Kosten und Modell im PRD.

## Technische Hinweise

- Alle Texte deutsch, Commit-Nachrichten englisch nach Conventional Commits.
- Der Betrag nutzt `strconv.FormatFloat(cost, 'f', 2, 64)`, nicht die Locale.
- `ParseHeader` hängt Folgezeilen mit `>` an den Freitext an. `RunInfo` muss deshalb mit Zeilenumbrüchen
  im Freitext umgehen. Felder stehen immer in der ersten Zeile, weil `SetRunInfo` sie dort einsetzt.
- Der Merge darf durch den Commit nicht erneut prüfen. Der Commit ändert nur das PRD.
- Gleichzeitig läuft das PRD `prd/workflow/projekt-ordner.md`, das `internal/prdtemplate` entfernt und die
  Prompts umbaut. Ist es vor diesem PRD gemergt, gibt es `prdtemplate.Of` nicht mehr. `RunInfo` hängt
  davon nicht ab.
- Prüfbefehl: `make test`.

## Erfolgskriterien

- `make test` läuft ohne Fehler.
- Nach einem Merge steht in der Statuszeile des PRDs im Ziel-Branch `Kosten: <betrag> USD · Modell: <id>`
  direkt nach dem Statuswert.
- Der Commit `docs(prd): record cost of <prd-name>` hat keinen Trailer.
- Ein PRD ohne Lauf-Zustand zeigt Kosten und Modell aus der Statuszeile in Liste, Summe und Details.
- Ein laufendes PRD zeigt Kosten und Modell aus dem Lauf-Zustand.
- Die Spalte „Modell“ zeigt `Opus 5.5` für `claude-opus-5-5`.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben die Felder.
