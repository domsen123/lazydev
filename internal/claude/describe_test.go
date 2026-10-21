package claude

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestDescribeRunnerLine(t *testing.T) {
	got := Describe([]byte(`{"type":"lazydev","time":"2026-10-20T17:00:53+02:00","text":"setup: worktree ready"}`))
	if len(got) != 1 {
		t.Fatalf("entries: %+v", got)
	}
	e := got[0]
	want, _ := time.Parse(time.RFC3339, "2026-10-20T17:00:53+02:00")
	if e.Kind != KindRunner || e.Text != "setup: worktree ready" || !e.At.Equal(want) || e.Sub {
		t.Errorf("entry: %+v", e)
	}
}

func TestDescribeTSWinsOverTime(t *testing.T) {
	got := Describe([]byte(`{"ts":"2026-10-20T10:00:00Z","time":"2026-10-20T11:00:00Z","type":"lazydev","text":"x"}`))
	want, _ := time.Parse(time.RFC3339, "2026-10-20T10:00:00Z")
	if len(got) != 1 || !got[0].At.Equal(want) {
		t.Errorf("entries: %+v", got)
	}
}

func TestDescribeAssistant(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[
		{"type":"text","text":"\n\n  First line  \nsecond line"},
		{"type":"text","text":"   \n"},
		{"type":"tool_use","name":"Read","input":{"file_path":"/a/b.go","command":"ignored"}},
		{"type":"tool_use","name":"Bash","input":{"command":"go test ./...\necho done"}},
		{"type":"tool_use","name":"TodoWrite","input":{"todos":[]}}
	]}}`
	got := Describe([]byte(line))
	want := []struct{ kind, text string }{
		{KindText, "First line"},
		{KindTool, "Read /a/b.go"},
		{KindTool, "Bash go test ./..."},
		{KindTool, "TodoWrite"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries: %+v", got)
	}
	for i, w := range want {
		if got[i].Kind != w.kind || got[i].Text != w.text || got[i].Sub {
			t.Errorf("entry %d: %+v, want %+v", i, got[i], w)
		}
	}
}

func TestDescribeSubagent(t *testing.T) {
	got := Describe([]byte(`{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"content":[{"type":"text","text":"sub"}]}}`))
	if len(got) != 1 || !got[0].Sub || got[0].Text != "sub" {
		t.Errorf("entries: %+v", got)
	}
}

func TestDescribeToolResult(t *testing.T) {
	line := `{"type":"user","message":{"content":[
		{"type":"tool_result","is_error":true,"content":"\nexit status 1\nmore"},
		{"type":"tool_result","is_error":true,"content":[{"type":"text","text":"part one"},{"type":"text","text":"part two"}]},
		{"type":"tool_result","is_error":false,"content":"fine"}
	]}}`
	got := Describe([]byte(line))
	if len(got) != 2 {
		t.Fatalf("entries: %+v", got)
	}
	if got[0].Kind != KindError || got[0].Text != "error: exit status 1" {
		t.Errorf("string content: %+v", got[0])
	}
	if got[1].Kind != KindError || got[1].Text != "error: part one" {
		t.Errorf("list content: %+v", got[1])
	}
}

func TestDescribeSystem(t *testing.T) {
	got := Describe([]byte(`{"type":"system","subtype":"permission_denied","tool_name":"Bash"}`))
	if len(got) != 1 || got[0].Kind != KindError || got[0].Text != "denied: Bash" {
		t.Errorf("permission_denied: %+v", got)
	}
	got = Describe([]byte(`{"type":"system","subtype":"task_started","description":"review diff"}`))
	if len(got) != 1 || got[0].Kind != KindTool || got[0].Text != "subagent: review diff" {
		t.Errorf("task_started: %+v", got)
	}
	if got := Describe([]byte(`{"type":"system","subtype":"init","session_id":"s"}`)); len(got) != 0 {
		t.Errorf("init: %+v", got)
	}
}

func TestDescribeResult(t *testing.T) {
	got := Describe([]byte(`{"type":"result","is_error":false,"num_turns":7,"duration_ms":12400,"total_cost_usd":0.4567}`))
	if len(got) != 1 || got[0].Kind != KindResult || got[0].Text != "result ok · 7 turns · 12 s · 0.46 USD" {
		t.Errorf("ok: %+v", got)
	}
	got = Describe([]byte(`{"type":"result","is_error":true,"num_turns":1,"duration_ms":1000,"total_cost_usd":0}`))
	if len(got) != 1 || !strings.HasPrefix(got[0].Text, "result error · 1 turns") {
		t.Errorf("error: %+v", got)
	}
}

func TestDescribeStderr(t *testing.T) {
	got := Describe([]byte(`{"type":"stderr","text":"boom"}`))
	if len(got) != 1 || got[0].Kind != KindError || got[0].Text != "stderr: boom" {
		t.Errorf("entries: %+v", got)
	}
}

func TestDescribeIgnores(t *testing.T) {
	for _, line := range []string{"not json", `{"type":"rate_limit_event"}`, `{}`, ``} {
		if got := Describe([]byte(line)); len(got) != 0 {
			t.Errorf("%q: %+v", line, got)
		}
	}
}

func TestDescribeTruncates(t *testing.T) {
	long := strings.Repeat("ä", 200)
	got := Describe([]byte(`{"type":"lazydev","text":"` + long + `"}`))
	if len(got) != 1 {
		t.Fatalf("entries: %+v", got)
	}
	text := got[0].Text
	if n := utf8.RuneCountInString(text); n != maxDescribeLen || !strings.HasSuffix(text, "…") {
		t.Errorf("length %d: %q", n, text)
	}
	exact := strings.Repeat("x", maxDescribeLen)
	if got := Describe([]byte(`{"type":"lazydev","text":"` + exact + `"}`)); got[0].Text != exact {
		t.Errorf("text of exactly %d runes was cut: %q", maxDescribeLen, got[0].Text)
	}
}
