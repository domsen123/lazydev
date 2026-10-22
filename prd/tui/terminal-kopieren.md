# PRD: Text aus Terminal-Tabs kopieren

> Status: geplant · Template: einfach · Scope: Terminal-Tabs bekommen einen Kopiermodus (`alt+c`) mit Scrollback, Zeilenauswahl per Tastatur und Kopieren in die Zwischenablage, dazu Mausauswahl und Mausrad direkt im Tab. Kopiert wird nur der Inhalt des Tabs ohne Rahmen und Nachbarspalten, umgebrochene Zeilen werden wieder zusammengefügt.

## Context

Container-Logs (`l` im Tab `Docker`, `updateBottom` in `internal/tui/keymap.go`) laufen als
`docker logs -f --tail 200 <container>` in einem Terminal-Tab der Mitte. Wer daraus Text kopieren will,
markiert mit der Maus im eigenen Terminal-Programm. lazydev läuft im Alt-Screen (`tea.WithAltScreen()`,
`internal/tui/main.go`) und zeichnet drei Spalten nebeneinander. Die native Markierung erfasst deshalb ganze
Bildschirmzeilen:

- Projekte, Rahmen `│` und Explorer landen mit in der Zwischenablage.
- Jede Zeile, die x/vt an der Tab-Breite umbricht, wird beim Einfügen zu einem harten Zeilenumbruch.
- Nur der sichtbare Bildschirm ist erreichbar. Der Scrollback des Emulators (`vt.DefaultScrollbackSize`,
  10 000 Zeilen) ist in lazydev nicht erreichbar.

Das gilt für jeden Terminal-Tab, also auch für Shell, Claude Code, Editor und Git-Tabs.

Nach der Änderung gibt es zwei Wege, die beide nur den Inhalt des Tabs kopieren und umgebrochene Zeilen wieder
zusammenfügen:

1. **Kopiermodus per Tastatur:** `alt+c` friert den Tab ein, `j/k` und Seitentasten bewegen eine Zeilenmarke
   durch Scrollback und Bildschirm, `v` beginnt eine Auswahl, `y` kopiert.
2. **Maus:** Ziehen im Tab markiert zeichengenau und kopiert beim Loslassen, das Mausrad scrollt durch den
   Scrollback.

Im Repo existiert bereits und wird wiederverwendet:

- `terminal.Terminal` mit `vt.SafeEmulator`, `View(showCursor)`, `Resize` (`internal/terminal/terminal.go`).
- `SafeEmulator.Scrollback()`, `ScrollbackLen()`, `IsAltScreen()`, `CellAt`, `Width`, `Height` aus
  `github.com/charmbracelet/x/vt` v0.1.0. `uv.Line.String()` und `uv.Line.Render()` aus
  `github.com/charmbracelet/ultraviolet`.
- `ansi.SetSystemClipboard` (OSC 52) aus `github.com/charmbracelet/x/ansi`, `ansi.Cut`, `ansi.Strip`.
- `github.com/atotto/clipboard` liegt schon als indirekte Abhängigkeit (über `bubbles`) im `go.mod`.
- `columns`, `rows`, `termSize`, `centerChrome` (`internal/tui/main.go`) für die Lage des Terminalbereichs.
- `inTerminal`, `global`, `globalKeys`, `footerKeys`, `updateKey` (`internal/tui/keymap.go`), `m.note` und
  `m.err` für die Fußzeile.
