# lazydev

<p align="center">
  <a href="README.md">English</a> · <b>Deutsch</b>
</p>

PRD-Workflow für Claude Code. Du schreibst ein PRD mit Claude, lazydev setzt es um, reviewt und merged es
ohne dich: jedes PRD in einem eigenen Git-Worktree mit eigenem Port, per Fast-Forward in den Ziel-Branch des
Projekts. Steuern lässt es sich aus jeder Claude-Code-Session, aus der Shell oder aus der optionalen
Terminal-UI. [SPEC.md](SPEC.md) beschreibt das Verhalten vollständig (englisch).

## So funktioniert es

```
prd/<bereich>/<name>.md  (geplant)
  │  lazydev start
  ▼
plan ─► setup ─► implement ─► review ─► merge  ─►  Ziel-Branch  (umgesetzt)
               │                         │
               └──── blockiert: offene Fragen ────► du oder Claude antworten ─► weiter
```

| Schritt | Macht |
|---|---|
| plan | Nur bei anderen offenen Läufen im Projekt: Claude liest deren PRDs und den Code und entscheidet, ob das PRD parallel läuft oder wartet, bis einige gemergt sind. Nur lesend |
| setup | Worktree auf `feat/<prd>`, ungetracktes PRD verschoben und committet, Datenbanken, Setup-Hook |
| implement | Claude setzt das PRD um. Erfolg: `## Ergebnis` `umgesetzt` und Status `umgesetzt (Datum)` |
| review | Claude reviewt und committet. Der Worktree muss danach sauber sein |
| merge | Rebase auf den Ziel-Branch (Claude bei Konflikten), Check-Hook (Claude bei Rot), Kosten in die Statuszeile, `git merge --ff-only`, Teardown-Hook, Aufräumen |

- Kein Mensch zwischen Start und Merge. Nichts erreicht den Ziel-Branch ohne grünen Check-Hook.
- Höchstens 3 Läufe gleichzeitig, der Rest wartet in einer Queue. Ports ab 3500.
- Jeder Schritt ist wiederholbar: Nach einem Fehler oder Stopp setzt `lazydev start` beim gescheiterten
  Schritt fort.
- Claude darf `.lazydev/` nicht anfassen. Prompts, PRD-Statuswerte und Claudes Berichte bleiben deutsch.

## Voraussetzungen

