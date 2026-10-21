# PRD: `.lazydev/` mit dem Wizard bearbeiten

> Status: umgesetzt (2026-10-21) · Scope: `i` öffnet den Wizard auch, wenn `.lazydev/` schon existiert. Er zeigt die aktuellen Einstellungen, ändert Ziel-Branch und Knowledge in `.lazydev/config.yaml` und hat eine neue Seite „Prompts“, die die Prompt-Dateien im Editor öffnet oder auf den Standard zurücksetzt.

## Context

Der Wizard (`internal/tui/initwizard.go`) legt `.lazydev/` einmalig an. Existiert der Ordner, zeigt `i`
nur `.lazydev/ existiert schon. Bearbeite die Dateien im Editor.` Danach lassen sich Ziel-Branch,
Knowledge und Prompts nur außerhalb von lazydev ändern. Für den Ziel-Branch gibt es `b`, für Knowledge und
Prompts nichts.

Nach der Änderung öffnet `i` den Wizard immer. Bei vorhandenem `.lazydev/` startet er im Modus
„bearbeiten“: Die Seiten sind mit den aktuellen Werten vorbelegt, und eine zusätzliche Seite „Prompts“
öffnet `create.md`, `implement.md`, `review.md` und `merge.md` im Editor. So lässt sich zum Beispiel der
Erstellen-Prompt eines Projekts direkt aus lazydev anpassen.

Im Repo existiert bereits und wird wiederverwendet:

- `initWizard`, `openInitWizard`, `updateInitWizard`, `viewInitWizard`, `writeInitWizard`,
  `knowledgeCandidates`, `wizardLine`, `scrollWindow` (`internal/tui/initwizard.go`).
- `newBranchPicker` (`internal/tui/branch.go`).
- `config.LoadRepo`, `config.SetRepoBaseBranch`, `config.RepoPath`, `config.RepoDir`, `mappingOf`,
  `writeFileAtomic` (`internal/config/repo.go`).
- `prompts.Steps`, `prompts.Default`, `prompts.FileName`, `prompts.Knowledge` (`internal/prompts/prompts.go`).
- `editorCommand`, `openPRDEditor`, `openInteractive`, `execInteractive`, `herdr.Available()`
  (`internal/tui/prdview.go`, `internal/tui/interactive.go`).
