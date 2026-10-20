package tui

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"lazydev/internal/claude"
	"lazydev/internal/runlog"
	"lazydev/internal/state"
)

type logKind int

const (
	logNormal logKind = iota
	logRunner
	logError
	logStep
)

// logLine ist eine Zeile der Log-Ansicht. raw ist das Ereignis, aus dem sie stammt.
type logLine struct {
	time string
	text string
	kind logKind
	raw  []byte
}

// logFile hält die gelesenen Zeilen einer Log-Datei. offset zeigt hinter die letzte vollständige
// Zeile, damit ein wachsendes Log nur den neuen Teil liest.
type logFile struct {
	offset int64
	lines  []logLine
}

// logCache liest die Logs eines Laufs schrittweise nach.
type logCache map[string]*logFile

// lines liefert die Zeilen aller Schritte eines Laufs in der Reihenfolge der Schritte.
func (c logCache) lines(projectID, prdName string, r *state.Run) []logLine {
	var out []logLine
	seen := map[string]bool{}
	for _, s := range r.Steps {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		path := state.LogPath(projectID, prdName, s.Name)
		f := c[path]
		if f == nil {
			f = &logFile{}
			c[path] = f
		}
		f.update(path)
		out = append(out, f.lines...)
	}
	return out
}

func (f *logFile) update(path string) {
	fh, err := os.Open(path)
	if err != nil {
		return
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil {
		return
	}
	if info.Size() < f.offset {
		// Die Datei ist neu, etwa nach Teardown und erneutem Start.
		*f = logFile{}
	}
	if info.Size() == f.offset {
		return
	}
	if _, err := fh.Seek(f.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(fh)
	if err != nil {
		return
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return
	}
	for _, raw := range bytes.Split(data[:end], []byte("\n")) {
		f.lines = append(f.lines, parseLogLine(raw)...)
	}
	f.offset += int64(end + 1)
}

// parseLogLine macht aus einer JSONL-Zeile null oder mehr Anzeigezeilen.
func parseLogLine(raw []byte) []logLine {
	raw = bytes.Clone(raw)
	var head struct {
		Type string `json:"type"`
		Time string `json:"time"`
		TS   string `json:"ts"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return nil
	}
	if head.Type == runlog.EventType {
		var e runlog.Event
		json.Unmarshal(raw, &e)
		kind := logRunner
		if strings.HasPrefix(e.Text, "Schritt ") || strings.HasPrefix(e.Text, "Fehler") {
			kind = logStep
		}
		if strings.HasPrefix(e.Text, "Fehler") {
			kind = logError
		}
		return []logLine{{time: clock(e.Time), text: "lazydev " + e.Text, kind: kind, raw: raw}}
	}
	var out []logLine
	for _, d := range claude.Describe(raw) {
		kind := logNormal
		trimmed := strings.TrimSpace(d)
		switch {
		case strings.HasPrefix(trimmed, "Fehler:"), strings.HasPrefix(trimmed, "abgelehnt:"),
			strings.HasPrefix(trimmed, "Ergebnis Fehler"):
			kind = logError
		case strings.HasPrefix(trimmed, "Ergebnis "):
			kind = logStep
		}
		out = append(out, logLine{time: clock(head.TS), text: d, kind: kind, raw: raw})
	}
	return out
}

// clock kürzt einen RFC-3339-Zeitstempel auf HH:MM.
func clock(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "     "
	}
	return t.Local().Format("15:04")
}

// prettyEvent formatiert ein Ereignis für die Detailansicht.
func prettyEvent(raw []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, raw, "", "  ") != nil {
		return string(raw)
	}
	// Mehrzeilige Strings wie Berichte von Claude lesen sich mit echten Zeilenumbrüchen besser.
	return strings.ReplaceAll(buf.String(), `\n`, "\n")
}
