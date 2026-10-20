# PRD: Benachrichtigungen für Rückfragen aus PRD-Chats

> Status: umgesetzt (2026-10-20) · Template: einfach · Scope: PRD-Chats laufen als losgelöste Prozesse im Hintergrund weiter. Stellt Claude eine Rückfrage oder scheitert eine Runde, erscheint sie in einer neuen Box „Benachrichtigungen“ rechts neben den PRDs und als Zähler am Projekt.

## Context

Ein neues PRD entsteht mit `n` in einem Chat-Fenster (`internal/tui/chat.go`). Der Chat läuft als Goroutine
in der TUI, es gibt genau einen (`Model.chat`). `Esc` schließt das Fenster und bricht die laufende Runde ab.
Während Claude den Code liest und Fragen vorbereitet, wartest du deshalb vor dem Fenster. Mit `q` endet der
Chat ebenfalls.

Nach der Änderung läuft jede Chat-Runde als losgelöster Prozess `lazydev chat <projekt-id> <chat-id>`, wie
der Runner. `Esc` blendet das Fenster nur aus. Du arbeitest in lazydev weiter, auch mit mehreren Chats
gleichzeitig. Endet eine Runde mit einer Rückfrage oder einem Fehler, erscheint ein Eintrag in der Box
„Benachrichtigungen“ ganz rechts neben der PRD-Box des Projekts. Die Projektliste zeigt die Zahl der Einträge.
`Enter` auf dem Eintrag oder die globale Taste `N` öffnet den Chat wieder. Chats überleben das Beenden der TUI.

Im Repo existiert bereits und wird wiederverwendet:

- `claude.Run`, `claude.Options`, `claude.Summary`, `claude.WithTimestamp`, `claude.Describe`,
  `claude.ToolLine`, `claude.FirstLine` (`internal/claude`).
- `chatAllowedTools`, `chatModel`, `renderEntry`, `chat.render`, `chat.handleEvent` (`internal/tui/chat.go`).
- `usage.Record` (`internal/usage`): Limits aus jeder stream-json-Zeile.
- `state.Dir`, `state.ChatLogPath`, `state.Lock`, `state.IsRunner`, `state.Now` (`internal/state`).
- `spawnRunner` und `Stop` (`internal/runner/start.go`): Start mit `setsid` über `os.Executable()` und
  SIGTERM an die Prozessgruppe.