- `Model.ask` (`internal/tui/actions.go`).

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Taste | `i` öffnet den Wizard immer. Ohne `.lazydev/` im Modus „anlegen“ wie heute, mit `.lazydev/` im Modus „bearbeiten“. |
| Titel | Anlegen: `.lazydev/ anlegen · <projekt>` wie heute. Bearbeiten: `.lazydev/ bearbeiten · <projekt>`. |
| Seiten beim Anlegen | Unverändert: Ziel-Branch, Knowledge-Dateien, Schritte je Datei, Zusammenfassung. |
| Seiten beim Bearbeiten | Ziel-Branch, Knowledge-Dateien, Schritte je Datei, Prompts, Zusammenfassung. Nummerierung in den Überschriften 1 bis 5. |
| Ungültige `config.yaml` | Liefert `LoadRepo` einen Fehler, öffnet `i` nichts und zeigt den Fehler in der Statuszeile. |
| Seite Ziel-Branch | Vorgewählt und mit ` (aktuell)` markiert ist `baseBranch` aus `.lazydev/config.yaml`, ohne Angabe `main`. |
| Seite Knowledge-Dateien | Kandidaten wie heute (`knowledgeCandidates`). Markiert sind alle Dateien, die in `knowledge` unter irgendeinem Schritt oder `all` stehen. Steht eine Datei in `knowledge`, ist aber kein Kandidat, erscheint sie trotzdem, markiert, mit ` (nicht getrackt)`. Sie bleibt erhalten, solange sie markiert bleibt. |
| Seite Schritte je Datei | Vorbelegt aus `knowledge`: Eine Datei unter `all` hat alle vier Schritte, sonst die Schritte, unter denen sie steht. Neu markierte Dateien bekommen wie heute create und implement. |
| Seite Prompts | Eine Zeile pro Schritt in der Reihenfolge `prompts.Steps`: `<schritt>  <datei>  <zustand>`. Zustand `eigener Prompt`, wenn die Datei existiert und vom Standard abweicht, `Kopie des Standards`, wenn sie existiert und dem Standard gleicht, sonst `Standard von lazydev`. |
| Tasten auf der Seite Prompts | `j`/`k` wählen, `o` öffnet die Datei im Editor, `r` setzt auf den Standard zurück, `Enter` weiter zur Zusammenfassung, `Backspace` zurück, `Esc` bricht ab. |
| Editor | Wie `o` in der PRD-Ansicht: `editorCommand()` mit dem absoluten Pfad, Arbeitsordner ist der Hauptordner. In Herdr ein neuer Pane, sonst unterbricht lazydev die Oberfläche bis zum Ende des Editors. Fehlt die Datei, legt lazydev sie vorher mit `prompts.Default(step)` an. |
| Nach dem Editor | Ohne Herdr bestimmt lazydev die Zustände der Seite Prompts neu und bleibt auf dieser Seite. In Herdr aktualisiert die Seite ihre Zustände bei jedem Tastendruck auf der Seite. |
| Zurücksetzen | `r` fragt `<datei> löschen und den Standard von lazydev nutzen? (j/n)`. Nach `j` löscht lazydev die Datei. Bei `Standard von lazydev` bewirkt `r` nichts. |
| Sofortige Wirkung | Editor und Zurücksetzen wirken sofort auf die Dateien, auch wenn du den Wizard danach mit `Esc` abbrichst. Die Seite zeigt das als Hinweis unter der Überschrift: `Änderungen an Prompts gelten sofort.` |
| Zusammenfassung beim Bearbeiten | Zeigt die künftige `config.yaml` und darunter `Prompts: <schritt>: <zustand>` für alle vier Schritte. `Enter` speichert. |
| Speichern beim Bearbeiten | lazydev ändert in `.lazydev/config.yaml` nur `baseBranch` und `knowledge` über `yaml.Node`. `preset`, `allowedTools`, `hooks`, Kommentare und die Reihenfolge der übrigen Felder bleiben. `baseBranch: main` entfällt, eine leere `knowledge` entfällt. Eine Datei mit allen vier Schritten steht unter `all`. |
| Keine Änderung | Ergeben Ziel-Branch und Knowledge denselben Inhalt wie vorher, schreibt lazydev nichts und meldet `Keine Änderung an .lazydev/config.yaml`. |
| Meldung nach dem Speichern | `.lazydev/config.yaml gespeichert. Committe die Änderung, damit das Team sie bekommt.` lazydev committet nicht. |
| Zentrale `config.yaml` | Der Modus „bearbeiten“ ändert die zentrale `config.yaml` nicht. |
| Vertrauen | Der Wizard ändert weder `allowedTools` noch `hooks`. Eine Bestätigung beim nächsten `s` entsteht dadurch nicht. |
| Fußzeile | Je Seite die Tasten der Seite, auf der Seite Prompts `↑ ↓ wählen · o Editor · r Standard · Enter weiter · Backspace zurück · Esc abbrechen`. |
| Hilfe | Eintrag zu `i`: `.lazydev/ anlegen oder bearbeiten: Ziel-Branch, Knowledge, Prompts`. |

## Scope

**In Scope**

- Modus „bearbeiten“ im Wizard mit vorbelegten Seiten.
- Seite „Prompts“ mit Editor und Zurücksetzen.
- Speichern von `baseBranch` und `knowledge` unter Erhalt der übrigen Datei.
- Tests, Hilfe, Fußzeile, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- `allowedTools`, `hooks` und `preset` im Wizard ändern.
- Ein eingebauter Editor in der TUI.
- Commit der Änderungen.
- Seite „Prompts“ im Modus „anlegen“. Dort legt der Wizard die Kopien wie heute an.

## Umsetzung

1. **Speichern** (`internal/config/repo.go`): `SetRepoSettings(projectDir, base string, knowledge map[string][]string) (changed bool, err error)`
   ändert `baseBranch` und `knowledge` über `yaml.Node` nach „Speichern beim Bearbeiten“ und „Keine
   Änderung“. `SetRepoBaseBranch` nutzt danach `SetRepoSettings` mit der vorhandenen `knowledge`. Tests:
   Kommentare und `hooks` bleiben, `main` entfällt, leere `knowledge` entfällt, unveränderter Inhalt liefert
   `changed == false`.
