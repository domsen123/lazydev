package claude

import (
	"slices"
	"strings"
	"testing"
)

const report = `Fertig.

## Ergebnis
umgesetzt

## Offene Fragen
- keine

## Nicht geprüft
- Browser
`

func TestSection(t *testing.T) {
	if got, ok := Section(report, "Ergebnis"); !ok || got != "umgesetzt" {
		t.Errorf("Ergebnis = %q, %v", got, ok)
	}
	if got, _ := Section(report, "Offene Fragen"); got != "- keine" {
		t.Errorf("Offene Fragen = %q", got)
	}
	if _, ok := Section(report, "Fehlt"); ok {
		t.Error("fehlender Abschnitt gefunden")
	}
}

func TestSummaryTakesLastResult(t *testing.T) {
	var s Summary
	lines := []string{
		`{"type":"system","subtype":"hook_started"}`,
		`{"type":"system","subtype":"init","session_id":"a"}`,
		`{"type":"result","is_error":false,"result":"erste Runde","total_cost_usd":0.1}`,
		`kein json`,
		`{"type":"system","subtype":"init","session_id":"a"}`,
		`{"type":"result","is_error":true,"result":"zweite Runde","total_cost_usd":0.4}`,
	}
	for _, l := range lines {
		if e, ok := ParseEvent([]byte(l)); ok {
			s.Add(e)
		}
	}
	if s.SessionID != "a" || !s.HasResult || !s.IsError || s.Result != "zweite Runde" || s.CostUSD != 0.4 {
		t.Errorf("Summary = %+v", s)
	}
}

func TestSummaryTakesFirstModel(t *testing.T) {
	var s Summary
	if e, _ := ParseEvent([]byte(`{"type":"system","subtype":"init","session_id":"a","model":"claude-opus-5-5"}`)); !s.Add(e) {
		t.Error("init mit Modell meldet keine Änderung")
	}
	// Ein späteres init, etwa eines Subagenten, ändert das Modell nicht.
	if e, _ := ParseEvent([]byte(`{"type":"system","subtype":"init","session_id":"b","model":"claude-haiku-5-5"}`)); s.Add(e) {
		t.Error("zweites init meldet eine Änderung")
	}
	if s.SessionID != "a" || s.Model != "claude-opus-5-5" {
		t.Errorf("Summary = %+v", s)
	}
}

func TestDescribe(t *testing.T) {
	cases := map[string]string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"pnpm typecheck"}}]}}`: "Bash pnpm typecheck",
		`{"type":"assistant","message":{"content":[{"type":"text","text":"\nHallo\nWelt"}]}}`:                                 "Hallo",
		`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":"kaputt\nmehr"}]}}`:             "Fehler: kaputt",
		`{"type":"system","subtype":"permission_denied","tool_name":"Bash"}`:                                                  "abgelehnt: Bash",
		`{"type":"stderr","text":"warn"}`: "stderr: warn",
	}
	for line, want := range cases {
		if got := strings.Join(Describe([]byte(line)), "|"); got != want {
			t.Errorf("Describe(%s) = %q, erwartet %q", line, got, want)
		}
	}
	if got := Describe([]byte(`{"type":"rate_limit_event"}`)); got != nil {
		t.Errorf("rate_limit_event = %q", got)
	}
}

func TestArgsJoinAllowedTools(t *testing.T) {
	args := Options{Prompt: "/x", AllowedTools: []string{"Read", "Bash(git status:*)"}}.Args()
	i := slices.Index(args, "--allowedTools")
	if i < 0 || args[i+1] != "Read,Bash(git status:*)" {
		t.Errorf("allowedTools in %q", args)
	}
	if i := slices.Index(args, "--agents"); i < 0 || args[i+1] != AgentsPath() {
		t.Errorf("--agents fehlt in %q", args)
	}
}
