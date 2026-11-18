# PRD: Sprache der Prompts und PRDs: Englisch oder Deutsch

> Status: umgesetzt (2026-11-18) · Kosten: 15.00 USD · Modell: claude-opus-5-5 · Scope: Neues Feld `language: en | de` in `.lazydev/config.yaml`, Standard `en`. Es wählt die Sprache der eingebauten Verträge, der Vorlagen des einfachen Workflows, der eingebauten Agents und aller Wörter, die lazydev in PRDs und Berichte schreibt (Statuswerte, `## Blocker`, Kosten, Modell). Beim Lesen versteht lazydev immer beide Sprachen, sodass bestehende deutsche PRDs und Berichte weiter funktionieren.

## Context

lazydev ist heute fest auf Deutsch: Die Verträge unter `internal/prompts/contract/`, die Vorlagen unter
`internal/repo/workflows/simple/`, die Agents unter `internal/claude/agents/` und der Skill sind deutsch.
Der Runner wertet deutsche Wörter aus und schreibt sie selbst. Für englischsprachige Teams und Repos ist
lazydev damit kaum nutzbar: PRDs, Berichte und Statuszeilen landen auf Deutsch im Repo.

Ziel: Pro Projekt einstellbar, ob lazydev auf Englisch oder Deutsch arbeitet. Englisch ist der Standard.
Lesen ist immer zweisprachig. Ein Projekt kann deshalb die Sprache wechseln, ohne dass alte PRDs
(`umgesetzt (2026-10-14)`) ihren Status verlieren. Antwortet Claude in der anderen Sprache, scheitert der
Schritt nicht.

Heute fest deutsche Wörter, die lazydev auswertet oder schreibt:

| Datei | Wörter |
|---|---|
| `internal/project/status.go` | `geplant`, `umgesetzt`, `verworfen`, `blockiert`, Datums-Regex `statusDateRe`, `## Blocker`, `Schritt:`, `Kosten:`, `Modell:` |
| `internal/project/change.go` | `Choices` (Statuswerte des Status-Pickers und von `set-status`) |
| `internal/runner/claudestep.go` | `## Ergebnis`, `umgesetzt`, `verifiziert`, `abgebrochen`, `bereit für fast-forward`, `## Offene Fragen`, `## Belege`, `keine` in `reportPoints` |
| `internal/runner/merge.go` | `## Offene Punkte` |
| `internal/runner/resolve.go` | `geklärt`, `## Entscheidungen`, `## Offene Fragen` |
| `internal/runner/plan.go` | `## Reihenfolge`, `Nach:`, `Grund:`, `keine` |
| `internal/prompts/prompts.go` | `## Pflichtlektüre` und der Satz darunter |