- Muster für Scroll-Tasten aus der Log-Ansicht (`internal/tui/logview.go`).

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Betroffene Tabs | Alle Terminal-Tabs (`*terminal.Terminal`). Log-Tabs (`logView`) bleiben unverändert. |
| Taste für den Kopiermodus | `alt+c`, globale Taste nach Regel 1, wirkt auch im Terminal. Ohne `alt` gibt es sie nicht, weil `c` im PRDs-Tab den Status ändert. |
| `alt+c` ohne Terminal | Ist der aktive Center-Tab des aktuellen Projekts kein Terminal oder gibt es keinen, Fehler `The active tab is not a terminal.` |
| `alt+c` von einem anderen Pane | Setzt den Fokus auf die Mitte und öffnet den Kopiermodus des aktiven Terminal-Tabs. |
| `alt+c` im Kopiermodus | Verlässt den Kopiermodus, wie `esc`. |
| Einfrieren | Beim Betreten nimmt lazydev einen Schnappschuss aller Zeilen: Scrollback (älteste zuerst), dann die Zeilen des Bildschirms. Der Kopiermodus zeigt und kopiert nur den Schnappschuss. Das Programm läuft weiter, seine Ausgabe geht in den Emulator und erscheint nach dem Verlassen. |
| Alt-Screen | Läuft das Programm im Alt-Screen (`IsAltScreen()`), enthält der Schnappschuss nur die Bildschirmzeilen. |
| Startposition | Die Ansicht zeigt das Ende des Schnappschusses, also den Bildschirm wie zuvor. Die Zeilenmarke steht auf der Zeile des Cursors, bei verstecktem oder ungültigem Cursor auf der letzten Zeile. |
| Zeilenmarke | Ganze Zeile in Reverse Video (`\x1b[7m`), über die volle Breite des Tabs. Der Cursor des Programms wird im Kopiermodus nicht gezeichnet. |
| Tasten im Kopiermodus | `j`/`↓` und `k`/`↑` eine Zeile, `ctrl+d`/`pgdown` und `ctrl+u`/`pgup` eine halbe Höhe, `g`/`home` erste Zeile, `G`/`end` letzte Zeile. Die Ansicht scrollt mit, sodass die Marke sichtbar bleibt. |
| Auswahl per Tastatur | `v` setzt den Anker auf die Zeile der Marke, ein zweites `v` hebt die Auswahl auf. Ausgewählt sind alle Zeilen zwischen Anker und Marke, beide eingeschlossen, in Reverse Video. Die Tastatur wählt nur ganze Zeilen. |
| `y` | Kopiert die Auswahl, ohne Auswahl die Zeile der Marke, und verlässt den Kopiermodus. |
| `esc`, `q` | Verlassen den Kopiermodus ohne zu kopieren. |
| Andere Tasten im Kopiermodus | Gehen nicht an das Programm. Globale `alt`-Tasten gelten. Ohne `alt` gilt Regel 2 nicht, unbelegte Tasten tun nichts. |
| Tab-Titel im Kopiermodus | Das Label in der Tab-Leiste bekommt die Endung ` [copy]`. |
| Fußzeile im Kopiermodus | ` j/k move · ctrl+d/u page · g/G top/end · v select · y copy · esc leave · alt+←/→ pane · alt+shift+←/→ tab · alt+w close · alt+m menu · alt+q quit` |
| Fokus wechseln oder Tab wechseln | Der Kopiermodus bleibt im Tab erhalten, bis er verlassen wird. Er ist ein Zustand des Tabs, nicht des Fokus. |
| Programm endet im Kopiermodus | Der Tab schließt wie bisher, der Kopiermodus endet mit ihm. |
| Größenänderung im Kopiermodus | Der Schnappschuss bleibt. Marke, Anker und Ansicht werden auf den gültigen Bereich geklemmt. Zeilen breiter als der Tab werden abgeschnitten. |
| Maus | lazydev startet mit `tea.WithMouseCellMotion()`. Mausereignisse wirken nur im Terminalbereich des aktiven Terminal-Tabs, sonst ignoriert lazydev sie. Bei offenem Overlay (Menü, Wizard, Status-Auswahl, Git-Cockpit, Hilfe, Frage, Eingabe) ignoriert lazydev jede Maus. |
| Native Markierung | Geht weiterhin mit gedrückter Shift-Taste beim Ziehen, so wie es die meisten Terminal-Programme anbieten. README nennt das. |
| Maus an Programme | Mausereignisse gehen nicht an das Programm im Tab, wie heute. |
| Mausrad im Tab | Rad hoch: 3 Zeilen nach oben. Außerhalb des Kopiermodus betritt Rad hoch zuerst den Kopiermodus (Schnappschuss wie bei `alt+c`) und scrollt dann. Rad runter: 3 Zeilen nach unten. Die Marke wandert mit der Ansicht, sie bleibt an derselben Stelle im Fenster. |
| Kopiermodus per Mausrad beenden | Wurde der Kopiermodus durch das Mausrad betreten und erreicht Rad runter das Ende ohne aktive Auswahl, endet der Kopiermodus. Mit `alt+c` betreten endet er nur mit `esc`, `q`, `alt+c` oder `y`. |
| Klick im Tab | Setzt den Fokus auf die Mitte. |
| Ziehen | Linke Taste drücken im Terminalbereich betritt den Kopiermodus, falls er nicht aktiv ist, und setzt den Anfang der Auswahl auf die Zelle unter der Maus. Bewegen mit gedrückter Taste setzt das Ende. Die Auswahl ist zeichengenau wie in einem Terminal: von der Anfangszelle bis zum Zeilenende, volle Zeilen dazwischen, von Zeilenanfang bis zur Endzelle. Anfang und Ende dürfen vertauscht sein. |
| Ziehen über den Rand | Steht die Maus beim Ziehen oberhalb oder unterhalb des Terminalbereichs, scrollt die Ansicht pro Bewegungsereignis eine Zeile in diese Richtung, das Ende der Auswahl folgt. Links und rechts klemmt die Spalte auf den Bereich. |
| Loslassen | Kopiert die Auswahl, entfernt die Markierung. Hat die Maus den Kopiermodus betreten, endet er danach, sonst bleibt er. Liegen Anfang und Ende auf derselben Zelle, wird nichts kopiert. |
| Tastatur und Maus gemischt | Drücken mit der Maus hebt eine Tastatur-Auswahl auf. `v` hebt eine Maus-Auswahl auf. |
| Kopierter Text | Ohne ANSI-Stile. Leerzeichen am Zeilenende entfernt. Zeilen getrennt durch `\n`, kein `\n` nach der letzten Zeile. Zellen von breiten Zeichen (`IsZero()`) zählen einmal. |
| Umgebrochene Zeilen | x/vt merkt sich keine weichen Umbrüche. Regel: Reicht eine Zeile bis zur letzten Spalte der Breite, mit der der Tab beim Schnappschuss lief (Breite in Zellen ≥ Tab-Breite), wird die nächste Zeile ohne `\n` angehängt. Gilt für Scrollback und Bildschirm. Nach einer Größenänderung des Tabs kann die Regel für ältere Zeilen danebenliegen, das ist akzeptiert. |
| Zwischenablage | Immer OSC 52 (`ansi.SetSystemClipboard`) an das Terminal von lazydev. Zusätzlich `clipboard.WriteAll` aus `github.com/atotto/clipboard`, wenn `clipboard.Unsupported` falsch ist und weder `SSH_CONNECTION` noch `SSH_TTY` gesetzt sind. Ein Fehler von `WriteAll` wird ignoriert, OSC 52 ist geschrieben. |
| Meldung nach dem Kopieren | `m.note`: `Copied 1 line.` bzw. `Copied <n> lines.` Gezählt werden Zeilen des kopierten Textes nach dem Zusammenfügen. Leerer Text: nichts kopieren, Fehler `Nothing to copy.` |
| Fehler beim Schreiben von OSC 52 | Fehler `Copy failed: <err>`. |
| Obergrenze | Keine eigene. Manche Terminals begrenzen OSC 52, das nennt README. |

