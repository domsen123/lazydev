# PRD: Pull Requests statt Fast-Forward und ein Schritt verify mit Belegen

> Status: geplant · Scope: `merge: pr` in `.lazydev/config.yaml` lässt den Merge den Branch pushen und mit `gh` einen Pull Request öffnen, statt per Fast-Forward zu mergen. Der Lauf wartet in der neuen Phase `pr`, bis der PR auf GitHub gemergt oder geschlossen ist. Ein neuer optionaler Schritt `verify` zwischen implement und review lässt Claude die laufende App prüfen und Belege in einen Ordner des Laufs schreiben. Im PR-Modus schreibt review daraus Titel und Beschreibung des PR.

## Context

lazydev bringt heute jedes PRD per `git merge --ff-only` in den Basis-Branch des Hauptordners
(`internal/runner/merge.go`). `SPEC.md` nennt Pull Requests ausdrücklich als Nicht-Ziel. Für Teams, deren
`main` durch Branch Protection geschützt ist (Pflicht-Checks in CI, PR-Vorlage mit Abschnitt „Evidence“),
umgeht dieser Weg jede Schranke: Code landet ohne PR, ohne CI und ohne Review im Basis-Branch.

Zweitens prüft heute kein Schritt, ob das Feature in der laufenden App funktioniert. implement führt die
Prüfbefehle aus `CLAUDE.md` aus, review liest den Diff (`structural-completeness-reviewer`), der Check-Hook
läuft vor dem Merge. Das sind statische Prüfungen. Ein Review von Belegen (Screenshots, Snapshots,
Antworten der API) statt des Diffs ist nicht möglich, weil kein Schritt Belege erzeugt und keine Stelle sie
aufbewahrt.

Auslöser ist ein Projekt (Inhouse), das nach dem Vortrag „All Code Is Slop (Except Mine)“ von Lauren Tan
arbeitet: Agenten verifizieren ihre Arbeit in der echten App mit einer CLI und einer Feature Map, der PR
trägt die Belege, Menschen prüfen die Belege, später mergen Agenten selbst.

Nach der Änderung:

- `.lazydev/config.yaml` bekommt `merge: ff | pr`, Standard `ff`. Mit `pr` rebased der Merge auf
  `origin/<base>`, führt den Check-Hook aus, pusht `feat/<prd>` nach `origin` und öffnet mit `gh pr create`
  einen PR. Der Lauf wartet in der Phase `pr`, ohne einen der drei Runner-Plätze zu belegen. Ist der PR
  gemergt, räumt lazydev auf wie nach einem Fast-Forward und zieht den Basis-Branch im Hauptordner nach.
- Ein neuer Schritt `verify` läuft zwischen implement und review, wenn `.lazydev/verify.md` existiert.
  Claude prüft das Feature in der laufenden App und legt Belege in den Ordner `<prd>.evidence/` des Laufs.
  Ohne `verify.md` überspringt der Runner den Schritt ohne Claude.
- Im PR-Modus schreibt review zusätzlich Titel und Beschreibung des PR nach `<prd>.evidence/pr.md`, nach
  der PR-Vorlage des Repos, mit den Belegen aus dem Bericht von verify.

Im Repo existiert bereits und wird wiederverwendet:

- `merge`, `rebaseAndCheck`, `mergeClaude`, `recordRunInfo`, `mainDirProblem`, `waitMergeLock`, `sleep`,
  `MergePollInterval` (`internal/runner/merge.go`).
- `Run`, `runStep`, `stepPhases`, `countRunning`, `startWaiting`, `prepare` (`internal/runner/runner.go`).
- `claudeStep`, `runClaude`, `callClaude`, `checkSummary`, `checkReport`, `reportPoints`,
  `checkLazydevUntouched`, `blockedError` (`internal/runner/claudestep.go`), `recordBlocker`,
  `commitResolvedPRD` (`internal/runner/blocker.go`), `resolve` (`internal/runner/resolve.go`).
- `hookEnv`, `runHook`, `StartDev`, `removeWorktree`, `requireClean` (`internal/runner/worktree.go`),
  `devserver.Stop` (`internal/devserver`), `Discard` (`internal/runner/discard.go`).
