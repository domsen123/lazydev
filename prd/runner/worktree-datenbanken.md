# PRD: Postgres-Datenbanken pro Worktree

> Status: umgesetzt (2026-10-28) · Kosten: 7.42 USD · Modell: claude-opus-5-5 · Scope: Neuer Abschnitt `databases` in `.lazydev/config.yaml`. Pro Eintrag legt das Setup eine leere Postgres-Datenbank `<basis>_<slug>` im Compose-Service an und schreibt ihren Namen in eine Env-Datei des Worktrees, der Merge legt sie nach einem Rebase neu an, Merge und `X` löschen sie. Der Wizard bekommt eine Seite „Databases“, die Postgres-Services aus der Compose-Datei erkennt.

## Context

Bis 825f23d („replace lazydev with the three-column TUI“, 2026-10-21) hatte lazydev das Preset
`nuxt-drizzle` (`internal/hooks/nuxtdrizzle.go`, `internal/hooks/envfile.go`, `internal/config/presets.go`).
Es kopierte `.env` und `.env.test` in den Worktree, hängte den Slug an `NUXT_DATABASE_NAME`, legte die
Datenbank im Postgres-Container an, migrierte mit `pnpm`, legte die Datenbank nach einem Rebase neu an und
löschte beim Teardown nur Datenbanken mit `_<slug>` am Ende. Der Commit hat das Preset ersatzlos entfernt.
Seitdem muss jedes Projekt das in eigenen Hook-Skripten nachbauen.

Das Preset war an ein Boilerplate gebunden: feste Variablennamen, feste `pnpm`-Skripte, Container-Suche über
den Host-Port. Allgemein gültig war nur der Ablauf. Dieses PRD bringt den Ablauf generisch für jedes
Projekt mit Postgres in Docker Compose zurück:

- `.lazydev/config.yaml` bekommt `databases`. Jeder Eintrag nennt einen Compose-Service sowie Env-Datei
  und Key, in dem der Datenbankname steht.
- Der Runner legt pro Worktree eine leere Datenbank an, schreibt ihren Namen in die Env-Datei des
  Worktrees, legt sie nach einem Rebase neu an und löscht sie nach Merge und `X`.
- Die Migrationen bleiben im Setup-Hook des Projekts. Sie laufen nach dem Anlegen und nach jeder
  Neuanlage.
- Der Wizard (`i`) erkennt Postgres-Services in der Compose-Datei und schreibt `databases`.

Im Repo existiert bereits und wird wiederverwendet:

- `Slug` und `maxSlugLen` (`internal/runner/worktree.go`). Der Slug ist schon auf 40 Bytes begrenzt, damit
  `<name>_<slug>` unter 63 Bytes bleibt.
- `setup`, `runHook`, `hookEnv` (`internal/runner/worktree.go`), `merge` und `rebaseAndCheck`
  (`internal/runner/merge.go`), `Discard` (`internal/runner/discard.go`).
- `Config`, `trusted`, `TrustSections`, `TrustHash`, `Read` (`internal/repo/repo.go`), `Settings` und
  `keyIndex` (`internal/repo/settings.go`).
- `docker.Container`, `label`, `labelWorkingDir`, `labelService`, `composeFiles`, `hasCompose`
  (`internal/docker/docker.go`).
- Der Wizard (`internal/tui/wizard.go`), die Projektansicht `projectDetailsView` (`internal/tui/details.go`),
  der Kopf des Log-Tabs `headerLines` (`internal/tui/logview.go`), `trustSummary`
  (`internal/tui/runview.go`).
- Aus der Git-Historie (`git show 825f23d^:<pfad>`) als Vorlage: `parseEnv`, `splitEnvLine`, `setEnv`
  (`internal/hooks/envfile.go`), `ensureContainer`, `databaseExists`, `createDatabase`, `dropDatabase`
  (`internal/hooks/nuxtdrizzle.go`). Die Kommentare dort sind deutsch und müssen englisch werden.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Datenbank-Engine | Nur Postgres. MySQL, MariaDB und andere sind out of scope. |
