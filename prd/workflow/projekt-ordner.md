# PRD: Projekt-Ordner `.lazydev/` für Einstellungen, Prompts und Knowledge

> Status: geplant · Template: einfach · Scope: Jedes Projekt trägt seine lazydev-Einstellungen im Repo unter `.lazydev/`: Ziel-Branch, Preset, Knowledge pro Schritt, Freigaben, Hooks und eigene Prompts für Erstellen, Umsetzen, Review und Merge. Alle im Team arbeiten damit gleich. lazydev behält nur den allgemeinen Ablauf „einfach“ als Standard und hängt an jeden Prompt einen festen Vertrag an. Ein Wizard in der TUI legt den Ordner an.

## Context

lazydev bettet heute zwei Templates ein (`internal/prdtemplate/`): „Nuxt-Feature“ mit fest eingebautem
Drizzle-Workflow (`pnpm db:generate` → `pnpm db:migrate`) und „Einfach“. Review und Merge
(`internal/runner/prompts/`) sind für alle Projekte gleich. Projektspezifische Einstellungen wie
`baseBranch`, `preset`, `allowedTools` und `hooks` stehen in der zentralen `config.yaml` auf dem Rechner.

Das hat zwei Probleme:

- **Feste Regeln passen nicht zu jedem Projekt.** Beispiel phoenix: Feature-Branches committen dort keine
  Migrationen, `pnpm db:generate` läuft nur beim Release auf `next`. Der eingebettete Nuxt-Prompt verlangt
  das Gegenteil. Jede solche Regel als Sonderfall in lazydev einzubauen, macht lazydev zu einem
  Release-Management-Werkzeug für einzelne Projekte.
- **Einstellungen pro Rechner laufen auseinander.** Ein neues Teammitglied muss Ziel-Branch, Freigaben
  und Hooks selbst richtig eintragen. Ein Fehler, etwa `main` statt `next` als Ziel-Branch, mergt in den
  falschen Branch.

Nach der Änderung gilt:

- **lazydev** liefert die Mechanik (Worktree, Datenbanken, Ports, Merge-Queue, Rebase, Prüfung,
  Fast-Forward), den allgemeinen Ablauf „einfach“ als Standard-Prompts und einen festen Vertrag pro
  Schritt.
- **Das Projekt** legt unter `.lazydev/` fest, wie lazydev und Claude dort arbeiten. Der Ordner ist mit
  dem Projekt versioniert. Jedes Teammitglied und jeder Worktree hat dieselben Einstellungen. Lokal
  überschreiben lässt sich nichts davon.

Im Repo existiert bereits und wird wiederverwendet:

- `internal/prdtemplate/einfach/create.md` und `implement.md`: Grundlage der Standard-Prompts.
- `internal/runner/prompts/arewedone.md` und `merge.md`: Grundlage von Standard-Review und -Merge.
- `claude.WriteAgents` und `--agents` (`internal/claude/agents.go`): Agenten `committer` und
  `structural-completeness-reviewer` bleiben eingebettet.
- `hooks.For` (`internal/hooks/hooks.go`): Preset-Hooks mit Überschreibung durch Skripte.
- `config.Project` mit `BaseBranch`, `Preset`, `AllowedTools`, `Hooks` (`internal/config/config.go`).
- `config.DetectPreset` (`internal/config/presets.go`): Preset-Erkennung für den Wizard.
- `branchPicker`, `localBranches`, `openBranchPicker` (`internal/tui/branch.go`): Branch-Auswahl für den
  Wizard.
