# PRD: Globale Rechte und Permission-Modus für Claude-Schritte

> Status: umgesetzt (2026-10-21) · Kosten: 1.68 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: `~/.config/lazydev/settings.json` bekommt `allowedTools` für alle Projekte und `permissionMode` (`acceptEdits` oder `auto`) für jeden Claude-Schritt des Runners. Die Contracts verbieten `cd` in den Worktree und verkettete Shell-Befehle.

## Context

Jeder Claude-Schritt des Runners läuft als `claude -p … --permission-mode acceptEdits --allowedTools <liste>`
(`claude.Run` in `internal/claude/run.go`). Die Liste besteht aus `claude.DefaultAllowedTools` und den
`allowedTools` aus `.lazydev/config.yaml` des Projekts (`runClaude` in `internal/runner/claudestep.go`).
Read-only-Befehle wie `head`, `tail`, `sed -n` oder `grep` muss heute jedes Projekt selbst erlauben.

Die Auswertung der `permission_denials` aller Läufe unter `~/.local/state/lazydev/runs/` zeigt:

| Anzahl | Abgelehnter Befehl |
|---|---|
| 14 | `cd <worktree> && …` |
| 3 | `make test`, obwohl `Bash(make test:*)` erlaubt ist |
| je 2 | `pnpm lint`, `(pnpm lint …`, `sed -i`, `grep -n`, `go vet`, `git log` |
| je 1 | `head -25`, `python3 -`, `make -C`, `git status`, `GIT_EDITOR=true git …` |

Die meisten Ablehnungen kommen von einem vorangestellten `cd <worktree> &&`, von Pipes und Subshells. Für
solche zusammengesetzten Befehle greift keine Präfix-Regel, auch wenn jeder Teil erlaubt ist.

Nach der Änderung gilt:

- Rechte, die jedes Projekt braucht, stehen einmal in `~/.config/lazydev/settings.json`.
- Der Permission-Modus ist wählbar. `auto` lässt einen Klassifikator von Claude Code über jeden
  Tool-Aufruf entscheiden statt der Präfix-Liste.
- Die Contracts weisen Claude an, im Worktree ohne `cd` und ohne Verkettung zu arbeiten.

Im Repo existiert bereits und wird wiederverwendet:

- `config.Settings`, `config.LoadSettings`, `config.SaveSettings` (`internal/config/config.go`): heute nur
  das Feld `Theme`.
- `claude.DefaultAllowedTools`, `claude.Options`, `claude.Run` (`internal/claude/run.go`).
- `runClaude` (`internal/runner/claudestep.go`): baut `claude.Options` für implement, review und merge.
- Die eingebetteten Contracts `internal/prompts/contract/implement.md`, `review.md` und `merge.md`.
- Das Fake-`claude` in `internal/runner/helpers_test.go`: schreibt alle Argumente nach `$FAKE_ARGS`,
  `fakeArgs(t)` liest sie.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Ort | `~/.config/lazydev/settings.json` (`config.Dir()`), keine neue Datei. |
