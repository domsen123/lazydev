# lazydev

> Status: umgesetzt (2026-10-20) · Meilensteine 1 bis 9

lazydev ist eine Terminal-Oberfläche (TUI) im Stil von lazydocker und lazygit. Sie zeigt alle Projekte und
deren PRDs auf einen Blick und steuert den PRD-Workflow: PRD erstellen, umsetzen, mit `/arewedone` prüfen,
committen und in den Ziel-Branch mergen, Standard `main`. Jedes PRD läuft in einem eigenen Git-Worktree,
sodass mehrere PRDs pro Projekt parallel laufen. lazydev ersetzt das Experiment mit OpenRig durch ein schlankes, eigenes Werkzeug.

## Ziel

- Du fügst Projektordner hinzu und klickst zwischen ihnen hin und her.
- Pro Projekt siehst du jedes PRD mit Phase, Branch und laufendem Log.
- Mit einer Taste startest du ein neues PRD-Interview oder die Umsetzung eines fertigen PRDs.
- Mehrere PRDs laufen parallel, jedes in einem eigenen Worktree mit eigener Datenbank und eigenem Port.
- Nach erfolgreichem Review mergt lazydev den Branch in den Ziel-Branch, nacheinander. Konflikte
  löst ein eigener Claude-Schritt. Danach räumt lazydev Worktree, Branch und Datenbank auf.

## Nicht-Ziele

- Keine dauerhaften Agenten-Teams, keine Queue zwischen Agenten, kein Daemon.
- Keine Wiederaufnahme laufender Schritte nach einem Neustart des Rechners. Ein abgebrochener Schritt wird
  als „abgebrochen“ markiert und lässt sich neu starten. Der Worktree bleibt erhalten.
- Kein `git push`, kein Pull Request. Gemergt wird nur lokal in den Ziel-Branch.
- Kein Release von einem Ziel-Branch wie `next` nach `main`. Das bleibt Handarbeit.

## Ausgangslage: eingebettete Prompts und Agenten

lazydev bringt alle Prompts und Agenten per `go:embed` im Binary mit. Nichts kommt aus `~/.claude`, deshalb
läuft lazydev auf jedem Rechner gleich, und Änderungen in `~/.claude` wirken nicht auf Chats und Runner.

| Baustein | Pfad im Repo | Zweck |
|---|---|---|
| Erstellen (`nuxt`) | `internal/prdtemplate/nuxt/create.md` | Interaktives Interview, schreibt `prd/<bereich>/<name>.md`. Liest Checkliste und Anti-Patterns aus der `_ARCHITECTURE.md` des Projekts. |
| Umsetzen (`nuxt`) | `internal/prdtemplate/nuxt/implement.md` | siehe „Umsetzen (`nuxt`)“ |
| Erstellen, Umsetzen (`einfach`) | `internal/prdtemplate/einfach/` | siehe „PRD-Templates“ |
| `/arewedone` | `internal/runner/prompts/arewedone.md` | Startet den Agenten `structural-completeness-reviewer`, behebt Befunde, committet über `committer`. |
| `/merge-prd` | `internal/runner/prompts/merge.md` | siehe „Merge-Prompt“ |
| `committer` | `internal/claude/agents/committer.md` | Conventional Commit mit `-m "<betreff>" -m "<body>"`, ohne `Co-Authored-By` und ohne jede Erwähnung von Claude oder KI. Sprache und Stil wie in `git log`. |
| `structural-completeness-reviewer` | `internal/claude/agents/structural-completeness-reviewer.md` | Strukturelles Review nach einer Änderung. |

Die Prompts haben keine Frontmatter und nutzen `$ARGUMENTS` als Platzhalter. Ihre erste Zeile erscheint im
Log, z. B. `# /arewedone: Review und Commit` oder `# /merge-prd feat/<prd-name>`. Die Namen `/arewedone`
und `/merge-prd` bezeichnen in dieser Spezifikation die Schritte, nicht globale Commands.

Die Agenten schreibt lazydev vor jedem Aufruf von Claude als JSON nach `agents.json` im
Zustandsverzeichnis (Frontmatter `description`, `tools`, `model`, Inhalt als `prompt`). Jeder Aufruf
bekommt `--agents <datei>`, auch die Chats.

Projekte folgen meist der Nuxt-4-Vertical-Slice-Architektur aus dem Boilerplate
`~/development/nuxt/nuxt-drizzle`: PostgreSQL in Docker, Drizzle-Migrationen unter
`server/database/migrations/`, `.env` und `.env.test` mit `DATABASE_URL`. Jedes Projekt hat eigene
`CLAUDE.md` und `_ARCHITECTURE.md`.

### PRD-Format

Ein PRD liegt unter `prd/<bereich>/<name>.md`. Die zweite Zeile trägt den Status:

```
# PRD: <Titel>
> Status: geplant · Scope: <Kurzfassung>
```

Bekannte Statuswerte: `geplant`, `umgesetzt (YYYY-MM-DD)`. Der Teil nach `·` ist Freitext.

Vor dem Fast-Forward schreibt lazydev die Felder `Kosten:` und `Modell:` in den Freitext, direkt nach
dem Statuswert und vor allen anderen Teilen, getrennt mit ` · `:

```
> Status: umgesetzt (2026-10-21) · Kosten: 4.12 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: …
```

- **`Kosten:`** Summe aller Schritte des Laufs zum Zeitpunkt des Schreibens, inklusive gescheiterter
  Versuche und eines `/merge-prd`. Zwei Nachkommastellen mit Punkt. Die Kosten des Chats, in dem das
  PRD entstand, zählen nicht mit.
- **`Modell:`** volle ID aus dem ersten `system`/`init`-Ereignis des letzten erfolgreichen Schritts
  `implement`. Fehlt sie, entfällt das Feld.

Die Felder stehen immer in der ersten Zeile der Statuszeile. Ein Betrag, der sich nicht als Zahl lesen
lässt, gilt als nicht vorhanden.

### PRD-Templates

Ein Template legt fest, wie lazydev ein PRD im Chat erstellt und wie der Runner es umsetzt. Review und
Merge sind für alle Templates gleich: `/arewedone` und `/merge-prd`.

| Template | Erstellen | Umsetzen | Für |
|---|---|---|---|
| Nuxt-Feature (`nuxt`) | `nuxt/create.md` | `nuxt/implement.md` | Features mit Datenmodell, Feature-Checkliste und UI-Handoff |
| Einfach (`einfach`) | `einfach/create.md` | `einfach/implement.md` | Projekte und Änderungen ohne Datenmodell, UI-Handoff und Architektur-Doku |

Die Prompts liegen unter `internal/prdtemplate/<template>/`. `$ARGUMENTS` steht für die Beschreibung bzw.
den PRD-Pfad.

Ein PRD trägt das Template im Freitext der Statuszeile:

```
# PRD: <Titel>
> Status: geplant · Template: einfach · Scope: <Kurzfassung>
```

Ein PRD aus „Einfach“ hat die Abschnitte `## Context`, `## Produktentscheidungen (final)`, `## Scope`,
`## Umsetzung`, `## Technische Hinweise` und `## Erfolgskriterien`. Ohne Angabe `Template:` gilt `nuxt`.

Pencil ist beim Template `nuxt` optional. Liegt eine `.pen`-Datei im Projekt und ist der MCP-Server
`pencil` verfügbar, trägt das PRD die Screens unter `## Pencil-Screens` ein. Sonst beschreibt
`## Frontend-Umsetzung (Handoff)` die UI als Text. Claude liest das Design nur und verändert es nie.

Der Vertrag zum Runner gilt für jedes Template: die Statuszeile und beim Umsetzen ein Bericht mit
`## Ergebnis` (`umgesetzt` oder `abgebrochen`) und `## Offene Fragen`.

## Umsetzen (`nuxt`) und Merge

### Umsetzen (`nuxt`)

1. Liest das PRD vollständig, dazu `CLAUDE.md` und `_ARCHITECTURE.md` des Projekts.
2. Arbeitet nur auf dem Branch `feat/<prd-name>`. Heißt der aktuelle Branch anders, bricht er ab.
   `<prd-name>` ist der Dateiname ohne `.md`.
3. Setzt `## Datenmodell` und `## Umsetzung (Feature-Anatomie)` in der Reihenfolge der Checkliste um,
   die UI erst nach dem Backend. Nennt das PRD Pencil-Screens, liest er sie über `pencil`. Fehlen
   `pencil` oder ein Frame, bricht er ab.
4. Trifft keine Entscheidung, die das PRD offenlässt. Fehlt eine Entscheidung, bricht er ab und nennt
   die Frage im Abschlussbericht.
5. Führt die Prüfbefehle des Projekts aus (aus `CLAUDE.md`, z. B. `pnpm lint`, `pnpm typecheck`,
   `pnpm test:run`) und behebt Fehler. Nie den Watch-Modus wie `pnpm test` starten.
6. Setzt die Statuszeile des PRDs auf `umgesetzt (YYYY-MM-DD)`.
7. Committet **nicht**. Das übernimmt `/arewedone`. Erlaubt ist nur `git mv`, wenn das PRD eine Datei
   verschiebt.