- `mainDirProblem` (`internal/runner/merge.go`): Ausnahmen für den Hauptordner, z. B. ungetrackte PRDs.
- `checkReport`, `checkImplemented`, `checkReviewed` (`internal/runner/claude.go`).
- `Model.ask` (`internal/tui/actions.go`): Rückfrage mit `j`/`n`.
- `dialog()` (`internal/tui/box.go`) und `overlay()`: Fenster über der TUI.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Ort | Ordner `.lazydev/` im Hauptordner des Projekts. |
| Dateien | `.lazydev/config.yaml`, `.lazydev/create.md`, `.lazydev/implement.md`, `.lazydev/review.md`, `.lazydev/merge.md`. Andere Dateien im Ordner ignoriert lazydev. |
| Felder in `.lazydev/config.yaml` | `baseBranch`, `preset`, `knowledge`, `allowedTools`, `hooks`. Unbekannte Felder sind ein Fehler mit Dateipfad und Feldname. |
| Felder in der zentralen `config.yaml` | Global: `maxParallel`, `portBase`, `claude.binary`. Pro Projekt nur noch `path` und `autoMerge`. `autoMerge` bleibt lokal, weil es eine persönliche Entscheidung ist, ob du Merges selbst freigibst. |
| Keine lokale Überschreibung | `baseBranch`, `preset`, `allowedTools` und `hooks` gibt es pro Projekt nur in `.lazydev/config.yaml`. Stehen sie für ein Projekt mit `.lazydev/config.yaml` noch in der zentralen `config.yaml`, ignoriert lazydev sie. Die Statuszeile zeigt beim Laden einmal `<projekt>: lokale Einstellungen ignoriert (baseBranch, …), es gilt .lazydev/config.yaml`. |
| Projekt ohne `.lazydev/config.yaml` | Übergang für bestehende Projekte: lazydev liest `baseBranch`, `preset`, `allowedTools` und `hooks` wie bisher aus der zentralen `config.yaml`. Der Titel der PRD-Box zeigt ` · ohne .lazydev`. Der Wizard übernimmt diese Werte und entfernt sie danach aus der zentralen `config.yaml`. |
| Standardwerte | `baseBranch`: `main`. `preset`: leer, also keine Preset-Hooks. `knowledge`, `allowedTools`, `hooks`: leer. |
| Schritte | `create` (Chat zum PRD), `implement` (Umsetzung), `review` (heute `/arewedone`), `merge` (heute `/merge-prd`). |
| Quelle der Dateien | Immer der Hauptordner des Projekts, gelesen beim Start jeder Chat-Runde und jedes Schritts. Uncommittete Änderungen gelten sofort. |
| Ziel-Branch eines Laufs | Der Lauf merkt sich `baseBranch` beim Start im Feld `base` wie heute. Eine spätere Änderung wirkt erst auf neue Läufe. |
| Aufbau eines Prompts | Vier Teile, getrennt durch eine Leerzeile: (1) Kopfzeile von lazydev, (2) Knowledge-Block, falls Dateien eingetragen sind, (3) Prompt aus `.lazydev/<schritt>.md` oder Standard-Prompt, (4) Vertrag des Schritts. |
| Kopfzeile | `# lazydev <schritt>: <argument>`, z. B. `# lazydev implement: prd/auth/token.md` oder `# lazydev review: feat/token → next`. Sie steht im Log (`claude.FirstLine`) und ist das stabile Merkmal für den Fake-`claude` in den Tests. |
| Platzhalter | In Prompts und Vertrag: `$ARGUMENTS` (create: Beschreibung, implement: PRD-Pfad, review: leer, merge: Branch und Fehlerkontext), `$BASE` (Ziel-Branch), `$BRANCH` (Branch des PRDs, bei create leer). |
| Frontmatter | Beginnt eine Prompt-Datei mit `---`, entfernt lazydev den Block bis zum nächsten `---`. So lassen sich Commands aus `.claude/commands/` übernehmen. |
| Vertrag | Eingebettet unter `internal/prompts/contract/<schritt>.md`, nicht überschreibbar. Er enthält, was der Runner auswertet oder was den Ablauf schützt. Inhalt je Schritt siehe „Umsetzung“, Schritt 3. |
| Standard-Prompts | Eingebettet unter `internal/prompts/default/<schritt>.md`. Das ist der Ablauf „einfach“: create und implement aus `internal/prdtemplate/einfach/` ohne die Teile, die jetzt im Vertrag stehen, review aus `arewedone.md`, merge aus `merge.md` ohne den Drizzle-Abschnitt. Stattdessen steht dort: „Generierte Dateien wie Migrationen behandelst du nach den Regeln aus `CLAUDE.md`. Führe sie nie von Hand zusammen.“ |
| Fehlt eine Prompt-Datei | lazydev nutzt für diesen Schritt den Standard-Prompt. |
| Nuxt-Prompts | Entfallen ersatzlos, ebenso `internal/prdtemplate/nuxt/`. Projekte mit eigenem Ablauf schreiben ihn in `.lazydev/<schritt>.md`. |
| Knowledge | In `.lazydev/config.yaml` unter `knowledge` als Listen von Pfaden relativ zum Projekt: `all` für jeden Schritt, dazu `create`, `implement`, `review`, `merge`. Reihenfolge im Prompt: `all`, dann der Schritt, doppelte Pfade fallen weg. |
| Knowledge-Block | `## Pflichtlektüre` mit dem Satz „Lies diese Dateien vollständig, bevor du etwas anderes tust:“ und einer Liste der Pfade. lazydev kopiert den Inhalt nicht in den Prompt. |
| Fehlende Knowledge-Datei | lazydev prüft vor dem Start von Claude, ob jede Datei im Arbeitsordner des Schritts existiert (create: Hauptordner, sonst Worktree). Fehlt eine, scheitert der Schritt mit `Knowledge-Datei fehlt: <pfad>`, ohne Claude zu starten. Im Chat erscheint derselbe Text als Fehler. |
| Freigaben | `allowedTools` ergänzt die Standard-Freigaben für implement, review und merge. Der Chat (create) behält seine festen Freigaben. |
| Hooks | `hooks` mit `setup`, `teardown`, `check`, `dev` wie heute. Sie ersetzen einzelne Hooks des Presets. Relative Pfade gelten ab dem Hauptordner. |
| Vertrauen | `hooks` und `allowedTools` aus dem Repo führen Code aus bzw. erweitern Claudes Rechte. lazydev speichert pro Projekt einen SHA-256 über beide Abschnitte in `$XDG_STATE_HOME/lazydev/trust/<projekt-id>.json`. Weicht der Hash ab oder fehlt er, startet kein Runner. Sind beide Abschnitte leer, ist keine Bestätigung nötig. `baseBranch`, `preset` und `knowledge` brauchen keine Bestätigung. |
| Vertrauen in der TUI | `s` fragt vor dem Start: `.lazydev/config.yaml von <projekt> setzt Hooks oder Freigaben:`, darunter beide Abschnitte, dann `Erlauben? (j/n)`. Nach `j` speichert lazydev den Hash und fragt wie bisher nach dem Start. Nach `n` startet nichts. |
| Vertrauen per CLI | `lazydev run` und `lazydev start` scheitern ohne passenden Hash mit `Hooks oder Freigaben in .lazydev/config.yaml nicht bestätigt. Prüfen und bestätigen: lazydev trust <projekt-id>`. `lazydev trust <projekt-id>` zeigt beide Abschnitte und speichert den Hash nach `j` auf stdin. |
| Schutz von `.lazydev/` | Claude darf `.lazydev/` nicht ändern. Der Vertrag verbietet es. lazydev prüft nach implement, review und merge mit `git diff --name-only $BASE...HEAD -- .lazydev` und `git status --porcelain -- .lazydev` im Worktree. Ist eines nicht leer, scheitert der Schritt mit `.lazydev/ im Worktree geändert: <dateien>`. |
| Hauptordner beim Merge | Geänderte und ungetrackte Dateien unter `.lazydev/` blockieren den Merge nicht. Würde der Fast-Forward eine solche Datei überschreiben, scheitert er mit der Meldung von git. |
| Templates | Die eingebauten Templates und die Template-Auswahl entfallen. `n` öffnet direkt das Fenster `Neues PRD · <projekt>`. Aus einem Issue startet `n` den Chat direkt. |
| `Template:` in der Statuszeile | Wird nicht mehr geschrieben und nicht ausgewertet. Bestehende PRDs mit `Template: …` bleiben gültig, der Wert bleibt Freitext. |
| Taste `b` | Schreibt `baseBranch` in `.lazydev/config.yaml`. Ohne `.lazydev/config.yaml` zeigt die Statuszeile `Kein .lazydev/ im Projekt. Mit i anlegen.` als Fehler. Steht der Wert auf `main`, entfernt lazydev das Feld. Andere Felder und Kommentare der Datei bleiben erhalten (`yaml.Node`). |
| Wizard | Taste `i` bei gewähltem Projekt. Fenster `.lazydev/ anlegen · <projekt>` mit vier Seiten: Ziel-Branch, Knowledge-Dateien, Schritte je Datei, Zusammenfassung. `Enter` geht weiter, `Backspace` zurück, `Esc` bricht ohne Änderung ab. |
| Wizard, Seite Ziel-Branch | Liste aus `localBranches`. Vorgewählt ist `baseBranch` aus der zentralen `config.yaml`, sonst der aktuelle Branch des Hauptordners. `j`/`k` wählen. |
| Wizard, Seite Knowledge-Dateien | Liste aller getrackten `.md`-Dateien (`git ls-files -- '*.md'`) ohne `prd/` und `.lazydev/`, sortiert. `Leertaste` markiert, `j`/`k` wählen. Vorgewählt sind `CLAUDE.md`, `ARCHITECTURE.md` und `_ARCHITECTURE.md`, falls vorhanden. Mehr als die sichtbaren Zeilen scrollen mit der Auswahl. |
| Wizard, Seite Schritte je Datei | Eine Zeile pro markierter Datei mit vier Feldern `[c] [i] [r] [m]` für create, implement, review, merge. `j`/`k` wählen die Datei, `1` bis `4` schalten das Feld um. Vorgewählt sind create und implement. Ohne markierte Datei überspringt der Wizard diese Seite. |
| Wizard, Seite Zusammenfassung | Zeigt die entstehende `config.yaml` und die Dateien, die lazydev anlegt. `Enter` schreibt. |
| Wizard, Ergebnis | Schreibt `.lazydev/config.yaml` mit `baseBranch` (nur wenn nicht `main`), `preset` (aus der zentralen `config.yaml`, sonst `config.DetectPreset`), `knowledge`, `allowedTools` und `hooks` aus der zentralen `config.yaml`. Eine Datei, die bei allen vier Schritten markiert ist, steht unter `all`. Dazu kopiert er die vier Standard-Prompts nach `.lazydev/create.md`, `implement.md`, `review.md`, `merge.md`, damit das Projekt sie anpassen kann. |
| Wizard, danach | lazydev entfernt `baseBranch`, `preset`, `allowedTools` und `hooks` des Projekts aus der zentralen `config.yaml` und speichert sie. Meldung: `.lazydev/ angelegt. Committe den Ordner, damit das Team ihn bekommt.` lazydev committet nicht. |
| Wizard bei vorhandenem `.lazydev/` | `i` zeigt `.lazydev/ existiert schon. Bearbeite die Dateien im Editor.` als Fehler und öffnet nichts. |
| Agenten | `committer` und `structural-completeness-reviewer` bleiben eingebettet und kommen über `--agents`. Projekt-eigene Agenten in `.claude/agents/` lädt Claude wie bisher. Bei gleichem Namen gewinnt `--agents`. |
| Log | Vor jedem Claude-Aufruf schreibt der Runner `Prompt: .lazydev/<schritt>.md` oder `Prompt: Standard` und, falls vorhanden, `Knowledge: <pfade>` ins Log. |

