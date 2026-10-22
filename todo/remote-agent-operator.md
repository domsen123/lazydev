# Idee: Remote-Aufträge über Agent und Operator

> Notiz, noch kein PRD. Stand der Überlegung vom 2026-10-22.

## Ziel

lazydev läuft auf einem Server und nimmt Aufträge von überall an, z. B. vom Handy, ohne VPN, ohne offenen
Port und ohne direkten Zugriff auf den Server. Wie die Aufträge entstehen (Web-UI, Chat-Bot, GitHub
Issues, Jira, …), soll lazydev egal sein.

## Grundprinzip: Pull statt Listener

- lazydev öffnet keinen Port. Es baut nur ausgehende Verbindungen zu einer Queue auf, die ohnehin im Netz
  erreichbar ist. Der Server bleibt von außen unsichtbar.
- Muster wie bei self-hosted Runnern (GitHub Actions, Buildkite): lazydev ist der **Agent**, eine
  austauschbare Gegenstelle ist der **Operator**.
- lazydev definiert nur das **Protokoll**. Jeder kann einen Operator bauen, egal wie er die Queue befüllt.
  Ein eigenes SaaS wäre nur eine komfortable Implementierung desselben Protokolls.

```sh
lazydev agent register https://ops.example.com --token <einmal-token>
lazydev agent run          # systemd-Dienst, Long Poll nur über ausgehende Verbindungen
```

## Rollen

| Rolle | Aufgabe |
|---|---|
| Agent (lazydev) | Führt aus: Projekte, Worktrees, Runner, Merge. Entscheidet lokal, was er annimmt |
| Operator | Queue, Benutzer, Rechte, UI, Integrationen. Führt nie selbst Code aus |
| Protokoll | Kleine HTTP/JSON-API, versioniert, dokumentiert in `PROTOCOL.md` neben `SPEC.md` |

## Protokoll-Skizze

| Aufruf | Zweck |
|---|---|
| `POST /v1/agents/register` | Einmal-Token gegen Agent-Identität tauschen. Agent erzeugt sein Schlüsselpaar selbst, nur der Public Key geht an den Operator |
| `GET /v1/jobs/next?wait=30` | Long Poll: nächster Job für die Projekte dieses Agenten, mit Lease |
| `POST /v1/jobs/{id}/events` | Schritte (setup → implement → review → merge), Logzeilen, Kosten |
| `POST /v1/jobs/{id}/heartbeat` | Lease verlängern. Läuft sie ab, darf der Operator den Job neu vergeben |
| `POST /v1/jobs/{id}/complete` | Ergebnis `merged`, `failed` oder `aborted`, mit Commit und Kosten |
| Abbruch im Poll oder `GET /v1/jobs/{id}/control` | Abbruch durch den Operator (entspricht `x`) |

Job-Format: nur feste Aktionen, keine freien Befehle.

```json
{ "id": "j_123", "action": "create_and_run", "project": "lazydev",
  "prd": { "title": "...", "body": "..." }, "requested_by": "dominic", "signature": "..." }
```

Aktionen für den Anfang: `create_prd`, `run_prd`, `create_and_run`, `stop_prd`, `status`.

Transport: Long Polling über HTTPS. Geht durch jede Firewall und jeden Proxy und ist trivial
nachzubauen. WebSocket oder SSE eventuell später.

Weitere Punkte:

- Idempotenz: verarbeitete Job-IDs merkt sich der Agent, damit nichts doppelt läuft.
- Protokollversion im Handshake, damit Agent und Operator unabhängig aktualisiert werden können.

## Vertrauensmodell (der wichtigste Teil)

Ein Remote-Auftrag heißt im Kern: Code auf dem Server ausführen lassen. Claude läuft mit `allowedTools`,
dazu kommen Hooks. Wer den Operator kontrolliert, könnte auf jedem angebundenen Server Claude laufen
lassen. Deshalb ist der Agent die letzte Instanz:

