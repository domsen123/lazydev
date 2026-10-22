# PRD: GitHub Releases, Installationsskript und Update-Hinweis

> Status: umgesetzt (2026-10-22) · Kosten: 2.09 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: Ein Tag `v*` baut per GoReleaser statische Binaries für linux/amd64 und linux/arm64 und legt ein GitHub Release mit Changelog aus den Conventional Commits an. `install.sh` installiert ein Release über `gh`. lazydev kennt seine Version (`lazydev version`) und zeigt in der TUI einen Hinweis, wenn ein neueres Release existiert.

## Context

lazydev lässt sich heute nur aus dem Quellcode installieren: `make install` baut nach `~/.local/bin/lazydev`.
Auf einer neuen Linux-Maschine brauchst du dafür Go 1.27, einen Clone und einen Build. Es gibt keine Tags,
keine Versionsnummer im Binary und kein `.github/workflows/`.

Das Repo `domsen123/lazydev` ist privat. Ein Download ohne Anmeldung ist deshalb nicht möglich. `gh` ist auf
den Zielmaschinen vorhanden oder lässt sich installieren, der Zugriff läuft über `gh auth login`.

Nach der Änderung gilt:

- Du pushst einen Tag `vX.Y.Z`. Eine GitHub Action prüft mit `make test`, baut mit GoReleaser und legt das
  Release mit Archiven, `checksums.txt` und Release-Notes an.
- `install.sh` lädt das passende Archiv per `gh release download`, prüft die Prüfsumme und installiert
  `lazydev` nach `~/.local/bin`. Ein erneuter Aufruf aktualisiert.
- `lazydev version` gibt die Version aus. Die TUI prüft beim Start das neueste Release und zeigt einen
  Hinweis, wenn es neuer ist.

Im Repo existiert bereits und wird wiederverwendet:

- `Makefile`: Targets `build`, `test`, `install` mit `PREFIX`.
- `cmd/lazydev/main.go`: `usage`-Text und `switch cmd` für die Befehle.
- `model.note` (`internal/tui/main.go`): Meldung im Footer, die die nächste Taste löscht.
- `tabDescriptions` (`internal/tui/menu.go`): Beschreibung unter dem Eintrag des Tabs General.
- `model.Init` (`internal/tui/main.go`): startet die Hintergrund-Abrufe als `tea.Cmd`.
- Conventional Commits in der Git-Historie, z. B. `feat(tui): …`, `fix(runner): …`, `docs(prd): …`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Build und Release | GoReleaser v2 in einer GitHub Action, ausgelöst durch Push eines Tags `v*`. |
| Plattformen | `linux/amd64` und `linux/arm64`, `CGO_ENABLED=0`. Keine anderen Betriebssysteme. |
| Archive | `lazydev_<version>_linux_<arch>.tar.gz` mit `lazydev`, `LICENSE`, `README.md`, `README.de.md`. `<version>` ohne führendes `v`, `<arch>` ist `amd64` oder `arm64`. Dazu `checksums.txt` (SHA-256). |
| Version im Binary | Paket `internal/version`, Variable `Version`, Standardwert `dev`. GoReleaser setzt sie per `-ldflags -X` auf den Tag, z. B. `v0.1.0`. `make build` und `make install` setzen sie auf `git describe --tags --always --dirty`. |
| Versionsformat | Tags folgen `vMAJOR.MINOR.PATCH` ohne Suffix. Den ersten Tag `v0.1.0` setzt du nach dem Merge selbst. |
| Changelog | GoReleaser allein, keine `CHANGELOG.md` im Repo. Quelle sind die Commits seit dem vorigen Tag. Gruppen: `Features` (`feat`), `Fixes` (`fix`), `Performance` (`perf`), `Other` (Rest). Ausgefiltert: `docs`, `test`, `chore`, `ci`, `style`, `build`. Breaking Changes (`!` nach dem Typ) stehen in `Features` oder `Fixes` wie andere Commits. |
| Installation | `install.sh` im Repo-Root. Aufruf: `gh api repos/domsen123/lazydev/contents/install.sh -H 'Accept: application/vnd.github.raw' \| sh`. Optional ein Argument mit dem Tag, z. B. `sh install.sh v0.2.0`. Ohne Argument das neueste Release. |
| Zielordner | `$PREFIX/bin/lazydev`, `PREFIX` Standard `$HOME/.local`, wie im `Makefile`. |
| Voraussetzungen von `install.sh` | `gh` mit Anmeldung, `tar`, `sha256sum`, `uname`. Fehlt etwas, endet das Skript mit Exit-Code 1 und einer Zeile auf stderr, die das fehlende Werkzeug nennt. |
| Update | Erneuter Aufruf von `install.sh`. Kein Befehl `lazydev update`. |
| Befehl `version` | `lazydev version`, `lazydev --version` und `lazydev -v` geben `lazydev <Version>` aus, z. B. `lazydev v0.1.0`, Exit-Code 0. `lazydev help` nennt `version`. |
| Update-Prüfung | Die TUI prüft einmal beim Start im Hintergrund mit `gh release view --repo domsen123/lazydev --json tagName --jq .tagName`, Timeout 10 s. Keine Wiederholung während der Laufzeit. |
| Wann geprüft wird | Nur wenn `version.Version` dem Format `vMAJOR.MINOR.PATCH` ohne Suffix entspricht. `dev` und `git describe`-Versionen wie `v0.1.0-3-gabc123` prüfen nicht. |
| Fehler bei der Prüfung | `gh` fehlt, keine Anmeldung, kein Netz, Timeout oder unlesbare Ausgabe: kein Hinweis, keine Fehlermeldung. |
| Hinweis im Footer | Ist das neueste Release größer als die laufende Version, setzt die TUI `note` auf `lazydev <neu> available, you run <alt>. Run install.sh again to update.` Die nächste Taste löscht ihn wie jede `note`. Ist schon eine `err` oder `note` gesetzt, überschreibt der Hinweis sie nicht. |
| Hinweis im Menü | Der Tab General zeigt unter seiner Beschreibung eine Zeile `lazydev <Version>`. Ist ein neueres Release bekannt, lautet sie `lazydev <Version> · <neu> available, run install.sh again.` Die Zeile bleibt, bis lazydev endet. |
| Laufende Runner | Ein Update ändert laufende Runner nicht, sie nutzen ihre Kopie unter `$XDG_STATE_HOME/lazydev/bin/`. `install.sh` gibt am Ende `Installed lazydev <Version> to <Pfad>. Restart lazydev to use it.` aus. |