## Scope

**In Scope**

- Neues Paket `internal/prompts`: Standard-Prompts, Verträge, Aufbau der Prompts, Anlegen von `.lazydev/`.
- Lesen und Schreiben von `.lazydev/config.yaml`.
- Umzug von `baseBranch`, `preset`, `allowedTools`, `hooks` aus der zentralen Konfiguration, mit Übergang
  für Projekte ohne `.lazydev/`.
- Vertrauen über Hash, Rückfrage bei `s`, Unterbefehl `lazydev trust`.
- Wizard mit Taste `i`. Taste `b` schreibt in `.lazydev/config.yaml`.
- Prüfung auf Änderungen an `.lazydev/` nach implement, review und merge.
- Ausnahme für `.lazydev/` in `mainDirProblem`.
- Entfernen von `internal/prdtemplate`, der Template-Auswahl und von `internal/runner/prompts/`.
- Tests, `SPEC.md`, `README.md`, `README.de.md`, `CLAUDE.md`.

**Out of Scope**

- Eigene Agenten pro Projekt über `.lazydev/`.
- Mehrere Abläufe pro Projekt oder eine Auswahl beim Erstellen.
- `allowedTools` und `hooks` im Wizard bearbeiten. Er übernimmt nur vorhandene Werte.
- Bearbeiten von Prompts in der TUI.
- Ein Unterbefehl `lazydev init`. Der Wizard ist nur in der TUI.
- Vertrauen für Prompts, Knowledge, `baseBranch` und `preset`.