- `state.Steps`, `state.LogSteps`, `Run.NextStep`, `Run.Block`, `Run.Fail`, `state.LogPath`,
  `state.RunsDir`, `state.Remove` (`internal/state/state.go`).
- Phasen in `internal/project/phase.go`, Statuswerte und `SetRunInfo` (`internal/project/status.go`).
- `prompts.Build`, `prompts.Vars`, die Verträge unter `internal/prompts/contract/`, der einfache Workflow
  unter `internal/repo/workflows/simple/`, `repo.Steps`, `repo.Init`, `repo.Read`, `repo.Config`
  (`internal/repo/repo.go`).
- `claude.Options` und `claude.Run` (`internal/claude/run.go`).
- `notify` mit den Ereignissen in `internal/notify/notify.go`, `notifyRun` (`internal/runner/notify.go`).
- `Overview`, `fillRun`, `stepInfos`, `Changes`, `Busy` (`internal/runner/overview.go`) für `status`,
  `wait` und den Mod.
- `git.Run`, `git.Lines` (`internal/git`).

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Schalter | Feld `merge` in `.lazydev/config.yaml`, Werte `ff` und `pr`. Fehlt es oder ist es leer, gilt `ff`. Ein anderer Wert ist der Config-Fehler `config.yaml: merge must be ff or pr, not "<wert>"`. Er erscheint wie kaputtes YAML in der Detailansicht, Läufe starten nicht. |
| Trust | `merge` gehört nicht zu den vertrauten Abschnitten und geht nicht in `TrustHash` ein. Es führt keinen Code aus und erweitert keine Rechte, es pusht nur den eigenen Branch nach `origin` des Projekts. |
| Werkzeug | `gh` (GitHub CLI) für PRs, `git` für Fetch und Push. Remote ist immer `origin`. Andere Hoster als GitHub sind out of scope. |
| Vorbedingung im PR-Modus | Zu Beginn des Schritts merge prüft lazydev `gh auth status`. Fehlt `gh`: Fehler `gh not found, install the GitHub CLI for merge: pr`. Nicht angemeldet: `gh is not authenticated, run gh auth login`. Beides ist Phase error, `s` setzt beim Merge fort. |
| Rebase im PR-Modus | `git fetch origin <base>` im Worktree, dann Rebase auf `origin/<base>` statt auf den lokalen Basis-Branch. `rebaseAndCheck` und `mergeClaude` bekommen das Ziel als Parameter, im Vertrag von merge steht dafür `$BASE` = `origin/<base>`. Konflikte, Neuanlage der Datenbanken und Check-Hook laufen wie heute. |
| Warten auf den Hauptordner | Entfällt vor dem Push. Der Hauptordner spielt erst nach dem Merge des PR eine Rolle (siehe unten). |
| Push | `git push --force-with-lease -u origin feat/<prd>` im Worktree, nach `recordRunInfo`. `--force-with-lease`, weil ein erneuter Merge nach einem Fehler einen rebasten Branch pusht. Ein Fehler von git ist Phase error mit der Meldung von git. |
| PR öffnen | Gibt es für `feat/<prd>` schon einen offenen PR (`gh pr view feat/<prd> --json url,state`), nimmt lazydev ihn und aktualisiert Titel und Beschreibung mit `gh pr edit`. Sonst `gh pr create --base <base> --head feat/<prd> --title <titel> --body-file <datei>`. Titel und Datei kommen aus `<prd>.evidence/pr.md` (siehe review). Fehlt die Datei, gilt als Titel der Titel des PRDs und als Beschreibung der Scope der Statuszeile plus die Zeile ``PRD: `<pfad des PRDs>` ``. |
| Merge-Lock | Gilt für Rebase, Check, Push und PR. Danach gibt lazydev den Lock frei, bevor es wartet. |
| Phase `pr` | Neue Phase nach dem Öffnen des PR. Der Runner lebt weiter und fragt alle 60 s (`PRPollInterval`, in Tests kürzer) `gh pr view <url> --json state` ab. Ein Runner in Phase `pr` zählt nicht für `MaxParallel`. Beim Wechsel in `pr` ruft der Runner einmal `startWaiting`, damit wartende PRDs sofort starten. |
| Fehler beim Abfragen | Ein Fehler von `gh` beendet das Warten nicht. lazydev schreibt ihn einmal ins Log (`PR state not readable: <erste Zeile>`) und setzt die Notiz `pull request state not readable`. Die nächste erfolgreiche Abfrage löscht die Notiz und loggt `PR state readable again`. |
| PR gemergt | Status `MERGED`: Teardown-Hook, Datenbanken löschen, Worktree entfernen, lokalen Branch mit `git branch -D` löschen (GitHub kann squashen oder rebasen, `-d` scheitert dann). Danach im Hauptordner `git fetch origin <base>` und, wenn `mainDirProblem` leer ist, `git merge --ff-only origin/<base>`. Sonst die Notiz `main folder not updated: <problem>`. Kein Warten. Fehler beim Aufräumen werden wie heute zu `cleanup incomplete: …`. Phase merged, Benachrichtigung `merged`. |
| PR geschlossen ohne Merge | Status `CLOSED`: Phase error mit `pull request <url> closed without merge`. Worktree und Branch bleiben. `s` setzt beim Merge fort: Rebase, Check, Push und ein neuer PR. |
| Remote-Branch nach dem Merge | lazydev löscht ihn nicht. Das übernimmt die GitHub-Einstellung „Automatically delete head branches“. |
| Merge des PR | lazydev merged keinen PR, weder sofort noch per `gh pr merge --auto`. Menschen oder GitHub-Regeln mergen. Auto-Merge ist out of scope. |
| `X` im PR-Modus | Hat der Lauf einen offenen PR, schließt `X` ihn mit `gh pr close <url> --delete-branch` vor dem Teardown-Hook. Ein Fehler kommt in die Fehlerliste von `Discard`, der Rest läuft weiter. |
| Run-State | Neues Feld `pullRequest` (URL). Es bleibt nach dem Merge erhalten. |
| Neustart eines Runners in Phase `pr` | Ein Runner, der in Phase `pr` endet (Neustart, `x`), zeigt die Phase aborted wie heute. `s` setzt mit dem Warten fort: Ist `pullRequest` gesetzt und der Branch-Kopf gleich dem Kopf des PR (`gh pr view --json headRefOid`), wartet der Runner direkt weiter, ohne erneuten Rebase und Push. Sonst läuft der Merge von vorn. |
| Schritt `verify` | Neuer Schritt zwischen implement und review. Reihenfolge: plan → setup → implement → verify → review → merge. `state.Steps` enthält ihn immer, `stepPhases` bekommt `verify` → Phase `verify`. |
| Wann verify läuft | Wenn `.lazydev/verify.md` im Hauptordner existiert. Sonst endet der Schritt ohne Claude mit dem Log `No .lazydev/verify.md, verify skipped` und gilt als erfolgreich. |
| Prompt `verify.md` | `repo.Steps` bekommt `verify` nach `implement`. `repo.Init` legt `verify.md` nicht an, auch nicht in neuen Projekten: der Schritt ist opt-in. `internal/repo/workflows/simple/verify.md` ist die Vorlage, die der Wizard mit `r` schreibt. |
| Ordner für Belege | `$XDG_STATE_HOME/lazydev/runs/<id>/<prd>.evidence/`, angelegt vor verify und review. Er bleibt nach dem Merge bestehen und verschwindet mit `X` (`state.Remove`). Claude bekommt ihn per `--add-dir` (neues Feld `AddDirs` in `claude.Options`), Hooks und Dev-Server als `LAZYDEV_EVIDENCE`. |
| Neue Platzhalter | `$EVIDENCE` (absoluter Pfad des Ordners) und `$PORT` (Port des Laufs) in Prompts und Verträgen, neben `$ARGUMENTS`, `$BASE`, `$BRANCH`. |
| Dev-Server bei verify | Ist `hooks.dev` gesetzt, startet der Runner ihn vor verify wie `D` und stoppt ihn danach, auch bei Fehler und Abbruch. Ohne `hooks.dev` startet der Runner nichts. Das Projekt startet die App dann im Prompt mit Befehlen, die von selbst enden, z. B. `pnpm app start`. |
| Vertrag `verify` | Neu: `internal/prompts/contract/verify.md`. Claude prüft das Feature des PRDs `$ARGUMENTS` in der laufenden App wie ein Nutzer, legt Belege (Screenshots, Snapshots, Antworten, Log-Auszüge) nur unter `$EVIDENCE` ab und vergleicht sie mit `## Erfolgskriterien`. Findet es einen Fehler, behebt es ihn im Worktree, führt die Prüfbefehle aus `CLAUDE.md` erneut aus und verifiziert erneut. Es committet nicht, das macht review. Prozesse, die es selbst startet, stoppt es vor dem Ende. Kein Watch-Modus und kein Befehl, der nicht von selbst endet. Kein `git push`, nichts unter `.lazydev/`, keine zusammengesetzten Befehle. |
| Bericht von verify | Abschnitte `## Ergebnis` (`verifiziert` oder `abgebrochen`), `## Belege` (je Zeile `- <dateiname>: <was die Datei zeigt>`), `## Geänderte Dateien`, `## Prüfungen`, `## Offene Fragen`, `## Hinweise`, `## Nicht geprüft`. |
| Prüfung nach verify | Erfolg verlangt Exit-Code 0, ein letztes `result` ohne Fehler, `## Ergebnis` `verifiziert`, mindestens eine Zeile unter `## Belege` und jede dort genannte Datei im Ordner der Belege. Sonst Fehler `verify: <grund>`, z. B. `verify: evidence file list.png missing in <ordner>`. `abgebrochen` mit Punkten unter `## Offene Fragen` blockiert den Lauf wie implement, ohne Punkte Fehler `verify aborted`. Danach `checkLazydevUntouched`. Der Runner schreibt den Text des letzten `result` nach `<prd>.evidence/report.md`. |
| Blocker in verify | `## Blocker` mit `Schritt: verify`. Dialog `unblock` und Schritt `resolve` setzen die Statuszeile danach auf `umgesetzt (<heute>)`, wie bei `Schritt: merge`, weil die Umsetzung schon steht. Danach läuft verify erneut. |
| review im PR-Modus | Der Vertrag von review bekommt im PR-Modus einen zusätzlichen Block: Claude schreibt `$EVIDENCE/pr.md`. Erste Zeile `# <Titel>`, als Conventional Commit auf Englisch, sofern `CLAUDE.md` nichts anderes vorgibt. Danach die Beschreibung nach `.github/pull_request_template.md` des Repos, falls vorhanden, sonst mit den Abschnitten `## Summary`, `## Evidence`, `## PRD`. Unter Evidence stehen die Belege aus `$EVIDENCE/report.md` mit Dateinamen und dem Pfad des Ordners. Fehlt `report.md`, steht dort `No runtime verification ran (.lazydev/verify.md is missing).` Erfolg im PR-Modus verlangt zusätzlich eine nicht leere `pr.md` mit einer ersten Zeile `# …`, sonst Fehler `review wrote no pull request description to <pfad>`. |
| Belege im PR | Nur als Text. lazydev lädt keine Bilder zu GitHub hoch und committet keine Belege ins Repo. |
| Benachrichtigung | Neues Ereignis `pr` beim Wechsel in Phase `pr`: Titel `⇡ Pull request: <area>/<name>`, Farbe `0x9b59b6`, zusätzliches Feld `Pull request` (`inline: false`) mit der URL. |
| `lazydev wait` | Neue Ereignisart `pr <id>/<prd>: <url>` beim Wechsel in Phase `pr`. Ein Lauf in Phase `pr` zählt nicht als „works or waits“, `wait` meldet also `idle`, wenn sonst nichts läuft. |
| `lazydev status` | Phase `pr`, Schritt `merge`. `--json` bekommt `pullRequest` (URL) und `evidence` (absoluter Pfad des Ordners, sobald er existiert). Unter der Tabelle die Zeile `<prd>: pull request <url>`. |
| plan | Läufe in Phase `pr` sind offene Läufe. Ein PRD, das auf einen solchen Lauf wartet, startet erst, wenn sein PR gemergt ist. |
| TUI | Phasensymbol `◑` für `pr`. In der Laufliste ersetzt die URL die letzte Logzeile. Der Kopf des Log-Tabs zeigt die Zeile `pull request <url>` und die Zeile `evidence <pfad>`, sobald es sie gibt. Der Log-Tab zeigt den Abschnitt `── verify ──`. Die Projektansicht der Detailansicht zeigt `merge ff` oder `merge pr` neben dem Basis-Branch. |
| Wizard | Seite Knowledge-Schritte: Taste `6` schaltet `verify` um (Reihenfolge create, implement, verify, review, merge, unblock, also `1`–`6`). Seite Prompts listet `verify.md`, ohne Datei mit `missing, step off`. `r` schreibt die Vorlage, `o` öffnet die Datei im Editor und legt sie vorher aus der Vorlage an, wenn sie fehlt. Den Schalter `merge` setzt der Wizard nicht. |
| Mod `lazydev-runs` | Phase `pr` in der Gruppe `Needs you` als `pull request open` mit URL. `verify` erscheint als Schritt mit Fortschritt wie die anderen. |
| Fast-Forward-Modus | Bleibt unverändert bis auf den Schritt `verify`. |

