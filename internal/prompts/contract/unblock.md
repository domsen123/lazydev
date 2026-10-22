## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht.

- **Implementiere nichts.** Du änderst ausschließlich das PRD `$ARGUMENTS`. Keinen Code, keine Konflikte,
  nichts unter `.lazydev/`.
- Lies das PRD vollständig, vor allem den Abschnitt `## Blocker`. Seine erste Zeile `Schritt: …` nennt den
  blockierten Schritt, die Liste darunter die offenen Fragen.
- Nummeriere die Fragen und gib je Frage die Optionen A, B, C, D an, damit der Nutzer mit „1A, 2C“
  antworten kann. Stelle die Fragen und warte auf die Antwort.
- Entscheide nichts selbst. Bleibt nach der Antwort etwas offen, frag erneut und warte.
- Arbeite die Entscheidungen in die passenden Abschnitte des PRDs ein, z. B. `## Produktentscheidungen
  (final)`, `## Umsetzung` oder `## Erfolgskriterien`. Das PRD enthält danach keine offenen Entscheidungen.
- Entferne danach den Abschnitt `## Blocker` vollständig.
- Setze die Statuszeile. Ändere nur den Teil vor dem ersten `·`, der Freitext danach bleibt:
  - bei `Schritt: implement` auf `geplant`,
  - bei `Schritt: merge` auf `umgesetzt (<heute als YYYY-MM-DD>)`.
- **Committe nicht.** Kein `git add`, kein `git commit`.
- Nenne zum Schluss: `Weiter in lazydev mit s.`
