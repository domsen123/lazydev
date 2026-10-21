# PRD: Discord-Benachrichtigungen und zweispaltiges Menü

> Status: umgesetzt (2026-10-21) · Scope: Der Runner meldet Merge, Fehler, Abbruch und Warten auf den Hauptordner per Discord-Webhook, global und pro Projekt schaltbar. Das Menü (`alt+m`) wird zweispaltig mit den Tabs General, Discord, Git und Quit.

## Context

Ein PRD läuft im losgelösten Runner `lazydev run` (`internal/runner/runner.go`) durch setup, implement,
review und merge. Ob es fertig ist, gescheitert ist oder auf den Hauptordner wartet, siehst du heute nur in
der TUI. Läuft die TUI nicht oder sitzt du nicht davor, erfährst du davon nichts.

Nach der Änderung schickt der Runner bei vier Ereignissen eine Nachricht an einen Discord-Webhook:

- PRD gemergt,
- Schritt gescheitert (Phase `error`),
- Runner mit `x` abgebrochen (Phase `aborted`),
- Merge wartet auf einen sauberen Hauptordner (Note `main folder: …`).

Die Webhook-URL steht in `~/.config/lazydev/settings.json`, global und pro Projekt. Sie gelangt nie in ein
Repository. Du richtest sie im Menü ein. Das Menü bekommt dafür zwei Spalten wie das Options-Fenster von
btop: links die Tabs `General`, `Discord`, `Git` und `Quit`, rechts die Einträge oder eine Beschreibung des
gewählten Tabs.

Im Repo existiert bereits und wird wiederverwendet:

- `config.Settings`, `config.LoadSettings`, `config.SaveSettings` (`internal/config/config.go`): heute
  `Theme`, `AllowedTools`, `PermissionMode`. `SaveSettings` schreibt mit `fsutil.WriteAtomic`, das die
  Rechte der Datei per `os.Chmod` setzt.
- `menu`, `updateMenu`, `viewMenu`, `setTheme` (`internal/tui/menu.go`) und `box` (`internal/tui/main.go`).
- `saveTheme`, `loadTheme`, `themes` (`internal/tui/theme.go`).
- `openCockpit` (`internal/tui/gitcockpit.go`) und `m.current()` (`internal/tui/main.go`) für das
  ausgewählte Projekt, `projectEntry.path` und `projectEntry.name()` (`internal/tui/projects.go`).
- `m.err` und `m.note` für Footer-Fehler und -Hinweise.
- `runStep`, `Run` (`internal/runner/runner.go`), `waitMainClean` (`internal/runner/merge.go`),
  `openLog` und `logger.printf` (`internal/runner/log.go`).
- `state.Run` mit `Cost()`, `ImplementModel()`, `StartedAt`, `Branch`, `Base`, `PRD`
  (`internal/state/state.go`), `state.Now`.
- Test-Helper `isolate`, `newProject`, `loadRun` (`internal/runner/helpers_test.go`) und das Fake-`claude`
  mit `FAKE_CLAUDE_MODE`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Ereignisse | `merged`, `failed` (Schritt scheitert, Phase `error`), `aborted` (laufender Runner endet durch `x`), `main folder` (Merge beginnt auf den Hauptordner zu warten). |