## Scope

**In Scope**

- Feld `merge` in `config.yaml`, Validierung, Anzeige in der Projektansicht.
- PR-Modus im Schritt merge: Vorbedingung `gh`, Fetch, Rebase auf `origin/<base>`, Check, Push, PR öffnen
  oder aktualisieren, Phase `pr` mit Abfrage, Aufräumen nach dem Merge, Fehler nach dem Schließen.
- `X` schließt einen offenen PR.
- Schritt `verify` mit Vertrag, Vorlage, Belegordner, Dev-Server-Start, Prüfung, Blocker und Resolve.
- PR-Beschreibung durch review im PR-Modus.
- Benachrichtigung `pr`, `wait`, `status`, TUI, Wizard und Mod für Phase `pr` und Schritt `verify`.
- `SPEC.md`, `README.md`, `README.de.md`, die Kopie `internal/skill/files/references/spec.md`
  (`go generate ./internal/skill`) und `SKILL.md` des Skills, wo es Schritte und Merge nennt.

**Out of Scope**

- Auto-Merge von PRs (`gh pr merge --auto`) und jedes Mergen eines PR durch lazydev.
- Andere Hoster als GitHub, andere Remotes als `origin`.
- Hochladen von Belegen (Bildern) zu GitHub, Belege im Repo.
- CI-Status des PR in lazydev anzeigen oder auswerten.
- Rebase eines offenen PR, wenn der Basis-Branch auf GitHub weiterzieht. Konflikte zeigt GitHub, der
  Mensch schließt den PR oder löst sie dort.
