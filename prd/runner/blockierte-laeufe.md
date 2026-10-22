# PRD: Blockierte Läufe

> Status: umgesetzt (2026-10-22) · Scope: Bricht Claude in implement oder im Claude-Schritt merge mit offenen Fragen ab, endet der Lauf in der neuen Phase `blocked` statt `error`. Der Runner schreibt die Fragen als `## Blocker` ins PRD und setzt die Statuszeile auf `blockiert (YYYY-MM-DD)`. Taste `f` öffnet einen interaktiven Claude-Tab im Worktree, der das PRD mit dem Nutzer klärt. Danach geht es mit `s` weiter.

## Context

Heute endet jeder fehlgeschlagene Schritt in der Phase `error` (`state.Run.Fail`,
`internal/state/state.go`), mit dem Event `failed` und rotem `✗`. Das gilt auch, wenn Claude sauber
arbeitet und bewusst mit `## Ergebnis` `abgebrochen` endet, weil das PRD eine Entscheidung offenlässt
(`checkReport`, `internal/runner/claudestep.go`). Auslöser war `feat/workspace-projects` in
net.marx-media.start: Die Umsetzung war fertig, alle Prüfungen grün, aber das PRD widersprach dem
bestehenden Code bei geheimen Artikeln im Schlüsselkanal. lazydev zeigte das als Fehler, nur die erste
offene Frage landete in `error`, und der Weg zurück war, das PRD im Worktree von Hand zu suchen und zu
ändern.

Ein solcher Abbruch ist gewollt und kein Fehler. Er braucht eine Entscheidung des Nutzers, keinen
zweiten Versuch. Nach der Änderung endet der Lauf in `blocked`. Alle offenen Fragen stehen im PRD, die
Statuszeile sagt `blockiert`, und `f` startet einen Dialog mit Claude, der das PRD klärt. `s` setzt den
Lauf danach beim blockierten Schritt fort.

Außerdem stehen im Bericht von implement heute auch reine Hinweise unter `## Offene Fragen` (z. B.
„einmal versehentlich `cd` benutzt“). Mit `blocked` würden sie den Lauf blockieren. Sie bekommen einen
eigenen Abschnitt `## Hinweise`, der das Ergebnis nicht beeinflusst.

Im Repo existiert bereits und wird wiederverwendet:

- `checkReport`, `claudeStep`, `mergeClaude`, `runClaude` (`internal/runner/claudestep.go`,
  `internal/runner/merge.go`).
- `runStep` (`internal/runner/runner.go`): Ende eines Schritts mit `Fail`, `notifyRun` und Log.
- `recordRunInfo` (`internal/runner/merge.go`): Vorbild für einen Commit des PRDs im Worktree ohne
  Claude und ohne Trailer.
- `requireClean` (`internal/runner`) vor dem Merge.
- `project.ParseStatusLine`, `project.SetStatus`, `project.HeaderScanLines`, `ValueDiscarded`
  (`internal/project/status.go`), `Phase`, `Symbol`, `Done`, `Active` (`internal/project/phase.go`).
- `state.Run.Fail`, `Effective`, `NextStep` (`internal/state/state.go`).
- `notify.Event`, `titles`, `Payload` (`internal/notify/notify.go`), `notifyRun`
  (`internal/runner/notify.go`).
- `createPRD` (`internal/tui/create.go`): Vorbild für einen interaktiven Claude-Tab mit
  `prompts.Build` und `repo.Init`.
- `openTerminal` (`internal/tui/main.go`), `updatePRDs` (`internal/tui/keymap.go`), `startPRD`
  (`internal/tui/runview.go`), `phaseColor` (`internal/tui/prds.go`).
