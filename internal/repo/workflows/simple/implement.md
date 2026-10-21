# PRD umsetzen

## Deine Aufgabe

Setze das PRD `$ARGUMENTS` vollständig um. Tech-Stack und Konventionen stehen in `CLAUDE.md` und
`README.md` des Projekts. Was du nicht entscheiden darfst, landet im Abschlussbericht.

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

### Schritt 2: Umsetzen

1. Setze die Schritte aus `## Umsetzung` in ihrer Reihenfolge um.
2. Halte dich an `## Technische Hinweise` und die Konventionen aus `CLAUDE.md`.
3. Ändere Dateien nur mit den Tools Edit und Write. Skripte wie `python3`, `sed -i` oder Heredocs sind
   nicht freigegeben.
4. Schreibe die Tests, die das PRD oder `CLAUDE.md` verlangen.
5. Passe die Doku an, die das PRD nennt.

### Schritt 3: Prüfen

1. Führe alle Prüfbefehle des Projekts aus und behebe jeden Fehler. Wiederhole, bis alle durchlaufen.
2. Lehnt Claude Code einen Prüfbefehl ab, weil er nicht freigegeben ist, brich ab. Nenne den Befehl
   unter „Offene Fragen“.
3. Gleiche das Ergebnis mit `## Erfolgskriterien` ab.
4. Lässt sich ein Fehler nicht beheben, ohne vom PRD abzuweichen, brich ab und nenne ihn unter
   „Offene Fragen“.
5. Kriterien, die sich nur von Hand prüfen lassen, nennst du unter „Nicht geprüft“. Sie blockieren die
   Umsetzung nicht.