- Löschen des Remote-Branches nach dem Merge.
- Ein eingebauter Browser-Treiber oder eine Feature Map in lazydev: beides gehört dem Projekt, sein
  `verify.md` nennt Werkzeug und Befehle.

## Umsetzung

1. **Spec zuerst.** `SPEC.md`: Nicht-Ziele (Pull Requests raus, „merging pull requests“ rein), `.lazydev/`
   (Feld `merge`, Prompt `verify`), Workflow (Schritt verify, Merge im PR-Modus, Phase `pr`), Blocked runs
   (`Schritt: verify`), Discord (Ereignis `pr`), Hooks (`LAZYDEV_EVIDENCE`), UI (Phasensymbol, Laufliste,
   Log-Tab, Wizard), Data (`pullRequest`, `<prd>.evidence/`), CLI (`status`, `wait`). Danach
   `go generate ./internal/skill`. `README.md` und `README.de.md` mit gleichem Inhalt anpassen, dazu ein
   Beispiel `merge: pr` im YAML-Block von `.lazydev/`.
2. **Konfiguration.** `internal/repo/repo.go`: Feld `Merge string` mit yaml `merge` in `Config`, Methode
   `MergeMode()` (`ff` als Standard), Validierung in `Read`. `Steps` um `verify` nach `implement`
   erweitern, `Init` überspringt `verify`. Neue Vorlage `internal/repo/workflows/simple/verify.md`
   (Deutsch): App starten oder den laufenden Dev-Server auf `$PORT` nutzen, Erfolgskriterien des PRDs der
   Reihe nach in der App prüfen, je Kriterium einen Beleg nach `$EVIDENCE`. Tests in
   `internal/repo/repo_test.go`.
