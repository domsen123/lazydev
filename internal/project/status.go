package project

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

type StatusKind int

const (
	StatusUnknown StatusKind = iota
	StatusPlanned
	StatusImplemented
)

// Status ist die geparste Zeile "> Status: <wert> · <freitext>".
type Status struct {
	Kind StatusKind
	// Value ist der Teil vor "·", z. B. "umgesetzt (2026-10-14)".
	Value string
	// Date ist das Datum aus "umgesetzt (YYYY-MM-DD)", sonst leer.
	Date string
	// Detail ist der Freitext nach "·", z. B. "Scope: Backend-only".
	Detail string
}

var (
	statusLineRe  = regexp.MustCompile(`^>\s*Status:\s*(.*)$`)
	implementedRe = regexp.MustCompile(`^umgesetzt\s*\((\d{4}-\d{2}-\d{2})\)`)
)

// headerScanLines begrenzt die Suche nach Titel und Statuszeile auf den Kopf der Datei.
const headerScanLines = 20

// ParseStatusLine erkennt eine Statuszeile. ok ist false, wenn die Zeile keine ist.
func ParseStatusLine(line string) (s Status, ok bool) {
	m := statusLineRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Status{}, false
	}
	value, detail, _ := strings.Cut(m[1], "·")
	s.Value = strings.TrimSpace(value)
	s.Detail = strings.TrimSpace(detail)
	switch {
	case s.Value == "geplant":
		s.Kind = StatusPlanned
	case strings.HasPrefix(s.Value, "umgesetzt"):
		s.Kind = StatusImplemented
		if d := implementedRe.FindStringSubmatch(s.Value); d != nil {
			s.Date = d[1]
		}
	}
	return s, true
}

// ResetStatus setzt den Wert der ersten Statuszeile auf "geplant". Der Freitext nach "·"
// bleibt erhalten. Ohne Statuszeile liefert ResetStatus den Inhalt unverändert.
func ResetStatus(content string) string {
	lines := strings.SplitAfter(content, "\n")
	for i, line := range lines {
		if i >= headerScanLines {
			break
		}
		s, ok := ParseStatusLine(line)
		if !ok {
			continue
		}
		newline := ""
		if strings.HasSuffix(line, "\n") {
			newline = "\n"
		}
		reset := "> Status: geplant"
		if s.Detail != "" {
			_, rest, _ := strings.Cut(strings.TrimRight(line, "\r\n"), "·")
			reset += " ·" + rest
		}
		lines[i] = reset + newline
		break
	}
	return strings.Join(lines, "")
}

// ParseHeader liest Titel und Statuszeile aus dem Kopf eines PRDs.
// Das Format sieht die Statuszeile als zweite Zeile vor. In der Praxis steht oft eine
// Leerzeile davor, deshalb sucht ParseHeader in den ersten Zeilen. Folgezeilen desselben
// Zitatblocks hängt ParseHeader zeilenweise an Status.Detail an, damit Markdown wie Listen
// erhalten bleibt.
func ParseHeader(r io.Reader) (title string, status Status, err error) {
	sc := bufio.NewScanner(r)
	found := false
	for i := 0; i < headerScanLines && sc.Scan(); i++ {
		line := sc.Text()
		if found {
			cont, ok := strings.CutPrefix(line, ">")
			if !ok {
				break
			}
			status.Detail = strings.TrimSpace(status.Detail + "\n" + strings.TrimPrefix(cont, " "))
			continue
		}
		if title == "" {
			if t, ok := strings.CutPrefix(line, "# "); ok {
				title = strings.TrimSpace(strings.TrimPrefix(t, "PRD:"))
				continue
			}
		}
		status, found = ParseStatusLine(line)
	}
	return title, status, sc.Err()
}