- Linux (`/proc`, `setsid`, `flock`), Go 1.27+ zum Bauen.
- `git` und [Claude Code](https://claude.com/claude-code) (`claude`) im `PATH`.
- Optional: `docker` für Datenbanken pro Worktree und den Docker-Tab, `$VISUAL`/`$EDITOR` zum Öffnen von
  Dateien.

## Installation

```sh
curl -fsSL https://raw.githubusercontent.com/domsen123/lazydev/main/install.sh | sh
sh install.sh v0.2.0  # ein bestimmtes Release statt des neuesten
lazydev update        # später: neuestes Release über das Binary installieren
```

`install.sh` braucht `curl`, `tar`, `sha256sum` und `uname`, prüft die Prüfsumme und installiert nach
`$PREFIX/bin` (`PREFIX` standardmäßig `~/.local`). Aus dem Quellcode: `make install` oder
`go run ./cmd/lazydev`. Laufende Runner behalten den Code, mit dem sie gestartet sind.

## Erste Schritte

```sh
cd ~/code/shop
lazydev add            # nimmt das Projekt auf, legt .lazydev/ an, installiert den Claude-Code-Skill
lazydev mod install    # optional: Pane mit den Läufen neben dem Claude-Code-Chat
git add .lazydev && git commit -m "chore: add lazydev"
```

Dann in einer Claude-Code-Session im Projekt:

1. Feature besprechen, dann „mach daraus ein lazydev PRD“ sagen. Claude stellt nummerierte Fragen mit den
   Optionen A bis D und schreibt `prd/<bereich>/<name>.md` mit Status `geplant`.
2. „Starte es“ sagen. Claude führt `lazydev start <prd>` aus und überwacht den Lauf mit `lazydev wait`.
3. Blockiert der Lauf, stellt Claude dir die offenen Fragen, schreibt die Antworten ins PRD und setzt fort.
4. Nach dem Merge steht im PRD auf dem Ziel-Branch `umgesetzt (Datum) · Kosten: … · Modell: …`.

Dasselbe aus der Shell:

```sh
lazydev start login-form   # läuft im Hintergrund, keine Session nötig
lazydev status             # Phase, Schritt, Port, Kosten, Blocker-Fragen
lazydev log login-form -f  # Schritt-Logs folgen
lazydev wait               # blockiert, bis ein Lauf blockiert, scheitert, abbricht oder merged
```

## PRDs

`prd/<bereich>/<name>.md`, Statuszeile in den ersten 20 Zeilen:

```markdown
# PRD: Login-Formular

> Status: geplant · Scope: …
```

| Status | Bedeutung |
|---|---|
| `geplant` | geplant, kann starten |
| `blockiert (Datum)` | Lauf wartet auf Antworten, siehe `## Blocker` |
| `umgesetzt (Datum) · Kosten: x.xx USD · Modell: …` | gemergt |
| `verworfen (Datum)` | verworfen |

Abschnitte, die lazydev und Claude anlegen: `## Blocker` (offene Fragen eines blockierten Laufs),
`## Entscheidungen` (Fragen, die Claude selbst beantwortet hat, mit Option und Begründung), `## Hinweise` im
Bericht von implement (Bemerkungen, die nicht blockieren). `lazydev set-status <prd> planned|merged|discarded`
ändert den Status von Hand und committet ein getracktes PRD.

## Läufe

### Reihenfolge und Parallelität

Bei anderen offenen Läufen im Projekt entscheidet plan, ob das PRD neben ihnen starten darf. Ein
zurückgehaltenes PRD wartet ohne einen Slot zu belegen und startet, sobald die PRDs, auf die es wartet,
gemergt oder verworfen sind. `lazydev status` zeigt `waiting → <prds>` und den Grund.

```sh
lazydev after login-form user-model   # plan überschreiben: erst nach dem Merge von user-model starten
lazydev after login-form              # auf nichts warten
```

### Stoppen, fortsetzen, verwerfen

```sh
lazydev stop <prd>             # der Schritt hält einen Abbruch fest
lazydev start <prd>            # setzt beim gescheiterten, blockierten oder nächsten Schritt fort
lazydev discard [--yes] <prd>  # Worktree, Branch und Commits weg, PRD zurück auf geplant
```

Der Merge wartet, solange der Hauptordner nicht auf dem Ziel-Branch steht oder Änderungen außerhalb von
`prd/` und `.lazydev/` hat. Ist der Ziel-Branch inzwischen weitergezogen, rebased er erneut.

### Blockierte Läufe

Lässt das PRD eine Entscheidung offen, bricht Claude implement oder den Merge mit offenen Fragen ab. Das ist
kein Fehler: Der Lauf endet in der Phase `blocked`, das PRD im Worktree bekommt `blockiert (Datum)` und
`## Blocker`.

- **In Claude Code:** Der Skill führt `lazydev prompt unblock <prd>` aus, stellt dir die Fragen, arbeitet die
  Antworten ins PRD ein und führt `lazydev start <prd>` aus. „Beantworte die Fragen selbst“ hängt `--auto` an.
- **Auto-Unblock** (`lazydev auto-unblock on`, pro Projekt): Der Runner lässt Claude die Fragen im Worktree
  selbst beantworten, hält sie unter `## Entscheidungen` fest und lässt den blockierten Schritt erneut laufen,
  höchstens 2-mal pro Lauf. Fragen zu Datenverlust, Sicherheit, öffentlichen Schnittstellen oder Kosten darf es
  nicht entscheiden, dann bleibt der Lauf blockiert.
- `lazydev start` lehnt ab, solange das PRD noch `blockiert` sagt.

## Claude-Code-Integration

### Skill

`~/.claude/skills/lazydev/`, geschrieben von `lazydev add`, `lazydev skill install` und jedem Start der TUI,
so folgt er jedem Update. Eine normale Claude-Code-Session kennt damit lazydev und seine CLI: Sie schreibt
PRDs mit `lazydev prompt create` (derselbe Prompt und dieselben Fragen wie `n` in der TUI), startet, stoppt und
überwacht Läufe, löst Blocker und beantwortet Fragen zu lazydev aus der mitgelieferten Spezifikation. Sie
schickt dich nie in die TUI. `"skill": false` in `settings.json` stoppt das automatische Schreiben; zum
Entfernen zusätzlich den Ordner löschen.

### Mod

`lazydev mod install` fügt Claude Code ein Pane neben dem Chat hinzu, wie das Agents-Panel: Kosten, laufende
und wartende Läufe, darunter jedes laufende PRD mit Phase, Modell, Port, `3/5 · implement` mit
Fortschrittsbalken, Kosten und Zeit, die blockierten Läufe mit ihren Fragen, die Queue mit dem, worauf jedes PRD
wartet, und die geplanten PRDs. Es öffnet sich, sobald ein Lauf arbeitet, wartet oder blockiert ist;
`/lazydev-runs` blendet es ein oder aus. Es liest `lazydev status --here --json` alle 3 s und zeigt so nur das
Projekt des Ordners der Session. Liegt in `~/.claude/skills/lazydev-runs/`, `lazydev mod uninstall` entfernt
es, `/reload-plugins` lädt es in eine laufende Session.

## `.lazydev/`

Liegt versioniert im Projekt, für alle gleich. `lazydev add` legt fehlende Dateien an und überschreibt nie.

| Datei | Inhalt |
|---|---|
| `config.yaml` | `baseBranch`, `knowledge`, `allowedTools`, `hooks`, `databases` |
| `create.md`, `implement.md`, `review.md`, `merge.md`, `unblock.md` | Prompts der Schritte |

```yaml
baseBranch: develop          # Standard main
knowledge:                   # Dateien, die Claude zuerst liest: all, create, implement, review, merge, unblock
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

- Prompt = Kopfzeile + Knowledge-Block + `.lazydev/<schritt>.md` + eingebauter Vertrag. Den Vertrag kann ein
  Projekt nicht ändern.
- Hooks bekommen `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`, `LAZYDEV_SLUG`.
  Der Dev-Server (`lazydev dev <prd>`) bekommt zusätzlich `PORT`.
- Hooks, `allowedTools` und `databases` brauchen Vertrauen: `lazydev trust`. Ändern sie sich, fragt lazydev
  erneut.

### Datenbanken pro Worktree

Für Projekte mit Postgres in Docker Compose gibt `databases` jedem Worktree eine eigene leere Datenbank
`<name>_<slug>`. Der Wizard der TUI (`i`) findet Postgres-Services in der Compose-Datei und schreibt den
Abschnitt.

```yaml
databases:
  - service: db                        # Compose-Service
    env:
      file: .env                       # relativ zum Hauptordner, muss von Git ignoriert sein
      key: DATABASE_URL
  - service: db-test
    compose: docker-compose.dev.yml    # ohne die Standarddatei compose.yaml oder docker-compose.yml
    env:
      file: .env.test
      key: NUXT_DATABASE_NAME
```

- `DATABASE_URL="postgres://app:secret@localhost:5432/app?sslmode=disable"` und Slug `login_form` ergeben im
  Worktree `…/app_login_form?sslmode=disable`. Ein einfacher Wert wie `app_test` wird zu `app_test_login_form`.
- Das Setup kopiert die Env-Dateien in den Worktree, startet einen gestoppten Service, legt die Datenbanken an
  und führt dann den Setup-Hook aus. User ist `POSTGRES_USER` des Service, sonst `postgres`, alles per
  `docker exec`.
- Ist der Ziel-Branch vor dem Merge weitergezogen, laufen Datenbanken und Setup-Hook vor dem Check erneut.
- Merge und Verwerfen löschen sie nach dem Teardown-Hook, nur Namen, die auf `_<slug>` enden.

Migrationen und Seeds bleiben im Setup-Hook:

```sh
#!/bin/sh
# scripts/lazydev-setup.sh: die .env des Worktrees nennt schon ihre eigene Datenbank.
set -e
pnpm install --frozen-lockfile --prefer-offline
pnpm db:migrate
```

### Globale Einstellungen

`~/.config/lazydev/settings.json` (Rechte `0600`), gilt für jeden Claude-Schritt aller Projekte:

```json
{
  "permissionMode": "auto",
  "allowedTools": ["Bash(head:*)", "Bash(tail:*)", "Bash(sed -n:*)", "Bash(grep:*)"]
}
```

`permissionMode` ist `acceptEdits` (Vorgabe) oder `auto`; jeder andere Wert lässt den Schritt vor dem Start
von Claude scheitern. In `auto` entscheidet ein Klassifikator von Claude Code über jeden Tool-Aufruf, nicht
deterministisch. `allowedTools` hier brauchen kein Vertrauen und kommen vor denen aus `config.yaml`.

## Discord

Runner schreiben an einen Discord-Webhook, wenn ein PRD gemergt ist, ein Schritt scheitert oder blockiert,
Claude einen Blocker löst, ein Lauf gestoppt wird oder der Merge auf den Hauptordner wartet. Keine TUI nötig.

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

Die URL liegt nur in `settings.json`, nie in einem Repository. Das Menü der TUI (`alt+m` → `Discord`) setzt
sie auch und hat `Send test`. Jede Nachricht ist ein Embed mit Projekt, Branch, Kosten, Dauer und Modell;
`⊘ Blocked` bringt die Fragen mit. Scheitert der Versand, wiederholt der Runner ihn 3-mal im Abstand von 5 s,
danach vermerkt es das Step-Log, der Lauf geht weiter.

## CLI

```sh
lazydev add [<pfad>]                              # Projekt aufnehmen, ohne Pfad das Repo des aktuellen Ordners
lazydev remove <projekt>                          # aus der Liste entfernen, der Ordner bleibt
lazydev projects [--json]
lazydev trust [<projekt>]
lazydev auto-unblock [<projekt>] [on|off]         # Claude löst blockierte Läufe des Projekts selbst
lazydev status [<projekt>] [--all] [--here] [--json]  # PRDs mit Phase, Schritt, Port, Kosten und Blocker-Fragen
lazydev start [<projekt>] <prd>                   # im Hintergrund starten oder fortsetzen
lazydev stop [<projekt>] <prd>
lazydev discard [--yes] [<projekt>] <prd>
lazydev wait [<projekt>] [--json] [--timeout 30m] # blockiert, bis ein Lauf blockiert, scheitert, abbricht oder merged
lazydev log [<projekt>] <prd> [-f]                # Schritt-Logs eines Laufs, -f folgt ihnen
lazydev set-status [<projekt>] <prd> planned|merged|discarded
lazydev after [<projekt>] <prd> [<prd>...]        # plan überschreiben: erst nach diesen PRDs starten
lazydev dev [<projekt>] <prd> [start|stop]        # Dev-Server im Worktree
lazydev prompt create [--auto] [<beschreibung>]   # Create-Prompt des Projekts im aktuellen Ordner ausgeben
lazydev prompt unblock [--auto] [<projekt>] <prd> # Unblock-Prompt eines blockierten Laufs ausgeben
lazydev skill install                             # Claude-Code-Skill installieren
lazydev mod install|uninstall                     # Claude-Code-Pane mit Queue und Schritten der Läufe
lazydev update                                    # neuestes Release installieren
lazydev activity [--since 7d] [--json]            # Nutzungsbericht der TUI
lazydev version                                   # auch --version, -v
lazydev                                           # TUI starten
```

- `<projekt>` ist die ID (Ordnername) oder der Pfad eines Projekts der Liste, `<prd>` der Dateiname ohne
  `.md`. Ohne `<projekt>` gilt das Projekt des aktuellen Ordners, auch im Worktree eines Laufs.
- `--auto` lässt Claude die Fragen von create oder unblock selbst beantworten und unter `## Entscheidungen`
  festhalten.
- `wait` ist für eine Operator-Session gedacht: eine Zeile je Ereignis (`blocked`, `error`, `aborted`,
  `merged`, `note`, `resolved`), `idle`, wenn nichts läuft oder wartet, `--json` mit den Details von
  `status --json`.

## TUI

`lazydev` ohne Argumente öffnet eine Terminal-UI: links die Projekte, in der Mitte Terminal-Tabs (in jedem
Projekt startet Claude Code), darunter das Details-Pane mit den Läufen des Projekts, rechts PRDs und Dateien.
Praktisch, um mehrere Projekte im Blick zu behalten und Shells, Logs und Claude-Sessions an einem Ort zu
haben. Alles, was sie mit Läufen macht, kann die CLI auch.

| Taste | Aktion |
|---|---|
| `alt+←/→`, `tab` / `alt+shift+←/→` | Pane / Tab im Pane |
| `alt+j/k`, `alt+1…9` | Projekt, Tab in der Mitte |
| `alt+t` / `alt+w` | neuer Shell-Tab / Tab schließen |
| `n` | neues PRD mit Claude Code (PRDs) |
| `s` / `x` / `X` | starten oder fortsetzen / stoppen / Lauf verwerfen (PRDs, Runs) |
| `f` | blockierten Lauf mit Claude Code in seinem Worktree klären |
| `L` / `l` | Log des Laufs als Tab (PRDs / Runs), `z` klappt den Kopf ein |
| `D` | Dev-Server im Worktree des PRDs an/aus |
| `c` | Status: planned, merged, discarded |
| `a` / `d` / `i` | Projekt hinzufügen / entfernen / Wizard für `.lazydev/` (Projects) |
| `alt+g` | Git-Cockpit: Fetch, Pull (`--ff-only`), Push, Commit (`add -A`), jeweils als Tab |
| `alt+v` | Markdown-Viewer der gewählten Datei oder des PRDs, `e` öffnet den Editor |
| `alt+c` | Kopiermodus: `j/k`, `v` Zeilen wählen, `y` kopieren |
| `alt+u` | neueres Release aus der Fußzeile installieren |
| `alt+m` / `alt+q` | Menü (Farbschema, Discord, Auto-Unblock) / beenden |

- `alt` geht überall, auch in einem Terminal. Außerhalb eines Terminals ist es optional.
- Kopieren (Kopiermodus, Ziehen mit der Maus) nimmt nur den Inhalt des Tabs und fügt umbrochene Zeilen
  zusammen. Es läuft über OSC 52, geht also auch über SSH. Shift+Ziehen gibt die native Markierung.
- Pull und Push des Git-Cockpits scheitern während eines laufenden Merges, danach erneut versuchen.
- Phasen: `○` planned, `◌` waiting, `●` arbeitet, `◐` merge-queue, `✓` merged, `■` aborted, `✗`
  error/discarded, `⊘` blocked.
- `lazydev activity` berichtet, wie du die TUI nutzt (Zeit je Pane, Tasten, Reaktionszeiten nach einem
  blockierten oder gemergten Lauf). `"activity": false` in `settings.json` schaltet das Log ab.

## Dateien

| Pfad | Inhalt |
|---|---|
| `~/.config/lazydev/` | `projects.json`, `settings.json` (`theme`, `allowedTools`, `permissionMode`, `discord`, `skill`, `autoUnblock`, `activity`), Rechte `0600` |
| `~/.local/state/lazydev/` | `runs/<id>/<prd>.json` und Logs der Schritte, `trust/`, `locks/`, `usage.json`, `activity.jsonl`, `bin/` |
| `~/.local/share/lazydev/worktrees/<id>/<prd>/` | Worktrees |
| `~/.claude/skills/lazydev/`, `lazydev-runs/` | Claude-Code-Skill und -Mod |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` und `XDG_DATA_HOME` gelten, `CLAUDE_CONFIG_DIR` ersetzt `~/.claude`.

## Entwicklung und Release

```sh
make test   # go vet ./... und go test ./...
make build  # bin/lazydev
git tag v0.2.0 && git push origin v0.2.0
```

Ein Tag `vX.Y.Z` startet eine GitHub Action: `make test`, GoReleaser baut `linux/amd64` und `linux/arm64`,
Release mit Archiven und `checksums.txt`. Die Release-Notes kommen aus den Commits seit dem vorigen Tag
(Features, Fixes, Performance, Other; `docs`, `test`, `chore`, `ci`, `style`, `build` fallen weg).
