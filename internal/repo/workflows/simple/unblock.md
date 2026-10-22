# PRD klären

## Deine Aufgabe

Kläre die Blocker des PRDs `$ARGUMENTS` mit dem Nutzer.

Ein Lauf von lazydev hat bei diesem PRD abgebrochen, weil es eine Entscheidung offenlässt. Die offenen
Fragen stehen im PRD unter `## Blocker`. Tech-Stack und Konventionen stehen in `CLAUDE.md` und `README.md`
des Projekts.

Das geklärte PRD ist so klar, dass ein Agent es ohne Mensch umsetzen kann und dabei nichts selbst
entscheiden muss.

Arbeite strikt in dieser Reihenfolge.

### Phase 1: Verstehen

1. Lies das PRD vollständig.
2. Lies `CLAUDE.md` und `README.md`, falls vorhanden.
3. Lies die Dateien, die die Fragen unter `## Blocker` nennen, und den Code, dem das PRD widerspricht.

### Phase 2: Fragen

Stelle die Fragen aus `## Blocker`, jede mit den Optionen, die sich aus PRD und Code ergeben. Prüfe nach
der Antwort, ob noch etwas offen ist. Falls ja, stelle eine weitere Runde. Beginne Phase 3 erst, wenn
jede Entscheidung final ist.

### Phase 3: PRD anpassen

Arbeite jede Entscheidung in den passenden Abschnitt des PRDs ein, entferne `## Blocker` und setze die
Statuszeile.

Den Lauf setzt der Nutzer in lazydev fort.