- **Policy beim Agenten** pro Projekt in `.lazydev/config.yaml`: Remote an oder aus, erlaubte Aktionen,
  ob ein Start lokal oder per zweitem Faktor bestätigt werden muss. Die Policy gehört in den Trust-Hash,
  wie Hooks und `allowedTools`.
- **Signierte Aufträge (optional, stark):** Der Auftraggeber signiert mit eigenem Schlüssel (Passkey,
  SSH-Key), der Agent kennt die erlaubten Public Keys. Ein kompromittierter Operator ist dann nur noch
  ein Briefkasten.
- **Zweistufige Freigabe (optional):** Ein Auftrag erzeugt nur ein PRD mit Status `geplant` und eine
  Benachrichtigung, gestartet wird erst nach Bestätigung. Schützt vor Prompt-Injection durch fremden
  Text, z. B. aus einem Issue.
- **Hooks und `allowedTools` kommen nie vom Operator**, nur aus dem Repo.
- **Datenabfluss begrenzen:** Logs können Code, Pfade und Secrets enthalten. Per Policy einstellbar, ob nur
  Status und Kosten oder auch Logs an den Operator gehen.
- Text aus einem Auftrag landet nur im PRD, nie in einer Shell.

## Was das in lazydev bedeutet

1. **`create` ohne interaktive Sitzung:** `claude -p` mit dem Create-Prompt. Heute öffnet `n` einen
   interaktiven Tab, der Remote-Weg braucht eine Variante ohne TTY.
2. **`lazydev agent`:** Registrierung, Poll-Schleife, Lease und Heartbeat, Start über `runner.spawn`.
   `MaxParallel`, Warteschlange, Worktrees und Merge bleiben unverändert, die TUI steuert ohnehin nichts
   direkt.
3. **Events spiegeln:** Run-State und Step-Logs lesen und an den Operator schicken.
4. **Referenz-Operator:** z. B. `lazydev operator` als minimaler Server (SQLite, Web-Formular). Macht das
   Protokoll testbar und liefert die self-hosted Variante gleich mit.
5. Voraussetzungen auf dem Server: eingeloggte `claude`-CLI, Git-Zugangsdaten für die Projekte.

## Mögliche Operatoren bzw. Quellen

| Quelle | Vom Handy | Authentifizierung |
|---|---|---|
| GitHub Issues (Label `lazydev`) | GitHub-App | GitHub-Konto mit 2FA, Autor gegen Allowlist |
| Discord-Bot | Discord-App, gleicher Kanal wie die Benachrichtigungen | Discord-User-ID gegen Allowlist |
| Telegram-Bot | Telegram | Chat-ID gegen Allowlist |
| Eigenes SaaS / Web-UI | Browser | Benutzerkonten, optional signierte Aufträge |

Im Agent-Modell sind das alles nur Operatoren, die ihre Quelle in Jobs übersetzen. Kein Sonderfall in
lazydev.

## Mögliche Reihenfolge der PRDs

1. `PROTOCOL.md`: Endpunkte, Job-Format, Lease-Regeln, Vertrauensmodell.
2. Nicht-interaktives `create`.
3. `lazydev agent` mit Policy in `config.yaml` und Trust-Hash.
4. Referenz-Operator `lazydev operator`.
5. Signierte Aufträge und zweistufige Freigabe.

## Offene Fragen

- Ein Agent pro Server mit mehreren Projekten oder ein Agent pro Projekt?
- Wie werden Projekte beim Operator bekannt gemacht: meldet der Agent sie, oder legt der Operator sie an?
- Bestätigung für die zweistufige Freigabe: über den Operator, über Discord oder nur lokal in der TUI?
- Soll die TUI Remote-Aufträge und den Agent-Status anzeigen?
- Wie groß darf ein PRD-Text im Job sein, und kommen Anhänge (Bilder, Dateien) dazu?
