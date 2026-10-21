# PRD umsetzen (Template „nuxt“): $ARGUMENTS

## Deine Aufgabe

Setze das PRD `$ARGUMENTS` vollständig um, Backend und Frontend. Das Projekt folgt der Nuxt-4-Vertical-
Slice-Architektur (ItemService/DI, Pinia Colada, Drizzle, Zod v4, Nuxt UI). Du arbeitest ohne Mensch im
Print-Modus, Rückfragen sind nicht möglich. Was du nicht entscheiden darfst, landet im Abschlussbericht.

> **Grundregel: keine eigenen Entscheidungen.** Das PRD enthält nur finale Entscheidungen. Lässt es
> einen Punkt offen, den du für die Umsetzung brauchst, entscheidest du ihn nicht selbst. Brich ab und
> nenne die Frage im Abschlussbericht unter „Offene Fragen“. Kleine Details, die bestehender Code oder
> die Architektur-Doku eindeutig vorgeben, sind keine offenen Entscheidungen.

Arbeite strikt in dieser Reihenfolge.

### Schritt 1: Lesen

1. Lies das PRD vollständig. Fehlt die Datei, brich ab.
2. Lies `CLAUDE.md` und die Architektur-Doku (`_ARCHITECTURE.md`, in manchen Projekten
   `_SPEC_ARCHITECTURE.md`), besonders die Feature-Checkliste und die Anti-Patterns.
3. Lies die Prüfbefehle aus `CLAUDE.md`, z. B. `pnpm lint`, `pnpm typecheck`, `pnpm test:run`.
4. Lies die Dateien, die das PRD unter `## Umsetzung (Feature-Anatomie)` und `## Technische Hinweise`
   nennt, und die bestehenden Slices, auf die es verweist, bevor du etwas änderst.

### Schritt 2: Branch prüfen

`<prd-name>` ist der Dateiname des PRDs ohne `.md`. Prüfe mit `git branch --show-current`, dass der
Branch `feat/<prd-name>` heißt. Wechsle keinen Branch. Heißt er anders, brich ab.

### Schritt 3: Backend umsetzen

1. Setze `## Datenmodell` um. Ändere das Schema unter `server/database/schema/`, dann
   `pnpm db:generate` und `pnpm db:migrate`. Schreibe Migrationen nie von Hand.
2. Setze `## Umsetzung (Feature-Anatomie)` in der Reihenfolge der Feature-Checkliste um: Schema,
   ItemService, Feature-Service, Container, API-Routen, Client-API, Query-Keys, Composables.
3. Halte dich an `## Technische Hinweise`, die Konventionen aus `CLAUDE.md` und vermeide die
   Anti-Patterns der Architektur-Doku.
4. Schreibe die Tests, die das PRD oder `CLAUDE.md` verlangen.

### Schritt 4: Frontend umsetzen

Erst nach dem Backend. Grundlage ist `## Frontend-Umsetzung (Handoff)`.

1. Hat das PRD einen Abschnitt `## Pencil-Screens`, lies jeden genannten Frame über die
   `pencil`-MCP-Tools: Lies zuerst den Zustand der App und die Anleitung des `pencil`-MCP, dann
   Struktur, Werte und Brand-Tokens der Frames. Verändere das Design nie. `.pen`-Dateien liest du nie
   über Read oder Grep. Sind die `pencil`-Tools nicht verfügbar oder fehlt ein Frame, brich ab und
   nenne das unter „Offene Fragen“.
2. Ohne `## Pencil-Screens` baust du die UI nach der Beschreibung im Handoff. Sagt der Handoff, dass
   die UI entfällt, baust du keine.
3. Baue Components, Barrel-Export, Pages und Routen-Konstanten nach der Feature-Checkliste. Nutze
   Nuxt-UI-Komponenten und die Tokens aus dem Theme des Projekts statt fester Farben oder Abstände.
4. Ergänze weitere Schritte der Checkliste, etwa Übersetzungen.

### Schritt 5: Prüfen

1. Führe alle Prüfbefehle des Projekts aus und behebe jeden Fehler. Wiederhole, bis alle durchlaufen.
2. Starte nie einen Watch-Modus wie `pnpm test`, einen Dev-Server oder andere Befehle, die nicht von
   selbst enden.
3. Ändere Dateien nur mit den Tools Edit und Write. Skripte wie `python3`, `sed -i` oder Heredocs sind
   nicht freigegeben.
4. Lehnt Claude Code einen Prüfbefehl ab, weil er nicht freigegeben ist, brich ab. Nenne den Befehl
   unter „Offene Fragen“.
5. Gleiche das Ergebnis mit `## Erfolgskriterien` ab.
6. Lässt sich ein Fehler nicht beheben, ohne vom PRD abzuweichen, brich ab und nenne ihn unter
   „Offene Fragen“.
7. Kriterien, die einen Dev-Server oder Browser brauchen, etwa der Abgleich der UI mit den
   Pencil-Screens, nennst du unter „Nicht geprüft“. Sie blockieren die Umsetzung nicht.

### Schritt 6: Status setzen

Setze die Statuszeile des PRDs von `geplant` auf `umgesetzt (<heute als YYYY-MM-DD>)`. Ändere nur den
Teil vor dem ersten `·`, der Freitext danach bleibt:

```
> Status: umgesetzt (2026-10-20) · Template: nuxt · Scope: …
```

Setze den Status nur, wenn Schritt 3 bis 5 vollständig erfolgreich waren.

### Schritt 7: Abschlussbericht

**Committe nicht.** Das übernimmt der nächste Schritt. Kein `git add`, kein `git commit`, kein
`git push`. Ausnahme: `git mv`, wenn das PRD eine Datei verschiebt.

Schreibe zum Schluss einen kurzen Bericht mit genau diesen Abschnitten:

```
## Ergebnis
umgesetzt | abgebrochen

## Geänderte Dateien
- <pfad>: <ein Satz>

## Prüfungen
- <befehl>: ok | Fehler (<erste Zeile>)

## Offene Fragen
- <Frage oder Problem mit Datei und Stelle>, oder „keine“

## Nicht geprüft
- <Erfolgskriterium und Grund>, oder „nichts“
```

Regeln für `## Ergebnis`:

- `umgesetzt` nur, wenn alle Prüfungen `ok` sind und `## Offene Fragen` „keine“ enthält.
- Sonst `abgebrochen`. Die Statuszeile bleibt dann auf `geplant`.
- `## Nicht geprüft` beeinflusst das Ergebnis nicht.

lazydev wertet nur `## Ergebnis` aus. Halte die Überschriften und die beiden Werte genau so ein.

## Verboten

- Entscheidungen treffen, die das PRD offenlässt.
- Commits, `git push`, `git reset --hard`, `git worktree`, `pnpm docker:clean`.
- Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
- Das Pencil-Design verändern.
