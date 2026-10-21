# lazydev

<p align="center">
  <a href="README.md">English</a> · <b>Deutsch</b>
</p>

Terminal-UI für den PRD-Workflow mit Claude Code. Links die Projekte, in der Mitte Terminal-Tabs, rechts
Dateien und PRDs. Du schreibst ein PRD mit Claude in einem Tab, startest es mit einer Taste und verfolgst
das Live-Log. Jedes PRD läuft in einem eigenen Git-Worktree mit eigenem Port, lazydev bringt es per
Fast-Forward in den Ziel-Branch des Projekts. [SPEC.md](SPEC.md) beschreibt das Verhalten vollständig
(englisch).

## Voraussetzungen

- Linux (`/proc`, `setsid`, `flock`), Go 1.27+ zum Bauen.
- `git` und [Claude Code](https://claude.com/claude-code) (`claude`) im `PATH`.
- Optional: `docker` für den Docker-Tab, `$VISUAL`/`$EDITOR` zum Öffnen von Dateien.

## Installation

```sh
make install          # ~/.local/bin/lazydev, PREFIX überschreibbar
go run ./cmd/lazydev  # ohne Installation
```

Runner behalten den Code, mit dem sie gestartet sind. Nach `make install` lazydev neu starten.

## Erste Schritte

1. `lazydev`, dann `a` im linken Pane und den Pfad eines Projekts (`Tab` vervollständigt). lazydev legt
   `.lazydev/` mit den Prompts des simple workflow an. Committe den Ordner.
2. Rechtes Pane, Tab `PRDs`: `n` und ein Satz zum Feature. Claude Code öffnet sich als Tab in der Mitte
   und schreibt mit dir `prd/<bereich>/<name>.md`.
3. PRD wählen, `s`. Setup, implement, review und merge laufen im Hintergrund.
4. Das Details-Pane unter den Terminals zeigt Schritte und Live-Log. `z` vergrößert es.

## Aufbau

| Pane | Inhalt |
|---|---|
| Projects (links) | Projekte mit Branch und Commits vor/hinter dem Upstream |
| Mitte | Terminal-Tabs des Projekts. In einem Projekt ohne Tab startet Claude Code |
| Unter der Mitte | `Details`: Einrichtung von `.lazydev/` oder das gewählte PRD mit Lauf und Live-Log. `Docker`: Compose-Services und Container |
| Explorer (rechts) | `PRDs`: PRDs mit Phase, erledigte eingeklappt. `Files`: Dateibaum |

## Tasten

`alt` geht überall, auch in einem Terminal. Außerhalb eines Terminals ist `alt` optional.

| Taste | Aktion |
|---|---|
| `alt+←/→`, `tab` | Pane |
| `alt+shift+←/→` | Tab im Pane |
| `alt+j/k`, `alt+1…9` | Projekt, Tab in der Mitte |
| `alt+t` / `alt+w` | neuer Shell-Tab / Tab schließen |
| `alt+m` / `alt+q` | Menü (Farbschema) / beenden |
| `j/k`, `l`/`enter`, `h` | bewegen, öffnen, zurück (Listen, Baum) |
| `a` / `d` / `i` | Projekt hinzufügen / entfernen / Wizard für `.lazydev/` (Projects) |
| `n` | neues PRD mit Claude Code (PRDs) |
| `s` / `x` / `X` | starten oder fortsetzen / stoppen / Lauf verwerfen (PRDs) |
| `D` | Dev-Server im Worktree des PRDs an/aus (PRDs) |
| `c` | Status: planned, merged, discarded. Committet getrackte PRDs (PRDs) |
| `z` | Details-Pane vergrößern |
| `l` | Logs des Containers als Tab (Docker) |

## `.lazydev/`

Liegt versioniert im Projekt, für alle gleich. Hinzufügen und Wizard legen fehlende Dateien an.

| Datei | Inhalt |
|---|---|
| `config.yaml` | `baseBranch`, `knowledge`, `allowedTools`, `hooks` |
| `create.md`, `implement.md`, `review.md`, `merge.md` | Prompts der Schritte. Der Workflow liest sie nur von hier |

```yaml
baseBranch: develop          # Standard main
knowledge:                   # Dateien, die Claude zuerst liest: all, create, implement, review, merge
  all: [CLAUDE.md]
  implement: [docs/architecture.md]
allowedTools:                # zusätzlich zu den eingebauten Tools jedes Schritts
  - Bash(pnpm test:*)
hooks:                       # Skripte, relativ zum Hauptordner
  setup: scripts/lazydev-setup.sh
  check: scripts/lazydev-check.sh
  teardown: scripts/lazydev-teardown.sh
  dev: scripts/lazydev-dev.sh
```

- Jeder Prompt = Kopfzeile + Knowledge-Block + `.lazydev/<schritt>.md` + eingebauter Vertrag. Den Vertrag
  kann ein Projekt nicht ändern.
- Hooks bekommen `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`, `LAZYDEV_SLUG`.
  Der Dev-Server bekommt zusätzlich `PORT`.
- Hooks und `allowedTools` brauchen Vertrauen. `s` fragt, oder `lazydev trust <projekt>`. Ändern sie sich,
  fragt lazydev erneut.

## Läufe

- Schritte: setup (Worktree auf `feat/<prd>`, ungetracktes PRD verschoben und committet, Setup-Hook) →
  implement → review → merge (Rebase, Claude bei Konflikten, Check-Hook, Kosten in die Statuszeile,
  Fast-Forward, Teardown-Hook, Aufräumen).
- Höchstens 3 Runner gleichzeitig, der Rest wartet. Ports ab 3500.
- Nach einem Fehler oder `x` setzt `s` beim gescheiterten Schritt fort. `X` verwirft Worktree, Branch und
  dessen Commits, das PRD kommt mit Status `geplant` zurück.
- Der Merge wartet, solange der Hauptordner nicht auf dem Ziel-Branch steht oder Änderungen außerhalb von
  `prd/` und `.lazydev/` hat.
- Die Statuswerte im PRD bleiben deutsch (`geplant`, `umgesetzt (Datum)`, `verworfen (Datum)`), wie die
  Prompts.

## CLI

```sh
lazydev start <projekt> <prd>            # im Hintergrund starten oder fortsetzen
lazydev stop <projekt> <prd>
lazydev discard [--yes] <projekt> <prd>
lazydev trust <projekt>
```

`<projekt>` ist die ID (Ordnername) oder der Pfad eines Projekts der Liste, `<prd>` der Dateiname ohne
`.md`.

## Dateien

| Pfad | Inhalt |
|---|---|
| `~/.config/lazydev/` | `projects.json`, `settings.json` (Farbschema) |
| `~/.local/state/lazydev/` | `runs/<id>/<prd>.json` und Logs der Schritte, `trust/`, `locks/`, `usage.json`, `bin/` |
| `~/.local/share/lazydev/worktrees/<id>/<prd>/` | Worktrees |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` und `XDG_DATA_HOME` gelten.

## Entwicklung

```sh
make test   # go vet ./... und go test ./...
make build  # bin/lazydev
```