Wiederverwendet werden: `repo.Config`, `repo.Read`, `repo.Init`, die Validierung nach dem Muster von
`validateMerge` (`internal/repo/repo.go`), `prompts.Build`, `Plan`, `Resolve`, `Auto`, `Header`
(`internal/prompts/prompts.go`), `claude.Section`, `claude.FirstLine`, `WriteAgents`
(`internal/claude/run.go`, `internal/claude/agents.go`), `project.ParseStatusLine`, `SetStatus`,
`SetBlocker`, `RemoveBlocker`, `Blocker`, `RunInfo`, `IsRunInfo`, `SetRunInfo`, `ChangeStatus`,
`checkReport`, `reportPoints`, `parsePlan`, `recordBlocker`, `resolveStep`, das Fake-`claude`-Skript in
`internal/runner/helpers_test.go`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Schalter | Feld `language` in `.lazydev/config.yaml`, Werte `en` und `de`. Fehlt es oder ist es leer, gilt `en`. Ein anderer Wert ist der Config-Fehler `config.yaml: language must be en or de, not "<wert>"`, angezeigt wie kaputtes YAML in der Detailansicht, Läufe starten nicht. Methode `Config.Lang()` liefert `en` ohne Wert. |
| Ort der Einstellung | Pro Projekt in `config.yaml`, nicht in `settings.json`: die Sprache gehört zu den PRDs im Repo und ist für das ganze Team gleich. |
| Trust | `language` geht nicht in `TrustHash` ein. |
| Standard für bestehende Projekte | Projekte ohne `language` arbeiten nach dem Update auf Englisch. Ihre vorhandenen `.lazydev/<step>.md` bleiben unverändert (sie gehören dem Projekt), nur Verträge, Agents und geschriebene Wörter wechseln. Wer Deutsch behalten will, setzt `language: de`. README und SPEC nennen das ausdrücklich. |
| Lesen | Immer beide Sprachen, unabhängig von `language`: Statuswerte, Datum, `## Blocker` mit `Schritt:` oder `Step:`, `Kosten:`/`Cost:`, `Modell:`/`Model:`, Überschriften und Werte der Berichte, die Antwort von plan, `keine`/`none`. Groß- und Kleinschreibung der Ergebniswerte zählt nicht (wie heute `strings.ToLower`). |
| Schreiben | Nur in der Sprache von `language`: Statuswerte in `recordBlocker`, `Discard`, `ChangeStatus` und `set-status`, `## Blocker` mit `Schritt:`/`Step:`, Kosten und Modell in `SetRunInfo`. `SetRunInfo` ersetzt ein vorhandenes Feld der anderen Sprache an seiner Stelle, statt ein zweites anzulegen. `SetStatus` behält die Detail-Felder unverändert. |
| Englisches Vokabular | Status: `planned`, `implemented (YYYY-MM-DD)`, `discarded (YYYY-MM-DD)`, `blocked (YYYY-MM-DD)`. Statuszeile: `> Status: … · Cost: 4.12 USD · Model: … · Scope: …`. Blocker: `## Blocker`, `Step: <step>`. Berichte: `## Result` mit `implemented`, `verified`, `aborted`, `ready for fast-forward`, `resolved`; `## Open questions`, `## Open points`, `## Evidence`, `## Decisions`, leere Liste `none`. plan: `## Order`, `After: <prd>, …` oder `After: none`, `Reason: <satz>`. Wissensblock: `## Required reading` mit `Read these files completely before you do anything else:`. |
| Deutsches Vokabular | Genau das heutige, unverändert. |
| Zentrale Stelle | Neues Paket `internal/lang` mit `type Lang string`, Konstanten `EN`, `DE`, `Default = EN`, `Parse(string) (Lang, error)` und einem Struct `Vocab` mit allen Wörtern oben, `For(Lang) Vocab` und `All() []Vocab` für das Lesen. `project`, `runner` und `prompts` nehmen ihre Wörter nur noch von dort. Die heutigen Konstanten (`ValuePlanned` usw.) entfallen zugunsten von `Vocab`. |
| Verträge | `internal/prompts/contract/en/*.md` und `contract/de/*.md`, gleiche Dateinamen wie heute. Die deutschen sind die heutigen Dateien unverändert verschoben. Die englischen sind vollständige, inhaltlich gleiche Übersetzungen mit dem englischen Vokabular. `Build` wählt nach `cfg.Lang()`; `Plan`, `Resolve` und `Auto` bekommen die Sprache als Parameter. |
| PRD-Struktur im englischen create-Vertrag | `# PRD: <title>`, Statuszeile mit `planned`, `## Context`, `## Product decisions (final)`, `## Scope` (**In scope**, **Out of scope**), `## Implementation`, `## Technical notes`, `## Success criteria`, im autonomen Modus `## Decisions`. Der englische Vertrag sagt „write the PRD in English“, verify und review verweisen auf `## Success criteria`. Deutsch bleibt wie heute. |
| Vorlagen des einfachen Workflows | `internal/repo/workflows/simple/en/*.md` und `simple/de/*.md`. `repo.Init` und die Vorlage für `o`/`r` im Wizard nehmen die Sprache aus `config.yaml` (ohne Datei oder Feld: `en`). Init legt `language` nicht in `config.yaml` an. |
| Agents | `internal/claude/agents/en/*.md` und `agents/de/*.md`, gleiche Namen (`committer`, `structural-completeness-reviewer`). `Agents` und `WriteAgents` bekommen die Sprache, der Aufruf von Claude (`claude.Options`) trägt sie. Die geschriebene JSON-Datei hängt von der Sprache ab und darf zwischen Sprachen nicht kollidieren. |
| Kopfzeile | `# lazydev <step>: <argument>` bleibt in beiden Sprachen gleich. Das Fake-`claude`-Skript erkennt den Schritt weiter daran. |
| Wizard | Seite 1 (Basis-Branch) zeigt unter der Liste `Language: en` bzw. `Language: de`, Taste `l` schaltet um, der Footer nennt `l language`. Die Zusammenfassung zeigt das Feld. Speichern schreibt `language: de` und entfernt das Feld bei `en` (wie `baseBranch` bei `main`), andere Felder, Reihenfolge und Kommentare bleiben. `o` und `r` auf der Seite Prompts nutzen die im Wizard gewählte Sprache. Vorhandene Prompts übersetzt der Wizard nicht. |
| TUI-Anzeige | Die UI bleibt Englisch. Die Projektansicht der Detailansicht zeigt `language en` bzw. `language de` neben Basis-Branch und `merge`. Statuswerte zeigt die TUI wie heute aus der Datei. |
| CLI | `lazydev prompt create` und `lazydev prompt unblock` (auch mit `--auto`) nutzen die Sprache des Projekts. `lazydev set-status` schreibt in der Sprache des Projekts. Keine neuen Flags. |
| Skill | `internal/skill/files/SKILL.md` wird Englisch und nennt die Statuswerte beider Sprachen mit dem Hinweis, dass `language` in `.lazydev/config.yaml` entscheidet. Die `description` enthält die bisherigen deutschen Auslöser („PRD erstellen“, „mach daraus ein PRD“ usw.) zusätzlich zu englischen, damit der Skill in beiden Sprachen greift. |
| Mod `lazydev-runs` | Unverändert, er wertet keine deutschen Wörter aus. Liest er Status oder Blocker, gilt dasselbe zweisprachige Lesen. |
| Commits von lazydev | Bleiben wie heute (Englisch, ohne Claude, ohne Trailer). |
| Dieses Repo | Die eigene `.lazydev/` von lazydev ändert dieses PRD nicht (Regel: nichts unter `.lazydev/`). |

