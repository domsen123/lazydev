# PRD: Einheitliche Overlay-Fenster mit Innenabstand und gedimmtem Hintergrund

> Status: umgesetzt (2026-10-20) · Template: einfach · Scope: Chat-, Hilfe- und Limits-Fenster sowie Template-Auswahl und Beschreibung als Fenster mit 2 Spalten und 1 Zeile Innenabstand, gedimmter TUI dahinter und eigenem Rahmen für die Chat-Eingabe.

## Context

Das Chat-Fenster für neue PRDs (`n`) wirkt gedrängt und hebt sich kaum von der TUI ab:

- `chat.render` (`internal/tui/chat.go:252`) zeichnet das Fenster mit `box()` (`internal/tui/box.go:35`).
  Der Inhalt hat links 1 Leerzeichen Abstand, oben, unten und rechts keinen.
- `overlay()` (`internal/tui/chat.go:318`) setzt das Fenster zentriert über die TUI. Die TUI dahinter
  behält ihre Farben. Das Fenster verschwimmt deshalb mit den Rahmen von Projekten, PRDs und Log.
- Die Eingabe trennt eine `─`-Linie vom Verlauf ab.
- Hilfe (`viewHelp`, `internal/tui/view.go:521`) und Limits (`viewLimits`, `internal/tui/limits.go:161`)
  nutzen ebenfalls `box()` und `overlay()`, jede mit eigener Einrückung von 1 Leerzeichen.
- Template-Auswahl und Beschreibung stehen nicht in einem Fenster, sondern in der Statuszeile der
  Fußzeile (`viewFooter`, `internal/tui/view.go:288`, und `viewTemplatePicker`, `internal/tui/view.go:324`).

Nach der Änderung zeichnen alle Overlays denselben Fensterstil: runder Rahmen mit Titel wie bisher,
2 Spalten Abstand links und rechts, 1 Leerzeile oben und unten. Hinter einem offenen Overlay erscheint
die TUI ohne Farben in `colorMuted`. Die Fußzeile bleibt farbig und zeigt die Tastenhilfe wie bisher.
Template-Auswahl und Beschreibung werden zu zwei eigenen Fenstern. Die Chat-Eingabe bekommt einen eigenen
Rahmen mit dem Titel „Antwort“.

