# lazydev

<p align="center">
  <a href="README.md">English</a> · <b>Deutsch</b>
</p>

lazydev ist eine Terminal-Oberfläche für den PRD-Workflow mit Claude Code. Du siehst alle Projekte und
ihre PRDs, startest die Umsetzung mit einer Taste und verfolgst das Log live. Jedes PRD läuft in einem
eigenen Git-Worktree mit eigener Datenbank und eigenem Port. Nach dem Review mergt lazydev den Branch per
Fast-Forward in den Ziel-Branch des Projekts (Standard `main`) und räumt auf. Vor dem Fast-Forward
vermerkt lazydev die Gesamtkosten des Laufs und das Modell der Umsetzung in der Statuszeile des PRDs
(`Kosten: 4.12 USD · Modell: claude-opus-5-5`), so sieht jeder Rechner und jedes Teammitglied sie in der
PRD-Liste.

Die vollständige Beschreibung des Verhaltens steht in [SPEC.md](SPEC.md).

## Voraussetzungen

- Linux mit `/proc`. lazydev nutzt `setsid`, `flock` und `/proc` für Prozesse und Ports.
- Go 1.27 oder neuer zum Bauen.
- `git` und [Claude Code](https://claude.com/claude-code) (`claude`) im `PATH`.
- Keine globalen Commands oder Agenten. lazydev bringt Standard-Prompts für Erstellen, Umsetzen, Review und
  Merge und die Agenten `committer` und `structural-completeness-reviewer` im Binary mit. Die Agenten
  übergibt es mit `claude --agents`. Jedes Projekt kann die Prompts in seinem Ordner `.lazydev/` ersetzen.
- Für das Preset `nuxt-drizzle`: Docker, `pnpm` und die Compose-Container des Projekts.
- Für die Box „Issues“: das [GitHub CLI](https://cli.github.com) (`gh`) im `PATH`, angemeldet mit
  `gh auth login`.
- Optional: [Herdr](https://herdr.dev). In Herdr öffnen `o`, `w` und `L` einen neuen Pane statt die
  Oberfläche zu unterbrechen.

## Installation

```sh
make install
```

Der Befehl legt die Binärdatei unter `~/.local/bin/lazydev` ab. Um einen anderen Ort zu nutzen, setze
`PREFIX`, z. B. `make install PREFIX=/usr/local`.

Eine laufende TUI behält ihren Code. Um eine neue Version zu nutzen, führe `make install` aus und starte
lazydev neu. Runner, die danach starten, nutzen die neue Version. Für die Entwicklung startet
`go run ./cmd/lazydev` ohne Installation.

## Erste Schritte

1. Starte `lazydev`.
2. Drücke `a`, gib den Pfad eines Projekts ein und bestätige mit `Enter`. `Tab` vervollständigt den Pfad.
3. Drücke `i`, um den Ordner `.lazydev/` des Projekts anzulegen (siehe unten). Committe den Ordner, damit
   dein Team dieselben Einstellungen bekommt.
4. Drücke `n` und beschreibe das Feature in einem Satz. Im Chat-Fenster stellt Claude seine Fragen. Das
   fertige PRD erscheint ungetrackt unter `prd/<bereich>/<name>.md`.
   Jede Runde läuft im Hintergrund weiter, auch nach `Esc` oder `q`. Stellt Claude eine Rückfrage oder
   scheitert eine Runde, erscheint der Chat in der Box „Benachrichtigungen“ rechts neben den PRDs. Das
   Projekt zeigt dann ` <anzahl>✉`. Um den Chat wieder zu öffnen, drücke `N` oder `Enter` in der Box.
5. Wähle das PRD und drücke `s`. lazydev legt den Worktree an, setzt die Datenbanken auf, führt Umsetzung
   und Review aus und mergt. Setzt `.lazydev/config.yaml` Hooks oder Freigaben, fragt lazydev vorher, ob du
   sie erlaubst.
6. Verfolge den Lauf im Log unten. Mit `l` siehst du es in voller Höhe.

Am unteren Rand der PRD-Liste stehen die Docker-Services des Projekts mit Status, Ports und Image,
darunter die laufenden Dev-Server. Um Container zu starten, zu stoppen oder ihre Logs zu sehen, drücke `D`.

Zeigt `origin` eines Projekts auf `github.com`, lädt lazydev die bis zu 30 neuesten offenen Issues über
`gh`. Das passiert beim Start, alle 30 Minuten und mit `r`. Hat das Projekt offene Issues, erscheint die Box
„Issues“ in der rechten Spalte, unter „Benachrichtigungen“ oder allein. Das Projekt zeigt ` <anzahl>#`. In der Box zeigt `Enter`
das Issue im Fenster, `o` öffnet es im Browser. Um aus einem Issue ein PRD zu machen, drücke `n`. Der Chat
startet sofort mit Titel und Text des Issues. Nach einem Neustart zeigt lazydev sofort den letzten Stand.
Scheitert `gh`, nennt die Statuszeile den Grund.

## Projekt-Ordner `.lazydev/`

Jedes Projekt trägt seine lazydev-Einstellungen im Repo, im Ordner `.lazydev/` des Hauptordners. Alle im
Team und jeder Worktree arbeiten damit gleich. Lokal überschreiben lässt sich nichts davon.

| Datei | Inhalt |
|---|---|
| `.lazydev/config.yaml` | `baseBranch`, `preset`, `knowledge`, `allowedTools`, `hooks` |
| `.lazydev/create.md` | Prompt für den Chat zum PRD |
| `.lazydev/implement.md` | Prompt für die Umsetzung |
| `.lazydev/review.md` | Prompt für Review und Commit |
| `.lazydev/merge.md` | Prompt für Konflikte und rote Prüfungen beim Merge |

```yaml
# .lazydev/config.yaml
baseBranch: next          # Ziel-Branch, Standard main. Der Release nach main bleibt Handarbeit.
preset: nuxt-drizzle      # Setup, Teardown, Prüfbefehle, Env-Dateien. Standard: kein Preset
knowledge:                # Dateien, die Claude zuerst liest, relativ zum Projekt
  all: [CLAUDE.md]        # jeder Schritt
  implement: [_ARCHITECTURE.md]
allowedTools:             # ergänzt die Freigaben für Claude bei implement, review und merge
  - "Bash(pnpm test:e2e:*)"
hooks:                    # ersetzt einzelne Hooks des Presets
  setup: ./scripts/lazydev-setup.sh
  teardown: ./scripts/lazydev-teardown.sh
  check: ./scripts/lazydev-check.sh
  dev: ./scripts/lazydev-dev.sh
```

- **Prompts:** Fehlt eine Prompt-Datei, nutzt lazydev seinen Standard-Prompt, den allgemeinen Ablauf
  „einfach“. Eine Prompt-Datei darf mit einer Frontmatter zwischen `---`-Zeilen beginnen, lazydev entfernt
  sie. So lassen sich Commands aus `.claude/commands/` übernehmen. Prompts können `$ARGUMENTS`, `$BASE`
  (Ziel-Branch) und `$BRANCH` nutzen.
- **Vertrag:** lazydev hängt an jeden Prompt einen festen Vertrag an, beginnend mit
  `## Regeln von lazydev`. Er enthält, was der Runner auswertet, etwa Berichtsformat und Statuszeile, und
  verbietet Änderungen an `.lazydev/`. Ändert Claude im Worktree eine Datei unter `.lazydev/`, scheitert der
  Schritt.
- **Knowledge:** Dateien unter `knowledge.all` stehen in jedem Prompt, danach die des Schritts, als Liste
  unter `## Pflichtlektüre`. Fehlt eine Datei im Arbeitsordner des Schritts, scheitert der Schritt mit
  `Knowledge-Datei fehlt: <pfad>`, ohne Claude zu starten.
- **Vertrauen:** `hooks` und `allowedTools` führen Code aus bzw. erweitern Claudes Rechte. lazydev startet
  einen Runner erst, wenn du sie erlaubt hast: `s` fragt in der TUI, `lazydev trust <projekt-id>` auf der
  Kommandozeile. Jede Änderung an diesen beiden Abschnitten fragt erneut.
- lazydev liest die Dateien beim Start jedes Schritts und jeder Chat-Runde aus dem Hauptordner.
  Uncommittete Änderungen gelten sofort.

**Wizard:** `i` legt den Ordner in vier Seiten an: Ziel-Branch, Knowledge-Dateien, Schritte je Datei und
Zusammenfassung. `Enter` geht weiter, `Backspace` zurück, `Esc` bricht ab. Der Wizard schreibt
`config.yaml` und kopiert die vier Standard-Prompts in den Ordner, damit du sie anpassen kannst. Er
übernimmt `baseBranch`, `preset`, `allowedTools` und `hooks` aus der zentralen Konfiguration und entfernt
sie danach dort. lazydev committet den Ordner nicht.

**Projekte ohne `.lazydev/`** lesen `baseBranch`, `preset`, `allowedTools` und `hooks` weiter aus der
zentralen Konfiguration. Der Titel der PRD-Box zeigt ` · ohne .lazydev`.

## Tasten

Die Fußzeile zeigt nur die Tasten, die im aktiven Fokus und für das gewählte PRD gerade wirken. Alle
anderen Tasten funktionieren trotzdem, `?` zeigt die vollständige Liste.

| Taste | Wirkung |
|---|---|
| `↑` `↓` / `j` `k` | Auswahl in der aktiven Liste |
| `Tab` / `←` `→` | Fokus wechseln: Projekte, PRDs, Benachrichtigungen, Issues, Log oder Details. Im unteren Bereich scrollen `j` `k`. |
| `a` / `d` | Projekt hinzufügen / aus der Liste entfernen |
| `b` | Ziel-Branch des Projekts wählen und in `.lazydev/config.yaml` schreiben. Worktrees zweigen davon ab, der Merge geht dorthin. |
| `i` | Ordner `.lazydev/` des Projekts mit dem Wizard anlegen |
| `n` | Neues PRD: Feature beschreiben, dann Chat-Fenster mit Opus |
| `N` | Älteste Rückfrage eines Chats über alle Projekte öffnen |
| `Enter` / `X` | In „Benachrichtigungen“: Chat öffnen / Chat beenden, mit Rückfrage |
| `Enter` / `o` / `n` | In „Issues“: Issue im Fenster ansehen / im Browser öffnen / PRD-Chat aus dem Issue starten |
| `Esc` | Im Chat: Fenster ausblenden. Die Runde läuft weiter. |
| `Ctrl+X` | Im Chat: Chat beenden, mit Rückfrage. Eine laufende Runde bricht ab, das Log bleibt. |
| `Ctrl+R` | Im Chat: nach einem Fehler die letzte Runde wiederholen |
| `v` | PRD vollständig als Markdown im Fenster ansehen. Darin öffnet `o` die Datei im Editor. |
| `c` | Status eines PRDs ohne Lauf setzen: `geplant`, `umgesetzt` oder `verworfen`. Umgesetzte und verworfene PRDs landen in der Gruppe „erledigt“. Getrackte PRDs committet lazydev nach Rückfrage im Hauptordner. |
| `s` | Umsetzung starten. Nach Fehler oder Abbruch geht es beim gescheiterten Schritt weiter. Fragt vorher, ob du Hooks und Freigaben aus `.lazydev/config.yaml` erlaubst. |
| `m` | Merge freigeben, wenn `autoMerge` aus ist oder der Merge scheiterte |
| `x` | Runner abbrechen. Der Worktree bleibt. |
| `X` | PRD verwerfen: Datenbanken, Worktree und Branch löschen, PRD zurück in den Hauptordner |
| `o` | Letzte Claude-Sitzung des PRDs im Worktree öffnen |
| `p` | Dev-Server im Worktree starten oder stoppen |
| `w` | Shell im Worktree öffnen |
| `e` | Erledigte PRDs auf- und zuklappen. Beim Start sind sie zugeklappt. |
| `S` | PRDs nach Name oder Erstellzeit sortieren, neueste zuerst |
| `D` | Docker-Ansicht: `U` compose up, `S` compose stop, `L` Logs |
| `l` | Log in voller Höhe |
| `Enter` | Im Log: vollständiges Ereignis |
| `u` | Claude-Limits und Verbrauch. `r` aktualisiert mit einer kurzen Anfrage an Haiku. |
| `r` | Neu einlesen, Issues neu laden |
| `?` | Hilfe |
| `q` | Beenden. Laufende Runner und Chats arbeiten weiter. |

## Befehle ohne TUI

```sh
lazydev start <projekt-id> <prd-name>             # Runner im Hintergrund, mit maxParallel
lazydev run <projekt-id> <prd-name>               # Runner im Vordergrund
lazydev merge <projekt-id> <prd-name>             # Merge freigeben, wenn autoMerge aus ist
lazydev stop <projekt-id> <prd-name>              # Runner stoppen
lazydev teardown [--yes] <projekt-id> <prd-name>  # Lauf verwerfen
lazydev chat <projekt-id> <chat-id>               # eine Runde eines PRD-Chats, die TUI startet sie
lazydev trust <projekt-id>                        # Hooks und Freigaben aus .lazydev/config.yaml prüfen und erlauben
```

Die Projekt-ID ist der Ordnername des Projekts. Der PRD-Name ist der Dateiname ohne `.md`. Die Chat-ID
ist die Startzeit des Chats, z. B. `2026-10-20-140211`. Ohne passende Bestätigung scheitern `start` und
`run` mit einem Hinweis auf `lazydev trust`.

## Konfiguration

lazydev liest `~/.config/lazydev/config.yaml`. Mit `LAZYDEV_CONFIG` nutzt du eine andere Datei. Die TUI
schreibt die Datei selbst, wenn du Projekte hinzufügst oder entfernst. Die Einstellungen der Projekte
stehen in deren `.lazydev/config.yaml`.

```yaml
maxParallel: 3               # gleichzeitige Runner über alle Projekte
portBase: 3100               # erster Port für Worktrees
claude:
  binary: claude
projects:
  - path: ~/development/nuxt/nuxt-drizzle
    autoMerge: true          # Standard. false: Merge erst nach m. Bleibt bewusst lokal.
```

Hook-Skripte laufen im Worktree. Relative Pfade gelten ab dem Hauptordner des Projekts. Die Skripte
bekommen `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT` und `LAZYDEV_SLUG`.

## Preset nuxt-drizzle

- **Setup:**
  - kopiert `.env` und `.env.test` in den Worktree,
  - hängt `_<slug>` an `NUXT_DATABASE_NAME` und setzt `PORT`,
  - legt beide Datenbanken per `docker exec … createdb` an,
  - führt `pnpm install`, `pnpm db:migrate` und `pnpm test:db:push` aus.
- **Prüfung vor dem Merge:** `pnpm lint`, `pnpm typecheck`, `pnpm test:run`.
- **Dev-Server:** `pnpm dev` auf dem Port des Worktrees.
- **Teardown:** löscht nur Datenbanken mit `_<slug>` im Namen. Die Haupt-Datenbanken bleiben unberührt.

Der Wizard erkennt das Preset an `drizzle.config.ts` und `server/database/migrations`.

## Dateien

| Was | Pfad |
|---|---|
| Konfiguration | `~/.config/lazydev/config.yaml` |
| Einstellungen und Prompts des Projekts | `<hauptordner>/.lazydev/` |
| Bestätigte Hooks und Freigaben | `~/.local/state/lazydev/trust/<projekt-id>.json` |
| Lauf-Zustand | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.json` |
| Log pro Schritt | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.<schritt>.jsonl` |
| Ausgabe des Runners | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.runner.log` |
| Dev-Server | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.dev.log` |
| Letzter Stand der Claude-Limits | `~/.local/state/lazydev/usage.json` |
| Zustand des Chats für `n` | `~/.local/state/lazydev/chats/<projekt-id>/<chat-id>.json` |
| Log des Chats für `n` | `~/.local/state/lazydev/chats/<projekt-id>/<chat-id>.jsonl` |
| Letzter Stand der GitHub-Issues | `~/.local/state/lazydev/issues/<projekt-id>.json` |
| Worktrees | `~/.local/share/lazydev/worktrees/<projekt-id>/<prd-name>/` |

lazydev beachtet `XDG_CONFIG_HOME`, `XDG_STATE_HOME` und `XDG_DATA_HOME`.

## Fehler beheben

- **Ein PRD steht auf „Fehler“.** Die Kopfzeile des Logs nennt den Grund. Mit `o` fragst du in der Sitzung
  nach, mit `s` startest du den Schritt neu.
- **Ein PRD steht auf „abgebrochen“, ohne dass du `x` gedrückt hast.** Der Runner endete unerwartet. Seine
  Ausgabe steht in `<prd-name>.runner.log`.
- **Der Merge wartet mit „Hauptordner nicht sauber“.** Committe oder verwirf die Änderungen im Hauptordner.
  Änderungen unter `.lazydev/` blockieren den Merge nicht. Der Merge prüft alle 5 Sekunden neu.
- **Im Chat für `n` erscheint ein roter Fehler wie `Brace expansion`.** Claude hat einen Befehl versucht,
  den der Chat nicht freigibt, und liest die Dateien danach mit Read und Grep. Das PRD entsteht trotzdem.
  Alle Ereignisse stehen im Chat-Log.
- **Claude lehnt einen Befehl ab.** Die Zeile `abgelehnt: …` im Log nennt das Tool. Um es freizugeben,
  ergänze `allowedTools` in `.lazydev/config.yaml`.
- **Ein Schritt scheitert mit `Knowledge-Datei fehlt`.** Die Datei fehlt im Worktree, meist weil sie nicht
  committet ist. Committe sie oder entferne sie aus `knowledge`.

## Entwicklung

```sh
make test     # go vet und go test
make build    # Binärdatei unter bin/lazydev
```