8. Schreibt am Ende einen Bericht mit festen Abschnitten: `## Ergebnis` (`umgesetzt` oder
   `abgebrochen`), `## Geänderte Dateien`, `## Prüfungen`, `## Offene Fragen` und `## Nicht geprüft`.
   Offene Fragen blockieren und führen zu `abgebrochen`. Unter „Nicht geprüft“ stehen Kriterien, die
   einen Dev-Server oder Browser brauchen, etwa der Abgleich mit Pencil. Sie blockieren nicht.

Der Runner wertet den Abschnitt `## Ergebnis` aus. Steht dort `abgebrochen`, geht das PRD auf „Fehler“,
auch bei Exit-Code 0.

### Merge-Prompt (`/merge-prd <branch> [fehlerkontext]`)

Löst Konflikte, wenn `feat/<prd-name>` sich nicht konfliktfrei auf den Ziel-Branch bringen lässt, oder behebt eine
fehlgeschlagene Prüfung nach dem Rebase. Läuft im Worktree.

1. Liest das PRD des Branches und `git log <ziel-branch>..HEAD`, um die Absicht der Änderung zu verstehen.
2. Führt `git rebase <ziel-branch>` fort oder startet es und löst jeden Konflikt so, dass beide Seiten erhalten
   bleiben.
3. **Drizzle-Migrationen:** Kollidieren Migrationen (`server/database/migrations/*.sql`,
   `meta/_journal.json`, `meta/*_snapshot.json`), verwirft er die Migrationen des Branches, übernimmt die
   des Ziel-Branches und erzeugt die Migration des Branches neu mit `pnpm db:generate`. Migrationen werden nie
   von Hand zusammengeführt.
4. Führt die Prüfbefehle des Projekts aus und behebt Fehler.
5. Committet Korrekturen über `committer`. Kein Push.
6. Kann er einen Konflikt nicht eindeutig lösen, bricht er ab: `git rebase --abort`, Bericht mit Datei
   und Frage.
7. Bericht mit `## Ergebnis` (`bereit für Fast-Forward` oder `abgebrochen`), `## Konflikte`,
   `## Prüfungen` und `## Offene Punkte`.

## Bedienung

### Layout

```
┌ Projekte ─────────────┐┌ nuxt-drizzle ──────────────────────────────────────────────┐
│> nuxt-drizzle   ▲ 3● ││  PRD               Phase          Port  Branch    Modell    │
│  shop           ▲ 1● ││> auth/mobile-bear… ● Umsetzung    3101  feat/mob… Opus 5.5  │
│  blog           ▲    ││  profile/avatar-u… ● Review       3102  feat/ava… Opus 5.5  │
│  docs           △    ││  billing/invoice-… ◐ Merge-Queue  3103  feat/inv… Haiku 5.5 │
│                       ││  auth/oauth-micro… ○ geplant         –  –                   │
│                       ││────────────────────────────────────────────────────────────│
│                       ││   Docker   Status                Ports        Image         │
│                       ││ ● postgres Up 2 hours (healthy)  :5435        postgres:16   │
│                       ││ ● mailpit  Up 2 hours            :8025 :1025  axllent/mail… │
│                       ││ ○ redis    gestoppt                           –             │
│                       ││ dev    ● main :3000   ● mobile-bearer :3101                │
└───────────────────────┘└────────────────────────────────────────────────────────────┘
┌ Log: auth/mobile-bearer-token ─────────────────────────────────────────────────────────┐
│ 14:02 Edit server/database/schema/auth-tokens.ts                                        │
│ 14:05 Bash pnpm typecheck ✓                                                             │
└─────────────────────────────────────────────────────────────────────────────────────────┘
 a Projekt · n neues PRD · s starten · m mergen · x abbrechen · o öffnen · p dev · D docker · ? · q
```

- **Links:** Projekte. `▲` heißt: alle Container des Projekts laufen, `△`: einige oder keine laufen,
  kein Symbol: das Projekt hat kein Docker-Setup. Die Zahl mit `●` zeigt laufende PRDs.
- **Rechts unten:** Docker-Block und Dev-Server-Zeile des gewählten Projekts (siehe „Docker-Status“).
- **Rechts:** PRDs des gewählten Projekts mit Phase, Port des Worktrees, Branch, Modell und Kosten aller
  Schritte. Die Spalte `Modell` (Breite 9) steht zwischen `Branch` und `Kosten`. Sie zeigt das Modell
  des letzten erfolgreichen `implement` aus dem Lauf-Zustand, sonst das Feld `Modell:` aus der
  Statuszeile, sonst nichts. Die Kurzform lässt `claude-` am Anfang weg, schreibt den ersten Teil groß
  und verbindet die restlichen Teile mit `.`: `claude-opus-5-5` wird `Opus 5.5`. Eine ID ohne `claude-`
  erscheint unverändert, gekürzt auf die Spaltenbreite mit `…`. Die Kosten kommen aus dem Lauf-Zustand,
  wenn er Kosten über 0 hat, sonst aus dem Feld `Kosten:` der Statuszeile (Format `4.12 $`).
  Der Titel nennt die Summe der Kosten aller PRDs des Projekts nach derselben Regel, kein PRD zählt
  doppelt.
  Oben stehen die offenen PRDs, darunter die Gruppenzeile `▸ <n> erledigt` mit allen gemergten PRDs.
  In der Kostenspalte zeigt sie die Summe der Kosten aller erledigten PRDs nach derselben Regel.
  Die Gruppe ist beim Start zugeklappt. `e` oder `Enter` auf der Gruppenzeile klappt sie auf und zu.
  `S` sortiert beide Teile nach Name oder nach Erstellzeit, neueste zuerst. Die Spaltenüberschrift
  nennt die aktuelle Sortierung. Beide Einstellungen gelten bis zum Beenden der TUI.
  Die Erstellzeit ist der frühere Zeitpunkt aus der Erstellzeit der Datei (`statx`) und dem ersten
  Commit, der die Datei anlegt. Ohne Erstellzeit im Dateisystem nimmt lazydev die Änderungszeit.
- **Ganz rechts:** rechte Spalte mit den Boxen „Benachrichtigungen“ oben und „Issues“ darunter. Die Spalte
  ist so breit wie die Projektbox, `min(max(24, Breite/4), 36)`, die PRD-Box wird entsprechend schmaler.
  Sind beide Boxen sichtbar, bekommt „Benachrichtigungen“ die halbe Höhe des oberen Bereichs, abgerundet,
  und „Issues“ den Rest. Ist nur eine sichtbar, nimmt sie die volle Höhe. Ohne beide entfällt die Spalte.
  Klicks und Mausrad über der Spalte bewirken nichts.
- **Box „Benachrichtigungen · <anzahl>“:** die Chats des gewählten Projekts, die eine Rückfrage stellen
  oder deren Runde scheiterte (siehe „Neues PRD (`n`)“). Sie erscheint nur, wenn das gewählte Projekt
  mindestens eine Benachrichtigung hat. Eine Zeile pro Chat, die älteste oben: `?` für eine Rückfrage, `✗`
  für einen Fehler, dann die Beschreibung aus „Neues PRD“. Laufende und beendete Chats erscheinen nicht.
- **Box „Issues · <anzahl>“:** die offenen GitHub-Issues des gewählten Projekts (siehe „GitHub-Issues“). Sie
  erscheint nur, wenn das Projekt einen GitHub-`origin` und mindestens ein offenes Issue hat. Eine Zeile pro
  Issue, das neueste oben: `#<nummer>` in `colorMuted`, dann der Titel, gekürzt mit `…`.
- **Indikator am Projekt:** ` <anzahl>✉` hinter dem Projektnamen, nach Docker-Symbol und `<n>●`, zählt die
  Benachrichtigungen. Danach zählt ` <anzahl>#` in `colorMuted` die offenen Issues. Ohne Benachrichtigung
  oder ohne offene Issues entfällt der jeweilige Indikator.
- **Unten:** Log des gewählten PRDs, live, scrollbar.

**Fenster über der TUI:** Chat, Hilfe, Claude-Limits, „Template wählen“, „Neues PRD · <Template>“,
„PRD: …“ und „Issue #<nummer>“ erscheinen als Fenster mit rundem Rahmen über der TUI. Innen haben sie
2 Spalten Abstand links und rechts und 1 Zeile oben und unten. Hinter einem offenen Fenster erscheint die
TUI ohne Farben in `colorMuted`. Die Fußzeile bleibt ungedimmt und zeigt die Tastenhilfe.

### Tasten

Die Fußzeile zeigt nur die Tasten, die im aktiven Fokus und im Zustand des gewählten PRDs wirken. Vorn
stehen immer `? Hilfe` und `q beenden`, bei schmalem Terminal kürzt sich die Zeile am Ende mit `…`.

- **Projekte:** `a`, mit gewähltem Projekt dazu `d`, `D` und `n`.
- **PRDs:** `n`, mit gewähltem PRD dazu `v` und die Aktionen, die gerade greifen. `s` heißt „starten“ ohne
  Lauf und „fortsetzen“ mit Lauf. `m`, `x` und `X` erscheinen unter denselben Bedingungen, unter denen die
  Taste etwas tut. `o` erscheint mit Worktree und Claude-Sitzung, `p` und `w` mit Worktree. `e` erscheint
  mit erledigten PRDs, `S` ab zwei Zeilen in der Liste.
