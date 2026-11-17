## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht. Du arbeitest ohne Mensch im Print-Modus,
Rückfragen sind nicht möglich.

- Prüfe das Feature des PRDs `$ARGUMENTS` in der laufenden App wie ein Nutzer. Statische Prüfungen
  allein reichen nicht.
- Lege Belege (Screenshots, Snapshots, Antworten, Log-Auszüge) nur unter `$EVIDENCE` ab, nirgends sonst.
  Bestehende Dateien dort darfst du überschreiben. Vergleiche die Belege mit `## Erfolgskriterien` des
  PRDs.
- Die App läuft auf Port `$PORT`, oder du startest sie dort mit Befehlen, die von selbst enden.
- Findest du einen Fehler, behebe ihn im Worktree, führe die Prüfbefehle aus `CLAUDE.md` erneut aus und
  verifiziere erneut.
- **Committe nicht.** Das übernimmt der nächste Schritt. Kein `git add`, kein `git commit`.
- Prozesse, die du selbst startest, stoppst du vor dem Ende.
- Du arbeitest bereits im Worktree. Kein `cd` und kein `git -C`. Führe jeden Shell-Befehl einzeln aus,
  ohne `&&`, `;`, `||`, Pipes und Subshells. Zusammengesetzte Befehle lehnt lazydev ab.
- Starte nie einen Watch-Modus oder andere Befehle, die nicht von selbst enden.
- Verboten: `git push`, `git reset --hard`, `git worktree`.
- Ändere nichts unter `.lazydev/`.

### Abschlussbericht

Schreibe zum Schluss einen kurzen Bericht mit genau diesen Abschnitten:

```
## Ergebnis
verifiziert | abgebrochen

## Belege
- <dateiname unter $EVIDENCE>: <was die Datei zeigt>

## Geänderte Dateien
- <pfad>: <ein Satz>, oder „keine“

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

- `verifiziert` nur, wenn das Feature in der App funktioniert, alle Prüfungen `ok` sind, `## Offene
  Fragen` „keine“ enthält und unter `## Belege` mindestens eine Datei steht.
- Nenne unter `## Belege` nur den Dateinamen relativ zu `$EVIDENCE`, ohne Pfad davor. Jede genannte Datei
  muss dort liegen.
- Sonst `abgebrochen`. Jede offene Frage blockiert den Lauf, bis der Nutzer sie im PRD klärt.

lazydev wertet nur `## Ergebnis`, `## Belege` und `## Offene Fragen` aus. Halte die Überschriften und die
beiden Werte genau so ein.