## Scope

**In Scope**

- Feld `language` mit Validierung, `Config.Lang()`.
- Paket `internal/lang` mit beiden Vokabularen.
- Zweisprachiges Lesen und Schreiben in der eingestellten Sprache in `project`, `runner`, `prompts`, `tui`
  und `cmd/lazydev`.
- Englische Übersetzung aller Verträge, Vorlagen und Agents, deutsche Fassungen verschoben.
- Wizard-Schalter, Anzeige in der Projektansicht.
- Skill auf Englisch mit zweisprachigen Auslösern.
- `SPEC.md`, `README.md`, `README.de.md`, `CLAUDE.md` (Konvention zu deutschen Prompts), Kopie der Spec
  (`go generate ./internal/skill`).
- Tests für beide Sprachen.

**Out of Scope**

- Weitere Sprachen. Das Paket `lang` ist so gebaut, dass eine dritte Sprache nur ein neues `Vocab` und neue
  Ordner braucht, aber keine wird jetzt angelegt.
- Übersetzung der UI der TUI und der Fehlermeldungen (bleiben Englisch).
- Automatische Übersetzung vorhandener PRDs oder vorhandener `.lazydev/<step>.md`.
- Eine Sprache in `settings.json` oder pro PRD.
- Änderungen an `.lazydev/` dieses Repos.

## Umsetzung

1. **Spec zuerst.** `SPEC.md`: `.lazydev/` (Feld `language`, Fehlertext, Init nach Sprache), Prompt
   (Wissensblock in beiden Sprachen, Satz „Prompts, contracts and PRD status values are German“ ersetzen durch
   die Regel zu `language`), Wizard (Taste `l`, Speichern), PRDs (Statuswerte beider Sprachen, Lesen
   zweisprachig, Schreiben in `language`), Blocked runs (`Step:`/`Schritt:`), Run (plan-Antwort beider
   Sprachen), Claude Code skill (Englisch, zweisprachige Auslöser), UI (Projektansicht). Danach
   `go generate ./internal/skill`. `README.md` und `README.de.md` mit gleichem Inhalt anpassen, inklusive
   Beispiel `language: de` im YAML-Block und Hinweis zum Standard `en` für bestehende Projekte. `CLAUDE.md`:
   Konvention „Prompts, contracts, agents and PRD status values stay German“ ersetzen durch „exist in English
   and German under `en/` and `de/`, change both“.
2. **Paket `internal/lang`.** `lang.go` mit `Lang`, `EN`, `DE`, `Default`, `Parse`, `Vocab`, `For`, `All`.
   Tests: `Parse` mit `""`, `en`, `de`, `fr`; jedes Feld beider Vokabulare nicht leer.
3. **Konfiguration.** `internal/repo/repo.go`: Feld `Language string` mit yaml `language`, `Lang()`,
   Validierung in `Read` über `lang.Parse`. Vorlagen nach `workflows/simple/en/` und `simple/de/`, `Init` und
   die Vorlagenfunktion bekommen die Sprache. Die Bearbeitung des Wizards (`language` setzen und bei `en`
   entfernen) dort, wo heute `baseBranch` geschrieben wird. Tests in `internal/repo/repo_test.go`.
