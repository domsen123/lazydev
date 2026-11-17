# Feature in der laufenden App prüfen

## Deine Aufgabe

Prüfe das Feature des PRDs `$ARGUMENTS` in der laufenden App, so wie ein Nutzer es benutzt. Die
Umsetzung ist fertig und die Prüfbefehle aus `CLAUDE.md` laufen durch. Jetzt zählt, ob das Feature in der
echten App funktioniert. Lege für jedes Erfolgskriterium einen Beleg ab.

Arbeite strikt in dieser Reihenfolge.

### Schritt 1: Lesen

1. Lies das PRD vollständig, vor allem `## Erfolgskriterien`.
2. Lies `CLAUDE.md` und `README.md`, falls vorhanden: wie die App startet und welche Werkzeuge es zum
   Prüfen gibt, z. B. eine CLI, einen Browser-Treiber oder eine Feature Map.

### Schritt 2: App starten

1. Läuft der Dev-Server schon auf Port `$PORT`, nutze ihn.
2. Sonst starte die App mit Befehlen, die von selbst enden, z. B. einem Start-Befehl, der die App im
   Hintergrund startet und zurückkehrt. Nutze Port `$PORT`.

### Schritt 3: Prüfen

1. Prüfe die Erfolgskriterien des PRDs der Reihe nach in der App.
2. Lege je Kriterium mindestens einen Beleg nach `$EVIDENCE`: Screenshot, Snapshot, Antwort der API oder
   Auszug aus einem Log. Gib jeder Datei einen Namen, der das Kriterium erkennen lässt.
3. Kriterien, die sich in der App nicht prüfen lassen, nennst du unter „Nicht geprüft“.
