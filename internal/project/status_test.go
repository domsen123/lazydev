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

func TestRunInfo(t *testing.T) {
	tests := []struct {
		detail  string
		cost    float64
		hasCost bool
		model   string
	}{
		{"Kosten: 4.12 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: x", 4.12, true, "claude-opus-5-5"},
		{"Kosten: 0.80 USD · Scope: x", 0.8, true, ""},
		{"Kosten: viel USD · Modell: claude-haiku-5-5", 0, false, "claude-haiku-5-5"},
		{"Scope: x\n> Kosten: 1.00 USD", 0, false, ""},
		{"Scope: Kosten: 1.00 USD", 0, false, ""},
		{"", 0, false, ""},
	}
	for _, tt := range tests {
		cost, hasCost, model := RunInfo(Status{Detail: tt.detail})
		if cost != tt.cost || hasCost != tt.hasCost || model != tt.model {
			t.Errorf("RunInfo(%q) = %v, %v, %q", tt.detail, cost, hasCost, model)
		}
	}
	_, s, _ := ParseHeader(strings.NewReader("# PRD: X\n> Status: umgesetzt (2026-10-21) · Kosten: 2.50 USD · Modell: m · Scope: a\n> b\n"))
	if cost, ok, model := RunInfo(s); cost != 2.5 || !ok || model != "m" {
		t.Errorf("mit Folgezeile: %v, %v, %q", cost, ok, model)
	}
}

func TestSetRunInfo(t *testing.T) {
	tests := []struct {
		name, in   string
		cost       float64
		model, out string
	}{
		{
			name:  "Freitext ohne Felder",
			in:    "# PRD: X\n\n> Status: umgesetzt (2026-10-21) · Scope: a\n> b\n",
			cost:  4.123,
			model: "claude-opus-5-5",
			out:   "# PRD: X\n\n> Status: umgesetzt (2026-10-21) · Kosten: 4.12 USD · Modell: claude-opus-5-5 · Scope: a\n> b\n",
		},
		{
			name:  "mit Feldern",
			in:    "# PRD: X\n> Status: umgesetzt (2026-10-21) · Kosten: 1.00 USD · Modell: claude-haiku-5-5 · Scope: a\n",
			cost:  2,
			model: "claude-opus-5-5",
			out:   "# PRD: X\n> Status: umgesetzt (2026-10-21) · Kosten: 2.00 USD · Modell: claude-opus-5-5 · Scope: a\n",
		},
		{
			name:  "mit Template und Scope",
			in:    "# PRD: X\n> Status: umgesetzt (2026-10-21) · Template: einfach · Scope: a\n",
			cost:  0.8,
			model: "claude-opus-5-5",
			out:   "# PRD: X\n> Status: umgesetzt (2026-10-21) · Kosten: 0.80 USD · Modell: claude-opus-5-5 · Template: einfach · Scope: a\n",
		},
		{
			name:  "ohne Freitext",
			in:    "# PRD: X\n> Status: umgesetzt (2026-10-21)\r\n\nText\n",
			cost:  1,
			model: "claude-opus-5-5",
			out:   "# PRD: X\n> Status: umgesetzt (2026-10-21) · Kosten: 1.00 USD · Modell: claude-opus-5-5\r\n\nText\n",
		},
		{
			name: "ohne Modell",
			in:   "# PRD: X\n> Status: umgesetzt (2026-10-21) · Scope: a",
			cost: 1.5,
			out:  "# PRD: X\n> Status: umgesetzt (2026-10-21) · Kosten: 1.50 USD · Scope: a",
		},
		{
			name: "ohne Statuszeile",
			in:   "# PRD: X\n\nText\n",
			cost: 1,
			out:  "# PRD: X\n\nText\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetRunInfo(tt.in, tt.cost, tt.model)
			if got != tt.out {
				t.Errorf("SetRunInfo = %q, erwartet %q", got, tt.out)
			}
			// Ein zweiter Aufruf mit gleichen Werten ändert nichts.
			if again := SetRunInfo(got, tt.cost, tt.model); again != got {
				t.Errorf("zweiter Aufruf = %q", again)
			}
		})
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