| Absender | Der Runner sendet, nicht die TUI. Nachrichten kommen auch ohne laufende TUI. |
| Kein Runner, keine Nachricht | Ein wartendes PRD, das `x` ohne Runner abbricht (`runner.Stop`), meldet nichts. Fehler vor dem ersten Schritt (`checkClaude`, `CheckTrust`, `prepare`) melden nichts. |
| Warten auf den Hauptordner | Eine Nachricht pro Wartephase, wenn das Warten beginnt (Zweig `!waiting` in `waitMainClean`). Ein Wechsel des Grunds sendet nichts. |
| Ort der Einstellung | `~/.config/lazydev/settings.json`, Feld `discord`. `.lazydev/config.yaml` bekommt kein Discord-Feld. Keine URL landet in einem Repository. |
| Format in `settings.json` | `"discord": {"enabled": bool, "url": string, "projects": {"<absoluter projektpfad>": {"enabled": bool, "url": string}}}`. Projekte stehen unter ihrem Pfad wie in `projects.json`, nicht unter der ID. |
| Globaler Schalter | `discord.enabled`. Fehlt er oder ist er `false`, sendet kein Projekt. Vorgabe: aus. |
| Schalter pro Projekt | `projects.<pfad>.enabled`. Ein Projekt ohne Eintrag oder ohne Feld `enabled` gilt als eingeschaltet. `false` schaltet nur dieses Projekt ab. |
| URL-Auflösung | Projekt-URL, falls nicht leer, sonst globale URL. Ohne beide URLs sendet der Runner nichts und schreibt nichts ins Log. |
| Lesezeitpunkt | Der Runner liest `settings.json` bei jedem Ereignis neu. Eine Änderung im Menü wirkt auch in laufenden Runnern. |
| Dateirechte | `SaveSettings` schreibt `settings.json` mit `0600`, auch beim Speichern des Themes. Eine bestehende Datei mit `0644` bekommt beim nächsten Speichern `0600`. |
| Anzeige der URL | Im Menü gekürzt auf `…/<letzte 4 Zeichen>`. Ohne URL: `not set` für die globale URL, `global` für die Projekt-URL. |
| Nachricht | Discord-Embed. Payload: `{"username":"lazydev","embeds":[{"title":…,"color":…,"fields":[…],"timestamp":<RFC 3339>}]}`. Kein Fehlertext im Embed. |
| Titel und Farbe | `✓ Merged: <area>/<name>` mit `0x2ecc71`, `✗ Failed: <area>/<name>` mit `0xe74c3c`, `■ Aborted: <area>/<name>` mit `0x95a5a6`, `◐ Waiting for main folder: <area>/<name>` mit `0xf1c40f`. `<area>/<name>` kommt aus `state.Run.PRD` (`prd/<area>/<name>.md`). |
| Felder | Alle `inline: true`, in dieser Reihenfolge: `Project` (Projekt-ID), `Branch` (`<branch> → <base>`), `Cost` (`%.2f USD` aus `Run.Cost()`), `Duration` (seit `Run.StartedAt`, Format `1h 2m 3s`, führende Nullteile entfallen, z. B. `4m 5s`), `Model` (`Run.ImplementModel()`, leer `–`). |
| Versand | HTTP `POST` mit `Content-Type: application/json`, Timeout 10 s je Versuch. Erfolg ist ein Status 2xx. |
| Wiederholung | 1 Versuch plus 3 Wiederholungen im Abstand von 5 s. Jeder Fehler, auch 4xx, löst eine Wiederholung aus. |
| Fehler beim Versand | Nach dem letzten Versuch eine Zeile `Discord notification failed: <fehler>` im Step-Log. Phase, Fehler und Note des Runs ändern sich nicht. Der Schritt scheitert nicht. |
| Erfolg beim Versand | Eine Zeile `Discord notification sent: <event>` im Step-Log, `<event>` ist `merged`, `failed`, `aborted` oder `main folder`. |
| Log des Ereignisses | `failed` und `aborted` ins Log des gescheiterten Schritts, `main folder` und `merged` ins Log von merge. |
| Blockieren | Der Versand läuft synchron im Runner. Bei Discord-Ausfall verzögert er den Runner um höchstens etwa 55 s. Bei `aborted` nutzt er einen eigenen Context, nicht den abgebrochenen. |
| Menü-Aufbau | Zwei Spalten in einer Box `Menu`. Links die Tabs `General`, `Discord`, `Git`, dann eine Leerzeile und `Quit`. Rechts die Einträge des Tabs oder eine Beschreibung. Darunter eine Hinweiszeile. |
| Breite | `min(width − 4, 72)`. Linke Spalte 14 Zeichen innen, dann `│`, rechts der Rest. |
| Tasten links | `↑/↓`, `j/k`, `tab` bewegen mit Umlauf. `enter`, `→`, `l` öffnen: `General` und `Discord` wechseln in die rechte Spalte, `Git` schließt das Menü und öffnet das Git-Cockpit, `Quit` beendet lazydev. `esc`, `q`, `alt+m` schließen das Menü. |
| Tasten rechts | `↑/↓`, `j/k`, `tab` bewegen mit Umlauf. `←/→` ändern Schalter und Theme. `enter` und `space` schalten Schalter um, wechseln zum nächsten Theme, öffnen die URL-Eingabe oder senden den Test. `esc` und `h` gehen zurück in die linke Spalte. `q` und `alt+m` schließen das Menü. |
| Tab `General` | Eintrag `Color theme  ‹ <name> ›`. Darunter die Beschreibung: `Colors of borders, focus and selection. lazydev saves the choice in settings.json.` |
| Tab `Discord` | Einträge: `Notifications  ‹ on/off ›` (global), `Global URL  <anzeige>`, `<projektname>  ‹ on/off ›`, `URL for <projektname>  <anzeige>`, `Send test`. Ohne ausgewähltes Projekt fallen die beiden Projekt-Einträge weg. Unter den Einträgen die Beschreibung des gewählten Eintrags. |
| Beschreibungen Discord | Notifications: `Runners post to Discord when a PRD is merged, fails, gets aborted or waits for the main folder.` Global URL: `Webhook URL for all projects. Stored in settings.json, never in a repository.` Projekt-Schalter: `Off mutes this project only.` Projekt-URL: `Replaces the global URL for this project. Empty uses the global URL.` Send test: `Posts a test message to the URL of this project now, also while off.` |
| Tab `Git` | Rechts nur eine Beschreibung: `Fetch, pull, push and commit the main folder of the selected project. Each action runs in a center tab. Also alt+g.` |
| Tab `Quit` | Rechts nur eine Beschreibung: `Ends lazydev and its terminal tabs. Runners keep working in the background.` |
| URL-Eingabe | `enter` auf einer URL öffnet die Eingabe `Webhook URL: ` leer, im Menü an Stelle der Einträge. `enter` speichert, `esc` bricht ab. Leere Eingabe löscht die URL. Eine Eingabe, die nach `strings.TrimSpace` nicht mit `https://` beginnt, speichert nichts und zeigt den Fehler `Enter an https URL.`. |
| Speichern | Jede Änderung im Tab `Discord` schreibt `settings.json` sofort über `LoadSettings`, Änderung, `SaveSettings`. Ein Fehler beim Schreiben geht in `m.err`. |
| Send test | Sendet an die aufgelöste URL des ausgewählten Projekts, ohne Projekt an die globale URL. Ein Versuch, kein Retry, Timeout 10 s, auch bei ausgeschalteten Schaltern. Läuft als `tea.Cmd`. Ohne URL Fehler `No webhook URL set.`. Erfolg: Note `discord: test sent`. Fehler: `discord: <fehler>`. |
| Testnachricht | Embed `Test from lazydev` mit Farbe `0x3498db` und Feld `Project` (Projektname, ohne Projekt `–`). |
| Hinweiszeile | Links: `↑/↓ select · enter open · esc close`. Rechts: `↑/↓ select · ←/→ change · enter edit · esc back`. In der Eingabe: `enter save · esc cancel`. |
| Bestehendes Verhalten | Das Theme wechselt weiter sofort und wird gespeichert. `alt+g` bleibt. Entfernt `d` ein Projekt, bleibt sein Eintrag unter `discord.projects`. |

