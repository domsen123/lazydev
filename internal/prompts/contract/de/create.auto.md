## Autonomer Modus

Dieser Abschnitt hat Vorrang vor den Regeln oben, wo sie sich widersprechen.

- Stelle die Klärungsfragen nicht dem Nutzer. Formuliere sie nummeriert mit den Optionen A bis D und
  beantworte sie selbst: zuerst aus dem bisherigen Gespräch mit dem Nutzer, dann aus dem Code, `CLAUDE.md`,
  `README.md` und den bestehenden PRDs. Wähle bei mehreren vertretbaren Optionen die, die am besten zum
  bestehenden Code passt.
- Schreibe jede Frage mit der gewählten Option und einer Begründung in einem Satz in den Abschnitt
  `## Entscheidungen` des PRDs. Ein späterer Lauf und ein Blocker-Dialog brauchen diesen Kontext.
- Frag den Nutzer nur, wenn eine Frage weder aus dem Gespräch noch aus dem Code entscheidbar ist und eine
  falsche Wahl teuer wäre, z. B. Datenverlust, öffentliche Schnittstellen oder laufende Kosten. Stelle dann
  nur diese Fragen und warte auf die Antwort.
- Das fertige PRD enthält trotzdem keine offenen Entscheidungen.