- **Benachrichtigungen:** `j/k wählen · Enter öffnen · X beenden`.
- **Issues:** `j/k wählen · Enter ansehen · o Browser · n PRD`.
- **Log:** `j/k scrollen · Enter Ereignis · G Ende · l Log groß`.
- In „Projekte“ und „PRDs“ folgt `N Rückfragen`, solange ein Projekt eine Benachrichtigung hat.

Alle Tasten wirken weiter aus jedem Fokus, auch wenn die Fußzeile sie nicht zeigt. Greift eine Taste
nicht, nennt die Statuszeile den Grund. `?` zeigt die vollständige Liste.

| Taste | Wirkung |
|---|---|
| `↑` `↓` / `j` `k` | Auswahl in der aktiven Liste |
| `Tab` / `←` `→` | Wechsel zwischen Projekten, PRDs, Benachrichtigungen, Issues und unterem Bereich (Log oder Details). Unsichtbare Boxen „Benachrichtigungen“ und „Issues“ überspringt der Fokus. |
| `a` | Projekt hinzufügen: Pfad eingeben, mit Tab-Vervollständigung |
| `d` | Projekt aus der Liste entfernen (löscht keine Dateien), mit Rückfrage |
| `b` | Ziel-Branch des Projekts aus den lokalen Branches wählen, ohne `feat/*`. Siehe „Merge“. |
| `n` | Neues PRD: Template wählen, Beschreibung eingeben, öffnet ein Chat-Fenster mit Claude |
| `N` | Älteste Benachrichtigung über alle Projekte öffnen. Wählt ihr Projekt und setzt den Cursor der Box darauf. Ohne Benachrichtigung zeigt die Statuszeile `Keine offenen Rückfragen`. |
| `Enter` / `X` | In „Benachrichtigungen“: Chat öffnen / Chat beenden, mit Rückfrage. Alle übrigen Tasten wirken wie sonst. |
| `Enter` / `o` / `n` | In „Issues“: Issue im Fenster ansehen / im Browser öffnen / PRD-Chat aus dem Issue starten. Alle übrigen Tasten wirken wie sonst. |
| `Ctrl+X` | Im Chat: Chat beenden, mit Rückfrage. Eine laufende Runde bricht ab, das Log bleibt. |
| `Ctrl+R` | Im Chat: nach einem Fehler die letzte Runde mit derselben Eingabe wiederholen |
| `v` | PRD vollständig als Markdown im Fenster ansehen. Darin öffnet `o` die Datei im Editor. |
| `s` | Umsetzung des gewählten PRDs starten, mit Rückfrage. Nach Fehler oder Abbruch geht es beim gescheiterten Schritt weiter. |
| `m` | Merge des gewählten PRDs anstoßen (nur bei `autoMerge: false` oder nach einem Fehler) |
| `x` | Laufenden Schritt abbrechen, mit Rückfrage |
| `X` | PRD verwerfen: Worktree, Branch und Datenbank löschen, PRD zurück in den Hauptordner verschieben. Doppelte Rückfrage. |
| `o` | Letzte Claude-Sitzung des PRDs interaktiv öffnen (`claude --resume <id>` im Worktree) |
| `p` | Dev-Server im Worktree auf dessen Port starten oder stoppen |
| `w` | Shell im Worktree öffnen |
| `e` | Erledigte PRDs auf- und zuklappen, ebenso `Enter` auf der Gruppenzeile |
| `S` | PRDs nach Name oder Erstellzeit sortieren |
| `D` | Docker-Ansicht des Projekts: Container mit Status, Ports, Image und Laufzeit |
| `U` / `S` | In der Docker-Ansicht: `docker compose up -d` / `docker compose stop` im Hauptordner, mit Rückfrage |
| `L` | In der Docker-Ansicht: Logs des gewählten Containers (`docker logs -f --tail 200`) |
| `l` | Log in voller Höhe anzeigen und wieder verkleinern |
| `G` | Im Log: ans Ende springen und neuen Zeilen folgen |
| `Enter` | Im Log: vollständiges Ereignis der Zeile anzeigen |
| `r` | Projekte und PRDs neu einlesen, Issues aller Projekte neu laden |
| `u` | Claude-Limits und Verbrauch anzeigen, darin `r` zum Aktualisieren |
| `?` | Hilfe |
| `q` | Beenden. Laufende Schritte und Chat-Runden arbeiten weiter (siehe „Prozesse“). |

`o` und `w` öffnen in Herdr einen neuen Pane rechts und setzen den Fokus dorthin. Ohne Herdr unterbricht
lazydev seine Oberfläche und startet den Befehl im selben Terminal, wie lazygit beim Editor.

`p` startet den Dev-Server dagegen immer losgelöst im Hintergrund (`setsid`), auch in Herdr. So übersteht
er einen Neustart der TUI, und der Merge kann ihn vor dem Rebase stoppen. Die PID steht in
`<prd-name>.devpid` mit dem Worktree-Ordner, die Ausgabe in `<prd-name>.dev.log`. Die Kopfzeile des Logs
zeigt `dev :<port>`, solange er läuft. Den Befehl liefert das Preset (`pnpm dev`) oder `hooks.dev`.
Ist der Port belegt, startet lazydev den Server nicht.

### GitHub-Issues

lazydev zeigt die offenen Issues eines Projekts, dessen Remote `origin` auf `github.com` zeigt. Aus einem
Issue startet `n` den PRD-Chat.

- **Remote:** Nur `origin` zählt (`git remote get-url origin` im Hauptordner). Erkannte Formen:
  `https://github.com/<owner>/<repo>`, `git@github.com:<owner>/<repo>` und
  `ssh://git@github.com/<owner>/<repo>`, jeweils mit oder ohne `.git` und abschließenden `/`. Andere
  Hosts, andere Remotes und Projekte ohne `origin` haben keine Issues. lazydev ruft für sie `gh` nicht auf.
- **Abruf:** Nur über das CLI `gh`, ohne REST-API und ohne Token in der Konfiguration:
  `gh issue list --repo <owner>/<repo> --state open --limit 30 --json number,title,body,url,createdAt` im
  Hauptordner. lazydev zeigt bis zu 30 Issues, die neuesten zuerst nach `createdAt`.
- **Zeitpunkte:** beim ersten Laden eines Projekts (Start der TUI oder neues Projekt mit `a`), alle 30
  Minuten für alle Projekte mit GitHub-`origin` und mit `r` für alle sofort. Bei `r` und alle 30 Minuten
  liest lazydev `origin` neu. Ein Projekt, dessen Abruf noch läuft, startet keinen zweiten. Der Tick alle
  2 Sekunden ruft `gh` nicht auf.
- **Zwischenspeicher:** Nach jedem erfolgreichen Abruf schreibt lazydev `repo`, `fetchedAt` und `issues`
  atomar nach `~/.local/state/lazydev/issues/<projekt-id>.json`. Beim ersten Laden zeigt lazydev diesen
  Stand sofort, wenn `repo` zum aktuellen `origin` passt.
- **Fehler:** Fehlt `gh`, ist es nicht angemeldet oder endet es mit Fehler, zeigt die Statuszeile
  `Issues von <projekt> nicht geladen: <erste Zeile der Fehlermeldung>` als Fehler. Der bisherige Stand und
  die Datei bleiben unverändert. Die Box zeigt keine Fehler.
- **Fokus:** Die Box „Issues“ liegt in der Reihenfolge Projekte → PRDs → Benachrichtigungen → Issues → Log.
  Verschwindet sie mit Fokus, geht der Fokus auf die PRDs. Mit Fokus auf der Box zeigt die Fußzeile
  `j/k wählen · Enter ansehen · o Browser · n PRD`.
- **Fenster „Issue #<nummer>“:** `Enter` öffnet das Issue über der TUI wie „PRD: …“. Oben stehen die Zeilen
  `Titel`, `Erstellt` (`2006-01-02 15:04`, lokale Zeit) und `URL`, nach einer Leerzeile der Text als
  Markdown. Ohne Text steht dort `Kein Text`. `j` `k` scrollen, `PgUp` `PgDn` blättern seitenweise, `g` `G`
  springen an Anfang und Ende, `o` öffnet den Browser, `n` den PRD-Chat, `Esc` und `q` schließen. Der Inhalt
  ändert sich nach dem Öffnen nicht.
- **Browser:** `o` ruft `gh issue view <nummer> --repo <owner>/<repo> --web` im Hintergrund auf. Die
  Statuszeile meldet `Issue #<nummer> im Browser geöffnet`, bei Fehler die erste Zeile der Fehlermeldung.
- **PRD aus Issue:** `n` öffnet „Template wählen“ mit dem Template des Presets vorgewählt. `Enter` startet
  den Chat sofort, ohne Fenster „Neues PRD“. `Esc` bricht ab und verwirft den Bezug zum Issue. Die erste
  Nachricht ist `#<nummer> <titel>`, eine Leerzeile und der Text des Issues ohne führende und abschließende
  Leerzeichen, ohne Text nur die erste Zeile. Die Box „Benachrichtigungen“ zeigt als Beschreibung nur
  `#<nummer> <titel>`. Der Titel des Chats lautet wie bisher `Neues PRD (<Template>): <Projekt>`.
- lazydev schließt, kommentiert oder bearbeitet keine Issues.

## Docker-Status