## Scope

**In Scope**

- `internal/version` mit `Version`, Formatprüfung, Vergleich und Abfrage des neuesten Release.
- Befehl `lazydev version` mit `--version` und `-v`.
- Versions-ldflags im `Makefile`.
- `.goreleaser.yaml` und `.github/workflows/release.yml`.
- `install.sh`.
- Update-Hinweis in Footer und Menü-Tab General.
- Doku in `README.md`, `README.de.md`, `SPEC.md`.

**Out of Scope**

- `lazydev update` oder anderes Selbst-Update.
- macOS, Windows, andere Architekturen.
- Pakete für apt, rpm, AUR, Homebrew oder Docker-Images.
- `go install` und Umbenennen des Modulpfads `lazydev`.
- `CHANGELOG.md` im Repo, git-cliff, release-please.
- Signieren der Artefakte.
- Wiederholte Update-Prüfung während der Laufzeit.
- Setzen des ersten Tags. Das machst du nach dem Merge.

## Umsetzung

1. **Paket `internal/version`** (`internal/version/version.go`, neu):
   - `var Version = "dev"` mit Kommentar, dass Build und GoReleaser sie per `-ldflags -X lazydev/internal/version.Version=…` setzen.
   - `func Release(v string) (major, minor, patch int, ok bool)`: `ok` nur für `v<zahl>.<zahl>.<zahl>` ohne weitere Zeichen.
   - `func Newer(latest, current string) bool`: `true` nur, wenn beide `Release`-Format haben und `latest` numerisch größer ist.
   - `func Latest(ctx context.Context) (string, error)`: führt `gh release view --repo domsen123/lazydev --json tagName --jq .tagName` aus, gibt die getrimmte Ausgabe zurück. Das Repo steht als Konstante `Repo = "domsen123/lazydev"` im Paket. Den Binärnamen `gh` hält eine Variable `GH`, damit Tests ein Fake-Skript setzen.
   - Tests (`internal/version/version_test.go`): Tabellen für `Release` (`v1.2.3` ok; `1.2.3`, `v1.2`, `v1.2.3-rc1`, `v0.1.0-3-gabc`, `dev` nicht ok) und `Newer` (`v0.10.0` > `v0.9.0`, gleich ist nicht neuer, ungültige Werte sind nie neuer). `Latest` mit Fake-`gh`-Skript in `t.TempDir()`, das einen Tag ausgibt, und einem, das mit Exit-Code 1 endet.

2. **Befehl `version`** (`cmd/lazydev/main.go`):
   - `case "version", "--version", "-v":` gibt `lazydev ` + `version.Version` aus.
   - `usage` bekommt die Zeile `  lazydev version                          print the version`.

3. **Makefile** (`Makefile`):
   - `VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)`.
   - `LDFLAGS := -X lazydev/internal/version.Version=$(VERSION)`.
   - `build` und `install` bauen mit `-ldflags "$(LDFLAGS)"`.
   - `.gitignore` bekommt `dist/`, den Ausgabeordner von GoReleaser.

