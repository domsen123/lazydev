package project

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// StatusKind is what a status line says about a PRD.
type StatusKind int

const (
	StatusUnknown StatusKind = iota
	StatusPlanned
	StatusImplemented
	StatusDiscarded
	StatusBlocked
)

// Status values in PRD files. They stay German: lazydev's prompts, contracts and existing PRDs use
// them. umgesetzt, verworfen and blockiert get a date.
const (
	ValuePlanned     = "geplant"
	ValueImplemented = "umgesetzt"
	ValueDiscarded   = "verworfen"
	ValueBlocked     = "blockiert"
)

// The section the runner appends to a blocked PRD and its first line, German like the status values.
const (
	blockerHeading = "## Blocker"
	blockerStep    = "Schritt:"
)

// Fields of the detail that the runner writes before the merge, German like the status values.
const (
	costField  = "Kosten:"
	modelField = "Modell:"
)

// HeaderScanLines limits the search for the status line to the head of a file.
const HeaderScanLines = 20

// Status is the parsed line "> Status: <value> · <detail>".
type Status struct {
	Kind StatusKind
	// Value is the part before "·", e.g. "umgesetzt (2026-10-14)".
	Value string
	// Date is the date of "umgesetzt (YYYY-MM-DD)", "verworfen (YYYY-MM-DD)" and
	// "blockiert (YYYY-MM-DD)", empty otherwise.
	Date string
	// Detail is the text after "·", e.g. "Scope: backend only".
	Detail string
}

var (
	statusLineRe = regexp.MustCompile(`^>\s*Status:\s*(.*)$`)
	statusDateRe = regexp.MustCompile(`^(?:umgesetzt|verworfen|blockiert)\s*\((\d{4}-\d{2}-\d{2})\)`)
)

// ParseStatusLine recognizes a status line. ok is false for other lines.
func ParseStatusLine(line string) (s Status, ok bool) {
	m := statusLineRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Status{}, false
	}
	value, detail, _ := strings.Cut(m[1], "·")
	s.Value, s.Detail = strings.TrimSpace(value), strings.TrimSpace(detail)
	switch {
	case s.Value == ValuePlanned:
		s.Kind = StatusPlanned
	case strings.HasPrefix(s.Value, ValueImplemented):
		s.Kind = StatusImplemented
	case strings.HasPrefix(s.Value, ValueDiscarded):
		s.Kind = StatusDiscarded
	case strings.HasPrefix(s.Value, ValueBlocked):
		s.Kind = StatusBlocked
	}
	if d := statusDateRe.FindStringSubmatch(s.Value); d != nil {
		s.Date = d[1]
	}
	return s, true
}

// SetStatus sets the value of the first status line. The detail after "·" stays. ok is false
// without status line, then content stays as it is.
func SetStatus(content, value string) (string, bool) {
	lines := strings.SplitAfter(content, "\n")
	for i, line := range lines {
		if i >= HeaderScanLines {
			break
		}
		s, ok := ParseStatusLine(line)
		if !ok {
			continue
		}
		// The line ending stays as it is, \r\n included.
		newline := line[len(strings.TrimRight(line, "\r\n")):]
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

// SetBlocker writes the section "## Blocker" with the step and the points of a blocked run: at the
// end of content, or in place of an existing section up to the next "## " heading or the end.
func SetBlocker(content, step string, points []string) string {
	section := []string{blockerHeading, "", blockerStep + " " + step, ""}
	for _, p := range points {
		section = append(section, "- "+p)
	}
	lines := strings.Split(content, "\n")
	start, end, ok := blockerSection(lines)
	if !ok {
		body := strings.TrimRight(content, "\r\n")
		if body == "" {
			return strings.Join(section, "\n") + "\n"
		}
		return body + "\n\n" + strings.Join(section, "\n") + "\n"
	}
	// The empty line ends the last point: before the next heading it leaves a blank line, at the
	// end the final newline.
	section = append(section, "")
	return strings.Join(slices.Concat(lines[:start], section, lines[end:]), "\n")
}

// RemoveBlocker removes the section "## Blocker" with the blank lines before it. Without the
// section content stays as it is.
func RemoveBlocker(content string) string {
	lines := strings.Split(content, "\n")
	start, end, ok := blockerSection(lines)
	if !ok {
		return content
	}
	for start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start--
	}
	before := strings.Join(lines[:start], "\n")
	switch {
	case end == len(lines) && start == 0:
		return ""
	case end == len(lines):
		return before + "\n"
	case start == 0:
		return strings.Join(lines[end:], "\n")
	}
	return before + "\n\n" + strings.Join(lines[end:], "\n")
}

// blockerSection finds the lines of "## Blocker" up to the next "## " heading, end excluded.
func blockerSection(lines []string) (start, end int, ok bool) {
	start = slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == blockerHeading })
	if start < 0 {
		return 0, 0, false
	}
	for end = start + 1; end < len(lines); end++ {
		if strings.HasPrefix(strings.TrimSpace(lines[end]), "## ") {
			break
		}
	}
	return start, end, true
}

// RunInfo reads cost and model from the first line of the detail, e.g.
// "Kosten: 4.12 USD · Modell: claude-opus-5-5 · Scope: …". An amount that is no number counts as
// missing.
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

// IsRunInfo is true for the parts of a detail that RunInfo reads.
func IsRunInfo(part string) bool {
	return strings.HasPrefix(part, costField) || strings.HasPrefix(part, modelField)
}

// SetRunInfo sets cost and model in the first status line right after the value. Existing fields
// keep their place. Without model the field "Modell:" goes. ok is false without status line.
func SetRunInfo(content string, cost float64, model string) (string, bool) {
	costPart := costField + " " + strconv.FormatFloat(cost, 'f', 2, 64) + " USD"
	modelPart := ""
	if model != "" {
		modelPart = modelField + " " + model
	}
	lines := strings.SplitAfter(content, "\n")
	for i, line := range lines {
		if i >= HeaderScanLines {
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
		return strings.Join(lines, ""), true
	}
	return content, false
}
