# PRD: Claude-Code-Skill und `lazydev prompt create`

> Status: umgesetzt (2026-10-23) · Kosten: 5.65 USD · Modell: claude-fable-5-1 · Scope: lazydev installiert einen Skill `lazydev` nach `~/.claude/skills/lazydev/`, der Claude Code erklärt, wie lazydev und
> seine PRDs funktionieren. Der Befehl `lazydev prompt create` gibt den Create-Prompt des Projekts aus, damit eine normale Claude-Code-Session
> ein PRD genauso schreibt wie `n` in lazydev. Die TUI hält den Skill beim Start aktuell.

## Context

Ein PRD entsteht heute über `n` in der TUI: `createPRD` in `internal/tui/create.go` baut mit `prompts.Build` den Prompt
aus Header, Knowledge-Block, `.lazydev/create.md` und dem eingebauten Contract `internal/prompts/contract/create.md`
und öffnet Claude Code damit in einem Center-Tab. Außerhalb von lazydev fehlt dieses Wissen. Wer in einer normalen
Claude-Code-Session ein Feature diskutiert und dann sagt „mach daraus ein lazydev PRD“, bekommt eine Erkundung des
Repos statt eines PRDs: Claude sucht nach bestehenden PRDs und rät, was `.lazydev/` ist. Das Ergebnis weicht im
Format ab, die Klärungsfragen mit Optionen A bis D fehlen, und der Contract gilt nicht.

Die Lösung ist kein MCP-Server, denn das gesuchte Wissen ist statisch und liegt schon im Repo und im Binary. Ein Skill
von Claude Code transportiert es ohne Prozess und ohne Konfiguration pro Projekt. Nach der Änderung gilt:

- `lazydev prompt create [<beschreibung>]` gibt im Hauptordner eines Projekts denselben Prompt aus, den `n` baut.
- Der Skill `lazydev` liegt global unter `~/.claude/skills/lazydev/`. `SKILL.md` ist kurz: was lazydev ist, die
  Regeln für PRDs und eine Tabelle, welche Aufgabe wohin führt. „PRD schreiben“ führt zu `lazydev prompt create`.
  Alles andere führt in `references/spec.md`, eine Kopie von `SPEC.md`.
- `lazydev skill install` schreibt den Skill. Die TUI schreibt ihn beim Start, wenn er fehlt oder vom Binary abweicht,
  so bleibt er nach `lazydev update`, `install.sh` und `make install` aktuell. `"skill": false` in `settings.json`
  schaltet das ab.

Im Repo existiert bereits und wird wiederverwendet:

- `prompts.Build` und `prompts.Vars` (`internal/prompts/prompts.go`): bauen den Prompt eines Schritts.
- `repo.Read`, `repo.Dir`, `repo.PromptPath`, `Config.Base` (`internal/repo/repo.go`): lesen `.lazydev/`.
- `git.CurrentBranch` und `git.Run` (`internal/git/git.go`).
- `go:embed` für eingebaute Texte: `internal/prompts/contract/`, `internal/claude/agents/`,
  `internal/repo/workflows/simple/`.
- `fsutil.WriteAtomic` (`internal/fsutil/fsutil.go`).
- `config.Settings` und `config.LoadSettings` (`internal/config/config.go`).
- `cmd/lazydev/main.go`: `usage`, `switch cmd`, `fail`, `usageError`.
- `model.Init` und `model.note` (`internal/tui/main.go`): Hintergrund-Cmds beim Start und der Footer-Hinweis, siehe
  `checkUpdate` in `internal/tui/update.go` als Vorlage für einen stillen Hintergrundschritt.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Form | Ein Skill von Claude Code, kein MCP-Server, kein Slash-Command. Name `lazydev`. |
