# lazydev

<p align="center">
  <a href="README.md">English</a> · <b>Deutsch</b>
</p>

Terminal-UI für den PRD-Workflow mit Claude Code. Links die Projekte, in der Mitte Terminal-Tabs, rechts
Dateien und PRDs. Du schreibst ein PRD mit Claude in einem Tab, startest es mit einer Taste und verfolgst
sein Log. Jedes PRD läuft in einem eigenen Git-Worktree mit eigenem Port, lazydev bringt es per
Fast-Forward in den Ziel-Branch des Projekts. [SPEC.md](SPEC.md) beschreibt das Verhalten vollständig
(englisch).

## Voraussetzungen

- Linux (`/proc`, `setsid`, `flock`), Go 1.27+ zum Bauen.
- `git` und [Claude Code](https://claude.com/claude-code) (`claude`) im `PATH`.
- Optional: `docker` für den Docker-Tab, `$VISUAL`/`$EDITOR` zum Öffnen von Dateien.

## Installation

Das Repository ist privat, deshalb lädt `install.sh` ein Release über `gh`. Melde dich einmal mit
`gh auth login` an. Das Skript braucht `gh`, `tar`, `sha256sum` und `uname`.

```sh
gh api repos/domsen123/lazydev/contents/install.sh -H 'Accept: application/vnd.github.raw' | sh
sh install.sh v0.2.0  # ein bestimmtes Release statt des neuesten
```

Das Skript prüft die Prüfsumme und installiert `lazydev` nach `$PREFIX/bin`, `PREFIX` ist standardmäßig
`~/.local`. Zum Aktualisieren rufst du es erneut auf. `lazydev version` gibt die installierte Version aus.
Beim Start prüft die TUI, ob ein neueres Release existiert, und zeigt einen Hinweis im Footer und im
Menü-Tab `General`.

Aus dem Quellcode, mit Go 1.27+:

```sh
make install          # ~/.local/bin/lazydev, PREFIX überschreibbar
go run ./cmd/lazydev  # ohne Installation
```

Runner behalten den Code, mit dem sie gestartet sind. Nach einem Update lazydev neu starten.

## Release

Pushe einen Tag `vX.Y.Z`. Eine GitHub Action führt `make test` aus, baut `linux/amd64` und `linux/arm64`
mit GoReleaser und legt das Release mit den Archiven und `checksums.txt` an. Die Release-Notes kommen aus
den Commits seit dem vorigen Tag, gruppiert in Features, Fixes, Performance und Other. Commits vom Typ
`docs`, `test`, `chore`, `ci`, `style` und `build` fehlen darin.

```sh
git tag v0.2.0
git push origin v0.2.0
```

## Erste Schritte

1. `lazydev`, dann `a` im linken Pane und den Pfad eines Projekts (`Tab` vervollständigt). lazydev legt
   `.lazydev/` mit den Prompts des simple workflow an. Committe den Ordner.
2. Rechtes Pane, Tab `PRDs`: `n` und ein Satz zum Feature. Claude Code öffnet sich als Tab in der Mitte
   und schreibt mit dir `prd/<bereich>/<name>.md`.
3. PRD wählen, `s`. Setup, implement, review und merge laufen im Hintergrund.
4. Das Details-Pane unter den Terminals listet die Läufe des Projekts mit Phase, Schritt, Kosten und letzter
   Logzeile. `l` auf einem Lauf öffnet sein vollständiges Log als Tab in der Mitte, `z` klappt dort den Kopf ein.

## Aufbau

| Pane | Inhalt |
|---|---|
| Projects (links) | Projekte mit Branch und Commits vor/hinter dem Upstream |
| Mitte | Terminal-Tabs und Log-Tabs des Projekts. In einem Projekt ohne Tab startet Claude Code |
| Unter der Mitte | `Details`: Läufe des Projekts, ohne Läufe die Einrichtung von `.lazydev/`. `Docker`: Compose-Services und Container |
| Explorer (rechts) | `PRDs`: PRDs mit Phase, erledigte eingeklappt. `Files`: Dateibaum |

## Tasten

`alt` geht überall, auch in einem Terminal. Außerhalb eines Terminals ist `alt` optional.

| Taste | Aktion |
|---|---|
| `alt+←/→`, `tab` | Pane |
| `alt+shift+←/→` | Tab im Pane |
| `alt+j/k`, `alt+1…9` | Projekt, Tab in der Mitte |
| `alt+t` / `alt+w` | neuer Shell-Tab / Tab schließen |
| `alt+g` | Git-Cockpit: Fetch, Pull, Push, Commit |
| `alt+m` / `alt+q` | Menü (Farbschema, Discord, Git) / beenden |
| `j/k`, `l`/`enter`, `h` | bewegen, öffnen, zurück (Listen, Baum) |
| `a` / `d` / `i` | Projekt hinzufügen / entfernen / Wizard für `.lazydev/` (Projects) |
| `n` | neues PRD mit Claude Code (PRDs) |
| `s` / `x` / `X` | starten oder fortsetzen / stoppen / Lauf verwerfen (PRDs, Runs) |
| `D` | Dev-Server im Worktree des PRDs an/aus (PRDs, Runs) |
| `L` / `l` | Log des Laufs als Tab (PRDs / Runs) |
| `c` | Status: planned, merged, discarded. Committet getrackte PRDs (PRDs) |
| `z` | Details-Pane vergrößern, Kopf im Log-Tab |
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

## Git-Cockpit

`alt+g` zeigt Branch, Upstream, Änderungen und die letzten 5 Commits im Hauptordner des ausgewählten
Projekts. Jede Aktion läuft als Tab in der Mitte, Abfragen von Passphrase und Zugangsdaten funktionieren
deshalb wie in einer Shell.

| Taste | Aktion | Git |
|---|---|---|
| `f` | Fetch | `git fetch` |
| `p` | Pull | `git pull --ff-only` |
| `P` | Push | `git push`, ohne Upstream `git push -u origin <branch>` |
| `c` | Commit | `git add -A` und `git commit -m <nachricht>` |

- Bei Erfolg schließt sich der Tab, und die Fußzeile zeigt `git <action>: done`. Bei einem Fehler bleibt der
  Tab offen, bis du `enter` drückst.
- Pull und Push warten nicht auf einen laufenden Merge eines PRDs. Sie enden mit einem Fehler, versuche es
  nach dem Merge erneut.
- Commit nimmt alle Änderungen auf, auch ungetrackte Dateien, `.lazydev/` und `prd/`.

## Discord

Runner schreiben an einen Discord-Webhook, wenn ein PRD gemergt ist, ein Schritt scheitert, `x` einen
Runner abbricht oder der Merge auf den Hauptordner zu warten beginnt. Das klappt auch ohne laufende TUI.

1. Lege in Discord einen Webhook für einen Kanal an und kopiere seine URL.
2. `alt+m`, Tab `Discord`, `enter`. Wähle `Global URL`, drücke `enter`, füge die URL ein und drücke `enter`.
3. Schalte `Notifications` ein. `Send test` schickt sofort eine Testnachricht.

Ein Projekt lässt sich stummschalten oder bekommt eine eigene URL. Die URL landet nur in
`~/.config/lazydev/settings.json`, nie in einem Repository. Das Menü zeigt sie gekürzt auf
`…/<letzte 4 Zeichen>`.

```json
{
  "discord": {
    "enabled": true,
    "url": "https://discord.com/api/webhooks/…",
    "projects": {
      "/home/me/code/shop": {"enabled": false},
      "/home/me/code/blog": {"url": "https://discord.com/api/webhooks/…"}
    }
  }
}
```

Jede Nachricht ist ein Embed mit Projekt, Branch, Kosten, Dauer und Modell, ohne Fehlertext. Scheitert der
Versand, wiederholt der Runner ihn 3-mal im Abstand von 5 s. Danach bekommt das Step-Log
`Discord notification failed: <fehler>`. Der Lauf geht unverändert weiter.

## CLI

```sh
lazydev start <projekt> <prd>            # im Hintergrund starten oder fortsetzen
lazydev stop <projekt> <prd>
lazydev discard [--yes] <projekt> <prd>
lazydev trust <projekt>
lazydev version                          # auch --version, -v
```

`<projekt>` ist die ID (Ordnername) oder der Pfad eines Projekts der Liste, `<prd>` der Dateiname ohne
`.md`.

## Dateien

| Pfad | Inhalt |
|---|---|
| `~/.config/lazydev/` | `projects.json`, `settings.json` (`theme`, `allowedTools`, `permissionMode`, `discord`), Rechte `0600` |
| `~/.local/state/lazydev/` | `runs/<id>/<prd>.json` und Logs der Schritte, `trust/`, `locks/`, `usage.json`, `bin/` |
| `~/.local/share/lazydev/worktrees/<id>/<prd>/` | Worktrees |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` und `XDG_DATA_HOME` gelten.

`allowedTools` in `settings.json` gelten für jeden Claude-Schritt aller Projekte, nach den eingebauten Tools
und vor `allowedTools` aus `config.yaml`. Sie brauchen kein Vertrauen. `permissionMode` ist `acceptEdits`
(Vorgabe) oder `auto`. Jeder andere Wert lässt den Claude-Schritt vor dem Start von Claude scheitern.

```json
{
  "permissionMode": "auto",
  "allowedTools": ["Bash(head:*)", "Bash(tail:*)", "Bash(sed -n:*)", "Bash(grep:*)"]
}
```

Im Modus `auto` entscheidet ein Klassifikator von Claude Code über jeden Tool-Aufruf statt der Präfix-Liste.
Das Ergebnis ist nicht deterministisch.

## Entwicklung

```sh
make test   # go vet ./... und go test ./...
make build  # bin/lazydev
```