- `claude.Agents` (`internal/claude/agents.go`) und die Übergabe per `--agents` in `claude.Run`.
- `repo.Steps`, `repo.Init`, `repo.WritePrompt`, `workflows/simple/` (`internal/repo`).
- Contracts in `internal/prompts/contract/`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Wann blockiert | Claude-Schritt implement oder Claude-Schritt merge endet mit Exit-Code 0, letztem `result` ohne Fehler, `## Ergebnis` `abgebrochen` und mindestens einem Punkt unter `## Offene Fragen` (implement) bzw. `## Offene Punkte` (merge), der nicht „keine“ ist. |
| Wann weiter `error` | Alles andere: Exit-Code ≠ 0, fehlendes `## Ergebnis`, `abgebrochen` ohne Fragen, rote Prüfung, Status-Check, Hooks, Git-Fehler. Review blockiert nie. |
| Phase | Neu `project.PhaseBlocked = "blocked"`, Symbol `⊘`, Farbe gelb `#d7af00` in `phaseColor`. Nicht `Active`, nicht `Done`. Das PRD bleibt in der Gruppe der offenen PRDs. |
| Run-State | `Phase` `blocked`, `Error` `<step> blocked: <erste Frage>`, `PID` 0. Der Schritt endet mit `ok: false`, `NextStep` liefert ihn wieder. |
| Statuswert im PRD | Neu `blockiert (<heute als YYYY-MM-DD>)`, deutsch wie die übrigen Werte. `project.ValueBlocked`, `project.StatusBlocked`, Datum über `statusDateRe`. `PhaseFromStatus` bildet `StatusBlocked` auf `PhaseBlocked` ab. Der Freitext nach `·` bleibt. |
| Abschnitt im PRD | Der Runner hängt ans Ende des PRDs im Worktree `## Blocker`, erste Zeile `Schritt: implement` bzw. `Schritt: merge`, danach eine Leerzeile und alle Punkte des Abschnitts der offenen Fragen unverändert als Liste. Gibt es schon einen Abschnitt `## Blocker`, ersetzt er ihn bis zur nächsten `## `-Überschrift oder zum Dateiende. |
| Commit des Blockers | Der Runner committet nur das PRD im Worktree ohne Claude und ohne Trailer: `git commit -m "docs(prd): record blocker of <name>" -- <prd>`. Andere uncommittete Änderungen aus implement bleiben uncommittet. |
| Discord | Neues Event `blocked`, Titel `⊘ Blocked: <area>/<name>`, Farbe `0xe67e22`. Felder wie bei den anderen Events plus `Blocker` (nicht inline) mit allen Fragen, zeilenweise, auf 1024 Zeichen gekürzt mit `…` am Ende. Kein `failed` für blockierte Läufe. |
| Taste `f` | In der PRD-Liste und in der Run-Liste auf einem Lauf in Phase `blocked`. Sonst Fehler `<prd> is not blocked.`. Öffnet einen interaktiven Claude-Tab im Worktree des Laufs, Tab-Label `unblock`. |
| Prompt des Dialogs | Neuer Schritt `unblock` mit `.lazydev/unblock.md` aus `workflows/simple/` und Contract `internal/prompts/contract/unblock.md`. `$ARGUMENTS` ist der PRD-Pfad relativ zum Worktree. `repo.Init` vor dem Öffnen legt eine fehlende `unblock.md` an, wie bei create. |
| Aufruf des Dialogs | `claude <prompt> --agents <json der eingebauten Agenten>`, interaktiv, ohne `-p`, Arbeitsverzeichnis der Worktree. |
| Regeln des Dialogs | Claude liest `## Blocker`, stellt die Fragen nummeriert mit Optionen A bis D, wartet auf die Antwort und arbeitet die Entscheidungen ins PRD ein. Danach entfernt es den Abschnitt `## Blocker` und setzt die Statuszeile: bei `Schritt: implement` auf `geplant`, bei `Schritt: merge` auf `umgesetzt (<heute>)`. Es ändert nur das PRD, keinen Code, nichts unter `.lazydev/`, und committet nicht. |
| Nach dem Dialog | Kein automatischer Start. Der Nutzer startet mit `s`. |
| `s` auf blockiertem Lauf | Steht die Statuszeile des PRDs im Worktree noch auf `blockiert`, Fehler `<prd> is blocked. Resolve it with f first.` und kein Start. Sonst Rückfrage `Continue <prd>? (y/n)` wie heute, weiter mit dem blockierten Schritt. |
| Runner bei blockiertem PRD | `Run` prüft vor dem nächsten Schritt die Statuszeile im Worktree. Steht sie auf `blockiert`, endet der Runner mit Fehler `<prd> is still blocked`, ohne Run-State, Phase oder Log zu ändern. |
| Merge nach dem Dialog | Ist vor dem Schritt merge genau die PRD-Datei uncommittet, committet der Runner sie ohne Claude: `git commit -m "docs(prd): resolve blocker of <name>" -- <prd>`. Danach gilt `requireClean` wie heute. |
| Implement nach dem Dialog | Kein Commit, die Änderung am PRD committet der Schritt review mit dem Rest. |
| Verwerfen mit `X` | Holt das PRD wie heute zurück und setzt `geplant`. Zusätzlich entfernt lazydev einen Abschnitt `## Blocker`. |
| `## Hinweise` | Der Contract von implement bekommt den Abschnitt `## Hinweise` zwischen `## Offene Fragen` und `## Nicht geprüft`, für Bemerkungen ohne Entscheidung. Er beeinflusst das Ergebnis nicht. `## Offene Fragen` nur für Entscheidungen, die der Nutzer treffen muss. Der Contract von merge bleibt unverändert. |

## Scope

**In Scope**