4. **PRD-Status.** `internal/project/status.go`: `ParseStatusLine`, `statusDateRe`, `Blocker`,
   `blockerSection`, `RunInfo`, `IsRunInfo` lesen über `lang.All()`. `SetBlocker` und `SetRunInfo` bekommen
   `lang.Lang`. `internal/project/change.go`: `Choices` bzw. `ValueAt` liefern den Wert in der Sprache des
   Projekts, `ChangeStatus` bekommt die Sprache. Tests in `status_test.go` und `change_test.go` für beide
   Sprachen, gemischte Fälle (deutsche Zeile, englisches `SetRunInfo`).
5. **Prompts.** `internal/prompts/prompts.go`: Embed `contract/en/*.md` und `contract/de/*.md`, Auswahl nach
   Sprache in `Build`, `Plan`, `Resolve`, `Auto`, Wissensblock aus `Vocab`. Englische Verträge schreiben
   (create, create.auto, implement, verify, review, review.pr, merge, plan, resolve, unblock, unblock.auto).
   Tests in `prompts_test.go` für beide Sprachen, darunter: kein deutsches Wort des Vokabulars in einem
   englischen Vertrag.
6. **Agents.** `internal/claude/agents/en/` und `agents/de/`, `Agents(lang)`, `WriteAgents(lang)`,
   `claude.Options` um die Sprache. Englische Agents übersetzen. Tests in `internal/claude`.
7. **Runner.** `claudestep.go`: `checkReport`, `checkVerify`, `evidenceFiles`, `reportPoints` lesen beide
   Vokabulare; Konstanten `result*` entfallen zugunsten von `Vocab`. `merge.go`, `resolve.go`, `plan.go`
   (`parsePlan` versteht `## Order`/`## Reihenfolge`, `After:`/`Nach:`, `Reason:`/`Grund:`, `none`/`keine`),
   `blocker.go` und `discard.go` schreiben in `cfg.Lang()`. `runClaude`, `callClaude`, plan und resolve geben
   die Sprache an Prompts und Agents weiter.
8. **TUI und CLI.** `internal/tui/status.go`, `unblock.go`, `create.go`, der Wizard (`l`, Anzeige,
   Zusammenfassung, Vorlagen in der gewählten Sprache), die Projektansicht in `runview.go`.
   `cmd/lazydev/main.go` und `cli.go` für `prompt`, `set-status`. Tests in den zugehörigen `_test.go`.
9. **Skill.** `internal/skill/files/SKILL.md` auf Englisch mit zweisprachiger `description`.
10. **Runner-Tests.** Das Fake-`claude`-Skript in `internal/runner/helpers_test.go` antwortet in der Sprache
    aus einer Umgebungsvariable des Tests (`FAKE_CLAUDE_LANG`, ohne Wert Englisch) und liest Pfade und Werte
    weiter wie heute. Szenarien: ein vollständiger Lauf (plan bis merge) mit `language` leer, einer mit
    `language: de`; Blocker und resolve in beiden Sprachen; ein Projekt mit `language: en`, dessen PRD eine
    deutsche Statuszeile hat und in dem Claude deutsch antwortet, läuft trotzdem durch; `SetRunInfo` schreibt
    `Cost:`/`Model:` bei `en` und `Kosten:`/`Modell:` bei `de`.

## Technische Hinweise

- `SPEC.md` ist verbindlich und wird zuerst geändert. `internal/skill/files/references/spec.md` muss gleich
  `SPEC.md` sein, sonst schlägt `TestSpecCopy` fehl. `make test` ruft kein `go generate` auf.
- Alle Texte in Code, UI, Fehlern und Doku sind Englisch. `README.de.md` immer mit `README.md` ändern.
- Zweisprachiges Lesen macht die meisten bestehenden Tests mit deutschen Fake-Antworten weiter gültig. Tests,
  die Prompt-Texte prüfen (z. B. `## Pflichtlektüre` in `runner_test.go` und `prompts_test.go`), müssen die
  Sprache setzen oder das englische Pendant erwarten.
- Die Datums-Regex muss beide Sprachen kennen: `(?:umgesetzt|verworfen|blockiert|implemented|discarded|blocked)`,
  gebaut aus `lang.All()`, nicht von Hand.
