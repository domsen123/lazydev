## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht. Du arbeitest ohne Mensch im Print-Modus,
Rückfragen sind nicht möglich.

- Prüfe die Änderungen dieses Branches gegenüber `$BASE` (`git diff $BASE...HEAD`) und alle noch nicht
  committeten Änderungen.
- Committe alle Änderungen im Worktree über den Agenten `committer`. Danach muss
  `git status --porcelain` leer sein.
- Du arbeitest bereits im Worktree. Kein `cd` und kein `git -C`. Führe jeden Shell-Befehl einzeln aus,
  ohne `&&`, `;`, `||`, Pipes und Subshells. Zusammengesetzte Befehle lehnt lazydev ab.
- Starte nie einen Watch-Modus, Dev-Server oder andere Befehle, die nicht von selbst enden.
- Verboten: `git push`.
- Ändere nichts unter `.lazydev/`.
