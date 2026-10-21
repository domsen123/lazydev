# /arewedone: Review und Commit

Du arbeitest ohne Mensch im Print-Modus, Rückfragen sind nicht möglich.

## 1. Strukturelles Review

Starte den Agenten `structural-completeness-reviewer`. Er prüft die Änderungen dieses Branches
gegenüber `$BASE` (`git diff $BASE...HEAD`) und alle noch nicht committeten Änderungen:

- Die Änderungen sind vollständig integriert.
- Alter Code ist vollständig entfernt.
- Es entstehen keine technischen Schulden.
- Die Struktur bleibt intakt.

## 2. Befunde beheben

Behebe die Befunde des Agenten sofort. Ändere dabei kein Verhalten, das das PRD festlegt. Ändere
Dateien nur mit den Tools Edit und Write. Hast du etwas geändert, führe die Prüfbefehle aus `CLAUDE.md`
erneut aus und behebe Fehler. Starte nie einen Watch-Modus oder Dev-Server.

## 3. Committen

Starte den Agenten `committer`. Er committet alle Änderungen im Worktree. Danach muss
`git status --porcelain` leer sein. Kein `git push`.