lazydev zeigt pro Projekt, welche Container laufen und auf welchen Ports. Dazu kommen die Dev-Server, die
lazydev selbst gestartet hat.

### Container finden

Ein Container gehört zu einem Projekt, wenn sein Label `com.docker.compose.project.working_dir` gleich
dem Projektpfad ist. Das Label setzt Docker Compose selbst, der Compose-Projektname spielt keine Rolle.
Beispiel aus dem aktuellen System:

```
nuxt-drizzle-postgres-1   working_dir=/home/user/development/nuxt/nuxt-drizzle   0.0.0.0:5435->5432/tcp
```

Abfrage: `docker ps -a --format json` mit Labels, Status und Ports. lazydev wertet nur Container aus, deren
`working_dir` zu einem eingetragenen Projekt passt. Hat ein Projekt eine `docker-compose.yml`, aber keinen
Container, zeigt lazydev die Services aus `docker compose config --services` als `○` gestoppt.

### Anzeige

**Projektliste:** ein Symbol pro Projekt, `▲` alle Services laufen, `△` einige oder keine.

**Docker-Block** am unteren Rand der PRD-Box, eine Zeile pro Service:

```
  Docker   Status                Ports        Image            Container
● postgres Up 2 hours (healthy)  :5435        postgres:16      nuxt-drizzle-postgres-1
○ redis    gestoppt                           –                –
```

- `●` läuft, `○` gestoppt, `✗` beendet mit Fehler oder `unhealthy`.
- Pro Service der Compose-Servicename, der Status aus `docker ps`, die Host-Ports ohne `0.0.0.0` und
  ohne IPv6-Duplikate, das Image und der Containername.
- Passt eine Spalte nicht in die Breite, lässt lazydev sie von rechts weg: zuerst Container, dann Image.
- Bleiben über dem Block weniger als drei PRD-Zeilen, zeigt lazydev stattdessen die kurze Docker-Zeile:

```
docker ● postgres :5435   ● mailpit :8025 :1025   ○ redis
```

  Passt auch diese Zeile nicht in die Breite, kürzt lazydev auf „docker ● 3/4 · :5435 :8025 …“.

**Dev-Zeile** unter dem Docker-Block, falls Dev-Server laufen:

```
dev    ● main :3000   ● mobile-bearer :3101
```

- `main` ist ein Dev-Server im Hauptordner, erkannt am lauschenden Port aus der `.env` des Projekts
  (`PORT`, Standard 3000) und dem Prozess-Arbeitsordner.
- Die übrigen Einträge sind Dev-Server aus `p` in Worktrees.

**Worktree-Datenbanken** liegen im selben Postgres-Container wie die Haupt-Datenbank. Die Docker-Ansicht
`D` listet sie unter dem Postgres-Container auf, mit Name und zugehörigem PRD.

- Die Ansicht fragt dafür beim Öffnen jeden laufenden Container mit `postgres`-Image ab: `POSTGRES_USER`
  aus dem Container, dann `psql … SELECT datname FROM pg_database`. Ein Name gehört zu einem PRD, wenn er
  auf `_<slug>` eines Laufs des Projekts endet.
- Sie zeigt nur Container des Projekts selbst. Der Test-Container von nuxt-drizzle,
  `app-postgres-test`, gehört laut Label zu `example-app/web`. Die Test-Datenbanken der
  nuxt-drizzle-Worktrees erscheinen deshalb nicht in der Ansicht von nuxt-drizzle.
- Die Dev-Zeile findet den Prozess hinter einem Port über `/proc/net/tcp`, `/proc/net/tcp6` und die
  Socket-Links unter `/proc/<pid>/fd`. Ein Server gilt als `main`, wenn sein Arbeitsordner im Hauptordner
  liegt.
- `L` startet `docker logs -f` ohne Herdr im selben Terminal. lazydev fängt dabei `SIGINT` ab, damit
  `Ctrl+C` nur die Logs beendet und die TUI zurückkehrt. Dasselbe gilt für `o` und `w`.

### Aktualisierung

- `docker events --format json --filter type=container` als Hintergrundstrom. Bei jedem Ereignis liest
  lazydev den Status neu ein.
- Fällt der Strom aus oder fehlt Docker, prüft lazydev alle 10 Sekunden mit `docker ps`. Läuft der Docker-
  Daemon nicht, zeigt die Zeile `docker – nicht erreichbar`.

### Ports belegt

Vor dem Start eines Worktree-Dev-Servers prüft lazydev, ob der Port frei ist. Belegt ein fremder Prozess
den Port, zeigt lazydev Prozessname und PID und vergibt den nächsten freien Port.

## Ablauf eines PRDs

### Phasen

| Phase | Symbol | Bedeutung |
|---|---|---|
| geplant | `○` | PRD liegt im Hauptordner, Status `geplant`, kein Worktree |
| wartet | `◌` | Start angefordert, aber `maxParallel` ist erreicht |
| Setup | `●` | Worktree, Abhängigkeiten, Datenbank werden vorbereitet |
| Umsetzung | `●` | Umsetzungs-Prompt läuft |
| Review | `●` | `/arewedone` läuft |
| Merge-Queue | `◐` | Review fertig, wartet auf den Merge-Slot des Projekts |
| Merge | `●` | Rebase, ggf. `/merge-prd`, Prüfungen, Fast-Forward |
| gemergt | `✓` | Im Ziel-Branch, Worktree aufgeräumt |
| abgebrochen | `■` | Vom Menschen gestoppt oder Prozess beendet. Worktree bleibt. |
| Fehler | `✗` | Ein Schritt endete mit Fehler oder offener Frage. Worktree bleibt. |

Ohne Lauf-Zustand gilt die Statuszeile des PRDs: `geplant` → geplant, `umgesetzt` → gemergt.

### Neues PRD (`n`)

1. lazydev öffnet das Fenster „Template wählen“ mit den Templates untereinander und ihrer Beschreibung
   (siehe „PRD-Templates“). Vorgewählt ist „Nuxt-Feature“
   bei Preset `nuxt-drizzle`, sonst „Einfach“. `←` `→`, `h` `l`, `j` `k` oder `Tab` wechseln, `Enter` bestätigt,
   `Esc` bricht ab.
