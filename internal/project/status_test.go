package project

import (
	"strings"
	"testing"
)

func TestParseStatusLine(t *testing.T) {
	tests := []struct {
		line   string
		ok     bool
		kind   StatusKind
		value  string
		date   string
		detail string
	}{
		{"> Status: geplant · Scope: Backend-only", true, StatusPlanned, "geplant", "", "Scope: Backend-only"},
		{"> Status: umgesetzt (2026-10-14) · Scope: Theme", true, StatusImplemented, "umgesetzt (2026-10-14)", "2026-10-14", "Scope: Theme"},
		{">Status: geplant", true, StatusPlanned, "geplant", "", ""},
		{"> Status: umgesetzt", true, StatusImplemented, "umgesetzt", "", ""},
		{"> Status: in Arbeit · a · b", true, StatusUnknown, "in Arbeit", "", "a · b"},
		{"Status: geplant", false, StatusUnknown, "", "", ""},
		{"# PRD: Titel", false, StatusUnknown, "", "", ""},
	}
	for _, tt := range tests {
		s, ok := ParseStatusLine(tt.line)
		if ok != tt.ok || s.Kind != tt.kind || s.Value != tt.value || s.Date != tt.date || s.Detail != tt.detail {
			t.Errorf("ParseStatusLine(%q) = %+v, %v", tt.line, s, ok)
		}
	}
}

func TestParseHeader(t *testing.T) {
	tests := []struct {
		name, input, title, value, detail string
	}{
		{
			name:   "Statuszeile in Zeile 2",
			input:  "# PRD: Titel\n> Status: geplant · Scope: X\n\n## Ziel\n",
			title:  "Titel",
			value:  "geplant",
			detail: "Scope: X",
		},
		{
			name:   "Leerzeile und Folgezeilen",
			input:  "# PRD: Titel\n\n> Status: geplant · Scope: erste\n> zweite Zeile\n\n> anderer Block\n",
			title:  "Titel",
			value:  "geplant",
			detail: "Scope: erste\nzweite Zeile",
		},
		{
			name:  "ohne Statuszeile",
			input: "# Notizen\n\nText\n",
			title: "Notizen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, s, err := ParseHeader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if title != tt.title || s.Value != tt.value || s.Detail != tt.detail {
				t.Errorf("got title=%q value=%q detail=%q", title, s.Value, s.Detail)
			}
		})
	}
}

func TestResetStatus(t *testing.T) {
	tests := map[string]string{
		"# PRD: X\n\n> Status: umgesetzt (2026-10-20) · Scope: a\n> b\n": "# PRD: X\n\n> Status: geplant · Scope: a\n> b\n",
		"# PRD: X\n> Status: umgesetzt (2026-10-20)\n":                   "# PRD: X\n> Status: geplant\n",
		"# PRD: X\n> Status: geplant · Scope: a":                         "# PRD: X\n> Status: geplant · Scope: a",
		"# PRD: X\n\nText\n": "# PRD: X\n\nText\n",
	}
	for in, want := range tests {
		if got := ResetStatus(in); got != want {
			t.Errorf("ResetStatus(%q) = %q, erwartet %q", in, got, want)
		}
	}
}

func TestPhaseFromStatus(t *testing.T) {
	if got := PhaseFromStatus(Status{Kind: StatusPlanned}); got != PhasePlanned {
		t.Errorf("geplant: %s", got)
	}
	if got := PhaseFromStatus(Status{Kind: StatusImplemented}); got != PhaseMerged {
		t.Errorf("umgesetzt: %s", got)
	}
	if got := PhaseFromStatus(Status{}); got != PhaseUnknown {
		t.Errorf("unbekannt: %s", got)
	}
}