| Inhalt der neuen Datenbank | Leer. Migrationen und Seeds macht der Setup-Hook des Projekts. Es gibt keine Kopie der Haupt-Datenbank. |
| Format in `config.yaml` | Liste `databases`, je Eintrag `service` (Pflicht), `compose` (darf fehlen, Pfad relativ zum Hauptordner), `env.file` (Pflicht, Pfad relativ zum Hauptordner) und `env.key` (Pflicht). Beispiel siehe Technische Hinweise. |
| Validierung | `repo.Read` prüft jeden Eintrag. Ein fehlendes Pflichtfeld ist der Config-Fehler `config.yaml: databases[<i>]: <feld> missing`, ein absoluter Pfad oder ein Pfad mit `..` in `compose` oder `env.file` der Config-Fehler `config.yaml: databases[<i>]: <feld> must be a relative path inside the project`. Er erscheint wie kaputtes YAML in der Detailansicht, Läufe starten nicht. |
| Compose-Datei | Ohne `compose` gilt die Standarddatei, die `docker compose` im Hauptordner ohne `-f` findet (`compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`). Mit `compose` ruft lazydev `docker compose -f <compose>` im Hauptordner auf. Andere Dateien wie `docker-compose.dev.yml` sucht lazydev nicht selbst. |
| Erkennung eines Postgres-Service | Ein Service gilt als Postgres, wenn der Image-Name ohne Registry und Tag `postgres` ist oder auf `/postgres`, `/postgis`, `/pgvector` oder `/timescaledb` endet (z. B. `postgis/postgis`, `pgvector/pgvector`, `timescale/timescaledb`), oder wenn seine `environment` einen Key mit dem Präfix `POSTGRES_` hat. Nur der Wizard braucht diese Erkennung, der Runner nimmt jeden Service aus `databases`. |
| Postgres-User | `POSTGRES_USER` aus der `environment` des Service in `docker compose config --format json`, ohne Wert `postgres`. Ein Passwort braucht lazydev nicht, alle Befehle laufen per `docker exec` im Container. |
| Container finden | Über die Compose-Labels: `com.docker.compose.project.working_dir` gleich Hauptordner und `com.docker.compose.service` gleich `service`, Zustand `running`. Die Suche über den Host-Port des alten Presets entfällt. |
| Container läuft nicht (Setup, Neuanlage) | lazydev führt `docker compose [-f <compose>] up -d <service>` im Hauptordner aus und wartet bis zu 60 s, bis `pg_isready -q -U <user>` im Container gelingt. Danach Fehler `postgres in service <service> not ready after 60s`. |
| Basisname | Wert von `env.key` in `env.file` des Hauptordners. Enthält der Wert `://`, ist er eine URL und der Basisname ist ihr Pfad ohne führenden `/`. Sonst ist der ganze Wert der Basisname. |
| Name der Worktree-Datenbank | `<basisname>_<slug>`. Über 63 Bytes ist ein Fehler `database name <name> is longer than 63 bytes`. |
| Env-Datei im Worktree | Pro Setup kopiert lazydev jede Datei aus `env.file` aus dem Hauptordner in den Worktree und ersetzt darin die Keys aller Einträge mit dieser Datei. Bei einer URL ersetzt es nur den Pfad, Schema, User, Passwort, Host, Port und Query bleiben. Bei einem einfachen Wert ersetzt es den Wert. Alle anderen Zeilen bleiben unverändert. Die Datei bekommt Modus `0600`. Ein erneutes Setup schreibt sie neu. |
| Fehler bei der Env-Datei | Fehlende Datei im Hauptordner: `<file> missing in the main folder`. Fehlender oder leerer Key: `<file>: <key> is not set`. URL ohne Pfad: `<file>: <key> has no database name`. Ignoriert Git die Datei im Worktree nicht (`git check-ignore -q`): `<file> is not ignored by git, a worktree copy would dirty the worktree`. |
| Hook-Variablen | Keine neuen. Hooks und Dev-Server lesen den Namen aus der umgeschriebenen Env-Datei im Worktree. |
| Reihenfolge im Setup | Worktree, PRD verschieben, Datenbanken (Env-Dateien, Container, Datenbanken anlegen), dann der Setup-Hook. |
| Wiederholung | Eine schon vorhandene Datenbank bleibt stehen (Log `Database <name> in <container> exists already`), damit `s` nach einem Fehler im Setup-Hook weiterläuft. Mehrere Einträge mit demselben Service und demselben Namen legen die Datenbank einmal an. |
| Run-State | `state.Run` bekommt `databases`: Liste von `{service, name}`, gespeichert nach dem Anlegen. |
| Neuanlage beim Merge | Liegt der Branch vor dem Rebase nicht auf der Base (`git merge-base --is-ancestor <base> HEAD` schlägt fehl), löscht lazydev nach dem Rebase und einem eventuellen Claude-Schritt merge jede Datenbank aus `databases`, legt sie neu an und führt den Setup-Hook erneut aus. Erst danach läuft der Check-Hook. Log: `<base> moved, recreating databases`. Ohne `databases` ändert sich nichts. Das gilt auch für den erneuten Rebase, wenn die Base während des Wartens auf den Hauptordner weitergezogen ist. |
| Löschen nach dem Merge | Nach dem Teardown-Hook, vor dem Entfernen des Worktrees. Ein Fehler kommt wie beim Teardown-Hook in `cleanup incomplete: …`, der Merge bleibt erfolgreich. |
| Löschen bei `X` | Nach dem Teardown-Hook, auch wenn der Worktree fehlt. Fehler landen in der Fehlerliste von `Discard`. |
| Welche Datenbanken gelöscht werden | Die aus `databases` im Run-State. Ohne Run-State oder mit leerer Liste die aus `.lazydev/config.yaml`, berechnet aus den Env-Dateien des Hauptordners. Was nicht berechenbar ist (Datei oder Key fehlt), meldet `<file>: database not determinable`. |
| Schutz der Haupt-Datenbanken | lazydev löscht nur Namen mit dem Suffix `_<slug>` bei nicht leerem Slug, sonst Fehler `database <name> does not belong to the worktree, not dropping it`. Gelöscht wird mit `dropdb --if-exists --force`. |
| Container läuft nicht beim Löschen | Kein Start, kein Fehler. Log `Service <service> is not running, database <name> stays`. |
| Trust | `databases` gehört zu den vertrauten Abschnitten und geht in `TrustSections` und `TrustHash` ein. Ohne Trust legt das Setup nichts an. Bei `X` auf einem nicht vertrauten Projekt löscht lazydev keine Datenbank und meldet `databases not dropped, databases in .lazydev/config.yaml are not trusted`. |
| Trust-Frage bei `s` | `trustSummary` nennt zusätzlich `<n> databases`, z. B. `.lazydev/config.yaml sets setup hook, 2 databases. Trust it and start <prd>? (y/n)`. `lazydev trust` zeigt den Abschnitt mit an. |
| Wizard-Seiten | Base branch → Knowledge files → Steps per file → Databases → Prompts → Summary, nummeriert 1/6 bis 6/6. Ohne markierte Knowledge-Dateien springt `enter` von Knowledge direkt zu Databases, `backspace` auf Databases springt dann zurück zu Knowledge. |
| Wizard-Seite Databases | Überschrift `Databases`, Hinweis `Each worktree gets an empty database. Migrate in the setup hook.` Zeilen: jeder erkannte Postgres-Service als `<service>  <image>`, darunter eingerückt seine Einträge als `→ <file> <key>`. Einträge aus `config.yaml`, deren Service nicht erkannt wurde, erscheinen unter ihrem Service mit ` (not in compose file)` und bleiben erhalten. |
| Quelle der Services im Wizard | Die Standard-Compose-Datei des Hauptordners plus jede Datei aus `compose` vorhandener Einträge. Ein Eintrag, der unter einer Datei aus `compose` gefunden wurde, behält sein `compose`. Neue Einträge zu Services der Standarddatei bekommen kein `compose`. |
| Wizard ohne Postgres-Service | Gibt es keine Compose-Datei, zeigt die Seite `No compose.yaml or docker-compose.yml in the main folder. Set compose: in config.yaml for other files.` Ohne Postgres-Service: `No Postgres service in the compose file.` `enter` geht weiter. Kann `docker compose config` die Datei nicht lesen, zeigt die Seite die erste Zeile des Fehlers. |
| Wizard: Eintrag hinzufügen | `a` auf einer Service-Zeile öffnet die Auswahl der Env-Datei: alle regulären Dateien im Hauptordner, deren Name mit `.env` beginnt und die Git ignoriert, alphabetisch. `enter` wählt die Datei, dann folgt die Auswahl des Keys: alle Keys der Datei, zuerst die, deren Name `DATABASE`, `DB_NAME` oder `_URL` enthält (Groß-/Kleinschreibung egal), sonst in Dateireihenfolge. `enter` fügt den Eintrag hinzu, `esc` in einer Auswahl geht ohne Änderung zur Liste zurück. Ein schon vorhandener Eintrag (gleicher Service, Datei und Key) wird nicht doppelt angelegt. Ohne Env-Datei: Fehler im Wizard `No ignored .env file in the main folder.` |
| Wizard: Eintrag entfernen | `d` auf einer Eintragszeile entfernt den Eintrag sofort aus der Auswahl des Wizards. Gespeichert wird erst in der Zusammenfassung. |
| Wizard: Tasten der Seite | `j`/`k` wählen, `a` hinzufügen, `d` entfernen, `enter` weiter, `backspace` zurück, `esc` schließt den Wizard. Fußzeile `↑↓ select · a add · d remove · enter next · backspace back · esc cancel`. |
| Wizard: Speichern | `repo.Settings` schreibt zusätzlich `databases` über `yaml.Node`, in der Reihenfolge der Seite. Eine leere Liste entfernt den Key. Unverändertes `databases` wird nicht angefasst, Kommentare und andere Felder bleiben. Ändert sich `databases`, ändert sich der Trust-Hash, und das nächste `s` fragt nach Trust. |
| Projektansicht der Detailansicht | Neue Spalte `Databases` nach `Hooks`: je Eintrag `<service> → <file> <key>`, ohne Einträge `–`. Sie fällt wie die anderen Spalten von rechts weg. |
| Log-Tab | Im aufgeklappten Kopf unter der Zeile mit Phase, Branch, Port, Kosten und Dev eine Zeile `databases <name> (<service>) · <name> (<service>)`, nur wenn der Run-State Datenbanken hat. Eingeklappt bleibt der Kopf unverändert. |
| Step-Log | Jede Aktion bekommt eine Runner-Zeile: `<file>: <key> → <name>`, `Starting service <service>: docker compose … up -d <service>`, `Database <name> in <container> created`, `Database <name> in <container> dropped`. |

