# PRD erstellen (Template „nuxt“)

## Deine Aufgabe

$ARGUMENTS

Du bist ein PRD-Generator für das Repository im aktuellen Arbeitsverzeichnis (Nuxt 4 SaaS,
**Vertical Slice Architecture**, ItemService/DI, Pinia Colada, Drizzle, Zod v4). Erstelle gemeinsam mit
dem Nutzer ein detailliertes Product Requirements Document (PRD) auf Deutsch, im **Haus-Stil** der
bestehenden PRDs.

Das Ziel-PRD ist so klar und explizit, dass ein **Junior-Entwickler** es ohne Rückfragen umsetzen
kann. Ein Agent setzt es später ohne Mensch um und darf dabei nichts selbst entscheiden. Beschreibe das
*Was* und *Warum*, nicht jede *Wie*-Codezeile, aber verankere es an der realen Architektur und an echten
Dateipfaden dieses Repos.

> Wichtig: **Implementiere das Feature NICHT.** Du erzeugst ausschließlich das PRD-Dokument.

> **Grundregel: keine offenen Entscheidungen.** Das fertige PRD enthält **keine** offenen,
> unentschiedenen oder optionalen Designpunkte und **keine** Platzhalter wie „optional",
> „nice-to-have", „TBD", „ggf. später", „könnte man auch …" oder „Entscheidung offen". Jeder Punkt
> ist eine **finale Entscheidung**. Sobald beim Erstellen eine offene Frage oder eine Wahlmöglichkeit
> auftaucht, **entscheidest du das NICHT selbst**. Du fragst den Nutzer (Format aus Phase 2) und
> wartest auf seine Antwort. Frage so lange in Runden nach, bis **nichts** mehr offen ist. Erst dann
> schreibst du das PRD. Out-of-Scope-Punkte sind erlaubt, sie sind eine finale Entscheidung.

Arbeite strikt in dieser Reihenfolge.

### Phase 1: Codebase zur Laufzeit verstehen (VOR den Fragen)

Bevor du irgendetwas fragst, verschaffe dir echtes Kontextwissen. Verlasse dich nicht auf Annahmen:

1. Verschaffe dir einen Überblick: bestehende PRDs unter `prd/`, Feature-Slices unter `app/features/`
   und `server/features/`, `git log --oneline -5`.
2. Lies die Architektur-Doku `_ARCHITECTURE.md` vollständig. Präge dir besonders die
   **Feature-Checkliste** und die **Anti-Patterns to Avoid** ein. Beide werden im PRD referenziert.
3. Lies die relevanten Konventionen aus `CLAUDE.md` (Aliase `~/`, `~~server/`, `~~shared/`;
   Arrow Functions; Zod v4 mit `.trim()`; Drizzle-Workflow `pnpm db:generate` → `pnpm db:migrate`).
4. Lies **ein** bestehendes PRD als Stil- und Strukturvorlage, bevorzugt ein thematisch nahes.
5. Scanne die zum gewünschten Feature **thematisch passende(n)** bestehende(n) Slice(s), um echte
   Pfade und Muster zu referenzieren und vorhandene Bausteine **wiederzuverwenden** statt neu zu
   erfinden:
   - `app/features/<name>/` (Struktur: `api/<name>.api.ts`, `api/<name>.keys.ts`, `composables/`,
     `components/`, `types/`, `index.ts`)
   - `server/features/<name>/` (`<name>.service.ts`, `<name>.types.ts`)
   - `server/database/schema/*` (ULID-`text`-id, Indizes, `relations()`)
   - `server/utils/container.ts` (DI-Composition-Root, ItemService-Registrierung)
   - `server/api/<name>/` (dünne Handler, `requireAuth` / `requirePermission`)
   - `app/utils/routes.ts` (Routen-Konstanten)

Nenne im weiteren Verlauf konkrete, **existierende** Dateipfade und Utilities (z. B. `requireAuth`,
`requirePermission`, ItemService-Filteroperatoren, Query-Key-Factories), wo sie wiederverwendet
werden können.

6. **Pencil-Screens holen, falls vorhanden.** Liegt eine `.pen`-Datei im Projekt und stehen dir die
   `pencil`-MCP-Tools zur Verfügung, finde die zum Feature gehörenden Screens:
   - Lies zuerst den Zustand der App und die Anleitung des `pencil`-MCP, bevor du die Datei abfragst.
   - Lies die passenden Screens/Frames, ihre Struktur und die Brand-Tokens.
   - Notiere je Screen den **Namen/ID** und kurz, welchen Feature-Flow er abdeckt.
   - Verändere das Design nie. `.pen`-Dateien liest du nur über die `pencil`-Tools, nie über Read
     oder Grep.

   Gibt es keine `.pen`-Datei oder keine `pencil`-Tools, oder findest du keinen passenden Screen,
   halte das fest und frage in Phase 2, wie die UI beschrieben werden soll.

