# PRD: PRDs als erster Tab im Explorer

> Status: umgesetzt (2026-10-21) · Scope: Der Explorer zeigt die Tabs in der Reihenfolge `PRDs`, `Files`. `PRDs` ist beim Start aktiv, das Details-Pane zeigt damit sofort das gewählte PRD.

## Context

Der Explorer (rechte Spalte) hat zwei Tabs: `Files` (Index 0, beim Start aktiv) und `PRDs`. Definiert in
`internal/tui/files.go` über die Konstanten `rightFiles = iota`, `rightPRDs` und `rightTabs = []string{"Files", "PRDs"}`.
`model.rightTab` (`internal/tui/main.go`) hat den Nullwert, also startet lazydev im Tab `Files`. Wer mit PRDs
arbeitet, muss nach jedem Start erst mit `alt+shift+→` wechseln.

PRDs sind die Hauptarbeitsfläche von lazydev. Nach der Änderung steht `PRDs` links in der Tab-Leiste und ist
beim Start aktiv. Weil `detailsPRD` (`internal/tui/main.go`) dem aktiven Explorer-Tab folgt, zeigt das
Details-Pane beim Start das gewählte PRD statt des `.lazydev/`-Setups, sofern das Projekt PRDs hat.

Im Repo existiert bereits und wird wiederverwendet:

- `rightTabs`, `rightFiles`, `rightPRDs` (`internal/tui/files.go`).
- `stepTab` (`internal/tui/keymap.go`): wechselt über `len(rightTabs)` mit Wrap, bleibt unverändert.
- `rightBody`, `detailsPRD` (`internal/tui/main.go`), `updateRight`, `keyHelp` (`internal/tui/keymap.go`):
  vergleichen nur mit den Konstanten, nicht mit Zahlen.

## Produktentscheidungen (final)

| Thema | Entscheidung |
|---|---|
| Reihenfolge | `PRDs`, `Files`. Nur Tausch, kein neuer Tab. |
| Start-Tab | `PRDs`, der erste Tab. Über den Nullwert von `model.rightTab`. |
| Gespeichert | Nein. Kein Eintrag in `settings.json`, jeder Start beginnt mit `PRDs`. |
| Geltung | Global wie heute: ein `rightTab` für alle Projekte. |
| Details-Pane beim Start | Zeigt das gewählte PRD, weil `PRDs` aktiv ist. Ohne PRDs im Projekt zeigt es wie heute das Projekt-Setup. Gewollt, keine Sonderlogik. |
| Tab-Wechsel | `alt+shift+←/→` unverändert, mit Wrap. Von `PRDs` führt `→` zu `Files`. |
| Tasten | Keine Änderung. `n`, `s`, `x`, `X`, `D`, `c` und Tasten des Dateibaums bleiben an ihren Tab gebunden. |

## Scope

**In Scope**

- Reihenfolge der Konstanten und von `rightTabs` in `internal/tui/files.go`.
- Anpassung von `TestViewSmoke` an den neuen Start-Tab.
- `SPEC.md`, `README.md`, `README.de.md`.

**Out of Scope**

- Konfigurierbare Reihenfolge.
- Letzten Tab über Neustarts merken.
- Tab pro Projekt.
- Änderungen an den Tabs des Details-Panes (`Details`, `Docker`).

## Umsetzung

1. **Reihenfolge** (`internal/tui/files.go`): Konstanten auf `rightPRDs = iota`, dann `rightFiles` umstellen.
   `rightTabs` auf `[]string{"PRDs", "Files"}`. Kommentar über `rightTabs` bleibt. Sonst keine Codeänderung:
   prüfe mit `grep -rn "rightTab" internal/`, dass keine Stelle mit festen Zahlen statt der Konstanten
   arbeitet. Der `default`-Zweig in `rightBody` (`internal/tui/main.go`) rendert den Dateibaum und bleibt
   korrekt, weil `rightPRDs` vorher explizit geprüft wird.
2. **Tests** (`internal/tui/layout_test.go`):
   - `TestViewSmoke`: Der erste Durchlauf prüft heute `README.md` und `src`, die nur im Tab `Files` sichtbar
     sind. Entferne `README.md` und `src` aus der ersten Liste und prüfe stattdessen
     `No PRDs under prd/.` (Text aus `prdListView` in `internal/tui/prds.go`, `project.Dir` ist `prd`).
     Danach `m.rightTab = rightFiles` setzen und prüfen, dass die View `README.md` und `src` enthält.
     Die Zeile `m.bottomZoom, m.focus, m.rightTab = true, colRight, rightPRDs` bleibt.
   - Neuer Test `TestRightTabDefault`: `model{}.rightTab == rightPRDs` und `rightTabs[rightPRDs] == "PRDs"`,
     `rightTabs[rightFiles] == "Files"`.
   - `TestRenderTabs` nutzt eigene Labels und bleibt unverändert.
3. **Doku**:
   - `SPEC.md`, Abschnitt Panes: den Punkt **Explorer** umstellen. Zuerst Tab `PRDs` (default) mit der
     bestehenden Beschreibung, dann Tab `Files`. Schreibweise wie beim Details-Pane:
     `- **Explorer,** tab `PRDs` (default): …. Tab `Files`: …`.
   - `README.md`, Tabelle Layout, Zeile Explorer: `` `PRDs`: PRDs with phase, done ones folded. `Files`: file tree ``.
   - `README.de.md`, gleiche Zeile: `` `PRDs`: PRDs mit Phase, erledigte eingeklappt. `Files`: Dateibaum ``.
   - Schritt 2 im Quickstart beider READMEs („Right pane, tab `PRDs`“ bzw. „Rechtes Pane, Tab `PRDs`“) bleibt
     unverändert.

## Technische Hinweise

- Alle Texte in Code, Kommentaren und Doku auf Englisch, `README.de.md` ist die deutsche Fassung mit gleichem
  Inhalt. Beide READMEs immer gemeinsam ändern.
- Verhalten ist in `SPEC.md` bindend, Code und Doku müssen übereinstimmen.
- Fallstrick: Der Start-Tab ergibt sich ausschließlich aus dem Nullwert von `rightTab`. Setze ihn nirgends
  explizit im Konstruktor, sonst weicht er von der Konstantenreihenfolge ab.
- Fallstrick: `detailsPRD` folgt dem Tab, nicht dem Fokus. Mit `PRDs` als Start-Tab und mindestens einem PRD
  zeigt das Details-Pane beim Start das PRD. Das ist gewollt, nicht abfangen.
- Commit nach Conventional Commits, z. B. `feat(tui): show PRDs as first explorer tab`.

## Erfolgskriterien

- `make test` läuft ohne Fehler (`go vet ./...` und `go test ./...`).
- `go test ./internal/tui -run 'TestViewSmoke|TestRightTabDefault'` ist grün.
- Nach `go run ./cmd/lazydev` zeigt die Tab-Leiste des Explorers ` PRDs   Files `, `PRDs` ist hervorgehoben.
- In einem Projekt mit PRDs zeigt das Details-Pane direkt nach dem Start das gewählte PRD.
- `alt+shift+→` im Explorer wechselt von `PRDs` zu `Files`, ein weiteres `alt+shift+→` zurück zu `PRDs`.
- `SPEC.md`, `README.md` und `README.de.md` nennen `PRDs` vor `Files`, `SPEC.md` markiert `PRDs` als default.
