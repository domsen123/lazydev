# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Überblick

lazydev ist eine Go-TUI (Bubble Tea) für den PRD-Workflow mit Claude Code. Jedes PRD läuft in einem eigenen
Git-Worktree mit eigener Datenbank und eigenem Port und wird per Fast-Forward in den
Ziel-Branch des Projekts gemergt (`baseBranch` aus `.lazydev/config.yaml`, Standard `main`).
`SPEC.md` ist die verbindliche Beschreibung des Verhaltens. Um Verhalten zu ändern, passe `SPEC.md`,
`README.md` und `README.de.md` mit an.

## Befehle

```sh
make test                                   # go vet ./... und go test ./...
make build                                  # Binärdatei unter bin/lazydev
make install                                # nach ~/.local/bin/lazydev, PREFIX überschreibbar
go run ./cmd/lazydev                        # TUI ohne Installation
go test ./internal/runner -run TestRunChecks  # einzelner Test
```

Nur Linux: lazydev nutzt `/proc`, `setsid` und `flock`. Go 1.27 oder neuer.

## Architektur

### Prozessmodell

- Die TUI (`internal/tui`) steuert nichts direkt. Sie liest nur Dateien: Lauf-Zustand, Logs und
  `usage.json`. fsnotify meldet Änderungen unter `prd/` und im Zustandsverzeichnis. Zusätzlich liest die
  TUI alle 2 Sekunden neu ein, weil ein beendeter Runner kein Dateiereignis auslöst.
- Jedes PRD läuft in einem losgelösten Runner-Prozess. `runner.Start` (`internal/runner/start.go`) ruft das
  eigene Binary über `os.Executable()` als `lazydev run <projekt-id> <prd-name>` mit `setsid` auf.
  Eine laufende TUI startet Runner also mit der installierten Version, nicht mit dem Code im Repo.
- `maxParallel` gilt unter der Sperre `locks/start.lock`. Ein endender Runner startet das nächste
  wartende PRD, nicht die TUI.
- Ein Runner gilt als laufend, wenn seine PID lebt und zu `lazydev run` gehört (`internal/state/process.go`).
  Sonst gilt das PRD als „abgebrochen“.
- Chats zum PRD (Schritt `create`) laufen ebenfalls losgelöst (`internal/chatrun`). Jede Runde startet als
  `lazydev chat <projekt-id> <chat-id>`. Zustand und Log liegen unter `chats/<projekt-id>/` im
  Zustandsverzeichnis, die Sperre heißt `chat-<projekt>-<id>`. Eine Rückfrage oder ein Fehler erscheint
  in der TUI als Benachrichtigung. Tests ersetzen den Prozessstart über `chatrun.SetSpawn`.
- GitHub-Issues lädt die TUI über das `gh`-Binary (`internal/github`), wenn `origin` auf `github.com`
  zeigt. Der Abruf läuft beim Laden, alle 30 Minuten und mit `r`. Den letzten Stand speichert die TUI
  unter `issues/<projekt-id>.json` im Zustandsverzeichnis. Tests ersetzen `gh` über `github.Binary`.

### Ablauf eines Runners (`internal/runner`)

Setup (Worktree, PRD verschieben, Hooks) → implement → review → Merge-Queue → Merge → Aufräumen. Jeder
Schritt landet als Eintrag in `steps` des Lauf-Zustands. `start` und `run` setzen beim gescheiterten
Schritt fort, deshalb muss jeder Schritt wiederholbar sein.

- `start` und `run` prüfen vorher, ob Hooks und Freigaben aus `.lazydev/config.yaml` bestätigt sind
  (`runner.CheckTrust`, Hash unter `trust/<projekt-id>.json`, `internal/state/trust.go`).
- Claude läuft als `claude -p … --output-format stream-json --verbose` mit `stdin` auf `/dev/null` und
  `GIT_EDITOR=true` (`internal/claude`). Alle Freigaben kommen über `--allowedTools`, weil neue Worktrees
  keinen bestätigten Trust-Dialog haben.
- Ein Schritt gilt nur dann als erfolgreich, wenn Exit-Code 0 ist, das **letzte** `result`-Ereignis keinen
  Fehler meldet und die Nachprüfung passt. Nach implement prüft der Runner `## Ergebnis` und die
  Statuszeile des PRDs, nach review, ob der Worktree sauber ist. Nach implement, review und dem
  Claude-Schritt merge prüft `checkLazydevUntouched`, dass Claude nichts unter `.lazydev/` geändert hat.
- Der Runner ergänzt jede stream-json-Zeile um `ts`. Eigene Log-Zeilen schreibt `internal/runlog`.
- Merge (`merge.go`): eine `flock`-Sperre pro Projekt. Ablauf: `git rebase <ziel-branch>`, bei Konflikt
  `rebase --abort` und der Claude-Schritt merge, danach Prüfbefehle, Hauptordner prüfen und
  `git merge --ff-only`. Änderungen unter `.lazydev/` im Hauptordner blockieren den Merge nicht.

