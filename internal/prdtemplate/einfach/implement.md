# PRD umsetzen (Template „einfach“): $ARGUMENTS

## Deine Aufgabe

Setze das PRD `$ARGUMENTS` vollständig um. Tech-Stack und Konventionen stehen in `CLAUDE.md` und
`README.md` des Projekts. Du arbeitest ohne Mensch im Print-Modus, Rückfragen sind nicht möglich. Was
du nicht entscheiden darfst, landet im Abschlussbericht.

> **Grundregel: keine eigenen Entscheidungen.** Das PRD enthält nur finale Entscheidungen. Lässt es
> einen Punkt offen, den du für die Umsetzung brauchst, entscheidest du ihn nicht selbst. Brich ab und
> nenne die Frage im Abschlussbericht unter „Offene Fragen“. Kleine Details, die bestehender Code
> eindeutig vorgibt, sind keine offenen Entscheidungen.

Arbeite strikt in dieser Reihenfolge.

### Schritt 1: Lesen

1. Lies das PRD vollständig. Fehlt die Datei, brich ab.
2. Lies `CLAUDE.md` und `README.md`, falls vorhanden.
3. Lies die Prüfbefehle aus `CLAUDE.md`, z. B. `make test` oder `go test ./...`.
4. Lies die Dateien, die das PRD unter `## Umsetzung` und `## Technische Hinweise` nennt, bevor du
   sie änderst.

### Schritt 2: Branch prüfen

`<prd-name>` ist der Dateiname des PRDs ohne `.md`. Prüfe mit `git branch --show-current`, dass der
Branch `feat/<prd-name>` heißt. Wechsle keinen Branch. Heißt er anders, brich ab.

### Schritt 3: Umsetzen

1. Setze die Schritte aus `## Umsetzung` in ihrer Reihenfolge um.
2. Halte dich an `## Technische Hinweise` und die Konventionen aus `CLAUDE.md`.
3. Ändere Dateien nur mit den Tools Edit und Write. Skripte wie `python3`, `sed -i` oder Heredocs sind
   nicht freigegeben.
4. Schreibe die Tests, die das PRD oder `CLAUDE.md` verlangen.
5. Passe die Doku an, die das PRD nennt.

### Schritt 4: Prüfen

1. Führe alle Prüfbefehle des Projekts aus und behebe jeden Fehler. Wiederhole, bis alle durchlaufen.
2. Starte nie einen Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
3. Lehnt Claude Code einen Prüfbefehl ab, weil er nicht freigegeben ist, brich ab. Nenne den Befehl
   unter „Offene Fragen“.
4. Gleiche das Ergebnis mit `## Erfolgskriterien` ab.
5. Lässt sich ein Fehler nicht beheben, ohne vom PRD abzuweichen, brich ab und nenne ihn unter
   „Offene Fragen“.
6. Kriterien, die sich nur von Hand prüfen lassen, nennst du unter „Nicht geprüft“. Sie blockieren die
   Umsetzung nicht.

### Schritt 5: Status setzen

Setze die Statuszeile des PRDs von `geplant` auf `umgesetzt (<heute als YYYY-MM-DD>)`. Ändere nur den
Teil vor dem ersten `·`, der Freitext danach bleibt:

```
> Status: umgesetzt (2026-10-20) · Template: einfach · Scope: …
```

Setze den Status nur, wenn Schritt 3 und 4 vollständig erfolgreich waren.

### Schritt 6: Abschlussbericht

**Committe nicht.** Das übernimmt der nächste Schritt. Kein `git add`, kein `git commit`, kein
`git push`. Ausnahme: `git mv`, wenn das PRD eine Datei verschiebt.

Schreibe zum Schluss einen kurzen Bericht mit genau diesen Abschnitten:

```
## Ergebnis
umgesetzt | abgebrochen

## Geänderte Dateien
- <pfad>: <ein Satz>

## Prüfungen
- <befehl>: ok | Fehler (<erste Zeile>)

## Offene Fragen
- <Frage oder Problem mit Datei und Stelle>, oder „keine“

## Nicht geprüft
- <Erfolgskriterium und Grund>, oder „nichts“
```

Regeln für `## Ergebnis`:

- `umgesetzt` nur, wenn alle Prüfungen `ok` sind und `## Offene Fragen` „keine“ enthält.
- Sonst `abgebrochen`. Die Statuszeile bleibt dann auf `geplant`.
- `## Nicht geprüft` beeinflusst das Ergebnis nicht.

lazydev wertet nur `## Ergebnis` aus. Halte die Überschriften und die beiden Werte genau so ein.

## Verboten

- Entscheidungen treffen, die das PRD offenlässt.
- Commits, `git push`, `git reset --hard`, `git worktree`.
- Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