## Scope

**In Scope**

- Feld `Discord` in `config.Settings` mit Auflösung von Schalter und URL, Dateirechte `0600`.
- Neues Paket `internal/notify` für Payload und Versand mit Wiederholung.
- Aufrufe im Runner für die vier Ereignisse mit Log-Zeilen.
- Zweispaltiges Menü mit den Tabs General, Discord, Git, Quit, URL-Eingabe und Send test.
- Tests für `config`, `notify`, Runner und Menü.
- `SPEC.md`, `README.md`, `README.de.md`, Pakettabelle in `CLAUDE.md`.

**Out of Scope**

- Andere Dienste als Discord, z. B. Slack, E-Mail oder Desktop-Benachrichtigungen.
- Benachrichtigungen aus der TUI oder für wartende PRDs ohne Runner.
- Fehlertext, Links oder Log-Auszüge im Embed.
- Discord-Felder in `.lazydev/config.yaml`.
- Aufräumen von `discord.projects` beim Entfernen eines Projekts.
- Auswertung von Discords Rate-Limit-Headern (`Retry-After`).
- Benachrichtigungen bei `X` (discard) und bei Status-Änderungen mit `c`.
- Weitere Einträge im Tab `General` außer dem Theme.

## Umsetzung

1. **Settings** (`internal/config/config.go`):
   - Neue Typen `Discord` (`Enabled bool`, `URL string`, `Projects map[string]DiscordProject`) und
     `DiscordProject` (`Enabled *bool`, `URL string`). JSON-Namen `enabled`, `url`, `projects`, alle mit
     `omitempty`.
   - `Settings` bekommt `Discord Discord` mit `json:"discord,omitzero"`.
   - Neue Methode `func (d Discord) ProjectEnabled(dir string) bool`: `true` ohne Eintrag oder mit
     `Enabled == nil`, sonst `*Enabled`.
   - Neue Methode `func (d Discord) ProjectURL(dir string) string`: Projekt-URL, falls nicht leer, sonst
     globale URL.
   - Neue Methode `func (d Discord) Target(dir string) string`: leer, wenn `Enabled` false ist oder
     `ProjectEnabled(dir)` false ist, sonst `ProjectURL(dir)`.
   - `SaveSettings` schreibt mit `0o600`.