## Umsetzung

1. **Paket `internal/prompts` anlegen.**
   - `internal/prompts/default/{create,implement,review,merge}.md` nach „Standard-Prompts“.
   - `internal/prompts/contract/{create,implement,review,merge}.md` nach Schritt 3.
   - `prompts.go`: `type Step string` (`Create`, `Implement`, `Review`, `Merge`) und
     `Build(projectDir string, repo config.Repo, step Step, v Vars) (Prompt, error)`. `Vars` enthält
     `Arguments`, `Base`, `Branch` und `WorkDir` für die Knowledge-Prüfung. `Prompt` enthält `Text`,
     `Source` (`.lazydev/<schritt>.md` oder `Standard`) und `Knowledge`.
   - `Init(projectDir string, repo config.Repo) error`: legt `.lazydev/` an, schreibt `config.yaml` und die
     vier Standard-Prompts. Existiert `.lazydev/`, liefert `Init` einen Fehler und schreibt nichts.
2. **`.lazydev/config.yaml`** (`internal/config/repo.go`).
   - `type Repo struct` mit `BaseBranch`, `Preset`, `Knowledge map[string][]string`, `AllowedTools`,
     `Hooks` und den YAML-Feldern aus „Felder in `.lazydev/config.yaml`“.
   - `LoadRepo(projectDir) (Repo, bool, error)`. `bool` meldet, ob die Datei existiert. Unbekannte Felder
     und unbekannte Schritte unter `knowledge` sind ein Fehler.
   - `SetRepoBaseBranch(projectDir, branch string) error` über `yaml.Node`, damit Kommentare bleiben.
   - `func (r Repo) Base() string` mit Standard `main`, `func (r Repo) TrustHash() string`.
   - `config.Project` verliert `BaseBranch`, `Preset`, `AllowedTools`, `Hooks` nicht sofort: Sie bleiben
     als Übergangsfelder lesbar. Neue Funktion `Effective(p Project) (Repo, []string, error)` liefert die
     wirksamen Einstellungen und die Namen ignorierter lokaler Felder. Alle Stellen, die heute
     `p.Base()`, `p.Preset`, `p.AllowedTools` oder `p.Hooks` lesen, nutzen `Effective`.
   - `RemoveLocalProjectSettings(i int)` leert die Übergangsfelder eines Projekts.