## Scope

**In Scope**

- `databases` in `internal/repo`, mit Validierung und im Trust-Hash.
- Neues Package `internal/database`: Env-Dateien, Basisname, Postgres-Befehle per `docker exec`.
- Erweiterung `internal/docker`: Compose-Service lesen, Container per Label finden, Service starten,
  `exec`, Postgres-Erkennung, ersetzbares Docker-Binary für Tests.
- Runner: Anlegen im Setup, Neuanlage nach dem Rebase, Löschen nach Merge und `X`, Datenbanken im Run-State.
- Wizard-Seite Databases, `repo.Settings` mit `databases`.
- Detailansicht, Log-Tab, Trust-Frage, `lazydev trust`.
- Tests, `SPEC.md`, Skill-Kopie, `README.md`, `README.de.md`, `CLAUDE.md`.

**Out of Scope**

- Andere Engines als Postgres.
- Kopie oder Dump der Haupt-Datenbank, Seeds durch lazydev.
- Migrationen durch lazydev. Sie bleiben im Setup-Hook.
- Automatische Suche nach Compose-Dateien außerhalb der Standardnamen. Andere Dateien kommen nur über `compose`.
- Neue Hook-Variablen für Datenbanknamen.
- Datenbanken außerhalb von Docker (lokales Postgres, Cloud).
- Aufräumen verwaister Datenbanken ohne Run-State und ohne passenden Config-Eintrag.
- Rückkehr des Presets `nuxt-drizzle` oder anderer Presets.