- Phase `blocked`, Statuswert `blockiert`, Abschnitt `## Blocker`, Commit des Blockers.
- Erkennung in implement und im Claude-Schritt merge.
- Discord-Event `blocked`.
- Schritt `unblock` mit Prompt, Contract und Taste `f`.
- Prüfungen für `s` in TUI und Runner, Commit des geklärten PRDs vor dem Merge.
- Entfernen von `## Blocker` beim Verwerfen.
- Abschnitt `## Hinweise` im Contract von implement.
- Tests, Hilfe, Fußzeilen, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Blockieren im Schritt review.
- Automatisches Fortsetzen nach dem Dialog.
- Antworten auf Blocker über Discord oder ohne TUI.
- Code-Änderungen oder Konfliktlösung im Dialog.
- Umstellen bereits beendeter Läufe in Phase `error` auf `blocked`.

## Umsetzung

1. **Status und Phase** (`internal/project/status.go`, `phase.go`).
   - `ValueBlocked = "blockiert"`, `StatusBlocked`, Präfix-Erkennung in `ParseStatusLine`,
     `statusDateRe` um `blockiert` erweitern.
   - `PhaseBlocked` mit Symbol `⊘`, `PhaseFromStatus` erweitern.
   - `SetBlocker(content, step string, points []string) string` hängt `## Blocker` an oder ersetzt ihn
     nach „Abschnitt im PRD“. `RemoveBlocker(content string) string` entfernt ihn samt der Leerzeile davor.
   - Tests in `internal/project/status_test.go`: Parsen mit Datum, `SetBlocker` neu und ersetzend,
     `RemoveBlocker` am Dateiende und vor einer weiteren Überschrift.
2. **Blockierter Abbruch erkennen** (`internal/runner/claudestep.go`).
   - Neuer Typ `blockedError{step string; points []string}` mit `Error()` `<step> blocked: <erster Punkt>`.
   - `checkReport` liefert ihn bei `abgebrochen` mit Punkten. Alle Punkte des Abschnitts, ohne `- `
     am Anfang, leere Zeilen und „keine“ ausgenommen. Ohne Punkte bleibt `<step> aborted`.
   - `claudeStep` und `mergeClaude` reichen den Fehler unverändert durch, `merge` ebenso (kein Umwickeln
     mit `fmt.Errorf` ohne `%w`).
3. **Lauf blockieren** (`internal/runner/runner.go`, `internal/state/state.go`).
   - `state.Run.Block(msg string)` setzt `Error` und `Phase` `blocked`.
   - In `runStep` vor dem Fall `err != nil`: `errors.As(err, &blocked)` → PRD im Worktree lesen,
     `SetStatus` auf `blockiert (<heute>)`, `SetBlocker`, schreiben, Commit nach „Commit des Blockers“,
     `r.EndStep(0, false)`, `r.Block`, `PID` 0, speichern, `notifyRun(…, notify.Blocked, …)`. Schlägt
     Schreiben oder Commit fehl, wird der Lauf `error` mit diesem Fehler.
   - In `Run` vor dem nächsten Schritt die Prüfung nach „Runner bei blockiertem PRD“.
   - Vor dem Schritt merge der Commit nach „Merge nach dem Dialog“ (`git status --porcelain`, genau eine
     Zeile mit dem PRD-Pfad).
4. **Verwerfen** (`internal/runner/discard.go`): Beim Zurückholen des PRDs zusätzlich `RemoveBlocker`.
5. **Discord** (`internal/notify/notify.go`, `internal/runner/notify.go`): Event `Blocked`, Eintrag in
   `titles`, `Message.Blocker string`, Feld `Blocker` nur bei diesem Event. `notifyRun` bekommt die Punkte.
   Tests in `internal/notify/notify_test.go` (Titel, Farbe, Feld, Kürzung) und
   `internal/runner/notify_test.go`.
6. **Contracts und Prompts.**
   - `internal/prompts/contract/implement.md`: Abschnitt `## Hinweise` im Bericht und die Regel aus
     „`## Hinweise`“.
   - Neu `internal/prompts/contract/unblock.md` nach „Regeln des Dialogs“, deutsch, im Stil von
     `create.md`. Zum Schluss nennt Claude: `Weiter in lazydev mit s.`
   - Neu `internal/repo/workflows/simple/unblock.md`: Aufgabe „Kläre die Blocker des PRDs
     `$ARGUMENTS` mit dem Nutzer.“, im Stil von `create.md`.
   - `repo.Steps` um `unblock` nach `merge` erweitern. Prüfen, dass Wizard und `KnowledgeKeys` den neuen
     Schritt korrekt zeigen. Tests in `internal/prompts/prompts_test.go` und `internal/repo`.