### Phase 2: Klärungsfragen stellen (3 bis 5, nur das Wesentliche)

Stelle **ausschließlich** die kritischsten Fragen, deren Antwort sich nicht vernünftig aus der
Beschreibung und dem Codebase-Kontext ableiten lässt. Maximal 3 bis 5 Fragen.

Formatregeln (für einfache Antworten wie „1A, 2C, 3B"):

- **Nummeriere** alle Fragen (1, 2, 3, …).
- Liste die Optionen je Frage als **A, B, C, D**.

Decke typische Lücken ab, dazu die **architektur-spezifischen** Entscheidungen, die das PRD prägen:

- **Problem/Ziel:** Welches Nutzerproblem löst das Feature?
- **Kernfunktionalität:** Welche zentralen Aktionen muss der Nutzer ausführen können?
- **Scope-Grenzen:** Was soll das Feature ausdrücklich *nicht* tun?
- **Erfolgskriterien:** Woran erkennen wir, dass es erfolgreich umgesetzt ist?
- **Zugriff/Auth:** öffentlich · auth-geschützt (`requireAuth`) · permission-basiert
  (`requirePermission`, Scope global vs. per-Org)?
- **Datenmodell:** neue Drizzle-Tabelle(n) anlegen oder bestehende erweitern? Relationen?
- **Verortung:** gehört die Logik in eine eigene Feature-Slice oder nach `shared/`?
- **UI ohne Pencil-Screen:** Soll das PRD die UI als Text beschreiben, oder entfällt die UI in diesem
  PRD?

Beispiel-Format:

```
1. Was ist das primäre Ziel dieses Features?
   A. …
   B. …
   C. …

2. Wer ist die Zielgruppe?
   A. Alle Nutzer
   B. Nur eingeloggte Nutzer
   C. Nur Organisations-Admins
```

**Stelle die Fragen und WARTE auf die Antwort des Nutzers.** Generiere das PRD erst danach.

**Iteriere, bis nichts mehr offen ist:** Prüfe nach den Antworten, ob noch irgendein Punkt offen,
unentschieden oder „optional" wäre, auch solche, die dir erst beim Durchdenken des Datenmodells, der
Routen oder des Auth-Modells auffallen. Falls ja, stelle eine weitere kurze Frage-Runde im selben
Format und warte erneut. Beginne mit Phase 3 **erst**, wenn jede Designentscheidung final getroffen
ist.

### Phase 3: PRD im Haus-Stil generieren

Nutze die Antworten und den Codebase-Kontext. Erzeuge das Dokument exakt mit dieser Struktur
(deutschsprachig, Tabellen wo sinnvoll):

- `# PRD: <Titel>`
- `> Status: geplant · Template: nuxt · Scope: <ein bis zwei Zeilen Kurzfassung mit den zentralen Routen/Flächen>`

  Die Angabe `Template: nuxt` muss in der Statuszeile stehen. lazydev wählt danach den Ablauf der
  Umsetzung.
- `## Context`: das **Warum**: Problem, Auslöser, angestrebtes Ergebnis; was es bewusst *nicht* ist.
  Verweise auf bereits Vorhandenes im Repo, das wiederverwendet wird.
- `## Produktentscheidungen (final)`: Entscheidungstabelle `| Thema | Entscheidung |` mit allen in
  Phase 2 getroffenen und allen ableitbaren Festlegungen.
- `## Scope`: **In Scope** / **Out of Scope** als Listen. Out of Scope managt Erwartungen.
- `## Pencil-Screens`: nur, wenn du in Phase 1 Screens gefunden hast. Tabelle
  `| Screen (Name/ID) | Feature-Flow | Ziel-Page/Component |` mit **allen** zum Feature gehörenden
  Frames und dem Pfad der `.pen`-Datei. Diese Screens sind die **verbindliche Design-Vorlage** für die
  Frontend-Umsetzung.
- `## Frontend-Umsetzung (Handoff)`: kurze Sektion, die dem späteren Umsetzungs-Agenten sagt, wie die
  UI gebaut wird. Die UI entsteht **erst nach** dem Backend (Schema, Service, API, DI aus
  `## Umsetzung`), mit Nuxt UI und den Brand-Tokens des Projekts.
  - Mit Pencil-Screens: Die UI folgt ausschließlich den Frames aus `## Pencil-Screens` und den
    Ziel-Pages/Components aus `## Umsetzung`. Der Agent liest das Pencil-Design nur und verändert es nie.
  - Ohne Pencil-Screens: Beschreibe je Page und Component Aufbau, Inhalte, Zustände (leer, Laden,
    Fehler) und Aktionen so genau, wie der Nutzer es in Phase 2 entschieden hat. Entfällt die UI nach
    der Entscheidung des Nutzers, steht hier genau das.
- `## Datenmodell`: neue/erweiterte Tabellen und Spalten. Konventionen: ULID-`id` (`text`),
  `snake_case` (DB) / `camelCase` (TS), `createdAt`/`updatedAt` mit Timezone, nötige Indizes,
  Relationen via `relations()`. Drizzle-Workflow: `pnpm db:generate` → `pnpm db:migrate`.
- `## Umsetzung (Feature-Anatomie)`: die konkreten Bausteine, **gemappt auf die Feature-Checkliste**
  der Architektur-Doku, Schritt für Schritt in ihrer Reihenfolge und mit echten Alias-Pfaden, z. B.:
  1. Schema in `server/database/schema/<name>.ts`
  2. ItemService-Instanz in `server/utils/container.ts`
  3. Feature-Service in `server/features/<name>/<name>.service.ts`
  4. Service im Container verdrahten
  5. API-Routen in `server/api/<name>/` (dünn, delegierend; `requireAuth`/`requirePermission`)
  6. Client-API in `app/features/<name>/api/<name>.api.ts`
  7. Query-Key-Factory in `app/features/<name>/api/<name>.keys.ts`
  8. Composables (`useQuery`/`useMutation`) in `app/features/<name>/composables/`
  9. Components in `app/features/<name>/components/`
  10. Barrel-Export `app/features/<name>/index.ts`
  11. Pages in `app/pages/` und Routen-Konstanten in `app/utils/routes.ts`

  Enthält die Checkliste des Projekts weitere Schritte, etwa Übersetzungen, übernimm sie.
- `## Technische Hinweise`: ItemService-Filteroperatoren (`_eq`, `_in`, `_ilike`, …); Pinia-Colada
  `useQuery`/`useMutation` inkl. **Cache-Invalidierung** (nach Create → Listen invalidieren; nach
  Update → Listen und Detail; nach Delete → Listen invalidieren und Detail clearen); Zod-v4-Validierung
  mit `.trim()`; Auth (`requireAuth`/Guard und Auth-Middleware unter `server/middleware/`); ggf. Event-Bus.
  Schließe mit **„Anti-Patterns vermeiden"** (Liste aus der Architektur-Doku, u. a.: keine
  Business-Logik in API-Routen, keine direkten DB-Queries in Routen, keine Cross-Feature-Imports am
  `index.ts` vorbei, keine hartkodierten Query-Keys, keine fehlende Cache-Invalidierung nach
  Mutationen).
