package project

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"lazydev/internal/lang"
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

// The status values, the section "## Blocker" with its first line and the fields cost and model
// come from lang: lazydev reads every language and writes the language of the project.

// HeaderScanLines limits the search for the status line to the head of a file.
const HeaderScanLines = 20

// Status is the parsed line "> Status: <value> · <detail>".
type Status struct {
	Kind StatusKind
	// Value is the part before "·", e.g. "implemented (2026-10-14)".
	Value string
	// Date is the date of "implemented (YYYY-MM-DD)", "discarded (YYYY-MM-DD)" and
	// "blocked (YYYY-MM-DD)" or their German values, empty otherwise.
	Date string
	// Detail is the text after "·", e.g. "Scope: backend only".
	Detail string
}

var (
	statusLineRe = regexp.MustCompile(`^>\s*Status:\s*(.*)$`)
	statusDateRe = buildDateRe()
)

// buildDateRe matches the dated status values of every language, e.g.
// "(?:implemented|discarded|blocked|umgesetzt|verworfen|blockiert)".
func buildDateRe() *regexp.Regexp {
	var words []string
	for _, v := range lang.All() {
		for _, w := range []string{v.Implemented, v.Discarded, v.Blocked} {
			words = append(words, regexp.QuoteMeta(w))
		}
	}
	return regexp.MustCompile(`^(?:` + strings.Join(words, "|") + `)\s*\((\d{4}-\d{2}-\d{2})\)`)
}

// ParseStatusLine recognizes a status line in any language. ok is false for other lines.
func ParseStatusLine(line string) (s Status, ok bool) {
	m := statusLineRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Status{}, false
	}
	value, detail, _ := strings.Cut(m[1], "·")
	s.Value, s.Detail = strings.TrimSpace(value), strings.TrimSpace(detail)
	s.Kind = statusKind(s.Value)
	if d := statusDateRe.FindStringSubmatch(s.Value); d != nil {
		s.Date = d[1]
	}
	return s, true
}

// statusKind is the kind of a status value in any language.
func statusKind(value string) StatusKind {
	for _, v := range lang.All() {
		switch {
		case value == v.Planned:
			return StatusPlanned
		case strings.HasPrefix(value, v.Implemented):
			return StatusImplemented
		case strings.HasPrefix(value, v.Discarded):
			return StatusDiscarded
		case strings.HasPrefix(value, v.Blocked):
			return StatusBlocked
		}
	}
	return StatusUnknown
}

// StatusValue is the status value of kind in language l on day now: planned without date, the
// others with "(YYYY-MM-DD)". StatusUnknown gives "".
func StatusValue(kind StatusKind, l lang.Lang, now time.Time) string {
	v := lang.For(l)
	word := map[StatusKind]string{
		StatusPlanned:     v.Planned,
		StatusImplemented: v.Implemented,
		StatusDiscarded:   v.Discarded,
		StatusBlocked:     v.Blocked,
	}[kind]
	if kind == StatusPlanned || word == "" {
		return word
	}
	return fmt.Sprintf("%s (%s)", word, now.Format("2006-01-02"))
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

// SetBlocker writes the section "## Blocker" in language l with the step and the points of a
// blocked run: at the end of content, or in place of an existing section up to the next "## "
// heading or the end.
func SetBlocker(content, step string, points []string, l lang.Lang) string {
	v := lang.For(l)
	section := []string{v.BlockerHeading, "", v.BlockerStep + " " + step, ""}
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

// Blocker reads the section "## Blocker": the blocked step and its points, the continuation lines
// of a point joined with a space. ok is false without the section.
func Blocker(content string) (step string, points []string, ok bool) {
	lines := strings.Split(content, "\n")
	start, end, ok := blockerSection(lines)
	if !ok {
		return "", nil, false
	}
	for _, l := range lines[start+1 : end] {
		t := strings.TrimSpace(l)
		if label, ok := blockerStepLabel(t); ok {
			step = strings.TrimSpace(strings.TrimPrefix(t, label))
			continue
		}
		switch {
		case t == "":
		case strings.HasPrefix(t, "- "):
			points = append(points, strings.TrimPrefix(t, "- "))
		case len(points) > 0:
			points[len(points)-1] += " " + t
		}
	}
	return step, points, true
}

// blockerStepLabel is the label "Step:" or "Schritt:" that line starts with.
func blockerStepLabel(line string) (string, bool) {
	for _, v := range lang.All() {
		if strings.HasPrefix(line, v.BlockerStep) {
			return v.BlockerStep, true
		}
	}
	return "", false
}

// isBlockerHeading is true for the heading "## Blocker" of any language.
func isBlockerHeading(line string) bool {
	return slices.ContainsFunc(lang.All(), func(v lang.Vocab) bool { return strings.TrimSpace(line) == v.BlockerHeading })
}

// blockerSection finds the lines of "## Blocker" up to the next "## " heading, end excluded.
func blockerSection(lines []string) (start, end int, ok bool) {
	start = slices.IndexFunc(lines, isBlockerHeading)
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
// "Cost: 4.12 USD · Model: claude-opus-5-5 · Scope: …", in any language. An amount that is no
// number counts as missing.
func RunInfo(s Status) (cost float64, hasCost bool, model string) {
	first, _, _ := strings.Cut(s.Detail, "\n")
	for part := range strings.SplitSeq(first, "·") {
		part = strings.TrimSpace(part)
		if v, ok := cutField(part, costField); ok {
			v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "USD"))
			if c, err := strconv.ParseFloat(v, 64); err == nil && !math.IsNaN(c) && !math.IsInf(c, 0) {
				cost, hasCost = c, true
			}
		} else if v, ok := cutField(part, modelField); ok {
			model = strings.TrimSpace(v)
		}
	}
	return cost, hasCost, model
}

// costField and modelField pick a field of a vocabulary.
func costField(v lang.Vocab) string  { return v.Cost }
func modelField(v lang.Vocab) string { return v.Model }

// cutField removes the label of field in any language from the start of part.
func cutField(part string, field func(lang.Vocab) string) (string, bool) {
	for _, v := range lang.All() {
		if rest, ok := strings.CutPrefix(part, field(v)); ok {
			return rest, true
		}
	}
	return "", false
}

// hasField is true if part starts with the label of field in any language.
func hasField(part string, field func(lang.Vocab) string) bool {
	_, ok := cutField(part, field)
	return ok
}

// IsRunInfo is true for the parts of a detail that RunInfo reads, in any language.
func IsRunInfo(part string) bool {
	return hasField(part, costField) || hasField(part, modelField)
}

// SetRunInfo sets cost and model in language l in the first status line right after the value.
// Existing fields of any language keep their place and take the words of l. Without model the
// field goes. ok is false without status line.
func SetRunInfo(content string, cost float64, model string, l lang.Lang) (string, bool) {
	v := lang.For(l)
	costPart := v.Cost + " " + strconv.FormatFloat(cost, 'f', 2, 64) + " USD"
	modelPart := ""
	if model != "" {
		modelPart = v.Model + " " + model
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
			case hasField(part, costField):
				if costAt >= 0 {
					continue
				}
				part, costAt = costPart, len(parts)
			case hasField(part, modelField):
				if modelPart == "" || hasModel {
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
