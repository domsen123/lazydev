---
name: lazydev
description: lazydev und seine PRDs. Nutze diesen Skill bei lazydev, lazydev PRD, PRD erstellen, PRD schreiben, „mach daraus ein PRD“, „make this a lazydev PRD“, Dateien unter prd/, der Statuszeile „> Status:“, .lazydev/config.yaml, Hooks, Worktrees, Runs, Schritten (create, implement, review, merge, unblock), Läufe starten, überwachen und Blocker lösen, Projekt hinzufügen und Fragen zu lazydev.
---

<!-- Diese Datei verwaltet lazydev. Sie wird beim Start der TUI, bei "lazydev add" und bei "lazydev skill install" überschrieben. -->

# lazydev

lazydev setzt PRDs (Product Requirements Documents) mit Claude Code um. Ein PRD beschreibt ein Feature so
vollständig, dass ein Agent es ohne Mensch umsetzt. lazydev setzt jedes PRD in einem eigenen Git-Worktree
um: plan → setup → implement → review → merge, und bringt es per Fast-Forward in den Basis-Branch des Projekts.
Ein Projekt erkennst du am Ordner `.lazydev/` mit `config.yaml` und den Prompts der Schritte, die PRDs
liegen unter `prd/<bereich>/<name>.md`.

Alles geht über die Kommandozeile `lazydev`. Die TUI (`lazydev` ohne Argumente) ist optional. Schick den
Nutzer nie in die TUI, um eine Taste zu drücken, sondern nutze den passenden Befehl.

## Regeln

- Schreibe PRDs nur unter `prd/`. Implementiere nichts, wenn der Nutzer ein PRD will.
- Ändere nichts unter `.lazydev/`. Der Ordner gehört lazydev und ist für alle im Team gleich.
- Jedes PRD beginnt mit `# PRD: <Titel>` und direkt danach der Statuszeile
  `> Status: <wert> · Scope: <Kurzfassung>`. Werte: `geplant`, `umgesetzt (YYYY-MM-DD)`,
  `verworfen (YYYY-MM-DD)`, `blockiert (YYYY-MM-DD)`. Ein neues PRD bekommt `geplant`.
- Starte, stoppe oder verwirf einen Lauf nur, wenn der Nutzer das will. `discard` löscht Commits, frag
  vorher.
- `<projekt>` ist optional: Ohne Angabe gilt das Projekt des aktuellen Ordners, auch aus einem Worktree.
  `<prd>` ist der Dateiname ohne `.md`.

## Aufgaben

| Aufgabe | So gehst du vor |
|---|---|
| Projekt in lazydev aufnehmen | `lazydev add [<pfad>]`, ohne Pfad das Repository des aktuellen Ordners. Legt `.lazydev/` an. |
| PRD schreiben, auch aus dem bisherigen Gespräch | Führe im Projekt `lazydev prompt create "<Kurzfassung in einem Satz>"` aus. Die Ausgabe ist der vollständige Create-Prompt. Folge ihr Schritt für Schritt, inklusive der nummerierten Klärungsfragen mit den Optionen A bis D, und warte auf die Antworten. Ohne Argument ergibt sich die Aufgabe aus dem Gespräch. Soll der Nutzer nicht gefragt werden („beantworte die Fragen selbst“), hänge `--auto` an. |
| Stand der PRDs und Läufe | `lazydev status [<projekt>] [--json]`: Phase, Schritt, Port, Kosten, Fehler und die Fragen blockierter Läufe. `--all` zeigt auch erledigte PRDs. |
| Lauf starten, fortsetzen, stoppen, verwerfen | `lazydev start [<projekt>] <prd>`, `lazydev stop …`, `lazydev discard --yes …`. Höchstens 3 Läufe arbeiten gleichzeitig, weitere warten in der Queue und starten von selbst. `start` setzt nach Fehler oder Abbruch beim fehlgeschlagenen Schritt fort. Starte neue PRDs einfach, auch wenn andere laufen: Der erste Schritt `plan` entscheidet selbst, ob das PRD parallel läuft oder auf andere wartet. |
| Reihenfolge ändern | `lazydev after [<projekt>] <prd> [<prd>…]` überschreibt die Entscheidung von `plan`, ohne PRDs wartet es auf nichts. `lazydev status` zeigt `waiting → <prds>` und den Grund. |
| Läufe überwachen | `lazydev wait [<projekt>] [--json] [--timeout 30m]` blockiert, bis ein Lauf blockiert, fehlschlägt, abbricht, gemergt wird oder einen Hinweis bekommt, und gibt die Ereignisse aus. `idle` heißt: nichts läuft oder wartet. In Claude Code eignet sich dafür das Werkzeug `Monitor` oder ein Hintergrund-Befehl. |
| Log eines Laufs | `lazydev log [<projekt>] <prd>`, `-f` folgt ihm bis zum Ende des Laufs. |
| Blocker automatisch lösen lassen | `lazydev auto-unblock [<projekt>] on` lässt den Runner blockierte Läufe selbst klären (höchstens 2-mal pro Lauf), `off` schaltet es ab. Nur auf Wunsch des Nutzers. |
| Blockierten Lauf lösen | `lazydev prompt unblock [<projekt>] <prd>` gibt den Unblock-Prompt aus, das PRD steht darin mit absolutem Pfad im Worktree. Folge ihm. Mit `--auto` beantwortest du die Fragen selbst. Danach `lazydev start <projekt> <prd>`. |
| PRD-Status von Hand setzen | `lazydev set-status [<projekt>] <prd> planned\|merged\|discarded`, committet das PRD. |
| Dev-Server im Worktree | `lazydev dev [<projekt>] <prd> [start\|stop]`, der Port steht in `lazydev status`. |
| Hooks oder Datenbanken vertrauen | `lazydev trust [<projekt>]` fragt den Nutzer interaktiv. Bitte ihn, es selbst auszuführen. |
| Fragen zu PRDs: Format, Statuszeile, `## Blocker`, Branch | Lies `## PRDs` in `references/spec.md`. |
| Fragen zu `.lazydev/`: `config.yaml`, Prompts, Knowledge, Trust | Lies `## .lazydev/` in `references/spec.md`. |
| Fragen zu Läufen: Schritte, Worktrees, Merge, Hooks, Dev-Server, Discord | Lies `## Workflow` und `## Processes` in `references/spec.md`. |
| Fragen zur Kommandozeile und zu Dateien | Lies `## CLI` und `## Data` in `references/spec.md`. |
| Fragen zur TUI: Panes, Tasten, Git-Cockpit, Menü | Lies `## UI` in `references/spec.md`. |

`references/spec.md` ist die vollständige Spezifikation von lazydev. Lies nur den Abschnitt, der zur
Aufgabe passt, nicht die ganze Datei.

Fehlt `lazydev` im `PATH`, weise den Nutzer darauf hin, dass er lazydev installieren muss
(https://github.com/domsen123/lazydev). Baue Prompts und das PRD-Format dann nicht aus dem Gedächtnis
nach, die Prompts gehören zum Projekt und zur Version von lazydev.