4. **GoReleaser** (`.goreleaser.yaml`, neu):
   - `version: 2`, `project_name: lazydev`.
   - `builds`: ein Build, `main: ./cmd/lazydev`, `binary: lazydev`, `env: [CGO_ENABLED=0]`, `goos: [linux]`, `goarch: [amd64, arm64]`, `flags: [-trimpath]`, `ldflags: ["-s -w -X lazydev/internal/version.Version={{ .Tag }}"]`.
   - `archives`: `formats: [tar.gz]`, `name_template: "lazydev_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`, `files: [LICENSE, README.md, README.de.md]`.
   - `checksum`: `name_template: checksums.txt`, `algorithm: sha256`.
   - `changelog`: `use: git`, `sort: asc`, `filters.exclude` mit `'^(docs|test|chore|ci|style|build)(\(.*\))?!?:'`, `groups` mit `Features` (`'^feat(\(.*\))?!?:'`, order 0), `Fixes` (`'^fix(\(.*\))?!?:'`, order 1), `Performance` (`'^perf(\(.*\))?!?:'`, order 2), `Other` (Regex leer, order 999).
   - `release`: `github.owner: domsen123`, `github.name: lazydev`.

5. **GitHub Action** (`.github/workflows/release.yml`, neu):
   - `on: push: tags: ['v*']`, `permissions: contents: write`.
   - Ein Job `release` auf `ubuntu-latest`:
     1. `actions/checkout@v5` mit `fetch-depth: 0`, damit GoReleaser den vorigen Tag findet.
     2. `actions/setup-go@v6` mit `go-version-file: go.mod`.
     3. `make test`.
     4. `goreleaser/goreleaser-action@v6` mit `distribution: goreleaser`, `version: '~> v2'`, `args: release --clean`, `env: GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`.

6. **Installationsskript** (`install.sh`, neu, ausführbar, `#!/bin/sh`, POSIX sh, `set -eu`):
   - Prüft `gh`, `tar`, `sha256sum`, `uname` mit `command -v`. Prüft die Anmeldung mit `gh auth status >/dev/null 2>&1`. Fehler: `install.sh: <werkzeug> not found` bzw. `install.sh: gh is not logged in, run gh auth login`, Exit-Code 1.
   - Architektur aus `uname -m`: `x86_64` → `amd64`, `aarch64` und `arm64` → `arm64`, sonst `install.sh: unsupported architecture <wert>`, Exit-Code 1. Betriebssystem aus `uname -s` muss `Linux` sein, sonst `install.sh: lazydev runs on Linux only`, Exit-Code 1.
   - Tag aus `$1`, sonst `gh release view --repo domsen123/lazydev --json tagName --jq .tagName`.
   - Lädt in ein Verzeichnis von `mktemp -d` mit `gh release download <tag> --repo domsen123/lazydev --pattern "lazydev_*_linux_<arch>.tar.gz" --pattern checksums.txt`. Ein `trap` löscht das Verzeichnis bei jedem Ende.
   - Prüft mit `grep "_linux_<arch>.tar.gz" checksums.txt | sha256sum -c -`. Bei Fehler `install.sh: checksum mismatch`, Exit-Code 1.
   - Entpackt, legt `$PREFIX/bin` an, installiert mit `install -m 755 lazydev "$PREFIX/bin/lazydev"`.
   - Gibt `Installed lazydev <tag> to <pfad>. Restart lazydev to use it.` aus. Liegt `$PREFIX/bin` nicht in `PATH`, folgt `Add <ordner> to PATH.`

7. **Update-Prüfung in der TUI** (`internal/tui/update.go`, neu, und `internal/tui/main.go`):
   - `type updateMsg struct{ latest string }` und `func checkUpdate() tea.Cmd`: gibt `nil` zurück, wenn `version.Release(version.Version)` nicht ok ist. Sonst ruft der Cmd `version.Latest` mit 10 s Timeout auf und liefert `updateMsg{latest}` nur, wenn `version.Newer(latest, version.Version)`. Bei Fehler liefert er `nil`.
   - `model` bekommt das Feld `latest string` mit Kommentar.
   - `Init` hängt `checkUpdate()` an `tea.Batch` an.
   - `Update` behandelt `updateMsg`: setzt `m.latest`. Sind `m.err` und `m.note` leer, setzt es `m.note` auf den Hinweis aus den Produktentscheidungen.
   - Tests (`internal/tui/update_test.go`): `updateMsg` setzt `latest` und `note`; mit gesetzter `err` bleibt `note` leer und `err` unverändert. `checkUpdate` mit `version.Version = "dev"` gibt `nil` zurück. Setze und stelle `version.Version` im Test mit `t.Cleanup` zurück.

