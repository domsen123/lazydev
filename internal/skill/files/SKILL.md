---
name: lazydev
description: lazydev und seine PRDs. Nutze diesen Skill bei lazydev, lazydev PRD, PRD erstellen, PRD schreiben, „mach daraus ein PRD“, „make this a lazydev PRD“, Dateien unter prd/, der Statuszeile „> Status:“, .lazydev/config.yaml, Hooks, Worktrees, Runs, Schritten (create, implement, review, merge, unblock) und Fragen zu lazydev.
---

<!-- Diese Datei verwaltet lazydev. Sie wird beim Start der TUI und bei "lazydev skill install" überschrieben. -->

# lazydev

lazydev ist eine Terminal-UI für den PRD-Workflow mit Claude Code. Ein PRD (Product Requirements Document)
beschreibt ein Feature so vollständig, dass ein Agent es ohne Mensch umsetzt. lazydev setzt jedes PRD in
einem eigenen Git-Worktree um: setup → implement → review → merge, und bringt es per Fast-Forward in den
Basis-Branch des Projekts. Ein Projekt erkennst du am Ordner `.lazydev/` mit `config.yaml` und den Prompts
der Schritte, die PRDs liegen unter `prd/<bereich>/<name>.md`.

## Regeln

- Schreibe PRDs nur unter `prd/`. Implementiere nichts, wenn der Nutzer ein PRD will.
- Ändere nichts unter `.lazydev/`. Der Ordner gehört lazydev und ist für alle im Team gleich.
- Jedes PRD beginnt mit `# PRD: <Titel>` und direkt danach der Statuszeile
  `> Status: <wert> · Scope: <Kurzfassung>`. Werte: `geplant`, `umgesetzt (YYYY-MM-DD)`,
  `verworfen (YYYY-MM-DD)`, `blockiert (YYYY-MM-DD)`. Ein neues PRD bekommt `geplant`.
- Die Umsetzung startet der Nutzer in lazydev mit `s`. Du startest keinen Lauf.

## Aufgaben

| Aufgabe | So gehst du vor |
|---|---|
| PRD schreiben, auch aus dem bisherigen Gespräch | Führe im Hauptordner des Projekts `lazydev prompt create "<Kurzfassung in einem Satz>"` aus. Die Ausgabe ist der vollständige Create-Prompt des Projekts. Folge ihr Schritt für Schritt, inklusive der nummerierten Klärungsfragen mit den Optionen A bis D, und warte auf die Antworten. Ohne Argument ergibt sich die Aufgabe aus dem Gespräch. |
| Fragen zu PRDs: Format, Statuszeile, `## Blocker`, Branch | Lies `## PRDs` in `references/spec.md`. |
| Fragen zu `.lazydev/`: `config.yaml`, Prompts, Knowledge, Trust, Wizard | Lies `## .lazydev/` in `references/spec.md`. |
| Fragen zu Läufen: Schritte, Worktrees, Merge, blockierte Läufe, Hooks, Dev-Server, Discord | Lies `## Workflow` und `## Processes` in `references/spec.md`. |
| Fragen zur Bedienung: Panes, Tasten, Log-Tab, Markdown-Viewer, Kopiermodus, Git-Cockpit, Menü | Lies `## UI` in `references/spec.md`. |
| Fragen zur Kommandozeile und zu Dateien: Befehle, `settings.json`, State, Pfade | Lies `## CLI` und `## Data` in `references/spec.md`. |

`references/spec.md` ist die vollständige Spezifikation von lazydev. Lies nur den Abschnitt, der zur
Aufgabe passt, nicht die ganze Datei.

Fehlt `lazydev` im `PATH`, weise den Nutzer darauf hin, dass er lazydev installieren muss
(https://github.com/domsen123/lazydev). Baue den Prompt und das PRD-Format dann nicht aus dem Gedächtnis
nach, der Prompt gehört zum Projekt und zur Version von lazydev.