## Umsetzung

1. **Config** (`internal/repo/repo.go`):
   - Typen `Database{Service, Compose string; Env DatabaseEnv}` und `DatabaseEnv{File, Key string}` mit
     YAML-Tags `service`, `compose,omitempty`, `env`, `file`, `key`. Feld `Databases []Database` mit Tag
     `databases,omitempty` in `Config`.
   - `trusted` bekommt `Databases`. `TrustSections` ist leer, wenn alle drei Abschnitte leer sind.
   - `Read` ruft nach dem Unmarshal `validateDatabases` auf und setzt bei einem Fehler `info.Err` (Texte siehe
     Produktentscheidungen).
   - Tests in `internal/repo/repo_test.go`: Parsen, Validierung, Hash ändert sich mit `databases`, nicht mit
     Kommentaren.
2. **Wizard-YAML** (`internal/repo/settings.go`): `Settings(dir, base, knowledge, databases)` schreibt
   `databases` als Sequenz über `yaml.Node`, wenn sie sich von der Datei unterscheidet, und entfernt den Key bei
   leerer Liste. `changed` berücksichtigt `databases`. Tests in `internal/repo/settings_test.go`: Hinzufügen,
   Entfernen, unverändert ohne Schreiben, Kommentare und `hooks` bleiben.
