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
		{"> Status: geplant", true, StatusPlanned, "geplant", "", ""},
		{"> Status: geplant · Scope: backend only", true, StatusPlanned, "geplant", "", "Scope: backend only"},
		{">Status: geplant", true, StatusPlanned, "geplant", "", ""},
		{"  > Status: geplant  ", true, StatusPlanned, "geplant", "", ""},
		{"> Status: umgesetzt (2026-10-14) · Scope: theme", true, StatusImplemented, "umgesetzt (2026-10-14)", "2026-10-14", "Scope: theme"},
		{"> Status: umgesetzt", true, StatusImplemented, "umgesetzt", "", ""},
		{"> Status: verworfen (2026-10-21)", true, StatusDiscarded, "verworfen (2026-10-21)", "2026-10-21", ""},
		{"> Status: verworfen", true, StatusDiscarded, "verworfen", "", ""},
		{"> Status: blockiert (2026-10-22) · Scope: x", true, StatusBlocked, "blockiert (2026-10-22)", "2026-10-22", "Scope: x"},
		{"> Status: blockiert", true, StatusBlocked, "blockiert", "", ""},
		{"> Status: in progress · a · b", true, StatusUnknown, "in progress", "", "a · b"},
		{"> Status: geplant (2026-10-14)", true, StatusUnknown, "geplant (2026-10-14)", "", ""},
		{"> Status: umgesetzt (14.10.2026)", true, StatusImplemented, "umgesetzt (14.10.2026)", "", ""},
		{"Status: geplant", false, StatusUnknown, "", "", ""},
		{"# PRD: Title", false, StatusUnknown, "", "", ""},
		{"> Scope: geplant", false, StatusUnknown, "", "", ""},
		{"", false, StatusUnknown, "", "", ""},
	}
	for _, tt := range tests {
		s, ok := ParseStatusLine(tt.line)
		if ok != tt.ok || s.Kind != tt.kind || s.Value != tt.value || s.Date != tt.date || s.Detail != tt.detail {
			t.Errorf("ParseStatusLine(%q) = %+v, %v", tt.line, s, ok)
		}
	}
}