3. **State und Phasen.** `internal/state/state.go`: `StepVerify`, in `Steps` und `LogSteps` nach implement,
   Feld `PullRequest string` (`json:"pullRequest,omitempty"`), `EvidenceDir(id, prd)`. `Remove` löscht
   auch den Belegordner. `internal/project/phase.go`: `PhaseVerify`, `PhasePR`, `Active()` und die
   Symbole entsprechend. Tests der bestehenden Pakete erweitern.
4. **Prompts.** `internal/prompts/prompts.go`: `Vars` um `Evidence` und `Port`, Ersetzung von `$EVIDENCE`
   und `$PORT` überall, wo heute `$ARGUMENTS` ersetzt wird. Neuer Vertrag `contract/verify.md` nach der
   Entscheidung oben. `contract/review.md` bekommt den PR-Block; `Build` hängt ihn nur im PR-Modus an
   (eigene Datei `contract/review.pr.md`, analog zu `create.auto.md`). `contract/unblock.md`,
   `contract/unblock.auto.md` und `contract/resolve.md`: `Schritt: verify` setzt `umgesetzt (<heute>)`.
5. **Claude-Aufruf.** `internal/claude/run.go`: `Options.AddDirs`, je Eintrag `--add-dir <pfad>`.
   `internal/runner/claudestep.go`: `runClaude` füllt `Evidence`, `Port` und `AddDirs` für verify und
   review. `claudeStep` prüft nach verify den Bericht (`resultVerified = "verifiziert"`, `## Belege`,
   Dateien vorhanden), schreibt `report.md` und blockiert über `checkReport` mit „Offene Fragen“. Nach
   review im PR-Modus prüft es `pr.md`.
