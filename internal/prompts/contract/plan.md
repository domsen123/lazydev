# Reihenfolge planen

## Deine Aufgabe

lazydev setzt PRDs gleichzeitig in eigenen Git-Worktrees um und bringt sie nacheinander per Rebase und
Fast-Forward in `$BASE`. Entscheide, ob das PRD `$ARGUMENTS` jetzt parallel zu den offenen Läufen unten
starten kann oder erst, wenn einer oder mehrere von ihnen gemergt sind.

## Offene Läufe

$RUNS

## Vorgehen

1. Lies das neue PRD und die PRDs der offenen Läufe vollständig.
2. Prüfe im Code, welche Dateien, Funktionen, Tabellen, Schnittstellen und Konfigurationen jedes PRD
   ändert.
3. Entscheide:
   - **Nach einem Lauf**, wenn das neue PRD auf dessen Ergebnis aufbaut, wenn beide dieselben Funktionen
     oder Abschnitte derselben Datei wesentlich umbauen, sodass der zweite Rebase viel Konfliktarbeit
     bedeutet, oder wenn beide Migrationen oder das Schema derselben Tabellen ändern.
   - **Parallel** in allen anderen Fällen. Dieselbe Datei an verschiedenen Stellen, neue Dateien
     nebeneinander, Einträge in Listen oder Registrierungen lösen Rebase und der Claude-Schritt merge.
4. Nenne nur Läufe aus der Liste oben, und nur die, auf die du wirklich warten musst.

## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht.

- **Ändere nichts.** Du liest nur: keine Dateien schreiben, keine Befehle mit Seiteneffekten.
- Beende deine Antwort genau mit diesem Abschnitt, ohne Text danach:

```
## Reihenfolge

Nach: <prd>, <prd>
Grund: <ein Satz>
```

  `<prd>` ist der Name eines Laufs aus der Liste. Kann das PRD parallel starten, schreibe `Nach: keine`.