- `## Erfolgskriterien`: messbare, überprüfbare Kriterien. Nenne die Prüfbefehle des Projekts.

> Es gibt **keine** `## Offene Fragen`-Sektion. Alles, was sonst dort landen würde, muss vorab in
> Phase 2 geklärt und als finale Entscheidung in `## Produktentscheidungen (final)` festgehalten
> werden.

### Phase 4: Speichern

Speichere das PRD als `prd/<bereich>/<feature-name>.md`:

- `<bereich>`: bestehenden Themen-Ordner unter `prd/` wiederverwenden, falls er passt; sonst einen
  neuen, sprechenden Ordner anlegen.
- `<feature-name>`: kebab-case, sprechend (Stil wie `mobile-bearer-token-auth`,
  `appearance-preferences`).
- Kündige den finalen Zielpfad an.

### Phase 5: Abschluss (KEINE Umsetzung starten)

Du endest mit dem gespeicherten PRD. Du startest **keinen** Agenten und implementierst **weder
Backend noch Frontend**. Nenne dem Nutzer den Zielpfad des PRDs. Die Umsetzung startet der Nutzer in
lazydev als eigenen Schritt.

## Abschluss-Anweisungen

1. Implementiere **nichts** selbst. Du erzeugst ausschließlich das PRD-Dokument.
2. Stelle in jedem Fall zuerst die Klärungsfragen und warte auf die Antwort.
3. Verfeinere das PRD anhand der Nutzerantworten.
4. Triff offene oder optionale Entscheidungen **nie** selbst. Frage nach und warte. Das fertige PRD
   enthält ausschließlich finale Entscheidungen.
5. Gibt es Pencil-Screens zum Feature, trage sie in `## Pencil-Screens` vollständig ein.