3. **Verträge schreiben.** Jeder Vertrag beginnt mit `## Regeln von lazydev` und hat Vorrang vor allem,
   was davor steht.
   - create: PRD unter `prd/<bereich>/<name>.md`, Statuszeile `> Status: geplant · Scope: …`, keine
     Umsetzung, Klärungsfragen nummeriert mit Optionen A bis D, keine offenen Entscheidungen im PRD, nur
     unter `prd/` schreiben, `.lazydev/` nicht ändern, zum Schluss Zielpfad nennen.
   - implement: Branch muss `$BRANCH` heißen, kein Commit außer `git mv`, Statuszeile auf
     `umgesetzt (YYYY-MM-DD)` nur bei Erfolg, Bericht mit `## Ergebnis` (`umgesetzt` | `abgebrochen`),
     `## Geänderte Dateien`, `## Prüfungen`, `## Offene Fragen`, `## Nicht geprüft` mit den Regeln aus dem
     heutigen „Einfach“, keine Watch-Befehle oder Dev-Server, kein `git push`, `git reset --hard`,
     `git worktree`, `.lazydev/` nicht ändern.
   - review: Änderungen gegenüber `$BASE` prüfen, alles über den Agenten `committer` committen, danach
     `git status --porcelain` leer, kein `git push`, `.lazydev/` nicht ändern.
   - merge: Branch `$BRANCH` per Rebase auf `$BASE` bringen, Bericht mit `## Ergebnis`
     (`bereit für Fast-Forward` | `abgebrochen`), `## Konflikte`, `## Prüfungen`, `## Offene Punkte`,
     Korrekturen über `committer`, kein `git merge` nach `$BASE`, kein `git push`, bei Unklarheit
     `git rebase --abort` und abbrechen, `.lazydev/` nicht ändern.
