### Pull Request

Dieses Projekt merged über einen Pull Request. Schreibe nach dem Commit Titel und Beschreibung des Pull
Requests nach `$EVIDENCE/pr.md`:

- Erste Zeile `# <Titel>`: ein Conventional Commit auf Englisch, sofern `CLAUDE.md` nichts anderes
  vorgibt, z. B. `# feat(auth): add bearer tokens for mobile clients`.
- Danach die Beschreibung nach `.github/pull_request_template.md` des Repos, falls die Datei existiert,
  sonst mit den Abschnitten `## Summary`, `## Evidence` und `## PRD`.
- Unter Evidence stehen die Belege aus `$EVIDENCE/report.md` mit Dateinamen und dem Pfad des Ordners
  `$EVIDENCE`. Fehlt `report.md`, steht dort `No runtime verification ran (.lazydev/verify.md is missing).`
- Unter PRD steht der Pfad des PRDs.
- Die Datei liegt außerhalb des Worktrees und wird nicht committet. Lade keine Bilder hoch, lazydev
  übernimmt die Beschreibung nur als Text.
