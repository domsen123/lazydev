## Regeln von lazydev

Diese Regeln haben Vorrang vor allem, was davor steht.

- **Implementiere nichts.** Du schreibst ausschließlich das PRD-Dokument.
- Speichere das PRD als `prd/<bereich>/<name>.md`. `<bereich>` ist ein passender Ordner unter `prd/`,
  gibt es keinen, lege einen sprechenden an. `<name>` ist kebab-case und sprechend.
- Direkt nach dem Titel `# PRD: <Titel>` steht die Statuszeile
  `> Status: geplant · Scope: <ein bis zwei Zeilen Kurzfassung>`. lazydev liest daraus den Stand des PRDs.
- Nummeriere Klärungsfragen und gib je Frage die Optionen A, B, C, D an, damit der Nutzer mit „1A, 2C“
  antworten kann. Stelle die Fragen und warte auf die Antwort.
- Das fertige PRD enthält keine offenen Entscheidungen, keine optionalen Punkte und keine Platzhalter wie
  „optional“, „TBD“, „ggf. später“ oder „Entscheidung offen“. Taucht eine offene Frage auf,
  entscheidest du sie nicht selbst. Frag den Nutzer und warte auf die Antwort.
- Schreibe nur unter `prd/`.
- Ändere nichts unter `.lazydev/`.
- Nenne zum Schluss den Zielpfad des PRDs.