2. **Zustand der Prompts** (`internal/prompts/prompts.go`): `type FileState int` mit `StateDefault`,
   `StateCopy`, `StateCustom` und `func StateOf(projectDir string, step Step) (FileState, error)`. Vergleich
   nach Entfernen der Frontmatter und Leerraum am Ende. `func Label(FileState) string` liefert die Texte aus
   „Seite Prompts“. Tests für alle drei Zustände.
3. **Wizard** (`internal/tui/initwizard.go`).
   - `initWizard` bekommt `edit bool`, `promptCursor int` und `states []prompts.FileState`. Neue Seite
     `pagePrompts` zwischen `pageSteps` und `pageSummary`, nur bei `edit`.
   - `openInitWizard` lädt bei vorhandenem Ordner `config.LoadRepo` und belegt Ziel-Branch, Markierungen und
     Schritte nach den Produktentscheidungen vor. Nicht getrackte Knowledge-Dateien kommen an das Ende von
     `files`.
   - `updateInitWizard` für `pagePrompts`. `o` nutzt eine neue Funktion `openWizardEditor`, gebaut wie
     `openPRDEditor`, mit eigener Nachricht `wizardEditedMsg` für den Fall ohne Herdr. `r` über `ask`.
   - `writeInitWizard` verzweigt bei `edit` auf `config.SetRepoSettings` und lässt die zentrale
     `config.yaml` unverändert.
   - `viewInitWizard`: Titel, Nummerierung, Seite Prompts mit Hinweis, Zusammenfassung mit Prompt-Zuständen,
     ` (nicht getrackt)` auf der Seite Knowledge-Dateien.
   - `keys()` für die Fußzeile.
4. **Hilfe** (`internal/tui/view.go`, `helpSections`): Eintrag zu `i` nach „Hilfe“.
5. **Tests** (`internal/tui/initwizard_test.go`): `i` mit vorhandenem `.lazydev/` öffnet „bearbeiten“ mit
   vorbelegtem Ziel-Branch und Knowledge, nicht getrackte Datei bleibt erhalten, Speichern ändert nur
   `baseBranch` und `knowledge`, `Keine Änderung`, Seite Prompts zeigt die drei Zustände, `r` löscht nach
   `j`, `o` legt eine fehlende Datei an (Editor über `VISUAL=true`). Den bisherigen Test zu
   `existiert schon` ersetzen.
6. **Doku.** `SPEC.md`: Abschnitt zum Wizard um den Modus „bearbeiten“ und die Seite „Prompts“ ergänzen,
   Tastentabelle. `README.md` und `README.de.md`: Tastentabelle und ein Satz im Abschnitt zu `.lazydev/`.

## Technische Hinweise

- Alle Texte deutsch.
- `yaml.v3`: Beim Ersetzen von `knowledge` den Wert-Knoten neu aufbauen, Schlüssel-Knoten mit seinem
  Kommentar behalten. Fehlt der Schlüssel, am Ende des Mappings anhängen.
- Reihenfolge in `knowledge` beim Speichern: `all`, dann die Schritte in der Reihenfolge `prompts.Steps`.
  Dateien je Liste in der Reihenfolge von `files`.
- Tests setzen `VISUAL=true` und `HERDR_ENV` leer, damit kein echter Editor startet.
- Prüfbefehl: `make test`.

## Erfolgskriterien

- `make test` läuft ohne Fehler.
- `i` auf einem Projekt mit `.lazydev/` öffnet `.lazydev/ bearbeiten · <projekt>` mit dem aktuellen
  Ziel-Branch und den eingetragenen Knowledge-Dateien.
- Nach Änderung der Knowledge und `Enter` auf der Zusammenfassung enthält `.lazydev/config.yaml` die neue
  `knowledge`, und `preset`, `allowedTools`, `hooks` und Kommentare sind unverändert.
- Auf der Seite „Prompts“ öffnet `o` die Datei im Editor. Eine fehlende Datei existiert danach mit dem
  Standard-Inhalt.
- `r` löscht nach Bestätigung die Datei, die Zeile zeigt danach `Standard von lazydev`.
- Die zentrale `config.yaml` bleibt beim Bearbeiten unverändert.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben den Modus „bearbeiten“.