| Ort | `$CLAUDE_CONFIG_DIR/skills/lazydev/`, ohne gesetzte Variable `$HOME/.claude/skills/lazydev/`. Global für den Nutzer, nicht im Projekt. Begründung: Der Inhalt gehört zur lazydev-Version, nicht zum Repo, und veraltet im Projekt bei jedem Update einzeln. |
| Dateien des Skills | `SKILL.md` und `references/spec.md`. Keine weiteren Dateien. |
| Sprache | `SKILL.md` deutsch wie Prompts, Contracts und Agents. `references/spec.md` englisch, weil es `SPEC.md` ist. |
| Inhalt `SKILL.md` | Frontmatter `name: lazydev` und `description` mit den Auslösern (lazydev, PRD aus einem Gespräch machen, `prd/`, `> Status:`, `.lazydev/config.yaml`, Hooks, Worktrees, Runs, Schritte). Danach ein HTML-Kommentar, dass lazydev die Datei verwaltet und überschreibt. Körper: drei bis fünf Sätze, was lazydev ist; Regeln (nur unter `prd/` schreiben, nichts unter `.lazydev/` ändern, Statuszeile und ihre Werte, Umsetzung startet der Nutzer in lazydev); Tabelle `| Aufgabe \| So gehst du vor |` mit der Zeile für das PRD-Schreiben und je einer Zeile pro Themenbereich mit dem passenden Abschnitt von `references/spec.md` (`## PRDs`, `## .lazydev/`, `## Workflow` und `## Processes`, `## UI`, `## CLI` und `## Data`); Hinweis, nur den passenden Abschnitt zu lesen; Hinweis, bei fehlendem `lazydev` im `PATH` den Nutzer auf die Installation hinzuweisen und das Format nicht aus dem Gedächtnis nachzubauen. Unter 80 Zeilen. |
| PRD schreiben im Skill | Die Tabellenzeile lautet sinngemäß: Führe im Hauptordner des Projekts `lazydev prompt create "<Kurzfassung in einem Satz>"` aus und folge der Ausgabe Schritt für Schritt, inklusive der Klärungsfragen. Die Ausgabe ist der vollständige Prompt des Projekts. |
| Quelle von `references/spec.md` | Byte-gleiche Kopie von `SPEC.md` unter `internal/skill/files/references/spec.md`, erzeugt mit `go generate ./internal/skill`. Ein Test schlägt fehl, wenn beide Dateien abweichen. `go:embed` kann nicht über das Paketverzeichnis hinaus lesen, deshalb die Kopie. |
| Befehl `lazydev prompt create [<beschreibung>]` | Projektordner ist `git rev-parse --show-toplevel` des Arbeitsverzeichnisses, ohne Git-Repo das Arbeitsverzeichnis selbst. Gibt den Prompt auf stdout aus, Exit-Code 0. Mehrere Argumente werden mit Leerzeichen zu `$ARGUMENTS` verbunden. Ohne Argument ist `$ARGUMENTS` `Die Aufgabe ergibt sich aus dem bisherigen Gespräch mit dem Nutzer.` `$BASE` ist `Config.Base()`, `$BRANCH` ist `git.CurrentBranch`. |
| Fehler von `prompt create` | Fehlt `.lazydev/` im Projektordner: `<dir> has no .lazydev/, add the project in lazydev first`, Exit-Code 1. Der Befehl legt nichts an, anders als `n`. Fehler von `repo.Read` und `prompts.Build` (kaputtes YAML, fehlende Knowledge-Datei, fehlendes `create.md`) werden unverändert über `fail` ausgegeben. |
| Andere Schritte | Nur `create`. `lazydev prompt implement` und andere enden mit `unknown step: <step>` und der Usage, Exit-Code 2. |
| Befehl `lazydev skill install` | Schreibt den Skill in den Ordner aus der Entscheidung Ort und gibt `Installed skill lazydev to <dir>.` aus. War alles schon identisch: `Skill lazydev in <dir> is up to date.` Exit-Code 0. Fehler über `fail`. |
| Schreibregel | Jede eingebettete Datei wird nur geschrieben, wenn sie fehlt oder ihr Inhalt abweicht, atomar mit `fsutil.WriteAtomic`, Rechte `0644`, Ordner `0755`. Dateien im Skill-Ordner, die es im Binary nicht gibt, werden gelöscht, leere Unterordner danach auch. Nur innerhalb von `skills/lazydev/`. |
| Automatik in der TUI | `model.Init` startet einen `tea.Cmd`, der den Skill wie `skill install` schreibt. Erfolg und `up to date` erzeugen keine Ausgabe. Ein Fehler setzt `note` auf `skill install failed: <err>`, nur wenn `err` und `note` leer sind. Runner (`lazydev run`), `lazydev git` und die anderen Befehle schreiben den Skill nicht. |
| Abschalten | `settings.json` bekommt `"skill": false`. Fehlt der Schlüssel oder ist er `true`, schreibt die TUI den Skill. `lazydev skill install` funktioniert unabhängig davon. Entfernen bleibt Handarbeit: Ordner löschen und `"skill": false` setzen. |
| `CLAUDE.md` des Projekts | lazydev ändert kein `CLAUDE.md`. Begründung: `waitMainClean` in `internal/runner/merge.go` nimmt nur `.lazydev/` und untracked PRDs vom sauberen Hauptordner aus, eine Änderung an der getrackten `CLAUDE.md` durch `repo.Init` würde Merges blockieren, bis der Nutzer committet. |
| Tests und `$HOME` | Tests setzen `CLAUDE_CONFIG_DIR` auf `t.TempDir()`. Kein Test schreibt nach `~/.claude`. |