6. **Runner.** `internal/runner/runner.go`: `stepPhases` um verify. Ein neuer Fall `state.StepVerify` in
   `runStep`: ohne `.lazydev/verify.md` Log und Erfolg ohne Claude, sonst Belegordner anlegen, Dev-Server
   starten, falls `hooks.dev` gesetzt ist (`StartDev`), `claudeStep`, Dev-Server stoppen (`defer`).
   `countRunning` zählt Läufe in Phase `pr` nicht. `worktree.go`: `hookEnv` um `LAZYDEV_EVIDENCE`.
7. **Merge im PR-Modus.** `internal/runner/merge.go`: `merge` verzweigt nach `MergeMode()`. Neue Datei
   `internal/runner/pullrequest.go` mit `ghReady`, `pushBranch`, `openPullRequest` (inklusive Wiederverwenden
   eines offenen PR), `waitPullRequest` (Abfrage, Notiz bei Fehlern, `MERGED` und `CLOSED`) und
   `finishPullRequest` (Aufräumen, Hauptordner nachziehen). `rebaseAndCheck` und `mergeClaude` bekommen das
   Rebase-Ziel als Parameter. Der Merge-Lock endet vor `waitPullRequest`. Fortsetzen in Phase `pr` nach der
   Entscheidung „Neustart“. `gh` ruft lazydev über eine Variable `GH = []string{"gh"}` auf, die Tests
   ersetzen, wie `claude.Command`.
8. **Discard.** `internal/runner/discard.go`: offener PR → `gh pr close <url> --delete-branch` vor dem
   Teardown-Hook, Fehler in die Fehlerliste.
9. **Benachrichtigung.** `internal/notify/notify.go`: Ereignis `PR`, Titel, Farbe, Feld `Pull request`.
   `internal/runner/notify.go` übergibt die URL.
10. **Übersicht, status, wait.** `internal/runner/overview.go`: `Item` um `PullRequest` und `Evidence`,
    `stepInfos` mit verify, `Changes` erzeugt `pr`, `Busy` zählt `pr` nicht. `cmd/lazydev/cli.go`: Zeile
    `<prd>: pull request <url>` unter der Tabelle von `status`, Ausgabe des Ereignisses `pr` in `wait`.
