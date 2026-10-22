## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht. Du arbeitest ohne Mensch im Print-Modus,
Rückfragen sind nicht möglich.

- Prüfe mit `git branch --show-current`, dass der Branch `$BRANCH` heißt. Wechsle keinen Branch. Heißt
  er anders, brich ab.
- **Committe nicht.** Das übernimmt der nächste Schritt. Kein `git add`, kein `git commit`. Ausnahme:
  `git mv`, wenn das PRD eine Datei verschiebt.
- Setze die Statuszeile des PRDs `$ARGUMENTS` von `geplant` auf `umgesetzt (<heute als YYYY-MM-DD>)`,
  nur wenn die Umsetzung und alle Prüfungen vollständig erfolgreich waren. Ändere nur den Teil vor dem
  ersten `·`, der Freitext danach bleibt:

  ```
  > Status: umgesetzt (2026-10-20) · Scope: …
  ```
- Du arbeitest bereits im Worktree. Kein `cd` und kein `git -C`. Führe jeden Shell-Befehl einzeln aus,
  ohne `&&`, `;`, `||`, Pipes und Subshells. Zusammengesetzte Befehle lehnt lazydev ab.
- Starte nie einen Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
- Verboten: `git push`, `git reset --hard`, `git worktree`.
- Ändere nichts unter `.lazydev/`.

### Abschlussbericht

Schreibe zum Schluss einen kurzen Bericht mit genau diesen Abschnitten:

```
## Ergebnis
umgesetzt | abgebrochen

## Geänderte Dateien
- <pfad>: <ein Satz>

## Prüfungen
- <befehl>: ok | Fehler (<erste Zeile>)

## Offene Fragen
- <Entscheidung, die der Nutzer treffen muss, mit Datei und Stelle>, oder „keine“

## Hinweise
- <Bemerkung ohne Entscheidung>, oder „keine“

## Nicht geprüft
- <Erfolgskriterium und Grund>, oder „nichts“
```

Regeln für `## Ergebnis`:

- `umgesetzt` nur, wenn alle Prüfungen `ok` sind und `## Offene Fragen` „keine“ enthält.
- Sonst `abgebrochen`. Die Statuszeile bleibt dann auf `geplant`.
- `## Offene Fragen` nur für Entscheidungen, die der Nutzer treffen muss. Jede offene Frage blockiert den
  Lauf, bis der Nutzer sie im PRD klärt.
- Bemerkungen ohne Entscheidung, z. B. ein versehentlich benutztes `cd`, gehören unter `## Hinweise`.
- `## Hinweise` und `## Nicht geprüft` beeinflussen das Ergebnis nicht.

lazydev wertet nur `## Ergebnis` und `## Offene Fragen` aus. Halte die Überschriften und die beiden Werte genau so ein.