- `IsRunInfo` wird in der TUI genutzt, um Kosten und Modell aus dem Scope zu filtern. Es muss beide Sprachen
  erkennen.
- `go:embed` braucht pro Ordner ein Muster, z. B. `//go:embed contract/en/*.md contract/de/*.md`.
- `WriteAgents` schreibt heute eine Datei für `--agents`. Nach Sprache getrennt schreiben, damit parallele
  Läufe verschiedener Projekte sich nicht überschreiben.
- Jeder Schritt bleibt wiederholbar. Ein Wechsel von `language` zwischen zwei Schritten eines Laufs ist
  erlaubt, weil Lesen zweisprachig ist.
- Runner-Tests laufen `runner.Run` im Vordergrund gegen temporäre Repos. Isolation über `XDG_STATE_HOME`,
  `XDG_DATA_HOME`, `XDG_CONFIG_HOME`, Git-Tests setzen `GIT_AUTHOR_*` und `GIT_COMMITTER_*`.

## Erfolgskriterien

- `make test` ist grün, `go vet ./...` ohne Befund.
- `TestSpecCopy` ist grün nach `go generate ./internal/skill`.
- Ein Projekt ohne `language` bekommt englische Verträge, englische Vorlagen bei Init und schreibt
  `implemented (…)`, `blocked (…)`, `Step:`, `Cost:`, `Model:`; nachgewiesen durch Runner-Tests.
- Ein Projekt mit `language: de` verhält sich exakt wie heute; nachgewiesen durch einen vollständigen
  Runner-Test auf Deutsch.
- `language: fr` liefert `config.yaml: language must be en or de, not "fr"`.
- Eine deutsche Statuszeile (`umgesetzt (2026-10-14) · Kosten: 1.00 USD · Modell: x`) wird in einem
  englischen Projekt als implementiert mit Datum, Kosten und Modell erkannt, ebenso umgekehrt.
- `parsePlan` erkennt die Antwort in beiden Sprachen.
- Kein englischer Vertrag, keine englische Vorlage und kein englischer Agent enthält ein Wort des deutschen
  Vokabulars (Test).
- Wizard: `l` schaltet die Sprache, Speichern schreibt `language: de` bzw. entfernt das Feld bei `en`.
- `SPEC.md`, `README.md`, `README.de.md`, `CLAUDE.md` und `SKILL.md` beschreiben das Verhalten.

## Entscheidungen

1. Wo wird die Sprache eingestellt? A) `config.yaml` pro Projekt, B) `settings.json` global, C) pro PRD,
   D) Umgebungsvariable. **A**: Die Sprache gehört zu den PRDs im Repo und ist im Team gleich, wie `merge`.
2. Wie wird gelesen? A) beide Sprachen immer, B) nur eingestellte, C) beide nur für Statuszeilen, D) Sprache
   aus dem PRD erkennen. **A**: Wechsel der Sprache und Antworten in der falschen Sprache brechen so nichts,
   die Wörter beider Sprachen überschneiden sich nicht.
3. Standard ohne `language`? A) `en`, B) `de`, C) Fehler, D) aus vorhandenen PRDs raten. **A**: Ausdrücklicher
   Wunsch des Nutzers. Bestehende deutsche Projekte setzen `language: de`, alte PRDs bleiben lesbar.
4. Skill und Agents, die global bzw. pro Aufruf gelten? A) Agents pro Sprache, Skill Englisch mit
   zweisprachigen Auslösern, B) alles nur Englisch, C) alles zweisprachig in einer Datei, D) Skill pro
   Sprache installieren. **A**: Agents laufen pro Projekt und können die Sprache nehmen; der Skill liegt einmal
   in `~/.claude/skills/` und delegiert Prompts ohnehin an `lazydev prompt`, das die Projektsprache kennt.
5. Wie wählt man die Sprache in der TUI? A) Taste `l` auf Wizard-Seite 1, B) eigene Wizard-Seite, C) nur von
   Hand in `config.yaml`, D) Frage bei `lazydev add`. **A**: kleinste Änderung, die Seitenzahl `1/6`–`6/6` und
   die Tasten der anderen Seiten bleiben.
6. Was passiert mit vorhandenen `.lazydev/<step>.md` beim Wechsel? A) bleiben unverändert, B) werden
   überschrieben, C) Rückfrage, D) Warnung in der Detailansicht. **A**: Die Dateien gehören dem Projekt, Init
   überschreibt nie; `r` im Wizard setzt eine Datei auf Wunsch in der gewählten Sprache zurück.