## Scope

**In Scope**

- Paket `internal/skill` mit eingebettetem Skill, Zielordner und Installation.
- `internal/skill/files/SKILL.md` und die Kopie `internal/skill/files/references/spec.md` mit `go generate`
  und Gleichheitstest.
- Befehle `lazydev prompt create [<beschreibung>]` und `lazydev skill install`.
- Automatische Installation beim Start der TUI, abschaltbar über `settings.json`.
- Doku in `SPEC.md`, `README.md`, `README.de.md`, `CLAUDE.md`.

**Out of Scope**

- MCP-Server, Slash-Commands, Skills im Projektordner `.claude/skills/`.
- `lazydev prompt` für andere Schritte als `create`.
- `lazydev skill remove` oder ein Dialog in der TUI.
- Änderungen an `CLAUDE.md` von Projekten durch `repo.Init`.
- Weitere Referenzdateien im Skill außer `spec.md`, etwa Auszüge für PRD, Config oder Workflow.
- Skills von Claude Code für die Runner-Schritte. Die Runner laufen weiter über `claude -p` mit `--agents`.

## Umsetzung

1. **Paket `internal/skill`** (`internal/skill/skill.go`, neu):
   - `//go:generate cp ../../SPEC.md files/references/spec.md` und `//go:embed files` als `embed.FS`.
   - `const Name = "lazydev"`.
   - `func Dir() string`: `$CLAUDE_CONFIG_DIR`, sonst `$HOME/.claude` über `os.UserHomeDir`, plus `skills/lazydev`.
   - `func Install() (changed bool, err error)`: geht `files` mit `fs.WalkDir` durch, vergleicht jede Datei mit
     der Datei unter `Dir()`, schreibt bei Abweichung oder Fehlen atomar, legt Ordner mit `os.MkdirAll` an. Danach
     geht es `Dir()` durch und löscht Dateien, die es in `files` nicht gibt, und dann leere Unterordner.
     `changed` ist `true`, wenn geschrieben oder gelöscht wurde.
   - Tests (`internal/skill/skill_test.go`) mit `t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())`: erster `Install`
     meldet `changed`, legt `SKILL.md` und `references/spec.md` an; zweiter `Install` meldet nicht `changed`; eine
     geänderte `SKILL.md` wird zurückgesetzt; eine fremde Datei `references/alt.md` wird gelöscht; `Dir` ohne
     `CLAUDE_CONFIG_DIR` endet auf `.claude/skills/lazydev`. `TestSpecCopy` liest `../../SPEC.md` und
     `files/references/spec.md` und schlägt mit `references/spec.md differs from SPEC.md, run go generate
     ./internal/skill` fehl, wenn sie abweichen. `TestSkillFrontmatter` prüft, dass `SKILL.md` mit `---` beginnt
     und `name: lazydev` sowie `description:` enthält.

2. **Skill-Dateien** (`internal/skill/files/SKILL.md`, neu; `internal/skill/files/references/spec.md`, erzeugt):
   - `SKILL.md` nach der Produktentscheidung Inhalt. Die `description` nennt deutsche und englische Auslöser, z. B.
     `lazydev`, `lazydev PRD`, `PRD erstellen`, `mach daraus ein PRD`, `prd/`, `> Status:`,
     `.lazydev/config.yaml`, `Hooks`, `Worktree`, `Run`.
   - `spec.md` einmal mit `go generate ./internal/skill` erzeugen und einchecken.