11. **TUI.** `internal/tui/runview.go` (Laufliste, Projektansicht mit `merge ff|pr`),
    `internal/tui/logview.go` (Kopfzeilen, Abschnitt verify), `internal/tui/prds.go` (Symbol),
    `internal/tui/wizard.go` (Taste `6`, Prompt `verify.md` mit `missing, step off`, `o` legt aus der
    Vorlage an). Tests in den zugehörigen `_test.go`.
12. **Mod.** `internal/mod/files/hooks/register.tsx` und `types/index.d.ts`: Phase `pr` in `Needs you`,
    Feld `pullRequest`, Schritt verify. Tests unter `internal/mod/testdata/`.
13. **Skill.** `internal/skill/files/SKILL.md`: Schrittfolge mit verify und den Hinweis auf `merge: pr`.
14. **Tests des Runners.** Das Fake-`claude`-Skript in `internal/runner/helpers_test.go` erkennt
    `# lazydev verify:` und schreibt Belege nach dem Pfad aus dem Prompt. Ein Fake-`gh`-Skript bildet
    `auth status`, `pr view`, `pr create`, `pr edit`, `pr close` ab und liest den PR-Zustand aus einer Datei,
    die der Test setzt. Ein Bare-Repository als `origin`. Szenarien: verify übersprungen, verify ok,
    verify ohne Beleg-Datei (Fehler), verify blockiert und per resolve geklärt, PR-Modus bis `MERGED` mit
    nachgezogenem Hauptordner, Hauptordner mit Änderungen (Notiz), `CLOSED` und Fortsetzen, offener PR
    wird wiederverwendet, Runner in Phase `pr` belegt keinen Platz, `X` schließt den PR, `gh` fehlt.

## Technische Hinweise

- `SPEC.md` ist verbindlich und wird zuerst geändert. `internal/skill/files/references/spec.md` muss gleich
  `SPEC.md` sein, sonst schlägt `TestSpecCopy` fehl. `make test` ruft kein `go generate` auf.
- Alle Texte in Code, UI, Fehlern und Doku sind Englisch. Prompts, Verträge und PRD-Statuswerte bleiben
  Deutsch. `README.de.md` immer mit `README.md` ändern.
- Jeder Schritt muss wiederholbar sein: `s` setzt beim fehlgeschlagenen Schritt fort. Das gilt für Push
  (`--force-with-lease`), PR (offenen PR wiederverwenden) und den Belegordner (bestehende Dateien bleiben,
  verify überschreibt sie).