func TestSetStatus(t *testing.T) {
	tests := []struct {
		name, in, value, want string
		ok                    bool
	}{
		{
			name:  "keeps detail",
			in:    "# PRD: X\n\n> Status: geplant · Scope: a · b\n> more\n",
			value: "umgesetzt (2026-10-20)",
			want:  "# PRD: X\n\n> Status: umgesetzt (2026-10-20) · Scope: a · b\n> more\n",
			ok:    true,
		},
		{
			name:  "without detail",
			in:    "# PRD: X\n> Status: umgesetzt (2026-10-20)\n",
			value: "geplant",
			want:  "# PRD: X\n> Status: geplant\n",
			ok:    true,
		},
		{
			name:  "no final newline",
			in:    "# PRD: X\n> Status: geplant · Scope: a",
			value: "verworfen (2026-10-21)",
			want:  "# PRD: X\n> Status: verworfen (2026-10-21) · Scope: a",
			ok:    true,
		},
		{
			name:  "only first status line",
			in:    "> Status: geplant\n> Status: geplant\n",
			value: "umgesetzt (2026-10-20)",
			want:  "> Status: umgesetzt (2026-10-20)\n> Status: geplant\n",
			ok:    true,
		},
		{
			name:  "no status line",
			in:    "# Notes\n\nText\n",
			value: "geplant",
			want:  "# Notes\n\nText\n",
		},
		{
			name:  "status line after the head",
			in:    strings.Repeat("text\n", HeaderScanLines) + "> Status: geplant\n",
			value: "umgesetzt (2026-10-20)",
			want:  strings.Repeat("text\n", HeaderScanLines) + "> Status: geplant\n",
		},
		{
			name:  "status line on the last head line",
			in:    strings.Repeat("text\n", HeaderScanLines-1) + "> Status: geplant\n",
			value: "geplant",
			want:  strings.Repeat("text\n", HeaderScanLines-1) + "> Status: geplant\n",
			ok:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SetStatus(tt.in, tt.value)
			if got != tt.want || ok != tt.ok {
				t.Errorf("SetStatus = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSetStatusKeepsCRLF(t *testing.T) {
	for _, in := range []string{
		"# PRD: X\r\n> Status: geplant · Scope: a\r\n\r\nText\r\n",
		"# PRD: X\r\n> Status: geplant\r\n\r\nText\r\n",
	} {
		got, ok := SetStatus(in, "umgesetzt (2026-10-20)")
		if !ok {
			t.Fatalf("SetStatus(%q): no status line", in)
		}
		if strings.Count(got, "\r\n") != strings.Count(in, "\r\n") {
			t.Errorf("SetStatus(%q) = %q, line ending changed", in, got)
		}
	}
}

func TestSetBlocker(t *testing.T) {
	const section = "## Blocker\n\nSchritt: implement\n\n- Feld fehlt\n- Zweite Frage\n"
	points := []string{"Feld fehlt", "Zweite Frage"}
	tests := []struct {
		name, in, want string
	}{
		{"append", "# PRD: X\n\n> Status: geplant\n\nbody\n", "# PRD: X\n\n> Status: geplant\n\nbody\n\n" + section},
		{"append without final newline", "# PRD: X\n\nbody", "# PRD: X\n\nbody\n\n" + section},
		{"append after blank lines", "body\n\n\n", "body\n\n" + section},
		{"empty", "", section},
		{"replace at the end", "body\n\n## Blocker\n\nSchritt: merge\n\n- alt\n", "body\n\n" + section},
		{"replace before a heading", "body\n\n## Blocker\n\nSchritt: merge\n\n- alt\n\n## Notes\n\ntext\n", "body\n\n" + section + "\n## Notes\n\ntext\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetBlocker(tt.in, "implement", points); got != tt.want {
				t.Errorf("SetBlocker = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemoveBlocker(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"at the end", "body\n\n## Blocker\n\nSchritt: implement\n\n- a\n", "body\n"},
		{"before a heading", "body\n\n## Blocker\n\nSchritt: implement\n\n- a\n\n## Notes\n\ntext\n", "body\n\n## Notes\n\ntext\n"},
		{"only the section", "## Blocker\n\nSchritt: merge\n", ""},
		{"without section", "body\n\n## Notes\n", "body\n\n## Notes\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveBlocker(tt.in); got != tt.want {
				t.Errorf("RemoveBlocker = %q, want %q", got, tt.want)
			}
		})
	}
	// Set and remove give the PRD back.
	in := "# PRD: X\n\n> Status: geplant\n\nbody\n"
	if got := RemoveBlocker(SetBlocker(in, "merge", []string{"a"})); got != in {
		t.Errorf("round trip: %q", got)
	}
}

func TestRunInfo(t *testing.T) {
	tests := []struct {
		detail  string
		cost    float64
		hasCost bool
		model   string
	}{
		{"Kosten: 4.12 USD · Modell: claude-opus-5-5 · Scope: x", 4.12, true, "claude-opus-5-5"},
		{"Scope: x · Kosten: 0.50 USD", 0.5, true, ""},
		{"Kosten: 3", 3, true, ""},
		{"Modell: sonnet", 0, false, "sonnet"},
		{"Kosten: much USD · Modell: m", 0, false, "m"},
		{"Kosten: NaN USD", 0, false, ""},
		{"Kosten: Inf USD", 0, false, ""},
		{"Scope: x\nKosten: 1.00 USD · Modell: m", 0, false, ""},
		{"", 0, false, ""},
	}
	for _, tt := range tests {
		cost, hasCost, model := RunInfo(Status{Detail: tt.detail})
		if cost != tt.cost || hasCost != tt.hasCost || model != tt.model {
			t.Errorf("RunInfo(%q) = %v, %v, %q", tt.detail, cost, hasCost, model)
		}
	}
}

func TestIsRunInfo(t *testing.T) {
	for part, want := range map[string]bool{
		"Kosten: 1.00 USD": true,
		"Modell: m":        true,
		"Scope: x":         false,
		" Kosten: 1":       false,
		"":                 false,
	} {
		if got := IsRunInfo(part); got != want {
			t.Errorf("IsRunInfo(%q) = %v", part, got)
		}
	}
}

func TestSetRunInfo(t *testing.T) {
	tests := []struct {
		name, in string
		cost     float64
		model    string
		want     string
		ok       bool
	}{
		{
			name:  "insert after the value",
			in:    "# PRD: X\n\n> Status: geplant · Scope: a\n> more\n",
			cost:  4.123,
			model: "claude-opus-5-5",
			want:  "# PRD: X\n\n> Status: geplant · Kosten: 4.12 USD · Modell: claude-opus-5-5 · Scope: a\n> more\n",
			ok:    true,
		},
		{
			name:  "insert without detail",
			in:    "> Status: geplant\n",
			cost:  1,
			model: "m",
			want:  "> Status: geplant · Kosten: 1.00 USD · Modell: m\n",
			ok:    true,
		},
		{
			name:  "replace in place",
			in:    "> Status: geplant · Scope: a · Kosten: 1.00 USD · Modell: old · Risk: b\n",
			cost:  2.5,
			model: "new",
			want:  "> Status: geplant · Scope: a · Kosten: 2.50 USD · Modell: new · Risk: b\n",
			ok:    true,
		},
		{
			name:  "model goes right after existing cost",
			in:    "> Status: geplant · Scope: a · Kosten: 1.00 USD · Risk: b\n",
			cost:  2,
			model: "m",
			want:  "> Status: geplant · Scope: a · Kosten: 2.00 USD · Modell: m · Risk: b\n",
			ok:    true,
		},
		{
			name: "empty model drops the field",
			in:   "> Status: geplant · Kosten: 1.00 USD · Modell: old · Scope: a\n",
			cost: 3,
			want: "> Status: geplant · Kosten: 3.00 USD · Scope: a\n",
			ok:   true,
		},
		{
			name:  "keeps CRLF",
			in:    "> Status: geplant · Scope: a\r\nText\r\n",
			cost:  1,
			model: "m",
			want:  "> Status: geplant · Kosten: 1.00 USD · Modell: m · Scope: a\r\nText\r\n",
			ok:    true,
		},
		{
			name: "no status line",
			in:   "# Notes\n\nText\n",
			cost: 1,
			want: "# Notes\n\nText\n",
		},
		{
			name: "status line after the head",
			in:   strings.Repeat("x\n", HeaderScanLines) + "> Status: geplant\n",
			cost: 1,
			want: strings.Repeat("x\n", HeaderScanLines) + "> Status: geplant\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SetRunInfo(tt.in, tt.cost, tt.model)
			if got != tt.want || ok != tt.ok {
				t.Errorf("SetRunInfo = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSetRunInfoRoundTrip(t *testing.T) {
	out, ok := SetRunInfo("> Status: umgesetzt (2026-10-20) · Scope: a\n", 7.5, "opus")
	if !ok {
		t.Fatal("no status line")
	}
	s, _ := ParseStatusLine(strings.TrimSuffix(out, "\n"))
	cost, hasCost, model := RunInfo(s)
	if cost != 7.5 || !hasCost || model != "opus" || s.Date != "2026-10-20" {
		t.Errorf("round trip: %+v, %v, %v, %q", s, cost, hasCost, model)
	}
}
