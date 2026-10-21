// Package claude startet claude im Print-Modus und wertet das stream-json aus.
package claude

import (
	"encoding/json"
	"strings"
	"time"
)

// Event enthält die Felder eines stream-json-Ereignisses, die lazydev auswertet.
type Event struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	// Model steht im Ereignis system/init, z. B. "claude-opus-5-5".
	Model   string  `json:"model"`
	IsError bool    `json:"is_error"`
	Result  string  `json:"result"`
	CostUSD float64 `json:"total_cost_usd"`
}

// ParseEvent liefert ok=false für Zeilen, die kein JSON-Objekt mit Typ sind.
func ParseEvent(line []byte) (e Event, ok bool) {
	if err := json.Unmarshal(line, &e); err != nil || e.Type == "" {
		return Event{}, false
	}
	return e, true
}

// Summary sammelt das Ergebnis eines Laufs. Startet Claude Subagenten, enthält der Stream
// mehrere result-Ereignisse. Es zählt das letzte.
type Summary struct {
	SessionID string
	// Model stammt aus dem ersten init-Ereignis.
	Model string
	// HasResult ist false, wenn der Stream ohne result-Ereignis endet.
	HasResult bool
	IsError   bool
	Result    string
	CostUSD   float64
}

// Add übernimmt ein Ereignis und meldet, ob sich SessionID, Modell oder Ergebnis geändert haben.
func (s *Summary) Add(e Event) (changed bool) {
	switch {
	case e.Type == "system" && e.Subtype == "init":
		if e.SessionID != "" && s.SessionID == "" {
			s.SessionID = e.SessionID
			changed = true
		}
		if e.Model != "" && s.Model == "" {
			s.Model = e.Model
			changed = true
		}
		return changed
	case e.Type == "result":
		s.HasResult = true
		s.IsError = e.IsError
		s.Result = e.Result
		s.CostUSD = e.CostUSD
		if s.SessionID == "" {
			s.SessionID = e.SessionID
		}
		return true
	}
	return false
}

// Section liefert den Inhalt unter der Überschrift "## <name>" bis zur nächsten Überschrift
// der Ebene 2, ohne Leerzeilen am Rand. ok ist false, wenn die Überschrift fehlt.
func Section(text, name string) (content string, ok bool) {
	var lines []string
	in := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if h, isHeading := strings.CutPrefix(trimmed, "## "); isHeading {
			if in {
				break
			}
			in = strings.TrimSpace(h) == name
			ok = ok || in
			continue
		}
		if in {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), ok
}

// FirstLine liefert die erste nicht leere Zeile.
func FirstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			return l
		}
	}
	return ""
}

// WithTimestamp ergänzt ein JSON-Objekt um das Feld "ts", weil stream-json keine Zeit enthält.
// Andere Zeilen bleiben unverändert.
func WithTimestamp(line []byte) []byte {
	if len(line) < 2 || line[0] != '{' {
		return line
	}
	ts := `{"ts":"` + time.Now().Format(time.RFC3339) + `"`
	if line[1] != '}' {
		ts += ","
	}
	return append([]byte(ts), line[1:]...)
}