3. **Docker** (`internal/docker/docker.go`, neue Datei `internal/docker/compose.go`):
   - `var Binary = "docker"`, alle `exec.Command("docker", …)` nutzen es.
   - `ComposeConfig(dir, file string) (map[string]ComposeService, error)` über
     `docker compose [-f file] config --format json` im Ordner dir. `ComposeService` hat `Image` und
     `Environment map[string]string`.
   - `IsPostgres(ComposeService) bool` nach der Regel der Produktentscheidungen.
   - `HasCompose(dir)` exportiert (bisher `hasCompose`).
   - `RunningContainer(dir, service string) (string, bool, error)`: Name des laufenden Containers per Label.
   - `Up(dir, file, service string) error` und `Exec(container string, args ...string) (string, error)`.
     Fehler enthalten die erste Zeile von stderr.
   - Tests in `internal/docker/docker_test.go`: `IsPostgres` mit den Image-Beispielen und `POSTGRES_*`,
     Parsen der JSON-Ausgabe von `compose config`.
4. **Package `internal/database`** (neu):
   - `env.go`: `parseEnv`, `splitEnvLine`, `setEnv` aus der Historie. Dazu `BaseName(value string) (string, error)`
     und `ReplaceName(value, name string) string` für einfache Werte und URLs (`net/url`, nur `Path` ändert
     sich).
   - `postgres.go`: `Target{Container, User, Name string}`, `Exists`, `Create` (vorhanden bleibt stehen),
     `Drop(t Target, slug string)` mit Suffix-Schutz, `WaitReady(container, user string, timeout time.Duration)`.
   - `database.go`: `Plan(projectDir, worktree, slug string, dbs []repo.Database) ([]Planned, error)`
     berechnet pro Eintrag Service, Compose-Datei, User, Name und den neuen Inhalt der Env-Datei. `Prepare`
     schreibt die Env-Dateien mit `git check-ignore`-Prüfung, startet fehlende Container, wartet und legt an.
     `Recreate` löscht und legt neu an. `DropAll` löscht ohne Containerstart. Alle bekommen einen Logger.
   - Tests `internal/database/*_test.go`: `setEnv`, `BaseName`, `ReplaceName` (mit Query und Passwort in der URL),
     Name zu lang, Suffix-Schutz, Ablauf gegen ein Fake-Docker-Skript.
5. **Run-State** (`internal/state/state.go`): `Database{Service, Name string}` mit JSON-Tags `service`,
   `name` und Feld `Databases []Database` mit Tag `databases,omitempty` in `Run`.
6. **Setup** (`internal/runner/worktree.go`): `setup` ruft nach `movePRD` und vor `runHook(…, "setup")`
   `prepareDatabases(t, r, log)` auf. Die Funktion liest `repo.Read`, bricht bei `TrustPending` mit
   `databases not created, databases in .lazydev/config.yaml are not trusted` ab, ruft `database.Prepare` auf,
   setzt `r.Databases` und speichert den State.
7. **Merge** (`internal/runner/merge.go`):
   - `rebaseAndCheck` merkt sich vor dem Rebase, ob der Branch auf der Base liegt. Lag er nicht darauf und sind
     `databases` konfiguriert, ruft es nach dem Rebase und einem eventuellen `mergeClaude` `recreateDatabases` (Trust-Prüfung,
     `database.Recreate`, `runHook(…, "setup")`) und erst danach den Check-Hook auf.
   - `merge` ruft nach dem Teardown-Hook `dropDatabases(t, r, log)` auf, ein Fehler geht in `errs`.
