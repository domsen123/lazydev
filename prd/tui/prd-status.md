# PRD: Status eines PRDs in der TUI ändern

> Status: umgesetzt (2026-10-21) · Kosten: 3.20 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: Taste `c` öffnet eine Auswahl für den Status des gewählten PRDs: `geplant`, `umgesetzt` oder neu `verworfen`. lazydev schreibt den Wert in die Statuszeile und committet getrackte PRDs im Hauptordner. So lassen sich PRDs, die vor lazydev umgesetzt wurden, als erledigt markieren.

## Context

lazydev liest den Status eines PRDs ohne Lauf-Zustand aus der Statuszeile (`project.ParseStatusLine`,
`project.PhaseFromStatus`). Nur `umgesetzt (YYYY-MM-DD)` landet in der Gruppe „erledigt“ der PRD-Liste
(`prdList` in `internal/tui/prdlist.go`). Den Wert setzt heute nur die Umsetzung durch Claude.

Viele Repos haben PRDs, die vor lazydev umgesetzt wurden und trotzdem auf `geplant` stehen. In phoenix
betrifft das alle 7 PRDs. Sie stehen in der Liste der offenen PRDs, und `s` würde sie erneut umsetzen.
Heute hilft nur, die Datei von Hand zu ändern und zu committen. Für PRDs, die nie umgesetzt werden, gibt
es gar keinen passenden Status.

Nach der Änderung wählst du mit `c` den Status eines PRDs. lazydev schreibt die Statuszeile und
committet die Änderung, damit der Status im Repo liegt und das ganze Team ihn sieht.

Im Repo existiert bereits und wird wiederverwendet:

- `project.ParseStatusLine`, `project.ResetStatus`, `headerScanLines` (`internal/project/status.go`).
- `project.StatusKind`, `project.PhaseFromStatus`, `Phase.Label`, `Phase.Symbol`
  (`internal/project/phase.go`, `status.go`).
- `prdList` und `prdPhase` (`internal/tui/prdlist.go`, `internal/tui/runs.go`).
- `startBlocked` (`internal/tui/actions.go`): Muster für Gründe, warum eine Taste nichts tut.
- `Model.ask` (`internal/tui/actions.go`) und `action()` für Rückfrage und Hintergrundarbeit.
- `viewTemplatePicker` bzw. `viewBranchPicker` (`internal/tui/view.go`, `internal/tui/branch.go`):
  Vorbild für ein Auswahlfenster.
- `git.Run`, `git.IsTracked` (`internal/git`).
- `movePRD` (`internal/runner/runner.go`): Vorbild für einen Commit von lazydev ohne Claude und ohne
  Trailer.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Taste | `c` im Normalmodus, unabhängig vom Fokus. Ohne gewähltes PRD bewirkt `c` nichts. |
| Erlaubt | Nur für PRDs ohne Lauf-Zustand (`m.runs.get(p, d) == nil`). Sonst zeigt die Statuszeile als Fehler `<prd> hat einen Lauf von lazydev. Verwirf ihn zuerst mit X.` |
| Werte | `geplant`, `umgesetzt` und neu `verworfen`. |
| `umgesetzt` | Schreibt `umgesetzt (<heute als YYYY-MM-DD>)`. |
| `verworfen` | Neuer Statuswert für PRDs, die nicht umgesetzt werden. Schreibt `verworfen (<heute als YYYY-MM-DD>)`. |
| Neuer Status im Code | `project.StatusDiscarded` für Werte, die mit `verworfen` beginnen. `project.PhaseDiscarded` mit Label `verworfen` und Symbol `✗`. `PhaseFromStatus` bildet `StatusDiscarded` darauf ab. |
| Gruppe „erledigt“ | Enthält PRDs mit Phase `gemergt` und `verworfen`. Die Gruppenzeile bleibt `erledigt (<anzahl>)`. |
| Start eines verworfenen PRDs | `s` zeigt `<prd> ist verworfen` als Fehler, wie bei `umgesetzt`. |
| Fenster | `Status · <bereich>/<name>` mit den drei Werten. Vorgewählt ist der aktuelle Wert, markiert mit ` (aktuell)`. `j`/`k` wählen, `Enter` übernimmt, `Esc` bricht ab. Fußzeile: `↑ ↓ Status wählen · Enter übernehmen · Esc abbrechen`. |
| Gleicher Wert | Wählst du den aktuellen Wert, schließt das Fenster ohne Änderung. Ausnahme: `umgesetzt` und `verworfen` mit anderem Datum gelten als gleich und ändern nichts. |
| Freitext | Bleibt erhalten. lazydev ändert nur den Teil vor dem ersten `·`, wie `ResetStatus`. |
| Ohne Statuszeile | `c` zeigt `<prd> hat keine Statuszeile` als Fehler. lazydev legt keine an. |
| Ungetracktes PRD | lazydev ändert die Datei und committet nicht. Meldung: `Status von <prd>: <wert>`. |
| Getracktes PRD | lazydev ändert die Datei im Hauptordner und committet nur diese Datei: `git commit -m "docs(prd): mark <name> as <wert-ohne-datum>" -- <pfad>`, ohne Claude und ohne Trailer. `<wert-ohne-datum>` ist `planned`, `done` oder `discarded`. Meldung: `Status von <prd>: <wert>, committet`. |
| Voraussetzung für den Commit | Der Hauptordner steht auf dem Ziel-Branch des Projekts, und die PRD-Datei hat keine uncommitteten Änderungen. Sonst ändert lazydev nichts und zeigt als Fehler `Hauptordner steht auf <branch> statt <ziel-branch>` bzw. `<pfad> hat uncommittete Änderungen`. Andere geänderte Dateien im Hauptordner stören nicht. |
| Rückfrage | Für getrackte PRDs vor dem Schreiben: `Status von <prd> auf <wert> setzen und committen? (j/n)`. Für ungetrackte PRDs keine Rückfrage. |
| Push | Kein Push. |
| Nach der Änderung | lazydev liest die Projekte neu ein. Das PRD wechselt sofort in die passende Gruppe. |

