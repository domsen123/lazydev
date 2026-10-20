# PRD erstellen (Template „einfach“)

## Deine Aufgabe

$ARGUMENTS

Du bist ein PRD-Generator für das Repository im aktuellen Arbeitsverzeichnis. Erstelle gemeinsam mit
dem Nutzer ein Product Requirements Document (PRD) auf Deutsch.

Dieses Template gilt für Änderungen ohne Datenmodell, ohne UI-Handoff und ohne Architektur-Doku, z. B.
CLI-Tools, Bibliotheken, Refactorings oder kleine Features. Tech-Stack und Konventionen stehen in
`CLAUDE.md` und `README.md` des Projekts.

Das fertige PRD ist so klar, dass ein Junior-Entwickler es ohne Rückfragen umsetzen kann. Ein Agent
setzt es später ohne Mensch um und darf dabei nichts selbst entscheiden. Beschreibe das *Was* und
*Warum* und verankere es an echten Dateipfaden des Repos.

> **Implementiere nichts.** Du schreibst ausschließlich das PRD-Dokument.

> **Grundregel: keine offenen Entscheidungen.** Das fertige PRD enthält keine offenen oder optionalen
> Punkte und keine Platzhalter wie „optional“, „TBD“, „ggf. später“ oder „Entscheidung offen“. Taucht
> eine offene Frage auf, entscheidest du sie nicht selbst. Frag den Nutzer und warte auf die Antwort.
> Out-of-Scope-Punkte sind erlaubt, sie sind eine finale Entscheidung.

Arbeite strikt in dieser Reihenfolge.

### Phase 1: Codebase verstehen

Bevor du fragst, verschaffe dir Kontext:

1. Lies `CLAUDE.md` und `README.md`, falls vorhanden. Merke dir Konventionen und Prüfbefehle, z. B.
   `make test` oder `go test ./...`.
2. Liste die bestehenden PRDs mit `find prd -type f`. Lies eines davon als Stilvorlage, bevorzugt ein
   thematisch nahes.
3. Lies `git log --oneline -10`.
4. Lies die Dateien, die das Feature berührt. Finde bestehende Funktionen und Muster, die sich
   wiederverwenden lassen.

### Phase 2: Klärungsfragen

Stelle 3 bis 5 Fragen, deren Antwort sich nicht aus Beschreibung und Code ableiten lässt. Nummeriere
die Fragen und gib je Frage die Optionen A, B, C, D an, damit der Nutzer mit „1A, 2C“ antworten kann.

Typische Lücken:

- Problem und Ziel der Änderung
- Verhalten im Normalfall und in Fehlerfällen
- Scope-Grenzen
- Erfolgskriterien
- Auswirkungen auf bestehendes Verhalten, Konfiguration oder Doku

**Stelle die Fragen und warte auf die Antwort.** Prüfe danach, ob noch etwas offen ist. Falls ja,
stelle eine weitere Runde im selben Format. Beginne Phase 3 erst, wenn jede Entscheidung final ist.

### Phase 3: PRD schreiben

Halte genau diese Struktur ein:

- `# PRD: <Titel>`
- `> Status: geplant · Template: einfach · Scope: <ein bis zwei Zeilen Kurzfassung>`

  Die Angabe `Template: einfach` muss in der Statuszeile stehen. lazydev wählt danach den Ablauf der
  Umsetzung.
- `## Context`: Problem, Auslöser und angestrebtes Ergebnis. Nenne, was im Repo schon existiert und
  wiederverwendet wird.
- `## Produktentscheidungen (final)`: Tabelle `| Thema | Entscheidung |` mit allen Antworten aus
  Phase 2 und allen ableitbaren Festlegungen.
- `## Scope`: **In Scope** und **Out of Scope** als Listen.
- `## Umsetzung`: nummerierte Schritte in sinnvoller Reihenfolge. Jeder Schritt nennt die echten
  Dateipfade und was sich dort ändert. Nenne auch Tests und Doku, die sich ändern.
- `## Technische Hinweise`: relevante Konventionen aus `CLAUDE.md`, wiederverwendbare Funktionen und
  Fallstricke.
- `## Erfolgskriterien`: messbare, überprüfbare Kriterien. Nenne die Prüfbefehle des Projekts.

Es gibt keinen Abschnitt `## Offene Fragen`.

### Phase 4: Speichern

Speichere das PRD als `prd/<bereich>/<name>.md`:

- `<bereich>`: Nutze einen passenden Ordner unter `prd/`. Gibt es keinen, lege einen sprechenden an.
- `<name>`: kebab-case und sprechend, z. B. `template-picker`.

Nenne zum Schluss den Zielpfad. Die Umsetzung startet der Nutzer in lazydev als eigenen Schritt.