2. **Test der Settings** (`internal/config/config_test.go`): Roundtrip mit `Discord` inklusive Projekt mit
   `Enabled` false. Tabellentest für `Target`: global aus, global an ohne Eintrag, Projekt aus, Projekt-URL
   ersetzt globale URL, beide URLs leer. Test, dass `settings.json` nach `SaveSettings` die Rechte `0600`
   hat, auch wenn vorher eine Datei mit `0644` existierte.
3. **Paket `notify`** (`internal/notify/notify.go`, neu):
   - `type Event string` mit `Merged = "merged"`, `Failed = "failed"`, `Aborted = "aborted"`,
     `MainFolder = "main folder"`.
   - `type Message struct { Event Event; Project, PRD, Branch, Base, Model string; Cost float64; Duration time.Duration }`.
     `PRD` ist `<area>/<name>`.
   - `func Payload(m Message) ([]byte, error)` baut das Embed nach „Titel und Farbe“ und „Felder“.
     `func TestPayload(project string) ([]byte, error)` baut die Testnachricht.
   - `func Post(ctx context.Context, url string, body []byte) error`: ein Versuch mit `Timeout` (Variable,
     10 s), Fehler bei Status außerhalb 2xx mit `discord answered <status>`.
   - `func Deliver(url string, body []byte) error`: bis zu 4 Versuche mit `context.Background()`, zwischen
     den Versuchen `RetryDelay` (Variable, 5 s). Liefert den letzten Fehler.
   - `func FormatDuration(d time.Duration) string` nach dem Format aus „Felder“.
   - Tests (`internal/notify/notify_test.go`) mit `httptest.Server`: Payload enthält Titel, Farbe und alle
     fünf Felder in Reihenfolge. `204` ist Erfolg nach einem Request. `500` führt mit `RetryDelay = 0` zu
     genau 4 Requests und einem Fehler. Ein Server, der zweimal `500` und dann `204` antwortet, ergibt
     Erfolg nach 3 Requests. `FormatDuration` für `5s`, `4m 5s`, `1h 2m 3s`.
4. **Runner** (`internal/runner/notify.go`, neu): `func notifyRun(t Target, r *state.Run, event notify.Event, log *logger)`.
   Sie liest `config.LoadSettings().Discord.Target(t.Dir)`. Bei leerer URL endet sie ohne Log. Sonst baut
   sie die `Message` aus `t.ID` und `r`, ruft `notify.Deliver` und schreibt die Log-Zeile nach „Erfolg beim
   Versand“ oder „Fehler beim Versand“. `Duration` ist die Zeit seit `r.StartedAt`, ein ungültiger Wert
   ergibt 0.