Wiederverwendet werden `box()`, `printable()`, `padRight()`, `overlay()`, `chatSize()`, die Farben und
Stile aus `internal/tui/box.go` sowie `prdtemplate.All` mit `Label` und `Description`.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Schatten | Kein Schlagschatten. Hinter einem offenen Overlay dimmt lazydev die TUI: Jede Zeile verliert ihre ANSI-Stile und erscheint komplett in `colorMuted`. |
| Gedimmter Bereich | Alles oberhalb der Fußzeile: Projekte, PRDs, Details oder Log. Die Fußzeile bleibt ungedimmt. |
| Rahmen des Fensters | Unverändert: runder Rahmen und Titel im btop-Stil aus `box()`, in `colorFocus`. |
| Innenabstand | Für alle Overlays gleich: 2 Spalten links, 2 Spalten rechts, 1 Leerzeile oben, 1 Leerzeile unten. |
| Betroffene Fenster | Chat, Hilfe, Claude-Limits, „Template wählen“, „Neues PRD · <Template-Label>“. |
| Chat-Eingabe | Eigener runder Rahmen innerhalb des Fensters, gezeichnet mit `box("Antwort", …, false)`, also in `colorBorder` mit Titel „Antwort“. Die `─`-Trennlinie entfällt. |
| Chat-Statuszeile | Kosten und „● Claude arbeitet …“ stehen wie bisher direkt über der Eingabe, jetzt über dem Rahmen „Antwort“. |
| Abstand im Rahmen „Antwort“ | 1 Leerzeichen links vor dem Prompt `> `, rechts kein zusätzlicher Abstand. |
| Template-Auswahl | Eigenes Fenster mit Titel „Template wählen“. Pro Template eine Zeile mit Markierung und Label, darunter die Beschreibung in `colorMuted`, eingerückt um 2 Spalten. Zwischen den Templates steht eine Leerzeile. |
| Markierung in der Template-Auswahl | Das gewählte Template zeigt `> ` und das Label in `styleSelected` (fett). Die anderen zeigen `  ` und das Label in `colorMuted`. |
| Tasten der Template-Auswahl | Unverändert (`←` `→`, `h` `l`, `j` `k`, `↑` `↓`, `Tab`, `Shift+Tab`, `Enter`, `Esc`, `q`, `Ctrl+C`). Die Tastenhilfe zeigt `↑ ↓` statt `← →`, weil die Liste senkrecht steht. |
| Beschreibungsfenster | Eigenes Fenster mit Titel „Neues PRD · <Template-Label>“. Erste Zeile: „Beschreibe das Feature in einem Satz.“ in `colorMuted`. Danach eine Leerzeile, danach das Eingabefeld. |
| Prompt der Beschreibung | `> ` ohne Template-Label. Der Platzhalter bleibt „z. B. Avatar-Upload im Profil“. |
| Breite der Fenster „Template wählen“ und „Neues PRD“ | Wie der Chat: die Breite aus `chatSize()`, also `max(20, min(Breite − 6, 110))`. |
| Höhe der Fenster „Template wählen“ und „Neues PRD“ | Inhalt plus 2 Zeilen Abstand plus 2 Zeilen Rahmen. |
| Breite von Hilfe und Limits | Unverändert: `min(Breite − 4, 72)` und `min(Breite − 4, 96)`. |
| Höhe von Hilfe und Limits | `min(Höhe − 2, Zeilen + 4)`, damit der neue Abstand Platz hat. |
| Größe des Chats | Unverändert `chatSize()`. |
| Statuszeile der Fußzeile in `modeTemplate` und `modeNewPRD` | Zeigt keinen Picker und kein Eingabefeld mehr. Sie verhält sich wie im Normalmodus und zeigt eine vorhandene Meldung. |
| Tastenhilfe der Fußzeile | Unverändert pro Modus, außer `← →` → `↑ ↓` in `modeTemplate`. |
| Chat-Titel | Unverändert „Neues PRD (<Template-Label>): <Projekt>“. |

## Scope

**In Scope**

- Gemeinsame Funktion `dialog` in `internal/tui/box.go` für Rahmen mit Innenabstand.
- Dimmen des Hintergrunds in `overlay()`. Die Fußzeile liegt außerhalb des gedimmten Bereichs.
- Chat, Hilfe und Limits auf `dialog` umstellen.
- Template-Auswahl und Beschreibungseingabe als eigene Overlays.
- Eigener Rahmen „Antwort“ um die Chat-Eingabe.
- Anpassung der Tests in `internal/tui/interactive_test.go` und neue Tests für `dialog` und `overlay`.
- Anpassung von `SPEC.md` und `README.md`.

**Out of Scope**

- Schlagschatten aus Zeichen oder Hintergrundfarbe.
- Hintergrundfarbe für das Fensterinnere.
- Andere Rahmenarten (dick, doppelt).
- Änderungen an den Vollbildansichten Ereignis (`viewEvent`) und Docker (`viewDocker`).
- Die Eingabe beim Hinzufügen eines Projekts (`a`) und die Rückfragen `j/n` bleiben in der Fußzeile.
- Neue Tasten oder geändertes Verhalten von Chat, Template-Auswahl und Beschreibung.

## Umsetzung

1. **`dialog` in `internal/tui/box.go`.** Füge unter `box()` die Funktion
   `dialog(title string, lines []string, width, height int) string` mit Kommentar hinzu. Sie:
   - kürzt jede Zeile mit `ansi.Truncate(printable(line), width-6, "…")` und füllt sie mit `padRight` auf
     `width-6` Zellen auf,
   - setzt vor jede Zeile 2 Leerzeichen und dahinter 2 Leerzeichen,
   - stellt eine leere Zeile davor und füllt bis `height-2` Zeilen auf, sodass unten mindestens eine
     Leerzeile bleibt. Zeilen über `height-4` hinaus schneidet sie ab,
   - ruft `box(title, padded, width, height, true)` auf.

   Definiere die Konstanten `dialogPadX = 2` und `dialogPadY = 1` und nutze sie statt der Zahlen. Die
   Innenbreite für Aufrufer ist `width - 2 - 2*dialogPadX`.