8. **Discard** (`internal/runner/discard.go`): nach dem Teardown-Hook `dropDatabases`, unabhängig davon, ob der
   Worktree existiert. Bei fehlendem Trust der Fehler aus den Produktentscheidungen.
9. **Trust-Anzeige** (`internal/tui/runview.go` `trustSummary`, `lazydev trust` in `cmd/lazydev/main.go`):
   `trustSummary` nennt `<n> databases`, `lazydev trust` gibt den Abschnitt `databases` mit aus, der Hilfetext
   lautet `check and trust hooks, allowed tools and databases of .lazydev/config.yaml`.
10. **Wizard** (`internal/tui/wizard.go`): Konstante `pageDatabases` zwischen `pageSteps` und `pagePrompts`.
    Beim Öffnen liest der Wizard `databases` aus der Config und die Services per `docker.ComposeConfig`
    (Standarddatei und Dateien aus `compose`). Die Seite mit Unterauswahl Env-Datei und Key, Tasten und Texten
    wie in den Produktentscheidungen. `lastChoicePage` und die `backspace`-Sprünge passen sich an. Die
    Zusammenfassung zeigt die künftige `config.yaml` mit `databases`. Neue Testdatei `internal/tui/wizard_test.go`:
    Navigation, Hinzufügen, Entfernen, Doppelte, Speichern, Seite ohne
    Compose-Datei, über ein Fake-Docker.
11. **Detailansicht** (`internal/tui/details.go`): Spalte `Databases`. **Log-Tab** (`internal/tui/logview.go`
    `headerLines`): Zeile `databases …`. Tests in der neuen Datei `internal/tui/details_test.go` und in
    `internal/tui/logview_test.go`.
12. **Runner-Tests** (`internal/runner/*_test.go`, Helfer in `helpers_test.go`): Fake-Docker-Skript analog zum
    Fake-Claude, gesetzt über `docker.Binary`. Es protokolliert seine Aufrufe in eine Datei und beantwortet
    `compose config --format json`, `ps`, `compose up`, `exec … pg_isready`, `exec … psql` (Existenz) sowie
    `createdb`/`dropdb`. Szenarien: Setup legt an und schreibt `.env` mit URL und einfachem Wert, wiederholtes
    Setup legt nicht doppelt an, nicht ignorierte Env-Datei bricht ab, gestoppter Container wird gestartet,
    Merge ohne Bewegung der Base legt nicht neu an, Merge mit bewegter Base legt neu an und führt den Setup-Hook
    erneut aus, Merge und `X` löschen, `X` ohne Trust löscht nicht, Suffix-Schutz.
13. **Doku**:
    - `SPEC.md`: Tabelle in `## .lazydev/` um `databases` ergänzen, neuer Unterabschnitt `### Databases` unter
      `### Hooks and dev server` mit Ablauf, Namensregel, Neuanlage, Löschen und Schutz, Trust-Abschnitt,
      Wizard-Abschnitt (sechs Seiten, Seite Databases), Workflow-Schritte setup/merge/`X`, Detailansicht und
      Log-Tab.
    - Danach `go generate ./internal/skill`, damit `internal/skill/files/references/spec.md` gleich `SPEC.md` ist.
    - `README.md` und `README.de.md`: Abschnitt zu Datenbanken pro Worktree mit dem Beispiel aus den Technischen
      Hinweisen und einem Beispiel-Setup-Hook mit Migration, in beiden Sprachen gleich.
    - `CLAUDE.md`: Zeile `database` in der Package-Tabelle, Satz zu Datenbanken im Abschnitt Runner.

## Technische Hinweise

- Beispiel für `config.yaml`:

  ```yaml
  databases:
    - service: db
      env:
        file: .env
        key: DATABASE_URL
    - service: db-test
      compose: docker-compose.dev.yml
      env:
        file: .env.test
        key: NUXT_DATABASE_NAME
  ```

  Mit `DATABASE_URL="postgres://app:secret@localhost:5432/app?sslmode=disable"` im Hauptordner und Slug
  `login_form` steht im Worktree `DATABASE_URL="postgres://app:secret@localhost:5432/app_login_form?sslmode=disable"`.