5. **Aufrufe im Runner**:
   - `runStep` (`internal/runner/runner.go`): im Zweig `ctx.Err() != nil` nach `state.Save` mit
     `notify.Aborted`, im Zweig `err != nil` nach `state.Save` mit `notify.Failed`. Beide mit dem offenen
     Step-Log. Speichere das Ergebnis von `state.Save` vorher, damit der Rückgabewert gleich bleibt.
   - `Run`: nach dem Speichern der Phase `merged` mit `notify.Merged`. Öffne dafür das Log von merge mit
     `openLog(state.LogPath(t.ID, prdName, state.StepMerge))` und schließe es danach.
   - `waitMainClean` (`internal/runner/merge.go`): im Zweig `!waiting` nach `state.Save` mit
     `notify.MainFolder`. Die Funktion bekommt `log` schon übergeben.
6. **Runner-Tests** (`internal/runner/notify_test.go`, neu): `httptest.Server` zählt Requests und merkt
   sich die Bodies. `config.SaveSettings` schreibt `discord` mit `enabled: true` und der Server-URL.
   `notify.RetryDelay = 0` im Test.
   - Erfolgreicher Run: genau ein Request mit `✓ Merged: auth/new-token`, das Log von merge enthält
     `Discord notification sent: merged`.
   - `FAKE_CLAUDE_MODE=exit`: ein Request mit `✗ Failed: auth/new-token`.
   - Abbruch über einen abgebrochenen Context während implement: ein Request mit `■ Aborted`.
   - Dreckiger Hauptordner wie in `TestMergeWaitsForCleanMainFolder` (`internal/runner/merge_test.go`):
     genau ein Request mit `◐ Waiting for main folder`, danach der Merge-Request.
   - Global aus: kein Request. Projekt mit `enabled: false`: kein Request.
   - Server antwortet immer `500`: 4 Requests pro Ereignis, der Run endet trotzdem `merged`, das Log
     enthält `Discord notification failed:`.
7. **Menü** (`internal/tui/menu.go`):
   - `menu` bekommt `tab int` (General, Discord, Git, Quit), `right bool`, `cursor int` für die rechte
     Spalte und eine Eingabe (`textinput.Model`) mit dem Ziel `global` oder `project`.
   - `updateMenu` setzt die Tasten aus „Tasten links“, „Tasten rechts“ und „URL-Eingabe“ um. Die Einträge
     des Tabs `Discord` entstehen aus `config.LoadSettings().Discord` und `m.current()`.
   - Die Schalter und URLs speichern nach „Speichern“. Der Projekt-Schalter setzt `Enabled` als Pointer.
   - `Send test` liefert einen `tea.Cmd`, der `notify.Post` mit 10 s Timeout aufruft und eine neue
     Nachricht `discordTestMsg{err error}` zurückgibt. Das Model setzt daraus `m.note` oder `m.err`.
   - `viewMenu` zeichnet beide Spalten, die Beschreibungen, die gekürzten URLs und die Hinweiszeile. Die
     Funktion zum Kürzen: `…/` und die letzten 4 Zeichen.
   - `menuWidth` entfällt, die Breite folgt „Breite“.
8. **TUI-Tests** (`internal/tui/menu_test.go`, neu, Stil wie `internal/tui/gitcockpit_test.go`):
   - `enter` auf `Git` öffnet das Cockpit. Passe `TestMenuOpensCockpit` in
     `internal/tui/gitcockpit_test.go` an den neuen Aufbau an.
   - `→` auf `General`, dann `→`: Theme wechselt.
   - Tab `Discord`: Schalter `Notifications` schreibt `enabled: true` in `settings.json`.
   - URL-Eingabe mit `http://x` zeigt `Enter an https URL.` und speichert nichts. Mit `https://…/abcd`
     speichert sie und die Ansicht zeigt `…/abcd`.
   - Projekt-URL leer: Anzeige `global`.
   - `Send test` ohne URL: Fehler `No webhook URL set.`. Mit `httptest.Server`: Note `discord: test sent`.
   - `esc` rechts geht nach links, `esc` links schließt das Menü.