2. **Dimmen in `overlay()` (`internal/tui/chat.go:318`).** Vor dem Einsetzen von `fg` ersetzt `overlay`
   jede Zeile von `bg` durch `styleMuted.Render(ansi.Strip(line))`. Das gilt auch für die Teile links und
   rechts des Fensters. Passe den Kommentar an: „overlay dimmt bg und legt fg zentriert darüber.“

3. **`View()` in `internal/tui/view.go:106`.** Baue den Bereich über der Fußzeile und die Fußzeile getrennt:
   - `body := strings.Join(<Projekte+PRDs, Details oder Log>, "\n")`, `footer := m.viewFooter(l)`.
   - Bestimme das Overlay nach Modus: `modeHelp` → `m.viewHelp()`, `modeLimits` → `m.viewLimits(time.Now())`,
     `modeTemplate` → `m.viewTemplatePicker()`, `modeNewPRD` → `m.viewNewPRD()`,
     `modeChat` mit `m.chat != nil` → `m.chat.render(m.chatSize())`.
   - Gibt es ein Overlay, gilt `body = overlay(body, fg, m.width)`.
   - Gib `body + "\n" + footer` zurück.

4. **Fußzeile (`viewFooter`, `internal/tui/view.go:288`).**
   - Entferne `modeNewPRD` aus `case modeAddProject, modeNewPRD:` und entferne `case modeTemplate:`.
     Beide Modi landen damit im `default`-Zweig.
   - In `helpLine` (`internal/tui/view.go:345`) wird für `modeTemplate` aus `"← →"` der Text `"↑ ↓"`.

5. **Template-Fenster (`viewTemplatePicker`, `internal/tui/view.go:324`).** Schreibe die Funktion um.
   Sie baut pro Template aus `prdtemplate.All` zwei Zeilen: `styleSelected.Render("> "+t.Label)` für das
   gewählte, sonst `styleMuted.Render("  "+t.Label)`. Darunter steht `styleMuted.Render("    "+t.Description)`.
   Zwischen zwei Templates steht `""`. Rückgabe: `dialog("Template wählen", lines, w, len(lines)+4)` mit
   `w, _ := m.chatSize()`. Passe den Kommentar an.

6. **Beschreibungsfenster, neue Funktion `viewNewPRD` in `internal/tui/view.go`.** Platziere sie unter
   `viewTemplatePicker`. Inhalt: `styleMuted.Render("Beschreibe das Feature in einem Satz.")`, `""`,
   `m.input.View()`. Titel: `"Neues PRD · " + prdtemplate.All[m.template].Label`. Größe: Breite aus
   `chatSize()`, Höhe `len(lines)+4`.

7. **Eingabefeld der Beschreibung (`internal/tui/interactive.go:67`).**
   - Im `enter`-Zweig von `updateTemplate` wird `m.input.Prompt` zu `"> "`. Setze
     `m.input.Width = m.newPRDInputWidth()`.
   - Füge `newPRDInputWidth()` in `internal/tui/interactive.go` hinzu:
     `w, _ := m.chatSize(); return max(1, w-2-2*dialogPadX-ansi.StringWidth(m.input.Prompt)-1)`.
     Die `-1` lässt Platz für den Cursor.
   - In `update.go:21` (`tea.WindowSizeMsg`) setzt lazydev im Modus `modeNewPRD` zusätzlich
     `m.input.Width = m.newPRDInputWidth()`.
   - In `update.go:196` (Projekt hinzufügen) setzt lazydev `m.input.Width = 0`, damit die Projektpfad-Eingabe
     wie bisher ohne Breitenbegrenzung läuft.

