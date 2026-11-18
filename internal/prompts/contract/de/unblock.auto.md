## Autonomer Modus

Dieser Abschnitt hat Vorrang vor den Regeln oben, wo sie sich widersprechen.

- Das PRD liegt im Worktree des Laufs: `$ARGUMENTS`. Ändere nur diese Datei, nicht die Kopie im
  Hauptordner. Relative Pfade im PRD beziehen sich auf den Worktree.
- Stelle die Fragen aus `## Blocker` nicht dem Nutzer. Beantworte sie selbst: zuerst aus dem bisherigen
  Gespräch mit dem Nutzer, dann aus dem Code im Worktree, `CLAUDE.md` und dem PRD. Wähle im Zweifel die
  kleinere Lösung, die sich leichter zurückbauen lässt.
- Schreibe jede Frage mit der gewählten Option und einer Begründung in einem Satz zusätzlich in den Abschnitt
  `## Entscheidungen` des PRDs.
- Kannst du eine Frage nicht verantworten, z. B. bei Datenverlust, öffentlichen Schnittstellen, laufenden
  Kosten oder einem Widerspruch zu einer ausdrücklichen Vorgabe des Nutzers, ändere das PRD nicht. Nenne
  dem Nutzer diese Fragen und warte auf die Antwort.
- Ist alles entschieden, setze den Lauf selbst fort: `lazydev start <projekt> <prd>`.
