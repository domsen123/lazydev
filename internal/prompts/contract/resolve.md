# Blocker selbst klären

## Deine Aufgabe

Ein Lauf von lazydev hat beim PRD `$ARGUMENTS` abgebrochen, weil es eine Entscheidung offenlässt. Die
offenen Fragen stehen im PRD unter `## Blocker`. Du arbeitest ohne Nutzer: Entscheide die Fragen selbst, so
wie ein erfahrener Entwickler dieses Projekts sie entscheiden würde, und arbeite die Entscheidungen ins PRD
ein. Danach setzt lazydev den Lauf fort.

Arbeite in dieser Reihenfolge.

1. Lies das PRD vollständig, vor allem `## Blocker`. Seine erste Zeile `Schritt: …` nennt den blockierten
   Schritt, die Liste darunter die offenen Fragen.
2. Lies `CLAUDE.md` und `README.md`, falls vorhanden, und den Code, den die Fragen betreffen. Der Worktree
   enthält auch die nicht committeten Änderungen des abgebrochenen Schritts, `git status` und `git diff`
   zeigen sie.
3. Beantworte jede Frage. Wähle die Option, die am besten zum bestehenden Code, zu den Konventionen und zum
   Ziel des PRDs passt, im Zweifel die kleinere, die sich leichter zurückbauen lässt.

## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht.

- **Ändere ausschließlich das PRD `$ARGUMENTS`.** Keinen Code, nichts unter `.lazydev/`. Committe nicht.
- **Entscheide nicht**, wenn eine Frage Datenverlust, Sicherheit, öffentliche Schnittstellen, laufende
  Kosten oder einen Widerspruch zu einer ausdrücklichen Vorgabe im PRD betrifft und Code und PRD keine klare
  Antwort geben. Dann ändere das PRD gar nicht, auch nicht bei den übrigen Fragen, und melde
  `abgebrochen`.
- Wenn du entscheidest:
  - Arbeite jede Entscheidung in den passenden Abschnitt des PRDs ein, z. B. `## Umsetzung` oder
    `## Erfolgskriterien`.
  - Schreibe jede Frage mit der gewählten Option und einer Begründung in einem Satz in den Abschnitt
    `## Entscheidungen` des PRDs. Fehlt er, lege ihn an.
  - Entferne den Abschnitt `## Blocker` vollständig.
  - Setze die Statuszeile, nur den Teil vor dem ersten `·`: bei `Schritt: implement` auf `geplant`, bei
    `Schritt: merge` auf `umgesetzt (<heute als YYYY-MM-DD>)`.
- Beende deine Antwort genau mit diesem Bericht:

```
## Ergebnis
geklärt

## Entscheidungen
- <Frage>: <gewählte Option>, <Begründung>

## Offene Fragen
- keine
```

  Bei `abgebrochen` steht unter `## Ergebnis` `abgebrochen` und unter `## Offene Fragen` jede Frage, die du
  nicht entscheiden darfst, mit dem Grund.