- `prdtemplate.All`, `CreatePrompt`, `CreateDisplay` (`internal/prdtemplate`).
- `Model.ask`, `returnMode`, `updateConfirm` (`internal/tui/actions.go`): Rückfrage mit `j`/`n`.
- `box`, `cursorLine`, `scrollWindow`, `padRight` (`internal/tui`).
- `watcher` (`internal/tui/watch.go`) und der Tick alle 2 Sekunden.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Ereignisse | Nur Chats erzeugen Benachrichtigungen. Runner-Ereignisse erzeugen keine. |
| Prozessmodell | Jede Chat-Runde läuft als losgelöster Prozess `lazydev chat <projekt-id> <chat-id>` mit `setsid`. Zwischen zwei Runden läuft kein Prozess. Die TUI liest Zustand und Log aus Dateien. Chats laufen nach `q` weiter. |
| Chat-ID | Startzeit im Format `2006-01-02-150405`. Existiert die Zustandsdatei schon, hängt lazydev `-2`, `-3` usw. an. |
| Dateien | Zustand: `$XDG_STATE_HOME/lazydev/chats/<projekt-id>/<chat-id>.json`. Log: `<chat-id>.jsonl` im selben Ordner. |
| Status eines Chats | `läuft`: Runde aktiv. `rückfrage`: Runde fehlerfrei, Claude erwartet eine Antwort. `fehler`: Runde gescheitert. `beendet`: Runde fehlerfrei ohne Rückfrage. Steht `läuft` in der Datei, lebt der Prozess aber nicht oder gehört nicht zu `lazydev chat`, gilt der Chat als `fehler` mit dem Text `Runde abgebrochen`. |
| Fehler einer Runde | Fehler beim Start, Exit-Code ungleich 0, kein `result`-Ereignis oder `is_error` im letzten `result`. |
| Erkennung einer Rückfrage | Ohne Marker, nur über Muster im Text `Summary.Result` der Runde. Rückfrage, wenn eine Zeile nach optionalen Leerzeichen, Listenzeichen (`-`, `*`, `+`, `>`) und `**` mit `A`, `B`, `C` oder `D` gefolgt von `:` oder `)` beginnt. Ebenfalls Rückfrage, wenn die letzte nicht leere Zeile ohne abschließende Leerzeichen, `*`, `_` und `` ` `` auf `?` endet. |
| Benachrichtigung | Jeder Chat mit Status `rückfrage` oder `fehler`. Sie gilt als gelesen, sobald du im Chat antwortest oder wiederholst. Das Öffnen allein ändert nichts. |
| Box „Benachrichtigungen“ | Neue Box rechts neben der PRD-Box, gleiche Höhe. Sie erscheint nur, wenn das gewählte Projekt mindestens eine Benachrichtigung hat. Breite wie die Projektbox: `min(max(24, Breite/4), 36)`. Die PRD-Box wird entsprechend schmaler. |
| Inhalt der Box | Nur Benachrichtigungen, die älteste oben (nach `updatedAt`). Eine Zeile pro Chat: `?` in `colorFocus` für Rückfrage, `✗` in `styleError` für Fehler, Leerzeichen, dann die Beschreibung aus „Neues PRD“, gekürzt mit `…`. Titel: `Benachrichtigungen · <anzahl>`. Laufende und beendete Chats erscheinen nicht. |
| Indikator am Projekt | ` <anzahl>✉` in `colorFocus` hinter dem Projektnamen, nach Docker-Symbol und `<n>●`. Ohne Benachrichtigung entfällt er. |
| Fokus | Vierter Fokus in der Reihenfolge Projekte → PRDs → Benachrichtigungen → Log. `Tab`, `Shift+Tab`, `←` und `→` überspringen die Box, wenn sie nicht sichtbar ist. Verschwindet die Box bei Fokus darauf, geht der Fokus auf die PRDs. |
| Tasten in der Box | `j` `k` `↑` `↓` wählen. `Enter` öffnet den Chat. `X` beendet den Chat nach Rückfrage. Alle übrigen Tasten wirken wie im Normalmodus. |
| Taste `N` | Global im Normalmodus. Öffnet die älteste Benachrichtigung über alle Projekte, wählt ihr Projekt und setzt den Cursor der Box darauf. Ohne Benachrichtigung zeigt die Statuszeile `Keine offenen Rückfragen` (kein Fehler). |
| Maus | Klicks und Mausrad über der Box bewirken nichts. |
| `Esc` im Chat | Blendet das Fenster aus, die Runde läuft weiter. Status `beendet`: lazydev löscht die Zustandsdatei und zeigt die bisherige Meldung `Chat geschlossen. Fortsetzen: claude --resume <sitzung> · Log: <pfad>`. Sonst zeigt die Statuszeile `Chat läuft im Hintergrund weiter · Log: <pfad>`. |
| `Ctrl+X` im Chat | Rückfrage `Chat beenden? Eine laufende Runde bricht ab. (j/n)`. Bei `j` SIGTERM an die Prozessgruppe einer laufenden Runde, Zustandsdatei löschen, Fenster schließen, Meldung `Chat beendet. Fortsetzen: claude --resume <sitzung> · Log: <pfad>`. Ohne Sitzung entfällt der Teil `Fortsetzen: …`. Bei `n` bleibt das Fenster offen. Das Log bleibt immer erhalten. |
| `X` in der Box | Wie `Ctrl+X`, nach `n` bleibt lazydev im Normalmodus. |
| `Ctrl+R` im Chat | Nur bei Status `fehler`: startet eine neue Runde mit Eingabe und Anzeige der letzten Runde. Sonst ohne Wirkung. |
| `Enter` im Chat | Sendet wie bisher. Bei Status `läuft` ohne Wirkung. Bei `rückfrage`, `fehler` und `beendet` startet es eine neue Runde mit `--resume`. |
| Chat ohne Rückfrage | Status `beendet`. Er verschwindet ohne Benachrichtigung. Ist sein Fenster offen, bleibt es bis `Esc` offen und nimmt weitere Eingaben an. Beim Neueinlesen der Chats löscht die TUI die Zustandsdateien aller beendeten Chats, deren Fenster nicht offen ist. |
| Chat-Verlauf | Das Fenster baut den Verlauf aus dem Log auf und liest neue Zeilen nach. So zeigt es nach einem Neustart der TUI den vollständigen Verlauf. |
| Fußzeile im Chat | `Enter senden · Alt+Enter neue Zeile · PgUp/PgDn scrollen · Ctrl+R wiederholen · Ctrl+X beenden · Esc ausblenden`. |
| Fußzeile im Normalmodus | `N Rückfragen` nach `n neues PRD`. Mit Fokus auf der Box: `j/k wählen · Enter öffnen · X beenden` statt der Log-Tasten. |

## Scope

**In Scope**

- Neues Paket `internal/chatrun` mit Zustand, Runde, Erkennung der Rückfrage, Start und Stopp.
- Unterbefehl `lazydev chat <projekt-id> <chat-id>`.
- Umbau des Chat-Fensters auf Dateien, Box „Benachrichtigungen“, Indikator, Fokus, `N`, `X`, `Ctrl+X`, `Ctrl+R`.
- Dateiüberwachung des Ordners `chats/`.
- Tests, `README.md` und `SPEC.md`.

**Out of Scope**

- Benachrichtigungen für Runner-Ereignisse wie Fehler, Merge-Freigabe oder Merge.
- Ein Verlaufsdialog mit den letzten Chats eines Projekts. Dafür entsteht bei Bedarf ein eigenes PRD.
- Anzeige laufender Chats in Box oder Projektliste.
- Desktop-Benachrichtigungen, Töne oder Herdr-Meldungen.
- Ein Marker im System-Prompt oder `--append-system-prompt`.
- Grenze für die Zahl gleichzeitiger Chats.
- Mausbedienung der Box.
- Übernahme alter Chat-Logs ohne Zustandsdatei.

## Umsetzung

1. **`internal/state/state.go` und `internal/state/process.go`.**
   - Ersetze `ChatLogPath(projectID string, started time.Time)` durch `ChatLogPath(projectID, chatID string)`
     mit dem Pfad `chats/<projekt-id>/<chat-id>.jsonl` unter `Dir()`. Ergänze
     `ChatPath(projectID, chatID string)` für `<chat-id>.json` im selben Ordner.
   - Ziehe die Prüfung aus `IsRunner` in `isLazydev(pid int, sub string) bool`. `IsRunner(pid)` ruft
     `isLazydev(pid, "run")` auf. Neu: `IsChat(pid)` ruft `isLazydev(pid, "chat")` auf.

2. **Neues Paket `internal/chatrun`, Datei `chatrun.go`.**
   - `type Status string` mit den Konstanten `StatusRunning = "läuft"`, `StatusQuestion = "rückfrage"`,
     `StatusError = "fehler"`, `StatusDone = "beendet"`.
   - `type Chat struct` mit den JSON-Feldern `title`, `description`, `dir`, `sessionId,omitempty`, `costUsd`,
     `status`, `pid,omitempty`, `prompt`, `shown`, `error,omitempty`, `startedAt`, `updatedAt`. `prompt` und
     `shown` gehören zur laufenden oder letzten Runde.
   - `type Entry struct{ ProjectID, ID string; Chat *Chat }`.
   - `func (c *Chat) Effective() (Status, string)`: liefert `StatusError` und `Runde abgebrochen` für
     `StatusRunning` ohne lebenden Prozess (`c.PID == 0 || !state.IsChat(c.PID)`). Sonst Status und `c.Error`.
   - `func (c *Chat) Notifies() bool`: wahr bei effektivem `StatusQuestion` oder `StatusError`.
   - `func Load(projectID, id string) (*Chat, error)`, `func save(projectID, id string, c *Chat) error` (atomar
     über temporäre Datei und `os.Rename`, wie `state.Save`) , `func Remove(projectID, id string) error` und `func RemoveIfDone(projectID, id string) error`.
   - `func All() ([]Entry, error)`: liest `chats/*/*.json`, überspringt unlesbare Dateien, sortiert nach
     `updatedAt`, dann `startedAt`.
   - `func lockName(projectID, id string) string` liefert `chat-<projekt-id>-<chat-id>`. Jede
     Lese-Schreib-Folge auf eine Zustandsdatei läuft unter `state.Lock(lockName(…))`.

3. **`internal/chatrun/question.go`.** `func IsQuestion(text string) bool` nach der Regel „Erkennung einer
   Rückfrage“ in den Produktentscheidungen. Nutze für die Optionszeilen den regulären Ausdruck
   `^\s*(?:[-*+>]\s*)*(?:\*\*)?[A-D][:)]` pro Zeile.

4. **`internal/chatrun/start.go`.**
   - Verschiebe `chatAllowedTools` und `chatModel` aus `internal/tui/chat.go` hierher als `AllowedTools` und
     `Model`, mit den bestehenden Kommentaren.
   - `func Create(projectID, title, description, dir string, now time.Time) (id string, err error)`: legt die
     Zustandsdatei mit Status `beendet` an, ohne Runde. Vergibt die Chat-ID nach der Regel „Chat-ID“.
   - `var spawnChat = func(projectID, id string) (int, error)`: startet `os.Executable()` mit
     `chat <projekt-id> <chat-id>` und `setsid`, Ausgabe nach `/dev/null`, wie `spawnRunner`.
   - `func Send(projectID, id, shown, prompt string) error`: unter der Sperre Zustand laden, `prompt`,
     `shown`, Status `läuft` setzen, `error` leeren, `spawnChat` aufrufen, PID setzen, speichern. Scheitert
     der Start, speichert `Send` Status `fehler` mit der Meldung und gibt den Fehler zurück.
   - `func Stop(projectID, id string) error`: unter der Sperre SIGTERM an `-PID` und bei Fehler an `PID`, wenn
     der Prozess lebt und `state.IsChat` gilt. Danach `Remove`.

5. **`internal/chatrun/round.go`.** `func Round(cfg *config.Config, projectID, id string) error` ist die
   Runde im Unterbefehl:
   - Unter der Sperre laden, `PID = os.Getpid()` setzen, speichern. Fehlt die Datei, endet `Round` ohne
     Fehler.
   - Log öffnen (`state.ChatLogPath`, anhängen). Zuerst die Zeile
     `{"type":"user_prompt","text":<prompt>,"shown":<shown>}` mit `claude.WithTimestamp`.
   - `claude.Run` mit `Binary: cfg.Claude.Binary`, `Dir: c.Dir`, `Prompt`, `AllowedTools`, `Resume: c.SessionID`,
     `Model`. `Log` schreibt jede Zeile mit `claude.WithTimestamp` ins Log und ruft `usage.Record` auf.
     `OnChange` speichert eine neue `SessionID` unter der Sperre.
   - Der Kontext endet bei SIGTERM und SIGINT (`signal.NotifyContext`).
   - Am Ende unter der Sperre neu laden. Fehlt die Datei, nichts schreiben. Sonst `costUsd` erhöhen,
     `sessionId` übernehmen, `updatedAt = state.Now()`, `pid = 0` und den Status setzen: Fehler nach der
     Regel „Fehler einer Runde“ mit `error` wie bisher in `chat.handleDone` (`claude endete mit Exit-Code <n>`,
     `Claude meldet einen Fehler: <erste Zeile>`, Startfehler, für fehlendes `result` `Claude lieferte kein
     Ergebnis`). Sonst `rückfrage`, wenn `IsQuestion(summary.Result)`, und `beendet` in allen anderen Fällen.
     Fehlerzeilen schreibt `Round` zusätzlich als `{"type":"lazydev","text":…}` ins Log.

6. **`cmd/lazydev/main.go`.** `chat` in die Liste der Unterbefehle aufnehmen. Fall `chat` ruft
   `chatrun.Round(cfg, target.ID, rest[1])` auf. Neue Zeile in `usage`:
   `lazydev chat <projekt-id> <chat-id>             Eine Runde eines PRD-Chats ausführen (startet die TUI)`.

7. **`internal/tui/chat.go`.** Der Typ `chat` wird ein Fenster auf Dateien:
   - Entferne `send`, `sendAs`, `next`, `emit`, `chatWriter`, `writeLog`, `writeLogEvent`, `handleDone`,
     `chatEventMsg`, `chatDoneMsg` sowie die Felder `running`, `cancel`, `events`, `closed` und `log`.
   - Neue Felder: `projectID`, `id`, `logOffset int64`, `state *chatrun.Chat`.
   - `func openChat(projectID, id string) (*chat, error)` lädt den Zustand und liest das Log ab Anfang.
   - `func (c *chat) refresh()` lädt den Zustand neu und liest neue Log-Zeilen ab `logOffset`. Jede Zeile geht
     an `handleEvent`. `handleEvent` schreibt nicht mehr ins Log und ruft `usage.Record` nicht mehr auf.
     `user_prompt` wird ein Eintrag `roleUser` mit `shown`, ohne `shown` mit `text`. `lazydev` wird `roleError`.
   - `render` nimmt Status und Kosten aus `c.state`. `● Claude arbeitet …` erscheint bei effektivem `läuft`.
   - `updateChat`: Tasten nach den Produktentscheidungen. `Enter` ruft `chatrun.Send` auf, setzt die Eingabe
     zurück und ruft `refresh`. Fehler von `Send` zeigt die Statuszeile. `Ctrl+X` nutzt `m.ask` mit
     `m.returnMode = modeChat`.

8. **`internal/tui/interactive.go`.** `updateNewPRD` ruft `chatrun.Create` mit Titel
   `Neues PRD (<Template>): <Projekt>` und der Beschreibung auf, danach
   `chatrun.Send(p.ID, id, t.CreateDisplay(desc), t.CreatePrompt(desc))`, dann `openChat` und `modeChat`.
   Fehler zeigen die Statuszeile, der Modus bleibt `modeNormal`.

9. **Neue Datei `internal/tui/notifications.go`.**
   - Typ `chats map[string][]chatrun.Entry` (Projekt-ID → Benachrichtigungen, sortiert) und
     `type chatsLoadedMsg struct{ all []chatrun.Entry }`, geladen von `loadChatsCmd` über `chatrun.All`.
   - `func (m *Model) applyChats(all []chatrun.Entry)`: übernimmt nur `Notifies()`. Löscht beendete Chats,
     deren Fenster nicht offen ist, mit `chatrun.RemoveIfDone`. Die Funktion lädt den Zustand unter der Sperre
     neu und löscht nur, wenn der Status noch `beendet` ist. Ein veralteter Stand löscht so keinen Chat, der inzwischen eine Runde startet. Begrenzt `notifCursor`. Ruft bei offenem Fenster
     `m.chat.refresh()`.
   - `func (m Model) notifications(projectID string) []chatrun.Entry`, `func (m Model) showNotifications() bool`.
   - `func (m Model) viewNotifications(l layout) string` nach „Inhalt der Box“.
   - `func (m Model) openNotification(e chatrun.Entry) (Model, tea.Cmd)` und `func (m Model) nextNotification()
     (Model, tea.Cmd)` für `N`.
   - `func (m Model) endChat(projectID, id string) (Model, tea.Cmd)` für `X` und `Ctrl+X`.

10. **`internal/tui/model.go`.** Konstante `focusNotifications` zwischen `focusPRDs` und `focusLog`. Felder
    `chats chats` und `notifCursor int`. `Init` und `loadProjects` laden zusätzlich die Chats.

11. **`internal/tui/view.go`.**
    - `layout` bekommt `notifWidth`. Bei sichtbarer Box gilt `notifWidth = leftWidth` und
      `rightWidth = width − leftWidth − notifWidth`.
    - `View` setzt die Box mit `lipgloss.JoinHorizontal` rechts an.
    - `viewProjects` ergänzt den Indikator nach „Indikator am Projekt“.
    - `helpLine`: Tasten nach „Fußzeile im Chat“ und „Fußzeile im Normalmodus“.
    - `helpSections`: Abschnitt „PRDs“ nach `n`: `{"N", "älteste Rückfrage eines Chats öffnen"}`. Neuer Abschnitt
      „Chat“: `Esc` ausblenden, Runde läuft weiter. `Ctrl+X` beenden. `Ctrl+R` nach Fehler wiederholen.
      `X` in „Benachrichtigungen“ beenden.

12. **`internal/tui/update.go`.**
    - Fälle `chatEventMsg` und `chatDoneMsg` entfernen. Neu: `chatsLoadedMsg`.
    - `watchMsg` mit `chats: true` lädt die Chats. `tickMsg` lädt sie zusätzlich.
    - `updateNormal`: `N`, Fokuswechsel über eine Hilfsfunktion, die die unsichtbare Box überspringt. Bei
      Fokus auf der Box gehen `j`, `k`, `Enter` und `X` an die Box.
    - `handleMouse`: Koordinaten in der Box bewirken nichts.

13. **`internal/tui/watch.go`.** Überwache zusätzlich `chats/` unter `state.Dir()`. Ereignisse für `*.json`
    setzen `chats`, Ereignisse für `*.jsonl` setzen `logs`. Ergänze das Feld `chats` in `watchMsg`.

14. **Tests.**
    - `internal/chatrun/question_test.go`: tabellarisch. Rückfrage für `- A: …`, `**B:** …`, `1. Frage`
      mit `C) …`, Text mit `?` am Ende, `?**` am Ende. Keine Rückfrage für `PRD gespeichert unter prd/x.md.`,
      `Ein Absatz mit ? mitten im Satz.` und `Annahme: …`.
    - `internal/chatrun/round_test.go`: Fake-`claude` als Shell-Skript wie in
      `internal/tui/interactive_test.go`, gesteuert über `FAKE_CLAUDE_MODE` (`question`, `done`, `exit`,
      `noresult`, `iserror`). Prüft Status, Kosten, `sessionId`, `--resume` ab der zweiten Runde, die
      `user_prompt`-Zeile mit `shown`, `pid = 0` nach dem Ende und dass `Round` eine gelöschte Datei nicht
      neu anlegt. `Effective` mit `PID` eines nicht existierenden Prozesses liefert `fehler` und
      `Runde abgebrochen`. `Create` vergibt bei gleicher Sekunde die Suffixe `-2` und `-3`.
    - Tests ersetzen `spawnChat` durch eine Funktion, die `Round` in einer Goroutine startet und 0 zurückgibt.
      `Round` wartet auf die Sperre von `Send`. Exportiere dafür in `export_test.go` oder über eine
      Setter-Funktion `SetSpawn` für Tests aus anderen Paketen.
    - `internal/tui/interactive_test.go`: den bestehenden Chat-Test auf Dateien umstellen: `n`, Template,
      Beschreibung, warten bis der Status nicht mehr `läuft` ist, `chatsLoadedMsg` senden. Prüfen: Eintrag
      in der Box mit `?`, ` 1✉` am Projekt, `Esc` lässt den Eintrag stehen, `N` öffnet den Chat, Antwort mit
      `Enter` entfernt die Benachrichtigung, zweite Runde mit `--resume chat-1`.
    - `internal/tui/notifications_test.go`: Box nur bei Benachrichtigung sichtbar, Breite gleich
      `leftWidth`, Fokus überspringt die unsichtbare Box, `N` ohne Einträge zeigt `Keine offenen Rückfragen`,
      `X` mit `j` löscht die Zustandsdatei und lässt das Log stehen, `Ctrl+R` bei `fehler` startet eine Runde
      mit derselben Eingabe, beendete Chats verschwinden. `checkViewSize` mit sichtbarer Box bei 80 und
      200 Spalten.

15. **Doku.**
    - `SPEC.md`, „Layout“: Box „Benachrichtigungen“ rechts und Indikator `✉`. „Tasten“: Zeilen für `N`, `X` in
      der Box, `Ctrl+X` und `Ctrl+R` im Chat. „Neues PRD (`n`)“: Schritte 3, 4 und 7 auf losgelöste Runden,
      `Esc` ausblenden, Status, Erkennung der Rückfrage und die neuen Pfade umschreiben. „Prozesse“: Chat-Runden
      als losgelöste Prozesse, Prüfung über `/proc/<pid>/cmdline`. „Datenhaltung“: Zustandsdatei des Chats.
    - `README.md`, „Tasten“: `N`, und im Chat `Ctrl+X`, `Ctrl+R`, `Esc`. „Befehle ohne TUI“: `lazydev chat`.
      „Dateien“: Zustandsdatei und Log unter `chats/`.

## Technische Hinweise

- Alle Texte sind deutsch: UI-Strings, Fehlermeldungen, Kommentare. Kommentare kurz und im Präsens wie in
  den umliegenden Dateien.
- Die TUI liest nur Dateien. Schreibende Ausnahmen sind `chatrun.Create`, `Send`, `Stop` und `Remove`.
- Ein Paket mit dem Namen `chat` kollidiert mit dem Typ `chat` in `internal/tui`. Deshalb heißt das Paket
  `chatrun`.
- `Send` hält die Sperre, bis die PID gespeichert ist. `Round` wartet beim Start auf dieselbe Sperre. So
  überschreibt die Runde nie einen Zustand ohne PID. flock wirkt pro geöffneter Datei, deshalb funktioniert
  das auch in Tests im selben Prozess.
- Prüfe die Breite von `✉` in einem Test mit `ansi.StringWidth`. Der Wert muss 1 sein, sonst verschiebt sich
  die Projektliste (siehe Commit `ba0179a`).
- `Model` wird per Wert kopiert. `chat` bleibt ein Zeiger.
- `Update` ruft nach jeder Nachricht `refreshLog`, `syncDetail` und `syncPRDView` auf. `chat.refresh` liest
  nur bei `chatsLoadedMsg` und `watchMsg`, nicht bei jeder Nachricht.
- Eine laufende TUI startet Runden mit dem installierten Binary (`os.Executable()`). Teste den Unterbefehl
  nach `make install`.
- Claude findet eine Sitzung nur im Ordner, in dem sie lief. `dir` im Zustand ist der Hauptordner des Projekts.
- Tests isolieren sich über `t.Setenv("XDG_STATE_HOME", t.TempDir())` und `XDG_DATA_HOME`.

## Erfolgskriterien

- `make test` läuft ohne Fehler, also `go vet ./...` und `go test ./...`.
- `go test ./internal/chatrun ./internal/tui` läuft ohne Fehler.
- Nach `n` und der Beschreibung läuft ein Prozess `lazydev chat <projekt-id> <chat-id>`. `Esc` blendet das
  Fenster aus, die Runde läuft weiter.
- Endet die Runde mit Optionen `A:` bis `D:` oder einer Frage, erscheint die Box „Benachrichtigungen“ mit dem
  Eintrag und das Projekt zeigt ` 1✉`.
- `N` öffnet den ältesten Eintrag über alle Projekte. Nach deiner Antwort verschwindet der Eintrag.
- Nach `q` und Neustart der TUI zeigt der Chat seinen vollständigen Verlauf, und offene Rückfragen stehen
  wieder in der Box.
- Ein abgebrochener Rundenprozess erscheint als Fehler-Eintrag. `Ctrl+R` wiederholt die letzte Eingabe.
- `Ctrl+X` und `X` beenden einen Chat nach Rückfrage. Das Log bleibt erhalten.
- Ohne Benachrichtigung im gewählten Projekt hat die PRD-Box dieselbe Breite wie heute.
- `README.md` und `SPEC.md` beschreiben Box, Indikator, Tasten, Unterbefehl und Dateien.