| Feld für Rechte | `allowedTools` (`[]string`, `json:"allowedTools,omitempty"`), gleiche Syntax wie in `config.yaml`, z. B. `Bash(head:*)`. |
| Reihenfolge der Liste | `claude.DefaultAllowedTools`, dann `allowedTools` aus `settings.json`, dann `allowedTools` aus `config.yaml`. Doppelte Einträge entfallen, der erste bleibt. |
| Vorgabe ohne Feld | Keine zusätzlichen Rechte. lazydev schreibt keine Vorgabewerte in `settings.json`. `claude.DefaultAllowedTools` bleibt unverändert. |
| Vertrauen | `settings.json` gehört dem Nutzer. Ihre `allowedTools` brauchen keinen Trust-Hash und fließen nicht in `repo.Config.TrustHash` ein. |
| Feld für Modus | `permissionMode` (`string`, `json:"permissionMode,omitempty"`). |
| Erlaubte Werte | `acceptEdits` und `auto`. Leer oder fehlend bedeutet `acceptEdits`. `bypassPermissions`, `dontAsk`, `manual` und `plan` sind nicht erlaubt. |
| Ungültiger Wert | Der Claude-Schritt scheitert vor dem Start von Claude mit `settings.json: unknown permissionMode "<wert>", allowed: acceptEdits, auto`. `s` setzt nach einer Korrektur am gescheiterten Schritt fort. |
| Kaputte `settings.json` | Wie heute: `LoadSettings` liefert die Vorgaben, also `acceptEdits` und keine globalen Rechte. |
| Wirkung | Alle Claude-Schritte des Runners: implement, review und der Claude-Schritt merge. |
| Zeitpunkt des Lesens | Jeder Claude-Schritt liest `settings.json` beim Start neu. Eine Änderung wirkt ab dem nächsten Schritt, auch in einem laufenden Runner. |
| `--allowedTools` bei `auto` | lazydev übergibt die Liste auch im Modus `auto` unverändert. |
| Log | Vor der Zeile `$ claude …` schreibt der Runner `Permissions: <modus>, <n> allowed tools` ins Runner-Log. `<n>` zählt die Liste nach dem Entfernen der Duplikate. |
| Contract-Regel | `implement.md`, `review.md` und `merge.md` bekommen unter „Regeln von lazydev“ denselben Punkt: „Du arbeitest bereits im Worktree. Kein `cd` und kein `git -C`. Führe jeden Shell-Befehl einzeln aus, ohne `&&`, `;`, `\|\|`, Pipes und Subshells. Zusammengesetzte Befehle lehnt lazydev ab.“ |
| TUI | Keine Änderung. Die Spalte „Allowed tools“ in den Details zeigt weiter nur die Rechte aus `config.yaml`. Das Theme-Speichern behält die neuen Felder, weil `SaveSettings` das ganze `Settings` schreibt. |

## Scope

**In Scope**

- Felder `AllowedTools` und `PermissionMode` in `config.Settings`.
- Zusammenbau der Rechte und des Modus in `runClaude`, Übergabe an `claude.Run`.
- Prüfung des Modus mit Fehlermeldung.
- Log-Zeile `Permissions: …`.
- Contract-Regel gegen `cd` und verkettete Befehle.
- Tests, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Interaktive Claude-Tabs der TUI, z. B. der Create-Chat über `n`.
- Ein Editor für `settings.json` in der TUI.
- Weitere Modi wie `bypassPermissions`.
- Rechte pro Schritt.
- Auswertung oder Anzeige von `permission_denials` in der TUI.
- Löschen der veralteten Datei `~/.config/lazydev/config.yaml`.

## Umsetzung

1. **Settings** (`internal/config/config.go`): `Settings` bekommt `AllowedTools []string` und
   `PermissionMode string` mit den JSON-Namen aus den Produktentscheidungen. Neue Funktion
   `func (s Settings) Mode() (string, error)` liefert `acceptEdits` für einen leeren Wert, den Wert für
   `acceptEdits` und `auto` und sonst den Fehler aus „Ungültiger Wert“.
2. **Test der Settings** (`internal/config/config_test.go`): `TestSettings` vergleicht heute mit
   `s != (Settings{})`. Das kompiliert mit einem Slice-Feld nicht mehr. Ersetze die Vergleiche durch
   `reflect.DeepEqual` oder Feldvergleiche. Ergänze einen Roundtrip mit `AllowedTools` und
   `PermissionMode` und einen Test für `Mode()` mit leerem, gültigen und ungültigen Wert.
3. **Claude-Aufruf** (`internal/claude/run.go`): `Options` bekommt `PermissionMode string`. `Run` übergibt
   `--permission-mode` mit `o.PermissionMode`, bei leerem Wert `acceptEdits`. Passe den Kommentar über
   `DefaultAllowedTools` an: Die Liste ergänzen jetzt `settings.json` und `config.yaml`.
4. **Runner** (`internal/runner/claudestep.go`): `runClaude` ruft `config.LoadSettings()` und `Mode()`.
   Bei einem Fehler endet der Schritt vor dem Start von Claude mit diesem Fehler, wie heute bei einem
   Fehler aus `t.Repo()`. Die Rechte entstehen nach „Reihenfolge der Liste“ ohne Duplikate. Vor der Zeile
   `$ claude …` schreibt `log.printf` die Zeile aus „Log“.