3. **Befehl `prompt create`** (`cmd/lazydev/main.go`):
   - `case "prompt":` verlangt mindestens ein Argument, sonst `usageError`. Ist `rest[0]` nicht `create`:
     `unknown step: <step>` auf stderr und `usageError`.
   - `func promptCreate(args []string) error`: Ordner über `git.Run(cwd, "rev-parse", "--show-toplevel")`, bei
     Fehler `cwd`. Prüft `fsutil.IsDir(filepath.Join(dir, repo.Dir))`, sonst der Fehler aus den Entscheidungen.
     `info := repo.Read(dir, filepath.Base(dir))`, `info.Err` zurückgeben. `prompts.Build(dir, dir, info.Config,
     "create", prompts.Vars{Arguments: …, Base: info.Config.Base(), Branch: git.CurrentBranch(dir)})`, Ergebnis
     mit `fmt.Print` ausgeben.
   - `usage` bekommt die Zeilen
     `  lazydev prompt create [<description>]   print the create prompt of the project in the current folder`
     und `  lazydev skill install                    install the Claude Code skill lazydev into ~/.claude/skills/`.

4. **Befehl `skill install`** (`cmd/lazydev/main.go`):
   - `case "skill":` verlangt genau `install`, sonst `usageError`. Ruft `skill.Install()` und gibt die Meldung aus
     den Entscheidungen aus.

5. **Einstellung `skill`** (`internal/config/config.go`):
   - `Settings` bekommt das Feld `Skill *bool` mit dem Tag `json:"skill,omitempty"` und dem Kommentar: `nil` und `true` lassen die TUI den
     Skill beim Start schreiben.
   - `func (s Settings) SkillEnabled() bool`: `s.Skill == nil || *s.Skill`.
   - Test in `internal/config/config_test.go`: `{}` ist enabled, `{"skill": false}` nicht, `{"skill": true}` schon.

6. **Automatik in der TUI** (`internal/tui/skill.go`, neu, und `internal/tui/main.go`):
   - `type skillMsg struct{ err error }` und `func installSkill(s config.Settings) tea.Cmd`: gibt `nil` zurück,
     wenn `!s.SkillEnabled()`. Sonst ruft der Cmd `skill.Install()` und liefert `skillMsg` nur bei Fehler, sonst
     `nil`.
   - `Init` hängt `installSkill(m.settings)` an `tea.Batch` an. Heißt das Feld mit den Einstellungen anders, das
     bestehende verwenden.
   - `Update` behandelt `skillMsg`: setzt `note` auf `skill install failed: <err>`, wenn `err` und `note` leer
     sind.
   - Tests (`internal/tui/skill_test.go`): `installSkill` mit `skill: false` gibt `nil` zurück; mit
     `CLAUDE_CONFIG_DIR` auf `t.TempDir()` legt der Cmd `SKILL.md` an und liefert `nil`; `skillMsg` setzt `note`,
     mit gesetzter `err` bleibt `note` leer. Tests der TUI, die `Init` aufrufen, setzen `CLAUDE_CONFIG_DIR`.

7. **Doku**:
   - `SPEC.md`: Abschnitt `## CLI` nennt `prompt create [<description>]` und `skill install`. Neuer Abschnitt
     `### Claude Code skill` unter `## Processes` mit Ort, Dateien, Schreibregel, Zeitpunkt der Automatik, dem
     Footer-Hinweis bei Fehler, der Einstellung `skill` und dem Verhalten von `prompt create` (Ordner, Argumente,
     Fehler). Die Tabelle unter `## Data` bekommt die Zeile `$CLAUDE_CONFIG_DIR/skills/lazydev/` (ohne Variable
     `~/.claude/skills/lazydev/`) mit `SKILL.md`, `references/spec.md`. Die Stelle, die die Felder von
     `settings.json` nennt, bekommt `skill`.
   - `README.md` und `README.de.md`: neuer Abschnitt `## Claude Code skill` nach `## Install`: was der Skill kann,
     ein Beispiel „mach daraus ein lazydev PRD“, dass die TUI ihn aktuell hält, `lazydev skill install` für die
     Shell, `"skill": false` zum Abschalten. Abschnitt `## CLI` bekommt beide Befehle. Die Tabelle unter `## Files`
     nennt `skill` bei `settings.json` und bekommt die Zeile `~/.claude/skills/lazydev/`.
   - `CLAUDE.md`: Pakettabelle bekommt `skill` mit `Claude Code skill: embedded files, install into
     ~/.claude/skills/lazydev/`. Abschnitt Commands bekommt `go generate ./internal/skill` mit dem Hinweis, dass er
     nach jeder Änderung an `SPEC.md` läuft und ein Test das prüft.