8. **Menü** (`internal/tui/menu.go`, `viewMenu`):
   - Der Tab General zeigt nach seiner Beschreibung eine Leerzeile und die Versionszeile aus den Produktentscheidungen.
   - Test in `internal/tui/menu_test.go`: Tab General enthält `lazydev dev`; mit `m.latest = "v9.9.9"` und `version.Version = "v0.1.0"` enthält er `v9.9.9 available, run install.sh again.`

9. **Doku**:
   - `README.md` und `README.de.md`, Abschnitt Install: zuerst die Installation per `install.sh` mit dem `gh api`-Aufruf, Voraussetzung `gh auth login`, optionaler Tag, Update durch erneuten Aufruf. Danach `make install` aus dem Quellcode. Neuer kurzer Abschnitt Release: Tag `vX.Y.Z` pushen, die Action baut, Release-Notes kommen aus den Commits, `docs`/`test`/`chore`/`ci`/`style`/`build` fehlen darin.
   - `SPEC.md`: Abschnitt `CLI` nennt `version`. Abschnitt `Menu (alt+m)` nennt die Versionszeile im Tab General. Neuer Abschnitt `### Releases` unter `## Workflow` oder `## Processes` mit Update-Prüfung (Zeitpunkt, Bedingungen, Footer-Text, stilles Scheitern) und Release-Ablauf.

## Technische Hinweise

- Alle UI-Strings, Fehlermeldungen, Kommentare und Doku auf Englisch, außer `README.de.md`. Das PRD bleibt deutsch.
- `README.md` und `README.de.md` bekommen denselben Inhalt.
- Der Modulpfad heißt `lazydev`, der ldflags-Pfad lautet deshalb `lazydev/internal/version.Version`. Ein falscher Pfad scheitert still: das Binary meldet dann `dev`. Prüfe das nach `make build` mit `bin/lazydev version`.
- GoReleaser setzt `{{ .Version }}` ohne `v` und `{{ .Tag }}` mit `v`. Archivnamen nutzen `.Version`, die ldflags `.Tag`.
- `CGO_ENABLED=0` sollte ohne Änderung bauen. `setsid` und `flock` laufen über `syscall`. Prüfe das lokal mit `CGO_ENABLED=0 go build ./cmd/lazydev`.
- `go.mod` verlangt Go 1.27.3. `setup-go` liest die Version aus `go.mod`.
- Die Tests der TUI rufen kein echtes `gh` auf. `checkUpdate` und `version.Latest` nutzen die Variable `version.GH`, Tests setzen sie auf ein Fake-Skript oder prüfen nur den `dev`-Fall.
- `version.Latest` setzt `GH_PROMPT_DISABLED=1` in der Umgebung von `gh`, damit `gh` nie auf Eingaben wartet.
- `install.sh` läuft unter `sh` von `dash`. Verwende keine Bash-Features wie `[[`, Arrays oder `pipefail`.
- Der Runner kopiert sein Binary vor dem Start nach `$XDG_STATE_HOME/lazydev/bin/lazydev` (`runner.spawn`). Ein Update durch `install.sh` ist deshalb gefahrlos während laufender Runs.
- Kein neues Go-Modul als Abhängigkeit. Den Versionsvergleich schreibst du selbst mit `strconv.Atoi`.

## Erfolgskriterien

- `make test` läuft ohne Fehler, inklusive der neuen Tests in `internal/version` und `internal/tui`.
- `make build && bin/lazydev version` gibt `lazydev <git describe>` aus, nicht `lazydev dev`.
- `go run ./cmd/lazydev version` gibt `lazydev dev` aus.
- `CGO_ENABLED=0 go build ./cmd/lazydev` baut, `file lazydev` meldet `statically linked`.
- `goreleaser check` meldet keine Fehler für `.goreleaser.yaml`, falls GoReleaser lokal installiert ist. Sonst nennt der Bericht das als nicht geprüft.
- `goreleaser release --snapshot --clean` erzeugt in `dist/` zwei Archive und `checksums.txt`, falls GoReleaser lokal installiert ist. `dist/` steht in `.gitignore`.
- `sh -n install.sh` meldet keine Syntaxfehler. `shellcheck install.sh` meldet nichts, falls installiert.
- Mit `version.Version = "v0.1.0"` und einem Fake-`gh`, das `v0.2.0` ausgibt, zeigt der Footer `lazydev v0.2.0 available, you run v0.1.0. Run install.sh again to update.`
- `README.md`, `README.de.md` und `SPEC.md` beschreiben Installation, Release, `lazydev version` und den Update-Hinweis.