## Scope

**In Scope**

- Statuswert `verworfen` mit Status-Art, Phase, Label und Symbol.
- Funktion zum Setzen des Statuswerts in `internal/project`.
- Taste `c`, Auswahlfenster, Rückfrage und Commit in der TUI.
- Gruppe „erledigt“ mit verworfenen PRDs, `s` blockiert bei `verworfen`.
- Tests, Hilfe, Fußzeile, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Mehrere PRDs auf einmal ändern.
- Ein anderes Datum als heute für `umgesetzt` oder `verworfen`.
- Status ändern für PRDs mit Lauf-Zustand.
- Unterbefehl in der CLI.
- Push des Commits.

## Umsetzung

1. **Status `verworfen`** (`internal/project/status.go`, `phase.go`): `StatusDiscarded` in
   `ParseStatusLine` für Werte mit Präfix `verworfen`, Datum wie bei `umgesetzt` in `Status.Date`.
   `PhaseDiscarded` mit Label und Symbol, `PhaseFromStatus` erweitern. Tests in
   `internal/project/status_test.go`.
2. **Status setzen** (`internal/project/status.go`): `SetStatus(content, value string) (string, bool)`
   ersetzt den Teil vor dem ersten `·` der ersten Statuszeile innerhalb von `headerScanLines` und behält
   den Freitext. `bool` ist false ohne Statuszeile. `ResetStatus` nutzt danach `SetStatus(content, "geplant")`.
   Tests: mit und ohne Freitext, ohne Statuszeile, Statuszeile nicht in Zeile 2.
3. **Commit** (neue Datei `internal/project/setstatus.go`): `ChangeStatus(projectDir, base, rel, value string) (committed bool, err error)`
   prüft Ziel-Branch und uncommittete Änderungen der Datei nach „Voraussetzung für den Commit“, schreibt
   die Datei und committet getrackte PRDs nach „Getracktes PRD“. Tests gegen ein temporäres Git-Repo mit
   `GIT_AUTHOR_*` und `GIT_COMMITTER_*`.
4. **TUI** (`internal/tui`).
   - `modeStatus` und Datei `internal/tui/status.go` mit Auswahl, Tasten und Fenster nach „Fenster“.
   - `c` in `updateNormal`, Prüfung nach „Erlaubt“ und „Ohne Statuszeile“.
   - Nach `Enter`: bei getracktem PRD `ask` nach „Rückfrage“, dann `ChangeStatus` über `action()`; bei
     ungetracktem direkt. Danach Projekte neu einlesen.
   - `prdList`: `PhaseDiscarded` in die Gruppe „erledigt“.
   - `startBlocked`: `verworfen` wie `umgesetzt`.
   - Hilfe (`helpSections`, Abschnitt „PRDs“), Fußzeile (`prdKeys`: `c Status` nach `v ansehen`, nur ohne
     Lauf-Zustand), Fußzeile im Fenster.
   - Tests in `internal/tui/status_test.go`: Auswahl, Rückfrage, Commit, ungetracktes PRD, PRD mit Lauf,
     Gruppe „erledigt“ mit verworfenem PRD. Bestehende Tests der Fußzeile anpassen.
5. **Doku.** `SPEC.md`: „PRD-Format“ (Statuswert `verworfen`), Phasen-Tabelle, Tastentabelle, Abschnitt zu
   `c`. `README.md` und `README.de.md`: Tastentabelle.

## Technische Hinweise

- Alle Texte deutsch, Commit-Nachrichten englisch nach Conventional Commits.
- Git nur über `internal/git`. Prüfung auf uncommittete Änderungen: `git status --porcelain -- <pfad>`.
- Der Ziel-Branch kommt aus der Projekt-Konfiguration. Läuft parallel `prd/workflow/projekt-ordner.md`,
  liegt er danach in `.lazydev/config.yaml`. Nutze die Funktion, die zum Zeitpunkt der Umsetzung den
  wirksamen Ziel-Branch liefert.
- Ein Commit im Hauptordner ändert den Ziel-Branch. Laufende PRDs holen ihn beim Rebase im Merge.
- Prüfbefehl: `make test`.

## Erfolgskriterien

- `make test` läuft ohne Fehler.
- `c` auf einem getrackten PRD mit `geplant`, Auswahl `umgesetzt`, `j`: Die Statuszeile lautet
  `umgesetzt (<heute>)`, der Freitext ist unverändert, und `git log -1` zeigt
  `docs(prd): mark <name> as done` ohne Trailer.
- Das PRD erscheint danach in der Gruppe „erledigt“.
- `verworfen` legt das PRD ebenfalls in „erledigt“, Symbol `✗`, und `s` startet es nicht.
- Ein ungetracktes PRD ändert sich ohne Commit.
- Ein PRD mit Lauf-Zustand lässt sich nicht ändern.
- Steht der Hauptordner nicht auf dem Ziel-Branch, ändert lazydev nichts und meldet den Grund.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben `c` und `verworfen`.