4. **Runner** (`internal/runner/claude.go`, `merge.go`, `runner.go`, `start.go`).
   - `prompt()` und `mergePRD` nutzen `prompts.Build` mit `config.Effective`. `prdTemplate` entfällt.
   - `allowedTools` nutzt `Repo.AllowedTools`, `newRun` nutzt `Repo.Base()`.
   - Log-Zeilen `Prompt: …` und `Knowledge: …` vor jedem Claude-Aufruf.
   - Nach implement, review und `mergePRD`: `checkLazydevUntouched(r)` nach „Schutz von `.lazydev/`“.
   - `mainDirProblem`: Pfade unter `.lazydev/` überspringen.
   - Vertrauen prüfen in `Start` vor dem Start eines Runners und in `run` vor dem ersten Schritt.
   - `internal/runner/prompts/` löschen.
5. **Hooks** (`internal/hooks/hooks.go`): `For` bekommt die wirksame `Repo` statt `config.Project`. Erst
   Preset, dann `Repo.Hooks`. Alle Aufrufer anpassen.
6. **Vertrauen** (`internal/state/trust.go`): `TrustPath(projectID)`, `Trusted(projectID, hash) bool`,
   `Trust(projectID, hash) error`. Ein leerer Hash gilt immer als vertraut.
7. **Chat** (`internal/chatrun`, `internal/tui/interactive.go`): Die erste Runde nutzt
   `prompts.Build(…, prompts.Create, …)` mit dem Hauptordner als `WorkDir`. Die Anzeige bleibt die
   Beschreibung. Scheitert `Build`, zeigt die Statuszeile den Fehler, und kein Chat entsteht.
8. **TUI** (`internal/tui`).
   - Template-Auswahl entfernen: `modeTemplate`, `updateTemplate`, `viewTemplatePicker`, `Model.template`.
   - `b` schreibt über `config.SetRepoBaseBranch`.
   - `s`: Vertrauens-Rückfrage vor der Start-Rückfrage.
   - Wizard in `internal/tui/initwizard.go` mit `modeInit` und den Seiten aus den Produktentscheidungen.
     `branchPicker` wird für die erste Seite wiederverwendet.
   - Titel der PRD-Box: ` · ohne .lazydev` bei Projekten ohne `.lazydev/config.yaml`.
   - Meldung zu ignorierten lokalen Feldern beim Laden.
   - Hilfe, Fußzeile (`i` bei Fokus Projekte nach `b`) und Tastentabelle.
9. **CLI** (`cmd/lazydev/main.go`): Unterbefehl `trust <projekt-id>`.
10. **`internal/prdtemplate` löschen** samt Tests.
11. **Tests.**
    - `internal/prompts`: vier Teile, Kopfzeile, Platzhalter, Frontmatter entfernt, Projekt-Prompt statt
      Standard, Knowledge-Reihenfolge und Duplikate, fehlende Knowledge-Datei, `Init` schreibt alle Dateien
      und verweigert bei vorhandenem Ordner.
    - `internal/config`: `LoadRepo` ohne Datei, mit allen Feldern, mit unbekanntem Feld. `Effective` mit
      und ohne `.lazydev/config.yaml`, ignorierte lokale Felder. `SetRepoBaseBranch` erhält Kommentare.
      `TrustHash` stabil und leer ohne Hooks und Freigaben.
    - `internal/runner`: Fake-`claude` erkennt Schritte an `# lazydev <schritt>:`. Projekt-Prompt landet im
      Aufruf. `baseBranch` aus `.lazydev/config.yaml` bestimmt Worktree und Merge. Änderung an `.lazydev/`
      im Worktree lässt implement scheitern. Ungetrackte Datei unter `.lazydev/` im Hauptordner blockiert
      den Merge nicht. Run ohne Vertrauen scheitert mit dem Hinweis auf `lazydev trust`.
    - `internal/tui`: `n` ohne Template-Auswahl. Wizard durch alle Seiten, Ergebnis in `.lazydev/` und
      bereinigte zentrale `config.yaml`. `i` bei vorhandenem Ordner. `b` ohne `.lazydev/`. Rückfrage zum
      Vertrauen bei `s`.
