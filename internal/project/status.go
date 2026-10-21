package project

import (
	"bufio"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type StatusKind int

const (
	StatusUnknown StatusKind = iota
	StatusPlanned
	StatusImplemented
	// StatusDiscarded gilt für PRDs, die nicht umgesetzt werden.
	StatusDiscarded
)

// Statuswerte, die lazydev selbst schreibt. umgesetzt und verworfen bekommen das Datum dazu.
const (
	StatusValuePlanned     = "geplant"
	StatusValueImplemented = "umgesetzt"
	StatusValueDiscarded   = "verworfen"
)

// Status ist die geparste Zeile "> Status: <wert> · <freitext>".
type Status struct {
	Kind StatusKind
	// Value ist der Teil vor "·", z. B. "umgesetzt (2026-10-14)".
	Value string
	// Date ist das Datum aus "umgesetzt (YYYY-MM-DD)" oder "verworfen (YYYY-MM-DD)", sonst leer.
	Date string
	// Detail ist der Freitext nach "·", z. B. "Scope: Backend-only".
	Detail string
}

var (
	statusLineRe = regexp.MustCompile(`^>\s*Status:\s*(.*)$`)
	statusDateRe = regexp.MustCompile(`^(?:umgesetzt|verworfen)\s*\((\d{4}-\d{2}-\d{2})\)`)
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
	case s.Value == StatusValuePlanned:
		s.Kind = StatusPlanned
	case strings.HasPrefix(s.Value, StatusValueImplemented):
		s.Kind = StatusImplemented
	case strings.HasPrefix(s.Value, StatusValueDiscarded):
		s.Kind = StatusDiscarded
	}
	if s.Kind == StatusImplemented || s.Kind == StatusDiscarded {
		if d := statusDateRe.FindStringSubmatch(s.Value); d != nil {
			s.Date = d[1]
		}
	}
	return s, true
}

// SetStatus setzt den Wert der ersten Statuszeile auf value. Der Freitext nach "·" bleibt
// erhalten. ok ist false ohne Statuszeile, dann bleibt der Inhalt unverändert.
func SetStatus(content, value string) (string, bool) {
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
		set := "> Status: " + value
		if s.Detail != "" {
			_, rest, _ := strings.Cut(strings.TrimRight(line, "\r\n"), "·")
			set += " ·" + rest
		}
		lines[i] = set + newline
		return strings.Join(lines, ""), true
	}
	return content, false
}

// ResetStatus setzt den Wert der ersten Statuszeile auf "geplant". Der Freitext nach "·"
// bleibt erhalten. Ohne Statuszeile liefert ResetStatus den Inhalt unverändert.
func ResetStatus(content string) string {
	reset, _ := SetStatus(content, StatusValuePlanned)
	return reset
}

// Felder, die der Runner vor dem Merge in den Freitext der Statuszeile schreibt.
const (
	costField  = "Kosten:"
	modelField = "Modell:"
)

// RunInfo liest Kosten und Modell aus dem Freitext, z. B.
// "Kosten: 4.12 USD · Modell: claude-opus-5-5 · Scope: …". Die Felder stehen immer in der ersten
// Zeile. Ein Betrag, der sich nicht als Zahl lesen lässt, gilt als nicht vorhanden.
func RunInfo(s Status) (cost float64, hasCost bool, model string) {
	first, _, _ := strings.Cut(s.Detail, "\n")
	for part := range strings.SplitSeq(first, "·") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, costField); ok {
			v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "USD"))
			if c, err := strconv.ParseFloat(v, 64); err == nil && !math.IsNaN(c) && !math.IsInf(c, 0) {
				cost, hasCost = c, true
			}
		} else if v, ok := strings.CutPrefix(part, modelField); ok {
			model = strings.TrimSpace(v)
		}
	}
	return cost, hasCost, model
}

// SetRunInfo setzt Kosten und Modell in der ersten Statuszeile direkt nach dem Statuswert. Stehen
// die Felder schon im Freitext, ersetzt SetRunInfo die Werte an ihrer Stelle. Ohne model entfällt
// das Feld „Modell:“. Ohne Statuszeile liefert SetRunInfo den Inhalt unverändert.
func SetRunInfo(content string, cost float64, model string) string {
	costPart := costField + " " + strconv.FormatFloat(cost, 'f', 2, 64) + " USD"
	modelPart := ""
	if model != "" {
		modelPart = modelField + " " + model
	}
	lines := strings.SplitAfter(content, "\n")
	for i, line := range lines {
		if i >= headerScanLines {
			break
		}
		s, ok := ParseStatusLine(line)
		if !ok {
			continue
		}
		body := strings.TrimRight(line, "\r\n")
		_, rest, _ := strings.Cut(body, "·")
		var parts []string
		costAt, hasModel := -1, false
		for part := range strings.SplitSeq(rest, "·") {
			part = strings.TrimSpace(part)
			switch {
			case part == "":
				continue
			case strings.HasPrefix(part, costField):
				part, costAt = costPart, len(parts)
			case strings.HasPrefix(part, modelField):
				if modelPart == "" {
					continue
				}
				part, hasModel = modelPart, true
			}
			parts = append(parts, part)
		}
		if modelPart != "" && !hasModel {
			parts = slices.Insert(parts, costAt+1, modelPart)
		}
		if costAt < 0 {
			parts = slices.Insert(parts, 0, costPart)
		}
		lines[i] = "> Status: " + s.Value + " · " + strings.Join(parts, " · ") + line[len(body):]
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