8. **Chat-Fenster (`chat.render`, `internal/tui/chat.go:252`).**
   - Innenbreite `inner := width - 2 - 2*dialogPadX`, Innenhöhe `rows := height - 2 - 2*dialogPadY`.
   - Die Textarea bekommt `c.input.SetWidth(inner - 3)`: 2 Spalten Rahmen „Antwort“ und 1 Leerzeichen links.
   - Der Rahmen „Antwort“ ist `box("Antwort", <" "+Zeile pro Textarea-Zeile>, inner, len(inputLines)+2, false)`.
   - `historyRows := max(1, rows - 1 - (len(inputLines)+2))`: Statuszeile und Rahmen „Antwort“.
   - Verlauf mit `renderEntry(e, inner)`, ohne führendes Leerzeichen pro Zeile. Darauf folgen die Statuszeile
     ohne führendes Leerzeichen und die Zeilen des Rahmens „Antwort“. Die `─`-Trennlinie entfällt.
   - Rückgabe `dialog(c.title, body, width, height)`.

9. **Hilfe (`viewHelp`, `internal/tui/view.go:521`).** Entferne die Einrückung um 1 Leerzeichen:
   Abschnittstitel ohne Leerzeichen, Tastenzeilen mit 2 statt 3 Leerzeichen, Schlusszeile ohne Leerzeichen.
   Rückgabe `dialog("Hilfe", lines, width, min(m.height-2, len(lines)+4))`.

10. **Limits (`viewLimits`, `internal/tui/limits.go:161`).** Entferne das führende Leerzeichen aller Zeilen.
    Das betrifft die Hinweise ohne Stand, „Stand …“, die Fensterzeilen, „Status …“, „Läufe in lazydev …“,
    den Hinweis zum API-Preis, „● Frage Claude …“ und die Tastenzeile. Rückgabe
    `dialog("Claude-Limits", lines, width, min(m.height-2, len(lines)+4))`.

11. **Tests.**
    - `internal/tui/box_test.go`: neuer Test `TestDialogPadding`. `dialog("T", []string{"x"}, 20, 6)` liefert
      6 Zeilen mit je 20 Zellen (`ansi.StringWidth`). Nach `ansi.Strip` ist Zeile 2 `"│" + 18 Leerzeichen + "│"`.
      Zeile 3 beginnt mit `"│  x"` und endet mit `"  │"`. Zeile 5 ist leer zwischen den Rahmen.
      Ein zweiter Fall prüft, dass eine Zeile mit 30 Zeichen bei Breite 20 mit `…` endet und rechts
      2 Leerzeichen Abstand behält.
    - `internal/tui/box_test.go`: neuer Test `TestOverlayDimsBackground`. `bg` enthält zwei Zeilen
      `"\x1b[31mrot\x1b[0m" + 37 Leerzeichen`, `fg` ist eine Zeile `"X"`. Das Ergebnis von
      `overlay(bg, fg, 40)` enthält kein `"\x1b[31m"` mehr. Der Text `rot` und `X` bleiben sichtbar.
    - `internal/tui/interactive_test.go:78` (`TestNewPRDChat`): Erwarte statt `"Template: > Nuxt-Feature"`
      die Texte `"Template wählen"` und `"> Nuxt-Feature"` in `ansi.Strip(m.View())`. Prüfe nach dem
      ersten `Enter`, dass die Ansicht `"Neues PRD · Nuxt-Feature"` und
      `"Beschreibe das Feature in einem Satz."` enthält. Prüfe im Chat zusätzlich `"Antwort"`. Prüfe
      außerdem, dass `m.View()` genau `m.height` Zeilen hat und keine Zeile breiter als `m.width` ist.
    - `internal/tui/interactive_test.go:141` (`TestNewPRDSimpleTemplate`): Ersetze die Prüfung
      `strings.Contains(m.input.Prompt, "Einfach")` durch die Prüfung, dass `ansi.Strip(m.View())`
      `"Neues PRD · Einfach"` enthält.

