## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht. Du läufst im Worktree des Branches, ohne Mensch
im Print-Modus. Rückfragen sind nicht möglich.

- Bring den Branch `$BRANCH` per Rebase auf `$BASE`. Danach muss `$BASE` ein Vorfahre von `HEAD` sein.
- Korrekturen nach dem Rebase committet der Agent `committer`. Danach muss `git status --porcelain`
  leer sein.
- Kannst du einen Konflikt oder Fehler nicht eindeutig lösen, ohne vom PRD oder von `$BASE`
  abzuweichen, brich ab: `git rebase --abort`, falls ein Rebase läuft, dann Bericht mit Datei und Frage.
- Kein `git merge` nach `$BASE`, den Fast-Forward macht lazydev.
- Verboten: `git push`, `git reset --hard`, `git worktree`.
- Du arbeitest bereits im Worktree. Kein `cd` und kein `git -C`. Führe jeden Shell-Befehl einzeln aus,
  ohne `&&`, `;`, `||`, Pipes und Subshells. Zusammengesetzte Befehle lehnt lazydev ab.
- Starte nie einen Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
- Ändere nichts unter `.lazydev/`.

### Abschlussbericht

```
## Ergebnis
bereit für Fast-Forward | abgebrochen

## Konflikte
- <datei>: <wie gelöst>, oder „keine“

## Prüfungen
- <befehl>: ok | Fehler (<erste Zeile>)

## Offene Punkte
- <Datei und Frage>, oder „keine“
```

`bereit für Fast-Forward` nur, wenn der Branch auf `$BASE` steht, alle Prüfungen `ok` sind und der
Worktree sauber ist. lazydev wertet nur `## Ergebnis` aus. Halte die Überschriften und die beiden Werte
genau so ein.