### Projekt-Ordner, Prompts, Presets, Hooks

- `internal/config`: zentrale `config.yaml` unter `~/.config/lazydev/` oder `LAZYDEV_CONFIG` mit
  `maxParallel`, `portBase`, `claude.binary` und pro Projekt `path` und `autoMerge`. Die TUI schreibt die
  Datei selbst. `repo.go` liest und schreibt `.lazydev/config.yaml` im Hauptordner (`Repo`, `LoadRepo`,
  `SetRepoSettings`, `SetRepoBaseBranch` über `yaml.Node`). `Effective` liefert die wirksamen Einstellungen: mit
  `.lazydev/config.yaml` nur deren Werte, sonst die Übergangsfelder `baseBranch`, `preset`, `allowedTools`
  und `hooks` aus der zentralen Datei. Wer Ziel-Branch, Preset, Freigaben oder Hooks braucht, nutzt
  `Effective` bzw. `runner.Target.Repo`. `presets.go` erkennt `nuxt-drizzle`.
- `internal/prompts`: baut jeden Prompt aus Kopfzeile `# lazydev <schritt>: <argument>`, Knowledge-Block,
  `.lazydev/<schritt>.md` oder Standard-Prompt (`default/`) und Vertrag (`contract/`, nicht
  überschreibbar). Schritte: `create`, `implement`, `review`, `merge`. `Init` legt `.lazydev/` an, das
  nutzt der Wizard in `internal/tui/initwizard.go`.
- `internal/hooks`: Setup, Teardown, Check und Dev pro Preset. `hooks.For` bekommt die wirksame `Repo`,
  Skripte in `hooks.*` ersetzen einzelne Preset-Hooks. Hooks bekommen `LAZYDEV_PROJECT`,
  `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT` und `LAZYDEV_SLUG`.
- Standard-Prompts, Verträge und Agenten sind per `go:embed` eingebaut, nichts kommt aus `~/.claude`.
  Agenten liegen unter `internal/claude/agents/`. `claude.Run` schreibt sie nach `agents.json` im
  Zustandsverzeichnis, jeder Aufruf bekommt `--agents`. Die Kopfzeile eines Prompts erscheint im Log, z. B.
  `# lazydev review: feat/x → main`.
- `internal/project`: PRDs unter `prd/<bereich>/<name>.md`, Statuszeile `> Status: …` und Phasen. Ein PRD,
  das ein Lauf in seinen Worktree verschoben hat, liest lazydev aus dem Worktree.
- `internal/worktree`: Port ab `portBase` und Slug pro Worktree.
- Git läuft immer über das `git`-Binary (`internal/git`), nicht über eine Go-Bibliothek.

### Pfade

lazydev beachtet `XDG_CONFIG_HOME`, `XDG_STATE_HOME` und `XDG_DATA_HOME`. Zustand und Logs liegen unter
`$XDG_STATE_HOME/lazydev/runs/<projekt-id>/`, Worktrees unter
`$XDG_DATA_HOME/lazydev/worktrees/<projekt-id>/<prd-name>/`.

## Tests

- Tests isolieren sich über `t.Setenv("XDG_STATE_HOME", t.TempDir())` und `XDG_DATA_HOME`. Git-Tests
  setzen `GIT_AUTHOR_*` und `GIT_COMMITTER_*`.
- Runner- und Merge-Tests arbeiten gegen temporäre Git-Repos mit einem Fake-`claude`
  (`fakeClaude` in `internal/runner/runner_test.go`). Das Shell-Skript erkennt den Schritt an der Kopfzeile
  `# lazydev <schritt>:` in der ersten Zeile von `$2` und gibt vorbereitete stream-json-Zeilen aus.
  `FAKE_CLAUDE_MODE` (`ok`, `exit`, `noresult`, `abort`, `status`, `dirty`, `lazydev`) und
  `FAKE_MERGE_MODE` steuern sein Verhalten. Um neue Runner-Szenarien zu testen, erweitere dieses Skript.
- Tests mit `.lazydev/config.yaml`, die Hooks oder Freigaben setzen, bestätigen den Hash mit
  `state.Trust`, sonst startet kein Runner.

## Konventionen

- Alle Texte sind deutsch: UI-Strings, Fehlermeldungen, Kommentare und Doku. Ausnahme: `README.md` ist
  englisch, `README.de.md` ist die deutsche Fassung mit gleichem Inhalt. Ändere immer beide.
- Commits folgen Conventional Commits und sind englisch, z. B. `feat(tui): show keys for focus and state`.
- Commits, die lazydev selbst erstellt, schreibt es mit `git commit` ohne Claude und ohne Trailer.