- Commits, die lazydev selbst macht, laufen ohne Claude und ohne Trailer.
- Runner-Tests laufen `runner.Run` im Vordergrund gegen temporäre Repos mit Fake-Skripten
  (`claude.Command`, `docker.Binary`). Kein Test startet einen abgelösten Runner. Isolation über
  `XDG_STATE_HOME`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME`, Git-Tests setzen `GIT_AUTHOR_*` und
  `GIT_COMMITTER_*`.
- `--add-dir` ist nötig, weil der Belegordner außerhalb des Worktrees liegt und `acceptEdits` nur im
  Arbeitsverzeichnis ohne Rückfrage schreibt.
- Der Merge-Lock (`locks/<id>.merge`) darf nicht über die Phase `pr` gehalten werden, sonst blockieren
  Pull und Push des Git-Cockpits und alle anderen Merges des Projekts bis zum Merge auf GitHub.
- `mainDirProblem` ignoriert unversionierte PRDs und `.lazydev/`. Dieselbe Regel gilt für das Nachziehen
  des Hauptordners nach dem Merge des PR.
- Die Abfrage des PR nutzt den Kontext des Runners (`sleep(ctx, …)`), damit `x` sie sofort beendet.

## Erfolgskriterien

- `make test` ist grün, inklusive `TestSpecCopy`. `make test-mod` ist grün.
- Ein Projekt ohne `merge` und ohne `.lazydev/verify.md` verhält sich wie vorher: Der Log zeigt
  `No .lazydev/verify.md, verify skipped`, sonst ändert sich am Ablauf und am Fast-Forward nichts.
- Mit `.lazydev/verify.md` läuft verify zwischen implement und review. Nach Erfolg liegen die Belege und
  `report.md` in `runs/<id>/<prd>.evidence/`, `lazydev status --json` nennt den Ordner unter `evidence`.
- Ein verify-Bericht, der eine nicht vorhandene Beleg-Datei nennt, endet in Phase error mit
  `verify: evidence file <name> missing in <ordner>`.
- Mit `merge: pr` endet ein Lauf nach review mit einem gepushten `feat/<prd>` und einem offenen PR, dessen
  Titel und Beschreibung aus `pr.md` stammen. Der Lauf steht in Phase `pr`, belegt keinen Runner-Platz,
  `lazydev wait` meldet `pr <id>/<prd>: <url>`.
- Wird der PR gemergt, steht der Lauf innerhalb einer Abfrage auf merged, Worktree und lokaler Branch sind
  weg, der Hauptordner steht auf `origin/<base>`, wenn er sauber war, sonst trägt der Lauf die Notiz
  `main folder not updated: …`.
- Wird der PR geschlossen, steht der Lauf auf error mit `pull request <url> closed without merge`, und `s`
  öffnet nach Rebase und Check einen neuen PR.
- `X` auf einem Lauf mit offenem PR schließt den PR.
- Ein falscher Wert in `merge` zeigt `config.yaml: merge must be ff or pr, not "<wert>"` in der
  Detailansicht, `lazydev start` verweigert den Start.

## Entscheidungen

1. Wie wird der PR-Modus eingeschaltet? A: Feld in `config.yaml` · B: Schalter in `settings.json` pro
   Projekt · C: Flag an `lazydev start` · D: automatisch, wenn `origin` auf GitHub zeigt. **A**, weil der
   Weg nach `main` eine Eigenschaft des Repos ist, die für alle im Team gleich sein muss, wie `baseBranch`.
2. Wer merged den PR? A: niemand, lazydev wartet · B: lazydev mit `gh pr merge --auto` · C: lazydev sofort
   · D: konfigurierbar. **A**, weil das Gespräch Auto-Merge ausdrücklich erst für später vorsieht, wenn die
   innere Schleife verlässlich gute PRs liefert.
3. Wie wartet der Lauf auf den PR? A: Runner lebt weiter und fragt ab, ohne Platz zu belegen · B: Lauf
   endet, `s` prüft später · C: TUI prüft · D: Runner belegt seinen Platz weiter. **A**, weil nur Runner
   Zustand schreiben (die TUI liest nur) und ein belegter Platz Läufe tagelang blockieren würde.
4. Wo liegen die Belege? A: im State-Ordner des Laufs · B: im Repo committet · C: als PR-Anhang
   hochgeladen · D: in `.artifacts/` im Worktree. **A**, weil Belege das Repo nicht aufblähen sollen,
   der Worktree mit dem Merge verschwindet und ein Upload nach GitHub keine stabile CLI hat.
5. Wann läuft verify? A: zwischen implement und review · B: nach review · C: innerhalb von implement ·
   D: nach dem Check-Hook. **A**, weil verify Fehler beheben darf und review diese Änderungen dann
   mitprüft und committet.
6. Wie wird verify eingeschaltet? A: durch `.lazydev/verify.md` · B: Feld in `config.yaml` · C: immer an ·
   D: per Wizard-Schalter. **A**, weil Verifikation projektspezifisch ist (Werkzeug, Befehle, Feature Map)
   und ohne eigenen Prompt nichts Sinnvolles prüfen kann.
7. Wer schreibt die PR-Beschreibung? A: review im PR-Modus nach der PR-Vorlage des Repos · B: lazydev
   fest aus PRD und Bericht · C: ein eigener Schritt · D: verify. **A**, weil review Diff, PRD und Bericht
   schon kennt und eine Vorlage mit Pflichtabschnitten (z. B. „Evidence“) nur ein Agent sinnvoll füllt.
8. Gehört `merge` in den Trust-Hash? A: nein · B: ja. **A**, weil Trust Code-Ausführung und Rechte schützt
   und der Push nur den eigenen Branch zum eigenen `origin` bringt.