2. lazydev fragt im Fenster „Neues PRD · <Template>“ nach einer kurzen Beschreibung.
3. lazydev legt den Zustand des Chats an und öffnet ein Chat-Fenster über der TUI. Jede Runde läuft als
   losgelöster Prozess `lazydev chat <projekt-id> <chat-id>` (`setsid`), zwischen zwei Runden läuft kein
   Prozess. Die Chat-ID ist die Startzeit im Format `2006-01-02-150405`. Gibt es die Zustandsdatei schon,
   hängt lazydev `-2`, `-3` usw. an. Die erste Runde ist
   `claude -p "<prompt>"` im Hauptordner des Projekts mit dem Erstellen-Prompt des Templates und der
   Beschreibung. Der Chat zeigt nur die Beschreibung als erste Nachricht. Jede Antwort
   im Fenster läuft als weitere Runde mit `--resume <sitzung>`. Claude stellt seine Klärungsfragen als Text.
   Der Chat läuft immer mit Opus: lazydev übergibt `--model opus`.
   Nach jeder Runde setzt lazydev den Status des Chats:
   - `läuft`: Die Runde ist aktiv. Lebt der Prozess nicht oder gehört nicht zu `lazydev chat`, gilt der
     Chat als `fehler` mit dem Text `Runde abgebrochen`.
   - `fehler`: Fehler beim Start, Exit-Code ungleich 0, kein `result`-Ereignis oder `is_error` im letzten
     `result`.
   - `rückfrage`: Die Runde war fehlerfrei, und das Ergebnis enthält eine Rückfrage. Das gilt, wenn eine
     Zeile nach optionalen Leerzeichen, Listenzeichen (`-`, `*`, `+`, `>`) und `**` mit `A`, `B`, `C`
     oder `D` gefolgt von `:` oder `)` beginnt. Es gilt auch, wenn die letzte nicht leere Zeile ohne
     abschließende Leerzeichen, `*`, `_` und `` ` `` auf `?` endet. Einen Marker im Prompt gibt es nicht.
   - `beendet`: Die Runde war fehlerfrei, ohne Rückfrage.

   Chats mit `rückfrage` oder `fehler` erscheinen in der Box „Benachrichtigungen“. Eine Benachrichtigung gilt
   als gelesen, sobald du im Chat antwortest oder wiederholst. Das Öffnen allein ändert nichts. Einen
   beendeten Chat löscht die TUI beim Neueinlesen, wenn sein Fenster nicht offen ist.
4. Das Fenster baut den Verlauf aus dem Log auf und liest neue Zeilen nach. So zeigt es auch nach einem
   Neustart der TUI den vollständigen Verlauf. Es zeigt den Text von Claude vollständig, Tool-Aufrufe und
   Fehler als kurze Zeilen und über dem Eingabefeld „Antwort“ die Kosten. `Enter` sendet, während einer
   laufenden Runde ohne Wirkung. `Alt+Enter` oder `Ctrl+J` fügt eine Zeile ein, `PgUp`/`PgDn` scrollt.
   `Ctrl+R` startet nur bei Status `fehler` eine neue Runde mit Eingabe und Anzeige der letzten Runde.
   `Esc` blendet das Fenster aus, die Runde läuft weiter. Die Statuszeile zeigt dann
   `Chat läuft im Hintergrund weiter · Log: <pfad>`. Ist der Chat `beendet`, löscht lazydev seinen
   Zustand, und die Meldung nennt `claude --resume <sitzung>`, um im Terminal weiterzumachen.
   `Ctrl+X` fragt `Chat beenden? Eine laufende Runde bricht ab. (j/n)`. Nach `j` sendet lazydev `SIGTERM`
   an die Prozessgruppe einer laufenden Runde, löscht den Zustand, schließt das Fenster und meldet
   `Chat beendet. Fortsetzen: claude --resume <sitzung> · Log: <pfad>`. Nach `n` bleibt das Fenster offen.
   `X` in der Box wirkt genauso, nach `n` bleibt lazydev im Normalmodus.
5. Freigaben im Chat: `Read`, `Glob`, `Grep`, `Task`, `Edit(prd/**)`, für die Kontextbefehle
   `Bash(find:*)`, `Bash(ls:*)`, `Bash(git log:*)`, `Bash(git status:*)` und für Pencil
   `mcp__pencil__get_app_state`, `mcp__pencil__read_skill`, `mcp__pencil__execute`. Claude schreibt
   damit nur unter `prd/`. `execute` könnte das Design ändern, der Prompt verbietet das. Mit Haiku geprüft: Beide Runden liefen ohne Ablehnung, auch die Kontextbefehle
   mit Pipe.
6. Das PRD entsteht ungetrackt unter `prd/` im Hauptordner und erscheint über die Dateiüberwachung.
7. Jeder Chat schreibt seinen Zustand nach `~/.local/state/lazydev/chats/<projekt-id>/<chat-id>.json` und
   ein Log nach `~/.local/state/lazydev/chats/<projekt-id>/<chat-id>.jsonl`: jede Eingabe als
   `{"type":"user_prompt","text":…,"shown":…}`, jedes stream-json-Ereignis unverändert, Fehler einer Runde als
   `{"type":"lazydev","text":…}`, jede Zeile mit `ts`. Die Meldung beim Schließen nennt den Pfad. Das Log
   bleibt auch nach dem Beenden erhalten. Abgelehnte Befehle wie Brace-Expansion in Bash sieht der Chat als
   roten Fehler. Claude liest danach mit Read und Grep weiter.

### Umsetzung starten (`s`)

1. **Prüfen:** Läuft für dieses PRD schon ein Worktree, lehnt lazydev ab. Ist `maxParallel` erreicht,
   setzt lazydev das PRD auf „wartet“ und startet es, sobald ein Platz frei wird.
2. **Worktree anlegen:** `git worktree add -b feat/<prd-name> <worktree-pfad> <ziel-branch>`. Der Lauf
   merkt sich den Ziel-Branch im Feld `base`. Ändert sich `baseBranch` später, bleibt der Lauf dabei.
3. **PRD verschieben:** Ist das PRD im Hauptordner ungetrackt, **verschiebt** lazydev die Datei in den Worktree
   und committet sie dort als erster Commit `docs(prd): add <prd-name>`. Der Hauptordner bleibt sauber,
   und das PRD kommt mit dem Merge in den Ziel-Branch. Ist das PRD schon getrackt, entfällt der Schritt.
4. **Setup:** siehe „Isolation pro Worktree“.
5. **Umsetzung:** `claude -p "<prompt>"` im Worktree mit dem Umsetzungs-Prompt des Templates aus der
   Statuszeile und dem PRD-Pfad.
6. **Review:** nur nach Erfolg: `claude -p "<prompt>"` mit dem Prompt `/arewedone` im Worktree.
7. **Merge:** nach Erfolg in die Merge-Queue des Projekts (siehe „Merge“).

Commits, die lazydev selbst erstellt (Schritt 3 und „Kosten und Modell vermerken“ im Merge), schreibt es
direkt mit `git commit`, ohne Claude und ohne Trailer.

### Aufruf von Claude

```
claude -p "<prompt>" \
  --output-format stream-json --verbose \
  --permission-mode acceptEdits \
  --allowedTools "<allowlist>" \
  --agents <zustandsverzeichnis>/agents.json
```

- `--output-format stream-json --verbose` liefert eine JSON-Zeile pro Ereignis. lazydev liest daraus
  Tool-Aufrufe, Text und das Abschlussereignis mit `session_id`, Kosten und Fehlerstatus. Das Schema
  steht unter „Geprüfte Annahmen“.
- Die Allowlist erlaubt mindestens: `Read`, `Edit`, `Write`, `Glob`, `Grep`, `Task`,
  `Bash(git status:*)`, `Bash(git diff:*)`, `Bash(git log:*)`, `Bash(git add:*)`, `Bash(git commit:*)`,
  `Bash(git switch:*)`, `Bash(git rebase:*)`, `Bash(git checkout:*)`, `Bash(git rm:*)`, `Bash(git mv:*)`,
  `Bash(git show:*)`, `Bash(git branch:*)`, `Bash(pnpm lint:*)`,
  `Bash(pnpm typecheck:*)`, `Bash(pnpm test:run:*)`, `Bash(pnpm db:generate:*)`, `Bash(pnpm db:migrate:*)`,
  `mcp__pencil__get_app_state`, `mcp__pencil__read_skill`, `mcp__pencil__execute`.
  Sie ist pro Projekt in der Konfiguration erweiterbar.
- Alle Freigaben kommen über `--allowedTools`. Claude ignoriert `permissions.allow` aus
  `.claude/settings.json` in Ordnern ohne bestätigten Trust-Dialog, und jeder neue Worktree ist so ein
  Ordner.
- Verboten bleiben `git push`, `git reset --hard`, `git worktree`, `pnpm docker:clean` und alles außerhalb
  der Allowlist. Worktrees verwaltet nur lazydev.
- Der Runner startet Claude mit `stdin` auf `/dev/null` und mit `GIT_EDITOR=true`. Ohne `stdin` wartet
  Claude 3 Sekunden auf Eingabe. Ohne `GIT_EDITOR` öffnet `git rebase --continue` einen Editor.

Erfolg eines Schritts: Exit-Code 0 und das **letzte** `result`-Ereignis meldet keinen Fehler. Dazu prüft
der Runner:

- nach dem Umsetzungs-Prompt: `## Ergebnis` im Bericht ist `umgesetzt`, und die Statuszeile des PRDs im Worktree
  steht auf `umgesetzt`. Bei `abgebrochen` steht die erste offene Frage im Fehler.
- nach `/arewedone`: Der Worktree ist sauber, `committer` hat also alles committet.

 Startet
Claude Subagenten, laufen sie im Hintergrund. Der Stream enthält dann mehrere `result`-Ereignisse, eines
pro Runde. Bei Fehler stoppt das
PRD in Phase „Fehler“, und das Log zeigt den letzten Text von Claude. Mit `s` startest du den
fehlgeschlagenen Schritt neu, mit `o` öffnest du die Sitzung zum Nachfragen.

## Isolation pro Worktree

Parallele PRDs dürfen sich nicht gegenseitig stören. Pro Worktree vergibt lazydev:

| Ressource | Wert |
|---|---|
| Ordner | `~/.local/share/lazydev/worktrees/<projekt-id>/<prd-name>` |
| Branch | `feat/<prd-name>` |
| Port | erster freier Port ab `portBase` (Standard 3100), gemerkt im Lauf-Zustand |
| Slug | `<prd-name>` in `snake_case`, max. 40 Zeichen, für Datenbanknamen |

Das Setup läuft als Hook-Skript aus der Projekt-Konfiguration. lazydev übergibt die Werte als
Umgebungsvariablen: `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`,
`LAZYDEV_SLUG`.

**Standard-Setup für Projekte aus `nuxt-drizzle`** (liefert lazydev als eingebautes Preset `nuxt-drizzle`):

1. `.env` und `.env.test` aus dem Hauptordner in den Worktree kopieren.
2. In beiden Dateien `NUXT_DATABASE_NAME` um `_<slug>` ergänzen. In `.env` zusätzlich `PORT=<port>`
   setzen. nuxt-drizzle nutzt kein `DATABASE_URL`, sondern `NUXT_DATABASE_HOST`, `_PORT`, `_USER`,
   `_PASSWORD` und `_NAME`. Die Namen enthalten Bindestriche, z. B. `nuxt-starter`.
3. Die Datenbanken anlegen (`createdb`): die Dev-Datenbank im Postgres-Container des Projekts (Port aus
   `.env`), die Test-Datenbank im Test-Container (Port aus `.env.test`, z. B. `app-postgres-test`
   auf 5460). lazydev findet den Container über den veröffentlichten Host-Port und ruft `createdb` mit
   `docker exec` im Container auf. Auf dem Rechner braucht es keine Postgres-Programme. Existiert die
   Datenbank schon, bleibt sie stehen. Läuft ein Container nicht, startet das Setup ihn im Hauptordner mit
   `pnpm docker:up` bzw. `pnpm docker:test:up` und wartet bis zu 60 Sekunden auf `pg_isready`.
4. `pnpm install --frozen-lockfile --prefer-offline`.
5. `pnpm db:migrate` und `pnpm test:db:push`.

Hinweise zum Preset:

- `server/plugins/01.init.ts` migriert beim Start des Dev-Servers. Ein Dev-Server im Worktree darf
  deshalb nie auf die Haupt-Datenbank zeigen.
- Der Dev-Server schreibt seinen Port in die getrackten Dateien `.mcp.json` und `.vscode/mcp.json`. Der
  Worktree ist danach nicht mehr sauber, und `git rebase` bricht ab. Das Preset setzt beide Dateien vor
  jedem Claude-Schritt, nach dem Review und vor dem Merge mit `git checkout HEAD --` zurück.

**Standard-Teardown:** die beiden Datenbanken löschen (`dropdb --if-exists --force`). Gelöscht werden nur
Namen mit `_<slug>`, nie die Haupt-Datenbanken. Worktree und Branch entfernt lazydev selbst.

**Verwerfen ohne TUI:** `lazydev teardown [--yes] <projekt-id> <prd-name>` macht dasselbe wie `X`:
Datenbanken löschen, das PRD mit Status `geplant` ungetrackt zurück in den Hauptordner legen, falls es
dort fehlt, dann `git worktree remove --force`, `git branch -D` und Zustand und Logs löschen. Ohne `--yes`
fragt der Befehl nach. Er arbeitet auch ohne Lauf-Zustand, damit sich ein halb angelegter Lauf aufräumen
lässt.

## Merge

Pro Projekt gibt es **eine** Merge-Queue. Sie arbeitet die PRDs nacheinander in der Reihenfolge ab, in der
ihr Review fertig wurde. Bei `autoMerge: true` (Standard) startet der Merge automatisch, sonst mit `m` oder
`lazydev merge <projekt-id> <prd-name>`. Ohne Freigabe endet der Runner in Phase „Merge-Queue“, und der
Zustand trägt `mergeApproved: true`, sobald du freigibst.

Ziel-Branch: `baseBranch` des Projekts, Standard `main`. Von ihm zweigen Worktrees ab, in ihn geht der
Fast-Forward. Mit `b` wählst du ihn aus den lokalen Branches, lazydev schreibt ihn in `config.yaml`. `main`
steht dort nicht, weil es der Standard ist. Steht der Hauptordner nicht auf dem Ziel-Branch, zeigt der
Titel der PRD-Box `(Ziel <branch>)`. Die Prompts von Review und Merge bekommen den Ziel-Branch über
`$BASE`.

Ablauf pro PRD:

1. **Rebase im Worktree:** `git rebase <ziel-branch>`.
   - Ohne Konflikt weiter mit Schritt 2.
   - Mit Konflikt: `git rebase --abort`, dann Claude mit dem Prompt `/merge-prd feat/<prd-name>`. Scheitert der
     Schritt, geht das PRD auf „Fehler“, und die Queue arbeitet mit dem nächsten PRD weiter.
2. **Datenbank nachziehen:** Hat der Rebase Migrationen aus dem Ziel-Branch geholt, setzt lazydev die
   Worktree-Datenbanken zurück und migriert neu (Teil des Presets: `dropdb`, `createdb`, `pnpm db:migrate`).
3. **Prüfen:** Prüfbefehle des Projekts im Worktree. Das Preset `nuxt-drizzle` führt `pnpm lint`,
   `pnpm typecheck` und `pnpm test:run` aus, `hooks.check` ersetzt sie durch ein Skript. Schlägt eine
   Prüfung fehl, startet lazydev `/merge-prd` mit dem Fehler als Kontext. Scheitert auch das, geht das PRD
   auf „Fehler“. Nach jedem `/merge-prd` prüft lazydev selbst, dass der Branch auf dem Ziel-Branch steht und der
   Worktree sauber ist. Der Bericht allein reicht nicht.
4. **Kosten und Modell vermerken:** lazydev schreibt `Kosten:` und `Modell:` in die Statuszeile des PRDs
   im Worktree (siehe „PRD-Format“) und committet selbst mit
   `git commit -m "docs(prd): record cost of <prd-name>" -- <prd-pfad>`, ohne Claude und ohne Trailer.
   So zählt ein `/merge-prd` mit, und der Commit kommt mit dem Fast-Forward in den Ziel-Branch. Stehen
   die Felder schon da, ersetzt lazydev die Werte an ihrer Stelle. Ändert sich nichts, entsteht kein
   Commit. Die Prüfung läuft danach nicht erneut, der Commit ändert nur das PRD. Ist das PRD nicht lesbar
   oder hat es keine Statuszeile, scheitert der Merge mit „Kosten nicht im PRD vermerkt: <fehler>“. Das
   Log zeigt `Kosten im PRD vermerkt: 4.12 USD · claude-opus-5-5` und bei einem Commit
   `Commit: docs(prd): record cost of <prd-name>`.
5. **Hauptordner prüfen:** Der Hauptordner muss auf dem Ziel-Branch stehen und sauber sein. Ungetrackte Dateien
   unter `prd/` sind erlaubt, ebenso die vom Dev-Server geschriebenen `.mcp.json` und `.vscode/mcp.json`.
   Berührt der Merge diese Dateien, bricht der Fast-Forward ab. Sonst wartet das PRD in der Queue mit dem
   Hinweis „Hauptordner nicht sauber“ im Feld `note`, bis du aufräumst. lazydev prüft alle 5 Sekunden.
6. **Fast-Forward:** im Hauptordner `git merge --ff-only feat/<prd-name>`. Durch den Rebase ist das
   immer ein Fast-Forward. Es entstehen keine Merge-Commits.
7. **Datenbank im Hauptordner:** Brachte der Merge neue Migrationen, zeigt lazydev den Hinweis
   „`pnpm db:migrate` im Hauptordner ausführen“. lazydev migriert die Haupt-Datenbank nicht selbst.
8. **Aufräumen:** Teardown-Hook, `git worktree remove`, `git branch -d feat/<prd-name>`. Phase „gemergt“.
   Zustand und Logs bleiben, der Port ist wieder frei. Scheitert das Aufräumen, bleibt das PRD gemergt,
   und `note` nennt den Fehler.

Läuft während des Merges ein Dev-Server aus `p` im Worktree, stoppt lazydev ihn vorher.

Die Sperre ist `flock` auf `locks/<projekt-id>.merge.lock`. Ein PRD, das auf die Sperre oder den
Hauptordner wartet, steht in Phase „Merge-Queue“, mit der Sperre in Phase „Merge“. `lazydev stop`
beendet auch das Warten.

## Prozesse

- Jedes PRD läuft in einem eigenen Runner im Hintergrund: `lazydev run <projekt-id> <prd-name>`. Der
  Runner läuft losgelöst (`setsid`), schreibt Log und Zustand in Dateien und beendet sich nach dem Merge
  oder bei einem Fehler.
- `lazydev start <projekt-id> <prd-name>` startet den Runner losgelöst und beachtet `maxParallel`.
  stdout und stderr des Runners landen in `<prd-name>.runner.log`, auch bei einem Absturz.
  `lazydev run` startet ihn im Vordergrund, ohne `maxParallel`. `lazydev stop` sendet `SIGTERM`.
- Gibt es schon einen Lauf, machen `start` und `run` beim gescheiterten oder abgebrochenen Schritt
  weiter, sonst beim nächsten. Das Setup ist wiederholbar: Worktree, verschobenes PRD und Datenbanken
  bleiben stehen.
- Das Zählen der laufenden Runner und der Start neuer Runner laufen unter einer Sperre
  (`locks/start.lock`), damit `maxParallel` auch bei gleichzeitigen Starts hält.
- Die Merge-Queue ist eine Sperrdatei pro Projekt (`flock`). Nur der Runner mit der Sperre mergt. Die
  anderen warten in Phase „Merge-Queue“.
- `maxParallel` begrenzt die gleichzeitigen Runner über alle Projekte. Ein wartendes PRD startet, sobald
  ein Runner endet. Das übernimmt der Runner, der endet, nicht die TUI.
- Die TUI liest nur Dateien. Startest du lazydev neu, zeigt es laufende Runner wieder an.
- Ein Runner gilt als laufend, wenn seine PID lebt und zu `lazydev run` gehört. Sonst markiert lazydev
  das PRD als „abgebrochen“.
- `x` sendet `SIGTERM` an die Prozessgruppe des Runners.
- Jede Runde eines PRD-Chats läuft losgelöst (`setsid`) als `lazydev chat <projekt-id> <chat-id>`, Ausgabe
  nach `/dev/null`. Sie schreibt Zustand und Log in Dateien und übersteht das Beenden der TUI. Eine Runde
  gilt als laufend, wenn ihre PID lebt und `/proc/<pid>/cmdline` zu `lazydev chat` gehört. Jede
  Lese-Schreib-Folge auf den Zustand läuft unter der Sperre `locks/chat-<projekt-id>-<chat-id>.lock`.
  Die TUI hält die Sperre beim Start, bis die PID gespeichert ist. Die Runde wartet auf dieselbe Sperre.

## Datenhaltung

| Was | Pfad | Format |
|---|---|---|
| Konfiguration | `~/.config/lazydev/config.yaml` | YAML |
| Lauf-Zustand | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.json` | JSON |
| Logs | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.<schritt>.jsonl` | stream-json, jede Zeile um `ts` ergänzt |
| Runner-Ausgabe | `~/.local/state/lazydev/runs/<projekt-id>/<prd-name>.runner.log` | Text, Kurzfassung des Logs |
| Chat-Zustand | `~/.local/state/lazydev/chats/<projekt-id>/<chat-id>.json` | JSON |
| Chat-Log | `~/.local/state/lazydev/chats/<projekt-id>/<chat-id>.jsonl` | stream-json, jede Zeile um `ts` ergänzt |
| Issues | `~/.local/state/lazydev/issues/<projekt-id>.json` | JSON mit `repo`, `fetchedAt` und `issues` |
| Merge-Sperre | `~/.local/state/lazydev/locks/<projekt-id>.merge.lock` | `flock` |
| Worktrees | `~/.local/share/lazydev/worktrees/<projekt-id>/<prd-name>/` | Git-Worktree |

`config.yaml`:

```yaml
maxParallel: 3               # gleichzeitige Runner über alle Projekte
portBase: 3100
claude:
  binary: claude
projects:
  - path: ~/development/nuxt/nuxt-drizzle
    preset: nuxt-drizzle     # Setup, Teardown, Prüfbefehle, Env-Dateien
    autoMerge: true
  - path: ~/development/nuxt/phoenix
    preset: nuxt-drizzle
    baseBranch: next         # Ziel-Branch, Standard main
  - path: ~/development/nuxt/shop
    preset: nuxt-drizzle
    autoMerge: false
    allowedTools:            # ergänzt die Standard-Allowlist
      - "Bash(pnpm test:e2e:*)"
    hooks:                   # überschreibt einzelne Preset-Hooks: setup, teardown, check, dev
      setup: ./scripts/lazydev-setup.sh
```

Lauf-Zustand (Beispiel):

```json
{
  "prd": "prd/auth/mobile-bearer-token-auth.md",
  "branch": "feat/mobile-bearer-token-auth",
  "base": "main",
  "worktree": "/home/user/.local/share/lazydev/worktrees/nuxt-drizzle/mobile-bearer-token-auth",
  "port": 3101,
  "slug": "mobile_bearer_token_auth",
  "phase": "review",
  "pid": 123456,
  "startedAt": "2026-10-20T14:02:11+02:00",
  "steps": [
    { "name": "setup", "exitCode": 0, "ok": true, "endedAt": "…" },
    { "name": "implement", "sessionId": "…", "model": "claude-opus-5-5", "exitCode": 0, "ok": true, "costUsd": 1.42, "endedAt": "…" },
    { "name": "review", "sessionId": "…", "model": "claude-opus-5-5", "exitCode": null, "ok": false }
  ],
  "error": null
}
```

`ok` ist `false`, wenn ein Schritt trotz Exit-Code 0 scheiterte, etwa an einer offenen Frage. `model` ist
das Feld `model` aus dem ersten `system`/`init`-Ereignis von Claude im Schritt und fehlt bei Schritten
ohne Claude. Ein Schritt taucht nach einer Wiederholung mehrfach auf. In `phase` stehen Schlüssel wie
`review`, die TUI zeigt die deutschen Namen aus der Phasen-Tabelle. stderr von Claude steht im Log als
`{"type":"stderr","text":…}`.

Chat-Zustand (Beispiel). `prompt` und `shown` gehören zur laufenden oder letzten Runde, `shown` steht im
Verlauf statt eines langen eingebetteten Prompts:

```json
{
  "title": "Neues PRD (Einfach): nuxt-drizzle",
  "description": "Avatar-Upload im Profil",
  "dir": "/home/user/development/nuxt/nuxt-drizzle",
  "sessionId": "…",
  "costUsd": 0.42,
  "status": "rückfrage",
  "prompt": "…",
  "shown": "Avatar-Upload im Profil",
  "startedAt": "2026-10-20T14:02:11+02:00",
  "updatedAt": "2026-10-20T14:04:30+02:00"
}
```

`status` ist `läuft`, `rückfrage`, `fehler` oder `beendet`. `pid` steht nur während einer Runde, `error`
nur nach einem Fehler.

Die `<projekt-id>` ist der Ordnername. Bei Kollisionen hängt lazydev einen kurzen Hash des Pfads an.

## Claude-Limits

Jeder Aufruf von `claude -p` liefert im stream-json ein Ereignis `rate_limit_event` mit `status`,
`overageStatus` und `unifiedWindows`: pro Fenster (`five_hour`, `seven_day`) die Auslastung von 0 bis 1 und
`resetsAt` als Unix-Zeit. Runner und Chat speichern das neueste Ereignis in
`~/.local/state/lazydev/usage.json`.

- **Unten rechts:** `5h 11% · 7d 37%`. Ab 50 % gelb, ab 80 % rot. Ist der Stand älter als 15 Minuten,
  steht das Alter dahinter. Die TUI liest die Datei alle 2 Sekunden.
- **`u`:** Übersicht mit Balken, Reset-Zeit, Status und Extra-Nutzung. Dazu die Kosten der Läufe in lazydev,
  heute und gesamt, als rechnerischer API-Preis aus `total_cost_usd`.
- **`r` in der Übersicht:** `claude -p "Antworte nur mit OK." --model haiku` im Zustandsverzeichnis. Geprüft:
  rund 2 Sekunden, rechnerisch 0,004 USD. Andere Claude-Sitzungen außerhalb von lazydev aktualisieren den
  Stand nicht.

## Log-Darstellung

Aus dem stream-json zeigt lazydev eine kompakte Zeile pro Ereignis:

- Tool-Aufruf: `HH:MM <Tool> <wichtigstes Argument>`, z. B. `Edit server/api/auth/token.post.ts` oder
  `Bash pnpm typecheck`.
- Tool-Ergebnis mit Fehler: rot markiert, erste Zeile der Ausgabe.
- Text von Claude: erste Zeile, gekürzt auf die Breite.
- Schrittwechsel und Abschluss: Dauer, Kosten, Erfolg oder Fehler.
- Ausgaben von Hooks und Git-Befehlen des Runners erscheinen ebenfalls, mit Präfix `lazydev`.

`l` zeigt das Log in voller Höhe. Mit `Enter` auf einer Zeile siehst du das vollständige Ereignis.

- stream-json enthält keine Zeit. Der Runner setzt deshalb in jede Zeile das Feld `ts` (RFC 3339).
- Über dem Log steht eine Zeile mit Phase, Port, Kosten aller Schritte, PID und dem Fehler oder Hinweis
  aus dem Zustand.
- Ohne Lauf zeigt der untere Bereich die Details des PRDs. Unter dem Titel stehen Status, Datei,
  Git-Zustand, Erstellzeit, Kosten (`4.12 USD`) und Modell (volle ID) untereinander. Kosten und Modell
  kommen aus dem Lauf-Zustand, sonst aus der Statuszeile, und entfallen ohne Wert. Darunter steht der
  Freitext der Statuszeile unverändert, also auch mit den Feldern `Kosten:` und `Modell:`. Folgezeilen
  desselben Zitatblocks gehören zum Freitext. Die TUI rendert ihn mit Glamour als Markdown. Ist der
  untere Bereich im Fokus, scrollen `j` `k`, Bild-Tasten und `G` den Text. Das Mausrad scrollt ebenfalls. Passt der Text nicht in die Box, zeigt der Titel die Position in Prozent.
- `v` zeigt das gewählte PRD vollständig in einem Fenster wie der Chat, Titel `PRD: <bereich>/<name>`.
  Oben stehen die Metazeilen der Detailansicht, darunter die komplette Datei als Markdown, gerendert mit
  Glamour. Hat ein Lauf das PRD in seinen Worktree verschoben und ist es nicht gemergt, liest lazydev die
  Datei aus dem Worktree, sonst aus dem Hauptordner. Lazydev liest die Datei einmal beim Öffnen.
  Dateiänderungen bei offenem Fenster ändern den Inhalt nicht. `j` `k`, `PgUp` `PgDn` `Ctrl+U` `Ctrl+D`
  `Leertaste`, `g` `G` `Home` `End` und das Mausrad scrollen, `Esc` und `q` schließen. Passt der Inhalt
  nicht ins Fenster, zeigt der Titel die Position in Prozent. `o` öffnet die Datei in `$VISUAL`, sonst
  `$EDITOR`, sonst `vi`. In Herdr öffnet sich ein neuer Pane, das Fenster bleibt unverändert. Ohne Herdr
  unterbricht lazydev die Oberfläche bis zum Ende des Editors. Danach liest lazydev die Datei neu, behält
  die Scroll-Position und liest die Projekte neu ein. Ist die Datei nicht lesbar, öffnet sich kein Fenster
  oder es schließt sich, und die Statuszeile zeigt `PRD nicht lesbar: …`.
- Die TUI liest Logs schrittweise nach. fsnotify meldet Änderungen unter `prd/` und im
  Zustandsverzeichnis. Alle 2 Sekunden liest die TUI zusätzlich die Zustände ein, weil ein beendeter
  Runner kein Dateiereignis auslöst.
- Ein PRD, das ein Lauf in seinen Worktree verschoben hat, liest die TUI aus dem Worktree.

## Technik

- **Sprache:** Go (aktuelle stabile Version, über `pacman -S go`).
- **TUI:** Bubble Tea, Bubbles (Listen, Textfeld, Viewport) und Lip Gloss für Stile.
- **Dateiüberwachung:** fsnotify auf `prd/` jedes Projekts und auf das Zustandsverzeichnis.
- **Git:** Aufruf des `git`-Binärprogramms, keine Go-Git-Bibliothek.
- **Build:** einzelne Binärdatei, Installation nach `~/.local/bin/lazydev`.
- **Struktur:**

  ```
  cmd/lazydev/main.go      Einstieg: TUI oder Unterbefehle start, run, merge, stop, teardown
  internal/config/         config.yaml lesen und schreiben, Presets
  internal/project/        Projekte, PRDs, Statuszeile, Phasen, Git-Branch
  internal/git/            Aufruf des git-Programms
  internal/worktree/       Worktree anlegen und entfernen, Port und Slug vergeben
  internal/hooks/          Setup, Teardown, Prüfung, Dev-Befehl, Preset nuxt-drizzle
  internal/claude/         claude -p starten, stream-json auswerten, Kurzfassungen
  internal/runner/         Schritte ausführen, Start mit maxParallel, Merge-Queue, Teardown
  internal/state/          Lauf-Zustand, PID prüfen, Sperren
  internal/chatrun/        PRD-Chats: Zustand, losgelöste Runden, Erkennung der Rückfrage
  internal/runlog/         eigene Log-Zeilen des Runners
  internal/devserver/      Dev-Server in Worktrees starten und stoppen
  internal/docker/         Container, Compose-Services, Events
  internal/procnet/        Prozess hinter einem lauschenden Port
  internal/herdr/          Herdr erkennen, Pane öffnen
  internal/tui/            Bubble-Tea-Modelle und Ansichten, Chat für n
  ```

- **Tests:**
  - Unit-Tests für Statuszeile, stream-json, Lauf-Zustand, Port- und Slug-Vergabe.
  - Runner-Test mit einem Fake-`claude`, das vorbereitete stream-json-Zeilen ausgibt.
  - Merge-Tests gegen ein temporäres Git-Repo: Rebase ohne Konflikt, Konflikt mit Abbruch, Fast-Forward,
    Hauptordner nicht sauber.

## Meilensteine

1. **Gerüst:** Go-Modul, `config.yaml`, Projektliste mit `a` und `d`, PRD-Liste mit Status und Branch.
2. **Commands:** `/implement-prd` und `/merge-prd` global anlegen. `/implement-prd` einmal manuell in
   `nuxt-drizzle` mit `prd/auth/mobile-bearer-token-auth.md` prüfen.
3. **Worktree und Setup:** Worktree anlegen, PRD verschieben, Preset `nuxt-drizzle` mit eigener
   Datenbank und Port, Teardown. Ohne TUI testbar über `lazydev run`.
4. **Runner:** Umsetzung und Review im Worktree, Zustand und Log auf Platte, `maxParallel`.
5. **Merge:** Merge-Queue mit Sperre, Rebase, `/merge-prd` bei Konflikt, Fast-Forward, Aufräumen.
6. **Live-Ansicht:** Log-Pane, Phasen-Symbole, Dateiüberwachung, `s`, `m`, `x`, `X`.
7. **Interaktiv:** `n`, `o`, `p`, `w`, mit und ohne Herdr.
8. **Docker-Status:** Projekt-Symbol, Docker-Block und Dev-Zeile, Docker-Ansicht `D` mit `U`, `S`, `L`.
9. **Feinschliff:** Hilfe (`?`), Fehlermeldungen, Kosten pro PRD, README.

## Geprüfte Annahmen

Geprüft am 2026-10-20 mit Claude Code 2.1.304, Herdr 0.9.5 und einer Kopie von nuxt-drizzle. Die
Claude-Tests liefen in einem Test-Repo mit `--model haiku`.

| Annahme | Ergebnis |
|---|---|
| `claude -p "/<command>"` führt eigene Slash-Commands und Subagenten aus | Ja, globale und projekteigene, auch im Worktree. Subagenten laufen im Hintergrund. Nach ihrem Ende folgt eine zweite Runde mit eigenem `system/init` und eigenem `result`. Der Prozess endet erst danach. |
| `acceptEdits` mit Allowlist läuft ohne Dialog | Ja. Was nicht freigegeben ist, lehnt Claude sofort ab, ohne zu warten: Ereignis `system/permission_denied` und Liste `permission_denials` im `result`. Lesende Befehle wie `ls` sind ohne Freigabe erlaubt. Befehle mit `$VAR`, Verkettungen wie `a; b` und `git -C <pfad> …` passen nicht auf `Bash(git log:*)` und werden abgelehnt. |
| Worktree lädt `.claude/` des Projekts | Ja: Commands, Agenten und `env` aus `settings.json`. `env` aus `settings.local.json` des Hauptordners greift ebenfalls, obwohl die Datei im Worktree fehlt. `permissions.allow` aus `settings.json` ignoriert Claude ohne bestätigten Trust-Dialog. Ob ein Worktree den Trust des Hauptordners erbt, ist nicht getestet. |
| `committer` committet ohne `Co-Authored-By` | Ja, im Worktree getestet. Ein Trailer entstand nicht. |
| `--agents <datei>` mit `-p` lädt die eingebetteten Agenten | Ja, am 2026-10-21 mit `--model haiku` geprüft: `committer` und `structural-completeness-reviewer` stehen als Subagenten bereit. |
| Drizzle-Migrationen nach Rebase | Beide Seiten mit `0003_*`: Konflikte in `meta/_journal.json`, `meta/0003_snapshot.json` und `schema/index.ts`. Das Verfahren aus `/merge-prd` Schritt 3 liefert eine saubere `0004_*` mit nur der Branch-Tabelle. Es braucht `git rm`. |
| Nuxt übernimmt `PORT` aus `.env` | Ja (`@nuxt/cli` 4.0.1: `NUXT_PORT` vor `NITRO_PORT` vor `PORT`). Der Server lauscht nur auf `[::1]`. Die Port-Prüfung muss IPv4 und IPv6 abfragen. |
| Herdr startet einen Befehl im neuen Pane | Ja, mit Herdr 0.9.5 ausgeführt. `herdr pane split --current --direction right --cwd <pfad>` liefert die Pane-ID unter `.result.pane.pane_id`. `herdr pane run <pane-id> <befehl>` tippt den Befehl in die Shell des Panes, die Shell bleibt danach offen. Argumente mit Sonderzeichen übergibt lazydev mit `--env LAZYDEV_ARG<n>=<wert>` und schreibt `"$LAZYDEV_ARG<n>"` in den Befehl. Das funktioniert in fish, bash und zsh gleich. Herdr erkennst du an `HERDR_ENV=1`, das Programm liegt unter `HERDR_BIN_PATH`. |
| Platz von `node_modules` pro Worktree | Rund 25 MB zusätzlich zum Store (741 MB scheinbar, Rest Hardlinks). `pnpm install --frozen-lockfile --prefer-offline` dauerte 3,4 Sekunden. Gilt nur auf derselben Partition wie der Store (`~/.local/share/pnpm/store`). |

### stream-json-Schema

Ein Ereignis pro Zeile, Feld `type`, bei `system` und `result` zusätzlich `subtype`:

| `type` / `subtype` | Inhalt für lazydev |
|---|---|
| `system` / `hook_started`, `hook_response` | Globale Hooks, z. B. SessionStart. Ignorieren. |
| `system` / `init` | `session_id`, `model`, `permissionMode`, `cwd`, `slash_commands`, `agents`, `tools` |
| `system` / `permission_denied` | `tool_name`, `tool_use_id`, `message` |
| `system` / `task_started`, `task_notification` | Subagent gestartet oder fertig, mit `subagent_type`, `description`, `summary` |
| `system` / `thinking_tokens`, `background_tasks_changed`, `task_updated`, `vcs_state_changed` | Ignorieren |
| `assistant` | `message.content[]` mit `text` oder `tool_use` (`name`, `input`). `parent_tool_use_id` ist gesetzt, wenn ein Subagent spricht. |
| `user` | `message.content[]` mit `tool_result` (`tool_use_id`, `content`, `is_error`) |
| `rate_limit_event` | Ignorieren |
| `result` | `subtype` (`success`), `is_error`, `result` (letzter Text), `total_cost_usd`, `num_turns`, `duration_ms`, `permission_denials`, `stop_reason`, `terminal_reason`, `session_id` |

`total_cost_usd` steigt zwischen zwei `result`-Ereignissen desselben Laufs. Ob der Wert die Kosten des
ganzen Laufs enthält, ist nicht geprüft. lazydev nimmt den Wert des letzten `result`.

Ohne `--model` nutzt Claude das Standardmodell des Nutzers (aktuell Opus 5.5) und den Modus aus den
globalen Einstellungen (aktuell `auto`). `--permission-mode acceptEdits` überschreibt den Modus.

## Später

- Rückgabe von `/arewedone` an die Umsetzung bei groben Befunden, mit Höchstzahl an Runden.
- Benachrichtigung bei Ende, Fehler oder wartendem Merge, z. B. über `notify-send` oder
  Herdr-Notifications.
- Weitere Presets für Projekte ohne Postgres.

## Verhältnis zu OpenRig

OpenRig wurde am 2026-10-20 getestet und wieder entfernt. lazydev übernimmt dessen Aufgabe für den
PRD-Workflow und hat keine Abhängigkeit zu OpenRig.