5. **Runner-Tests** (`internal/runner/*_test.go`): Der Helper in `internal/runner/helpers_test.go` setzt
   `XDG_CONFIG_HOME` schon auf ein Temp-Verzeichnis. Schreibe `settings.json` dort mit
   `config.SaveSettings`. Neue Tests:
   - Ohne `settings.json`: `fakeArgs` enthält `--permission-mode` gefolgt von `acceptEdits`.
   - Mit `{"permissionMode":"auto","allowedTools":["Bash(head:*)","Read"]}`: `fakeArgs` enthält `auto`,
     die Liste nach `--allowedTools` enthält `Bash(head:*)` genau einmal und `Read` genau einmal.
   - Mit `allowedTools` in `config.yaml` und `settings.json`: die Einträge aus `settings.json` stehen vor
     denen aus `config.yaml`.
   - Mit `{"permissionMode":"bypassPermissions"}`: Der Schritt implement scheitert, die Fehlermeldung
     steht im Zustand, und `$FAKE_ARGS` enthält keinen Aufruf für implement.
6. **Contracts** (`internal/prompts/contract/implement.md`, `review.md`, `merge.md`): Ergänze die Regel aus
   „Contract-Regel“ in der Liste unter „Regeln von lazydev“. Prüfe, ob `internal/prompts/prompts_test.go`
   Contract-Text vergleicht, und passe ihn an.
7. **Doku**:
   - `SPEC.md`: Der Abschnitt über `settings.json` nennt `theme`, `allowedTools` und `permissionMode` mit
     erlaubten Werten, Vorgabe und Fehlerverhalten. Der Abschnitt „Claude call“ ersetzt
     `--permission-mode acceptEdits` durch den Modus aus `settings.json` und nennt die Reihenfolge der
     Rechte. Der Abschnitt über Vertrauen hält fest, dass `settings.json` keinen Trust-Hash braucht.
   - `README.md` und `README.de.md`: Die Pfadtabelle nennt die neuen Felder von `settings.json`. Ein
     kurzes Beispiel zeigt `settings.json` mit `permissionMode: "auto"` und `allowedTools` für `head`,
     `tail`, `sed -n` und `grep`. Ein Satz nennt das Risiko von `auto`: Ein Klassifikator entscheidet,
     das Ergebnis ist nicht deterministisch.

## Technische Hinweise

- Alle UI-Strings, Fehlermeldungen, Log-Zeilen und Code-Kommentare sind englisch. Die Contracts bleiben
  deutsch.
- `README.de.md` ist die deutsche Fassung von `README.md`. Ändere beide mit gleichem Inhalt.
- Der Runner läuft losgelöst mit einer Kopie des Binaries. Er liest `settings.json` über
  `config.Dir()`, das `XDG_CONFIG_HOME` beachtet. Tests isolieren mit
  `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`.
- `config.SaveSettings` schreibt das ganze `Settings`. Das Speichern des Themes in
  `internal/tui/theme.go` lädt vorher mit `LoadSettings`, die neuen Felder bleiben dadurch erhalten.
  Ändere das nicht.
- Das Fake-`claude` gibt jedes Argument in einer eigenen Zeile aus. Prüfe in Tests die Zeile nach
  `--permission-mode` und die Zeile nach `--allowedTools`.
- Ob `--permission-mode auto` mit `claude -p` ohne TTY auf jedem Account funktioniert, prüfen die Tests
  nicht. Sie prüfen nur die übergebenen Argumente.
- Ändere nichts unter `.lazydev/`.

## Erfolgskriterien

- `make test` läuft ohne Fehler, also `go vet ./...` und `go test ./...`.
- Ohne `settings.json` übergibt der Runner `--permission-mode acceptEdits` und dieselbe Liste wie vorher.
- Mit `permissionMode: "auto"` übergibt der Runner `--permission-mode auto`.
- `allowedTools` aus `settings.json` erscheinen in jedem Claude-Schritt aller Projekte, ohne Duplikate und
  ohne Trust-Abfrage.
- Ein ungültiger `permissionMode` lässt den Schritt vor dem Start von Claude scheitern, mit der Meldung
  aus den Produktentscheidungen.
- Das Runner-Log enthält vor jedem Claude-Aufruf die Zeile `Permissions: <modus>, <n> allowed tools`.
- Die drei Contracts enthalten die Regel gegen `cd` und verkettete Befehle.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben beide Felder.