- Konventionen aus `CLAUDE.md`: Alle Texte, Fehler, Kommentare und Doku sind englisch. Das PRD und die
  PRD-Statuswerte bleiben deutsch. `SPEC.md`, `README.md` und `README.de.md` ändern sich mit dem Code. Nach jeder
  Änderung an `SPEC.md` läuft `go generate ./internal/skill`, sonst schlägt `TestSpecCopy` fehl.
- Commits nach Conventional Commits, z. B. `feat(runner): create a Postgres database per worktree`.
- Die Vorlage aus der Historie liest man mit `git show 825f23d^:internal/hooks/nuxtdrizzle.go` und
  `git show 825f23d^:internal/hooks/envfile.go`. `docker.ByHostPort` und die `pnpm`-Aufrufe werden nicht
  übernommen.
- `setEnv` schreibt die Zeile als `KEY="value"`. Ein Wert mit `"` muss als `\"` escaped werden, sonst bricht die
  Env-Datei. Test dafür vorsehen.
- `docker compose config --format json` löst Interpolation auf, `environment` kommt als Map. Ältere Compose-
  Versionen liefern dort auch `null`-Werte, die als leer gelten.
- Der Compose-Label `working_dir` ist der Ordner des Compose-Projekts. Mit `-f <compose>` im Hauptordner ist das
  der Hauptordner. Liegt eine Compose-Datei in einem Unterordner, weicht das Label ab. Deshalb vergleicht
  `RunningContainer` mit dem Ordner der Compose-Datei, für die Standarddatei mit dem Hauptordner.
- Datenbanknamen laufen in SQL nur in `SELECT 1 FROM pg_database WHERE datname = '<name>'` mit verdoppeltem `'`.
  `createdb` und `dropdb` bekommen den Namen als Argument, nie über eine Shell.
- Der Review-Schritt verlangt einen sauberen Worktree und `checkLazydevUntouched` prüft `.lazydev/`. Die Env-Datei
  muss von Git ignoriert sein, sonst ist der Worktree nach dem Setup unsauber. Deshalb die Prüfung mit
  `git check-ignore -q` im Worktree vor dem Schreiben.
- `runHook` liest die Config bei jedem Aufruf neu und prüft Trust. Die neuen Funktionen machen es genauso, damit
  eine während des Laufs geänderte `databases`-Liste ohne Trust nichts tut.
- Tests isolieren mit `t.Setenv("XDG_STATE_HOME", t.TempDir())`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME` sowie
  `GIT_AUTHOR_*`/`GIT_COMMITTER_*`. Tests mit `databases` in der Config vertrauen dem Hash mit `state.Trust`. Kein
  Test braucht ein echtes Docker. `docker.Binary` wird in `t.Cleanup` zurückgesetzt.

## Erfolgskriterien

- `make test` (`go vet ./...` und `go test ./...`) ist grün, einschließlich `TestSpecCopy`.
- `make build` baut `bin/lazydev`.
- Mit dem Beispiel aus den Technischen Hinweisen legt `s` auf einem PRD in einem Projekt mit laufendem
  Postgres-Service `app_<slug>` an, die `.env` im Worktree nennt diesen Namen, und der Setup-Hook läuft danach.
- Ein zweites `s` nach einem fehlgeschlagenen Setup-Hook bricht nicht an der vorhandenen Datenbank ab.
- Ist die Base vor dem Merge weitergezogen, zeigt das Merge-Log `<base> moved, recreating databases`, und der
  Setup-Hook läuft vor dem Check-Hook erneut. Ohne Bewegung erscheint die Zeile nicht.
- Nach einem erfolgreichen Merge und nach `X` existiert `app_<slug>` nicht mehr, die Haupt-Datenbank `app` schon.
- Ein Name ohne `_<slug>` am Ende wird nie gelöscht (Unit-Test).
- Eine Änderung an `databases` ändert den Trust-Hash. Ohne Trust legt kein Setup etwas an, und `X` löscht nichts.
- Der Wizard zeigt Postgres-Services der Compose-Datei, `a` legt einen Eintrag an, `d` entfernt ihn, und die
  gespeicherte `config.yaml` behält Kommentare und `hooks`.
- Die Projektansicht zeigt die Spalte `Databases`, der aufgeklappte Log-Tab-Kopf die Datenbanken des Laufs.
- `SPEC.md`, `internal/skill/files/references/spec.md`, `README.md`, `README.de.md` und `CLAUDE.md` beschreiben
  das Verhalten.
