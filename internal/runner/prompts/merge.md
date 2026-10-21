# /merge-prd $ARGUMENTS

## Deine Aufgabe

Bring den Branch aus `$ARGUMENTS` auf den Ziel-Branch `$BASE`. Nach dem Branch-Namen kann ein
Fehlerkontext folgen, etwa eine fehlgeschlagene Prüfung nach dem Rebase. Du läufst im Worktree des
Branches, ohne Mensch im Print-Modus. Rückfragen sind nicht möglich.

## Ablauf

1. **Absicht verstehen.** Lies das PRD des Branches unter `prd/` (Dateiname wie der Branch ohne
   `feat/`) und `git log $BASE..HEAD`.
2. **Rebase.** Läuft kein Rebase, starte `git rebase $BASE`. Löse jeden Konflikt so, dass beide Seiten
   erhalten bleiben: die Änderungen aus `$BASE` und die Absicht des Branches. Nach jedem gelösten Konflikt
   `git add` und `git rebase --continue`.
3. **Drizzle-Migrationen.** Kollidieren Migrationen (`server/database/migrations/*.sql`,
   `meta/_journal.json`, `meta/*_snapshot.json`), führe sie nie von Hand zusammen:
   1. Übernimm die Migrationen von `$BASE` unverändert.
   2. Entferne die Migrationen des Branches mit `git rm`.
   3. Löse die Konflikte im Schema unter `server/database/schema/` so, dass beide Seiten erhalten
      bleiben.
   4. Erzeuge die Migration des Branches neu mit `pnpm db:generate`.
4. **Prüfen.** Führe die Prüfbefehle aus `CLAUDE.md` aus, z. B. `pnpm lint`, `pnpm typecheck`,
   `pnpm test:run`, und behebe Fehler. Nenne der Fehlerkontext eine Prüfung, behebe diesen Fehler
   zuerst. Starte nie einen Watch-Modus oder Dev-Server. Ändere Dateien nur mit Edit und Write.
5. **Committen.** Korrekturen nach dem Rebase committet der Agent `committer`. Danach muss
   `git status --porcelain` leer sein und `$BASE` ein Vorfahre von `HEAD`.
6. **Abbrechen.** Kannst du einen Konflikt oder Fehler nicht eindeutig lösen, ohne vom PRD oder von
   `$BASE` abzuweichen, brich ab: `git rebase --abort`, falls ein Rebase läuft, dann Bericht mit Datei
   und Frage.

## Abschlussbericht

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

## Verboten

- `git push`, `git reset --hard`, `git worktree`, `git merge` nach `$BASE`. Den Fast-Forward macht lazydev.
- Migrationen von Hand zusammenführen.
- Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