## Scope

**In Scope**

- Kopiermodus mit Schnappschuss, Zeilenmarke, Tastatur-Auswahl und `y` in `internal/terminal`.
- Mausauswahl, Mausrad und Klick im Terminalbereich in `internal/tui`.
- Zusammenfügen umgebrochener Zeilen und Text ohne Stile.
- Zwischenablage über OSC 52 und lokal über `atotto/clipboard`.
- `alt+c`, Tab-Endung ` [copy]`, Fußzeile, Meldungen.
- Tests, `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Kopieren aus Log-Tabs (`logView`), aus Panes und Overlays.
- Mausereignisse an Programme im Tab weiterreichen.
- Klicks auf Tab-Leiste, Panes oder Listen.
- Doppel- und Dreifachklick (Wort, Zeile), Blockauswahl, Suche im Kopiermodus.
- Einfügen aus der Zwischenablage über lazydev.
- Scrollback ohne Kopiermodus, z. B. dauerhaftes Hochscrollen bei laufender Ausgabe.
- Änderungen an `docker logs` oder an Runner und Log-Format.

## Umsetzung

1. **`go.mod`:** `github.com/atotto/clipboard` in den direkten `require`-Block verschieben (`go mod tidy`).

2. **`internal/terminal/terminal.go`:** Feld `mu sync.Mutex` in `Terminal`. Die Lese-Goroutine in `Start` hält
   `mu` um `t.emu.Write`. Neue Methode `snapshot() copyLines`, die unter `mu` Scrollback (nur wenn nicht
   `IsAltScreen()`) und Bildschirmzeilen liest. Grund: `SafeEmulator.Scrollback()` gibt den Zeiger ohne Lock
   zurück, das Lesen der Zeilen danach wäre sonst ein Data Race mit `Write`. Bildschirmzeilen über `CellAt(x, y)`
   zu einer `uv.Line` zusammensetzen.

3. **Neue Datei `internal/terminal/copy.go`:**
   - Typ `copyLine` mit `styled string` (`uv.Line.Render()`), `plain string` (`uv.Line.String()` ohne
     Leerzeichen am Ende) und `full bool` (Breite der Zeile in Zellen, ohne leere Zellen am Ende, ≥ Breite des
     Tabs beim Schnappschuss). `copyLines []copyLine`.
   - Typ `copyMode` mit Zeilen, `top` (erste sichtbare Zeile), `mark` (Zeile der Marke), `anchor` (−1 ohne
     Tastatur-Auswahl), Maus-Auswahl (`selecting bool`, Anfang und Ende als Zeile und Spalte), `byWheel bool`,
     `byMouse bool`.
   - Funktion `joinText(lines copyLines, from, to selectionPoint) string`: baut den Text nach den
     Produktentscheidungen. Spaltenschnitte mit `ansi.Cut` auf `plain`, `full` steuert das `\n`.
   - Methoden an `Terminal`: `EnterCopy(byWheel, byMouse bool)`, `LeaveCopy()`, `InCopy() bool`,
     `CopyKey(key string) (text string, done bool)` für die Tasten aus den Produktentscheidungen (liefert bei `y`
     den Text und `done`), `Scroll(lines int) (left bool)` für das Mausrad (liefert, ob der Kopiermodus beim
     Erreichen des Endes nach der Regel für `byWheel` endete), `MousePress(x, y int)`, `MouseDrag(x, y int)`,
     `MouseRelease(x, y int) (text string)`. `x` und `y` sind Zellen relativ zum Terminalbereich, `y` darf
     negativ oder ≥ Höhe sein (Ziehen über den Rand).
   - `TabTitle()` hängt im Kopiermodus ` [copy]` an.
   - `View(showCursor)`: im Kopiermodus die Zeilen `top` bis `top+Höhe−1` aus `styled`, mit `ansi.Truncate`
     auf die Breite, auf die Höhe aufgefüllt. Marke und Auswahl in Reverse Video über `plain` (ohne Stile),
     zeichengenau bei der Maus, auf die Breite mit Leerzeichen aufgefüllt bei ganzen Zeilen. Kein
     Programm-Cursor.
   - `Resize`: im Kopiermodus `top`, `mark`, `anchor` und Maus-Auswahl klemmen.

4. **`internal/terminal/copy_test.go`:** Tests ohne PTY mit `vt.NewSafeEmulator` oder direkt mit `copyLines`:
   - Eine Zeile mit 25 Zeichen in einem 10 breiten Tab ergibt nach dem Kopieren eine Zeile ohne `\n`.
   - Zwei kurze Zeilen ergeben `a\nb`, Leerzeichen am Ende fehlen.
   - Zeichengenaue Auswahl über drei Zeilen, auch mit vertauschtem Anfang und Ende.
   - Breite Zeichen (z. B. `日本`) werden einmal kopiert.
   - Tasten: `k` am Anfang bleibt bei 0, `G` springt ans Ende, `v` `k` `k` `y` kopiert 3 Zeilen und beendet den
     Modus, `esc` beendet ohne Text.
   - Schnappschuss: Ausgabe nach `EnterCopy` ändert `View` nicht, nach `LeaveCopy` erscheint sie.
   - `Scroll` mit `byWheel` endet am Ende, ohne `byWheel` nicht.

5. **`internal/tui/main.go`:** `tea.NewProgram(newModel(bus), tea.WithAltScreen(), tea.WithMouseCellMotion())`.
   Neue Funktion `termOrigin() (x, y int)` neben `termSize`: `x = side + 1`, `y = 3` (Rahmen, Tab-Leiste,
   Trennlinie, siehe `centerChrome`). `Update` behandelt `tea.MouseMsg` in einer neuen Methode `updateMouse`.

6. **Neue Datei `internal/tui/mouse.go`:** `updateMouse(msg tea.MouseMsg)`:
   - Bei offenem Overlay oder Eingabe (dieselben Fälle wie am Anfang von `updateKey`) nichts tun.
   - Aktiven Terminal-Tab des aktuellen Projekts holen, ohne ihn nichts tun.
   - Koordinaten über `termOrigin` und `termSize` umrechnen. Rad und Drücken nur innerhalb des Bereichs,
     Bewegung und Loslassen auch außerhalb, solange eine Maus-Auswahl läuft.
   - `tea.MouseButtonWheelUp`/`WheelDown` → `Scroll(−3/+3)`, vorher `EnterCopy(true, false)` falls nötig.
   - `tea.MouseActionPress` mit `tea.MouseButtonLeft` → Fokus auf `colCenter`, `MousePress`.
   - `tea.MouseActionMotion` → `MouseDrag`, `tea.MouseActionRelease` → `MouseRelease`, Text in die
     Zwischenablage.

7. **Neue Datei `internal/tui/clipboard.go`:** `copyToClipboard(text string) tea.Cmd` nach den
   Produktentscheidungen. OSC 52 in einem einzigen `os.Stdout.Write` schreiben. Das Ergebnis kommt als Message
   zurück und setzt `m.note` oder `m.err`. `clipboard.WriteAll` läuft im selben Cmd, nicht in `Update`.

8. **`internal/tui/keymap.go`:**
   - `globalKeys`: `"alt+c": actCopy`. Die Aktion nach den Produktentscheidungen.
   - `updateKey`: nach den globalen Tasten, vor `inTerminal`, bei aktivem Terminal-Tab im Kopiermodus und Fokus
     Mitte die Taste an `CopyKey` geben, bei `done` mit Text `copyToClipboard`. Im Kopiermodus geht keine Taste
     an `SendKey`.
   - `footerKeys`: neuer erster Fall für den Kopiermodus mit dem Text aus den Produktentscheidungen.
     `inTerminal` bleibt für die Hilfe außerhalb des Kopiermodus und bekommt `alt+c copy` hinter `alt+w close`.
   - Regel 2 darf `c` nicht zu `alt+c` machen: `updateKey` überspringt `c` wie `g`.

9. **`internal/tui/*_test.go`:** Tests in einer neuen `internal/tui/mouse_test.go`:
   - `termOrigin` passt zu `View`: Die Zelle bei `termOrigin` gehört zur ersten Zeile des Terminalbereichs
     (Muster aus `layout_test.go`).
   - Mausereignisse außerhalb des Bereichs und bei offenem Menü ändern nichts.
   - `alt+c` ohne Terminal setzt den Fehler `The active tab is not a terminal.`
   - `c` im Pane ohne eigene Belegung öffnet keinen Kopiermodus.

10. **Doku:**
    - `SPEC.md`: unter `### Keys` die Maus und `alt+c`, neuer Abschnitt `### Copy mode` nach `### Log tab` mit
      allen Produktentscheidungen, in `- **Center:**` der Hinweis auf den Kopiermodus.
    - `README.md` und `README.de.md`: Tastenliste um `alt+c`, Abschnitt zum Kopieren (Kopiermodus, Ziehen,
      Mausrad, Shift+Ziehen für die native Markierung, OSC 52 und mögliche Grenzen des Terminals).

## Technische Hinweise

- Alle UI-Texte, Fehler und Kommentare sind Englisch. `README.de.md` immer zusammen mit `README.md` ändern.
- Commits nach Conventional Commits, z. B. `feat(terminal): add copy mode with scrollback`.
- `tea.MouseMsg` in Bubble Tea v1.3: `X`, `Y`, `Action` (`MouseActionPress`, `MouseActionRelease`,
  `MouseActionMotion`), `Button` (`MouseButtonLeft`, `MouseButtonWheelUp`, `MouseButtonWheelDown`). Mit
  `WithMouseCellMotion` kommen Bewegungen nur bei gedrückter Taste.
- `Line.String()` überspringt `IsZero()`-Zellen (zweite Hälfte breiter Zeichen) und schreibt leere Zellen als
  Leerzeichen. Spalten in `plain` entsprechen damit Anzeigespalten, `ansi.Cut` schneidet korrekt.
- Der Scrollback speichert Zeilen ohne leere Zellen am Ende (`Scrollback.Push`). Die Breite für `full` daher aus
  der Länge der Zeile in Zellen nach dem Kürzen bestimmen, nicht aus `len(line)` des Bildschirms.
- Der Schnappschuss kann groß sein (bis 10 000 Zeilen). Nur Strings speichern, keine `uv.Line`-Kopien.
- `View` des Terminals wird bei jedem Frame gerufen, im Kopiermodus nur das sichtbare Fenster rendern.
- OSC 52 aus einem Cmd kann sich mit einem Frame des Renderers überschneiden. Ein einziger `Write`-Aufruf hält
  das Risiko klein. Nicht über `tea.Printf` schreiben, das funktioniert im Alt-Screen nicht.
- Die Fußzeile löscht `m.note` bei der nächsten Taste (`updateKey`). Die Kopier-Meldung kommt über eine
  Message nach dem Cmd, nicht direkt in `updateMouse`.
- Mausereignisse lösen keinen `m.err, m.note = "", ""`-Reset aus, nur Tasten.

## Erfolgskriterien

- `make test` läuft ohne Fehler, `go vet ./...` ohne Befund.
- In einem Tab mit `docker logs -f` markiert Ziehen nur Log-Text. Eingefügt fehlen Projekte, Explorer und `│`.
  Eine Logzeile, die im Tab über drei Bildschirmzeilen umbricht, kommt als eine Zeile an.
- `alt+c`, `g`, `v`, `G`, `y` kopiert den ganzen Scrollback samt Bildschirm. Die Fußzeile zeigt
  `Copied <n> lines.`
- Mausrad hoch scrollt in Zeilen, die nicht mehr auf dem Bildschirm stehen. Rad runter bis zum Ende zeigt wieder
  die laufende Ausgabe.
- Während des Kopiermodus geht keine Taste an das Programm im Tab. Nach `esc` erscheint die Ausgabe, die in der
  Zwischenzeit kam.
- Über SSH ohne lokale Zwischenablage landet der Text per OSC 52 in der Zwischenablage des lokalen Terminals
  (getestet z. B. mit kitty, WezTerm oder iTerm2).
- Shift+Ziehen markiert weiterhin nativ.
- `SPEC.md`, `README.md` und `README.de.md` beschreiben `alt+c`, Maus und Kopiermodus gleich.
