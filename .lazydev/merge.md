# Branch auf den Ziel-Branch bringen

## Deine Aufgabe

Bring den Branch aus `$ARGUMENTS` auf den Ziel-Branch `$BASE`. Nach dem Branch-Namen kann ein
Fehlerkontext folgen, etwa eine fehlgeschlagene Prüfung nach dem Rebase.

## Ablauf

1. **Absicht verstehen.** Lies das PRD des Branches unter `prd/` (Dateiname wie der Branch ohne
   `feat/`) und `git log $BASE..HEAD`.
2. **Rebase.** Läuft kein Rebase, starte `git rebase $BASE`. Löse jeden Konflikt so, dass beide Seiten
   erhalten bleiben: die Änderungen aus `$BASE` und die Absicht des Branches. Nach jedem gelösten Konflikt
   `git add` und `git rebase --continue`.
3. **Generierte Dateien.** Generierte Dateien wie Migrationen behandelst du nach den Regeln aus
   `CLAUDE.md`. Führe sie nie von Hand zusammen.
4. **Prüfen.** Führe die Prüfbefehle aus `CLAUDE.md` aus und behebe Fehler. Nennt der Fehlerkontext
   eine Prüfung, behebe diesen Fehler zuerst. Ändere Dateien nur mit Edit und Write.
5. **Committen.** Korrekturen nach dem Rebase committet der Agent `committer`.