12. **Doku.**
    - `SPEC.md`, Abschnitt „Neues PRD (`n`)“, Schritt 1: „lazydev zeigt in der Fußzeile die Templates“ wird
      zu „lazydev öffnet das Fenster ‚Template wählen‘ mit den Templates untereinander und ihrer Beschreibung“.
      Schritt 2: „lazydev fragt im Fenster ‚Neues PRD · <Template>‘ nach einer kurzen Beschreibung.“
      Schritt 4: Ersetze „oben die Kosten“ durch „über dem Eingabefeld ‚Antwort‘ die Kosten“.
    - `SPEC.md`, Abschnitt „Layout“: Ergänze nach der Liste einen Absatz „Fenster über der TUI“. Er nennt
      die fünf Fenster, den Innenabstand (2 Spalten, 1 Zeile), das Dimmen der TUI in `colorMuted` und die
      ungedimmte Fußzeile mit Tastenhilfe.
    - `README.md` bleibt unverändert. Schritt 3 der Schnellstart-Anleitung (`README.md:39`) und die
      Tastentabelle (`README.md:59`) nennen keine Fußzeile und stimmen weiter.

## Technische Hinweise

- Alle Texte, Kommentare und Testmeldungen sind deutsch. Commits folgen Conventional Commits, etwa
  `feat(tui): …`.
- `box()` kürzt zu lange Zeilen selbst. `dialog` kürzt trotzdem vorher auf `width-6`, sonst frisst eine
  lange Zeile den rechten Abstand.
- `printable()` muss vor jeder Breitenmessung laufen. Formatzeichen wie U+00AD verschieben sonst die
  Oberfläche (siehe `TestBoxDropsSoftHyphen`).
- `overlay()` dimmt per `ansi.Strip`. Dadurch verliert auch die Hinterlegung mit `colorSelected` (`highlight`)
  ihre Farbe. Das ist gewollt.
- Ohne TTY rendert lipgloss in Tests keine Farben. Tests, die Farben prüfen, bauen die Escape-Sequenzen
  deshalb selbst, wie in `TestOverlayDimsBackground`.
- `m.input` ist ein `textinput.Model`, das auch `modeAddProject` nutzt. Setze `Width` deshalb in beiden
  Einstiegen (Schritt 7). `textinput` berechnet den sichtbaren Ausschnitt in `Update` über `Width`. Den Wert
  nur in `View` zu setzen, reicht nicht.
- Der Rahmen „Antwort“ nutzt `box()` mit `focused=false`. `box()` zeichnet den Titel ab einer Innenbreite
  von 4 Zellen. Bei schmaleren Fenstern fällt er ohne Titel zurück. Das ist bestehendes Verhalten.
- Bei der Mindesthöhe 10 aus `chatSize()` bleibt für den Verlauf 1 Zeile: 10 − 2 Rahmen − 2 Abstand
  − 1 Status − 5 „Antwort“ = 0, `max(1, …)` hält mindestens 1.
- `View()` darf in Summe nie mehr als `m.height` Zeilen liefern. `overlay()` hängt keine Zeilen an, sondern
  ersetzt bestehende. Dadurch bleibt die Gesamthöhe gleich.

## Erfolgskriterien

- `make test` läuft ohne Fehler, inklusive `TestDialogPadding`, `TestOverlayDimsBackground`,
  `TestNewPRDChat` und `TestNewPRDSimpleTemplate`.
- `go test ./internal/tui -run 'TestDialog|TestOverlay|TestNewPRD'` läuft ohne Fehler.
- Chat, Hilfe, Limits, „Template wählen“ und „Neues PRD · <Template>“ zeigen 2 Spalten Abstand links und
  rechts und 1 Leerzeile oben und unten innerhalb des Rahmens.
- Bei offenem Overlay erscheint die TUI über der Fußzeile ohne Farben in `colorMuted`. Die Fußzeile zeigt
  die Tastenhilfe in den gewohnten Farben.
- Die Chat-Eingabe steht in einem eigenen runden Rahmen mit dem Titel „Antwort“. Eine `─`-Trennlinie gibt
  es nicht mehr.
- Die Statuszeile der Fußzeile zeigt in `modeTemplate` und `modeNewPRD` weder Picker noch Eingabefeld.
- `View()` liefert in allen Modi genau `m.height` Zeilen, keine breiter als `m.width`.
- `SPEC.md` beschreibt die Fenster „Template wählen“ und „Neues PRD · <Template>“, den Innenabstand und
  das Dimmen.
