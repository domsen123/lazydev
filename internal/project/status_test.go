package project

import (
	"strings"
	"testing"
	"time"

	"lazydev/internal/lang"
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
			if got := SetBlocker(tt.in, "implement", points, lang.DE); got != tt.want {
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
	for _, l := range lang.Langs() {
		if got := RemoveBlocker(SetBlocker(in, "merge", []string{"a"}, l)); got != in {
			t.Errorf("round trip %s: %q", l, got)
		}
	}
}

func TestSetBlockerEnglish(t *testing.T) {
	got := SetBlocker("body\n\n## Blocker\n\nSchritt: merge\n\n- alt\n", "implement", []string{"Which table?"}, lang.EN)
	if want := "body\n\n## Blocker\n\nStep: implement\n\n- Which table?\n"; got != want {
		t.Errorf("SetBlocker = %q, want %q", got, want)
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
		"Cost: 1.00 USD":   true,
		"Model: m":         true,
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
			got, ok := SetRunInfo(tt.in, tt.cost, tt.model, lang.DE)
			if got != tt.want || ok != tt.ok {
				t.Errorf("SetRunInfo = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSetRunInfoRoundTrip(t *testing.T) {
	for _, in := range []string{"> Status: umgesetzt (2026-10-20) · Scope: a\n", "> Status: implemented (2026-10-20) · Scope: a\n"} {
		for _, l := range lang.Langs() {
			out, ok := SetRunInfo(in, 7.5, "opus", l)
			if !ok {
				t.Fatal("no status line")
			}
			s, _ := ParseStatusLine(strings.TrimSuffix(out, "\n"))
			cost, hasCost, model := RunInfo(s)
			if cost != 7.5 || !hasCost || model != "opus" || s.Date != "2026-10-20" || s.Kind != StatusImplemented {
				t.Errorf("round trip %s of %q: %+v, %v, %v, %q", l, in, s, cost, hasCost, model)
			}
		}
	}
}

func TestSetRunInfoLanguages(t *testing.T) {
	tests := []struct {
		name, in string
		l        lang.Lang
		want     string
	}{
		{"english", "> Status: implemented (2026-10-20) · Scope: a\n", lang.EN, "> Status: implemented (2026-10-20) · Cost: 1.00 USD · Model: m · Scope: a\n"},
		{"german", "> Status: umgesetzt (2026-10-20) · Scope: a\n", lang.DE, "> Status: umgesetzt (2026-10-20) · Kosten: 1.00 USD · Modell: m · Scope: a\n"},
		{"german fields in an english project", "> Status: umgesetzt (2026-10-14) · Scope: a · Kosten: 0.50 USD · Modell: x\n", lang.EN, "> Status: umgesetzt (2026-10-14) · Scope: a · Cost: 1.00 USD · Model: m\n"},
		{"english fields in a german project", "> Status: implemented (2026-10-14) · Cost: 0.50 USD · Model: x · Scope: a\n", lang.DE, "> Status: implemented (2026-10-14) · Kosten: 1.00 USD · Modell: m · Scope: a\n"},
		{"both languages", "> Status: planned · Kosten: 0.50 USD · Cost: 0.70 USD · Model: x\n", lang.EN, "> Status: planned · Cost: 1.00 USD · Model: m\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SetRunInfo(tt.in, 1, "m", tt.l)
			if !ok || got != tt.want {
				t.Errorf("SetRunInfo = %q, %v; want %q", got, ok, tt.want)
			}
		})
	}
}

func TestParseStatusLineBothLanguages(t *testing.T) {
	for line, kind := range map[string]StatusKind{
		"> Status: planned · Scope: a":              StatusPlanned,
		"> Status: implemented (2026-10-14)":        StatusImplemented,
		"> Status: discarded (2026-10-14)":          StatusDiscarded,
		"> Status: blocked (2026-10-14) · Scope: a": StatusBlocked,
		"> Status: umgesetzt (2026-10-14)":          StatusImplemented,
		"> Status: blockiert (2026-10-14)":          StatusBlocked,
	} {
		s, ok := ParseStatusLine(line)
		if !ok || s.Kind != kind {
			t.Errorf("ParseStatusLine(%q) = %+v", line, s)
		}
		if kind != StatusPlanned && s.Date != "2026-10-14" {
			t.Errorf("ParseStatusLine(%q) date = %q", line, s.Date)
		}
	}
	// A German status line reads the same, whatever the project writes.
	s, _ := ParseStatusLine("> Status: umgesetzt (2026-10-14) · Kosten: 1.00 USD · Modell: x")
	cost, hasCost, model := RunInfo(s)
	if s.Kind != StatusImplemented || s.Date != "2026-10-14" || cost != 1 || !hasCost || model != "x" {
		t.Errorf("German line: %+v, %v, %v, %q", s, cost, hasCost, model)
	}
	s, _ = ParseStatusLine("> Status: implemented (2026-10-14) · Cost: 1.00 USD · Model: x")
	cost, hasCost, model = RunInfo(s)
	if s.Kind != StatusImplemented || s.Date != "2026-10-14" || cost != 1 || !hasCost || model != "x" {
		t.Errorf("English line: %+v, %v, %v, %q", s, cost, hasCost, model)
	}
}

func TestStatusValue(t *testing.T) {
	day := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	if got := StatusValue(StatusBlocked, lang.EN, day); got != "blocked (2026-10-22)" {
		t.Errorf("EN blocked = %q", got)
	}
	if got := StatusValue(StatusBlocked, lang.DE, day); got != "blockiert (2026-10-22)" {
		t.Errorf("DE blocked = %q", got)
	}
	if got := StatusValue(StatusPlanned, lang.EN, day); got != "planned" {
		t.Errorf("EN planned = %q", got)
	}
	if got := StatusValue(StatusUnknown, lang.EN, day); got != "" {
		t.Errorf("unknown = %q", got)
	}
}

func TestBlocker(t *testing.T) {
	content := SetBlocker("# PRD: X\n\n## Ziel\n\nx\n", "implement", []string{"Welche Tabelle?", "Soll die API\n  versioniert sein?"}, lang.DE)
	step, points, ok := Blocker(content + "## Danach\n")
	if !ok || step != "implement" || len(points) != 2 || points[0] != "Welche Tabelle?" || points[1] != "Soll die API versioniert sein?" {
		t.Errorf("Blocker = %q, %q, %v", step, points, ok)
	}
	step, points, ok = Blocker(SetBlocker("# PRD: X\n", "verify", []string{"Which table?"}, lang.EN))
	if !ok || step != "verify" || len(points) != 1 || points[0] != "Which table?" {
		t.Errorf("English Blocker = %q, %q, %v", step, points, ok)
	}
	if _, _, ok := Blocker("# PRD: X\n"); ok {
		t.Error("Blocker found a section in a PRD without one")
	}
}
