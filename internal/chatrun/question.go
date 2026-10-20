package chatrun

import (
	"regexp"
	"strings"
)

// optionLine erkennt eine Antwortoption wie "- A: …", "**B:** …" oder "C) …".
var optionLine = regexp.MustCompile(`^\s*(?:[-*+>]\s*)*(?:\*\*)?[A-D][:)]`)

// IsQuestion meldet, ob Claude mit text eine Antwort erwartet: Eine Zeile beginnt mit einer
// Option A bis D, oder die letzte nicht leere Zeile endet auf "?".
func IsQuestion(text string) bool {
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		if optionLine.MatchString(l) {
			return true
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		// Markdown wie "?**" oder "?`" zählt wie ein "?" am Ende.
		return strings.HasSuffix(strings.TrimRight(lines[i], " \t\r*_`"), "?")
	}
	return false
}