## Technische Hinweise

- Alle UI-Strings, Fehlermeldungen, Kommentare und Doku auf Englisch, außer `README.de.md`. `SKILL.md` ist wie
  Prompts, Contracts und Agents deutsch. Das PRD bleibt deutsch.
- `README.md` und `README.de.md` bekommen denselben Inhalt.
- `go:embed` folgt keinen Symlinks und liest nichts außerhalb des Paketverzeichnisses, deshalb die Kopie von
  `SPEC.md` mit `go generate` und dem Test als Wächter. `make test` führt `go generate` nicht aus, der Test meldet
  die veraltete Kopie.
- `//go:embed files` bettet den Ordner mit allen Dateien ein, auch `references/spec.md`. Dateien, die mit `.` oder
  `_` beginnen, lässt `go:embed` ohne `all:`-Präfix aus, der Skill braucht keine.
- Der Vergleich beim Schreiben ist `bytes.Equal` der kompletten Datei. Keine Zeitstempel, keine Versionsnummer in
  den Dateien, so ist ein `dev`-Build genauso aktuell wie ein Release.
- Löschen fremder Dateien nur unter `Dir()`, und `Dir()` endet immer auf `skills/lazydev`. Prüfe das im Code mit
  einem Kommentar, nicht mit einer zweiten Abfrage.
- `lazydev prompt create` läuft dort, wo Claude Code läuft, also im Hauptordner. `prompts.Build` prüft die
  Knowledge-Dateien in `workDir`, hier ebenfalls `dir`. `Header` setzt die erste Zeile von `$ARGUMENTS` in den
  Header, eine lange Kurzfassung erscheint dort gekürzt auf die erste Zeile. Das ist gewollt.
- `repo.Read` braucht eine ID nur für `Info.ID`. `filepath.Base(dir)` genügt, die Projektliste wird nicht gelesen,
  so funktioniert der Befehl auch in Projekten, die nicht in `projects.json` stehen.
- Die TUI-Tests laufen mit `t.Setenv`, deshalb ohne `t.Parallel()` in Tests, die `CLAUDE_CONFIG_DIR` setzen.
- `CLAUDE_CONFIG_DIR` ist die Variable, mit der Claude Code seinen Konfigurationsordner `~/.claude` verlegt. Skills
  liegen dort unter `skills/<name>/SKILL.md`.
- Kein neues Go-Modul als Abhängigkeit.

## Erfolgskriterien

- `make test` läuft ohne Fehler, inklusive der neuen Tests in `internal/skill`, `internal/config` und
  `internal/tui`.
- `cmp SPEC.md internal/skill/files/references/spec.md` meldet keinen Unterschied.
- `go run ./cmd/lazydev prompt create "Testlauf"` im Repo von lazydev gibt einen Text aus, der mit
  `# lazydev create: Testlauf` beginnt, `## Regeln von lazydev` enthält und mit
  `Nenne zum Schluss den Zielpfad des PRDs.` endet.
- `go run ./cmd/lazydev prompt create` in einem Ordner ohne `.lazydev/` endet mit Exit-Code 1 und
  `lazydev: <dir> has no .lazydev/, add the project in lazydev first`.
- `go run ./cmd/lazydev prompt implement` endet mit Exit-Code 2 und `unknown step: implement`.
- `CLAUDE_CONFIG_DIR=$(mktemp -d) go run ./cmd/lazydev skill install` legt `skills/lazydev/SKILL.md` und
  `skills/lazydev/references/spec.md` an und meldet `Installed skill lazydev to <dir>.` Ein zweiter Aufruf meldet
  `Skill lazydev in <dir> is up to date.`
- Nach einem Start der TUI mit `CLAUDE_CONFIG_DIR` auf einen leeren Ordner existiert dort `skills/lazydev/SKILL.md`.
  Mit `"skill": false` in `settings.json` existiert er nicht.
- In einer Claude-Code-Session im Hauptordner eines lazydev-Projekts führt „mach daraus ein lazydev PRD“ dazu, dass
  Claude den Skill lädt, `lazydev prompt create` ausführt und mit den nummerierten Klärungsfragen beginnt. Das ist
  eine manuelle Prüfung, der Bericht nennt sie.
- `README.md`, `README.de.md`, `SPEC.md` und `CLAUDE.md` beschreiben Skill, Automatik, Einstellung und beide
  Befehle.