7. **TUI** (`internal/tui`).
   - Neue Datei `internal/tui/unblock.go`: `unblockPRD(name string)` prüft Phase `blocked`, ruft
     `repo.Init`, baut den Prompt mit `prompts.Build(p.path, r.Worktree, cfg, "unblock", …)` und öffnet
     den Tab im Worktree. `openTerminal` bekommt dafür eine Variante mit Arbeitsverzeichnis.
   - `f` in `updatePRDs` und `updateRunList`.
   - `startPRD`: Prüfung nach „`s` auf blockiertem Lauf“, Statuszeile aus dem PRD im Worktree.
   - `phaseColor`: `PhaseBlocked` gelb. Run-Liste, Log-Kopf und Details zeigen bei `blocked` die erste
     Zeile von `Error` wie bei `error` (`internal/tui/runview.go`, `logview.go`).
   - Hilfe (Abschnitt PRDs und Runs) und Fußzeile: `f unblock`, nur bei Phase `blocked`.
   - Tests: `f` auf blockiertem und nicht blockiertem PRD, `s` bei `blockiert` und nach der Klärung,
     Fußzeile, Symbol und Farbe.
8. **Runner-Tests** (`internal/runner/runner_test.go`, `merge_test.go`, Fake-Skript in
   `helpers_test.go`). Das Fake-`claude` bekommt Szenarien für implement und merge mit `abgebrochen` und
   offenen Fragen. Geprüft wird:
   - Phase `blocked`, `Error`, Statuszeile `blockiert`, Abschnitt `## Blocker` mit allen Punkten, Commit
     `docs(prd): record blocker of <name>`, uncommittete Änderungen von implement bleiben uncommittet.
   - `abgebrochen` ohne Fragen bleibt `error`.
   - `Run` mit PRD noch auf `blockiert` ändert nichts.
   - Nach Klärung auf `geplant` läuft implement erneut. Nach Klärung auf `umgesetzt` committet der Runner
     das PRD vor dem Merge mit `docs(prd): resolve blocker of <name>` und merged.
   - `X` entfernt `## Blocker`.
9. **Doku.** `SPEC.md`: PRD-Format (Statuswert `blockiert`, Abschnitt `## Blocker`), Phasen und
   Symbole, Run (`s`), Schritte implement und merge, neuer Abschnitt zu `f` und `unblock`, Discord-Tabelle,
   Tastentabelle. `README.md` und `README.de.md`: Tasten, Phasen, Ablauf bei blockierten Läufen.

## Technische Hinweise

- UI-Texte, Fehler, Kommentare englisch. Statuswert `blockiert`, Abschnitt `## Blocker`, `Schritt:`,
  Contracts und Prompts deutsch, wie die bestehenden.
- Commits von lazydev ohne Claude und ohne Trailer, Git nur über `internal/git`.
- Das PRD eines Laufs liegt im Worktree (`r.Worktree`, `r.PRD`), nicht im Hauptordner. Die TUI liest
  es über `d.Root`.
- Die TUI steuert nichts direkt. `f` öffnet nur einen Tab, `s` startet wie heute den Runner.
- `checkLazydevUntouched` läuft weiter nach implement und merge. Der Dialog läuft außerhalb des Runners
  und wird nicht geprüft, der nächste Schritt prüft `.lazydev/` ohnehin.
- `NextStep` liefert nach `ok: false` den Schritt erneut, `merge` beginnt dabei wieder mit `requireClean`
  und Rebase.
- Prüfbefehl: `make test`.

## Erfolgskriterien

- `make test` läuft ohne Fehler.
- Ein implement mit `abgebrochen` und zwei offenen Fragen endet in Phase `blocked` mit `⊘` in gelb. Das
  PRD im Worktree steht auf `blockiert (<heute>)`, endet mit `## Blocker`, `Schritt: implement` und beiden
  Fragen, und `git log -1` im Worktree zeigt `docs(prd): record blocker of <name>` ohne Trailer.
- Discord bekommt `⊘ Blocked: <area>/<name>` mit dem Feld `Blocker` und kein `failed`.
- Ein Hinweis unter `## Hinweise` blockiert nicht.
- `f` öffnet einen Claude-Tab `unblock` im Worktree. Nach der Klärung steht das PRD auf `geplant`, ohne
  `## Blocker`, und `s` setzt bei implement fort.
- `s` vor der Klärung zeigt `<prd> is blocked. Resolve it with f first.`.
- Ein blockierter Claude-Schritt merge setzt nach der Klärung mit dem Commit
  `docs(prd): resolve blocker of <name>` fort und merged.
- `abgebrochen` ohne Fragen, rote Prüfungen und Fehler im Review enden weiter in `error`.
- `X` auf einem blockierten Lauf holt das PRD ohne `## Blocker` mit `geplant` zurück.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben `blocked`, `blockiert`, `f` und das Event `blocked`.