12. **Doku.**
    - `SPEC.md`: „Ausgangslage“, „PRD-Templates“ und „Umsetzen (`nuxt`) und Merge“ durch „Projekt-Ordner
      `.lazydev/`“ ersetzen: Dateien, Felder, keine lokale Überschreibung, Aufbau eines Prompts, Vertrag,
      Knowledge, Vertrauen, Wizard. Tastentabelle, Chat-Ablauf, Runner-Ablauf, Merge, Dateiablage und das
      Beispiel der zentralen `config.yaml` anpassen.
    - `README.md` und `README.de.md`: Abschnitt zu `.lazydev/` mit Beispiel, Tasten `i` und `b`,
      `lazydev trust`. Templates entfernen. Beispiel der zentralen `config.yaml` kürzen.
    - `CLAUDE.md`: Architektur-Abschnitte zu Prompts, `internal/prdtemplate` und `internal/config`.

## Technische Hinweise

- Alle Texte deutsch, auch Verträge, Standard-Prompts, Fehlermeldungen und Kommentare.
- YAML über `gopkg.in/yaml.v3`, beim Lesen mit `KnownFields(true)`. Beim Ändern von `baseBranch` über
  `yaml.Node`, damit Kommentare und Reihenfolge bleiben.
- Hash über `yaml.Marshal` einer Struktur mit nur `allowedTools` und `hooks`, damit Kommentare und andere
  Felder ihn nicht ändern.
- Der Chat läuft im Hauptordner, Runner-Schritte im Worktree. Eine nur lokal vorhandene Knowledge-Datei
  fehlt im Worktree und erzeugt die Fehlermeldung. Der Wizard bietet deshalb nur getrackte Dateien an.
- `claude.FirstLine` liefert die Kopfzeile. Der Fake-`claude` in `internal/runner/runner_test.go` liest die
  erste Zeile von `$2`.
- Merge-Tests: `FAKE_MERGE_MODE` und die bestehenden Szenarien bleiben, nur das Erkennungsmerkmal ändert
  sich.
- Prüfbefehl: `make test`.

## Erfolgskriterien

- `make test` läuft ohne Fehler.
- `internal/prdtemplate` und `internal/runner/prompts` existieren nicht mehr.
- Ein Projekt ohne `.lazydev/` durchläuft Erstellen, Umsetzen, Review und Merge mit den Standard-Prompts
  und den Übergangsfeldern der zentralen `config.yaml`.
- Ein Projekt mit `.lazydev/config.yaml` nutzt `baseBranch`, `preset`, `allowedTools` und `hooks` nur aus
  dieser Datei. Werte in der zentralen `config.yaml` wirken nicht.
- Ein Projekt mit `.lazydev/implement.md` nutzt diesen Prompt. Das Log zeigt `Prompt: .lazydev/implement.md`.
- Der Prompt jedes Schritts endet mit `## Regeln von lazydev`, auch mit Projekt-Prompt.
- Eine fehlende Knowledge-Datei lässt den Schritt ohne Claude-Aufruf scheitern.
- Hooks oder Freigaben in `.lazydev/config.yaml` starten keinen Runner ohne Bestätigung. Eine Änderung an
  einem der beiden Abschnitte verlangt erneut eine Bestätigung.
- Ändert Claude im Worktree eine Datei unter `.lazydev/`, scheitert der Schritt.
- Eine uncommittete Änderung unter `.lazydev/` im Hauptordner blockiert den Merge nicht.
- Der Wizard legt `.lazydev/config.yaml` und die vier Prompt-Dateien an und entfernt die übernommenen Felder
  aus der zentralen `config.yaml`.
- `n` öffnet ohne Template-Auswahl direkt das Fenster „Neues PRD“.
- `SPEC.md`, `README.md`, `README.de.md` und `CLAUDE.md` beschreiben `.lazydev/` und nennen keine Templates
  mehr.