9. **Doku**:
   - `SPEC.md`: Unter „Projects“ das Feld `discord` von `settings.json` mit Format, Auflösung und Rechten
     `0600`. Neuer Abschnitt `### Menu (alt+m)` unter „UI“ mit Spalten, Tabs, Tasten, Beschreibungen und
     Eingabe. Im Abschnitt „Git cockpit“ den Satz über den Menüeintrag an den Tab `Git` anpassen. Neuer
     Abschnitt `### Discord notifications` unter „Workflow“ mit Ereignissen, Embed, Wiederholung und
     Log-Zeilen.
   - `README.md` und `README.de.md`: Tastentabelle `alt+m` nennt `menu (theme, Discord, git)`. Neuer
     Abschnitt „Discord“ mit Einrichtung im Menü, Ereignissen und einem Beispiel für `discord` in
     `settings.json`. Die Pfadtabelle nennt `discord` unter `settings.json`.
   - `CLAUDE.md`: Pakettabelle bekommt `notify` mit der Rolle `Discord webhook: payload and delivery with
     retry`.

## Technische Hinweise

- Alle UI-Strings, Log-Zeilen, Fehlermeldungen und Kommentare sind englisch. `README.de.md` ist die
  deutsche Fassung von `README.md`, ändere beide mit gleichem Inhalt.
- Der Runner läuft mit einer Kopie des Binaries. `config.Dir()` beachtet `XDG_CONFIG_HOME`, `isolate`
  setzt es in Tests schon auf ein Temp-Verzeichnis.
- `omitzero` braucht Go 1.24 oder neuer, das Projekt verlangt 1.27. Ohne `discord` schreibt
  `SaveSettings` das Feld nicht.
- `saveTheme` lädt vor dem Speichern mit `LoadSettings`. So bleibt `discord` beim Theme-Wechsel erhalten.
  Das Menü macht es beim Speichern von Discord-Feldern genauso.
- Im Abbruchfall ist der Context des Runs beendet und die Prozessgruppe hat SIGTERM bekommen. Der Runner
  fängt SIGTERM ab (`RunMain`), der Versand läuft deshalb weiter. `notify.Deliver` nutzt
  `context.Background()`.
- `runStep` gibt heute `errors.Join(err, state.Save(...))` zurück. Rufe `notifyRun` nach dem Speichern und
  vor dem Return, ohne den Rückgabewert zu ändern.
- Discord antwortet auf einen erfolgreichen Webhook ohne `?wait=true` mit `204 No Content`.
- Ob echte Discord-Server die Embeds wie beschrieben anzeigen, prüfen die Tests nicht. Sie prüfen das
  JSON gegen einen `httptest.Server`.
- Ändere nichts unter `.lazydev/`.

## Erfolgskriterien

- `make test` läuft ohne Fehler, also `go vet ./...` und `go test ./...`.
- Ein gemergtes PRD erzeugt genau einen Request mit Titel `✓ Merged: <area>/<name>` und den Feldern
  `Project`, `Branch`, `Cost`, `Duration`, `Model`.
- Ein gescheiterter Schritt, ein Abbruch mit `x` und der Beginn des Wartens auf den Hauptordner erzeugen
  je genau einen Request mit Titel und Farbe aus den Produktentscheidungen.
- Mit globalem Schalter aus, Projekt-Schalter aus oder ohne URL gibt es keinen Request.
- Antwortet der Webhook nicht mit 2xx, gibt es 4 Versuche, der Run endet unverändert und das Step-Log
  enthält `Discord notification failed:`.
- `settings.json` hat nach jedem Speichern die Rechte `0600`.
- Das Menü zeigt links `General`, `Discord`, `Git`, `Quit` und rechts Einträge oder Beschreibung. URLs
  erscheinen nur gekürzt.
- `Send test` sendet auch bei ausgeschalteten Schaltern und meldet `discord: test sent`.
- `SPEC.md`, `README.md`, `README.de.md` und `CLAUDE.md` beschreiben Menü und Discord-Benachrichtigungen.
