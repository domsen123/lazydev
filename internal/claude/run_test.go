package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lazydev/internal/state"
)

func TestSummaryAdd(t *testing.T) {
	var s Summary
	if !s.Add(Event{Type: "system", Subtype: "init", SessionID: "s1", Model: "opus"}) {
		t.Fatal("first init reports no change")
	}
	// A subagent's init must not replace session and model.
	if s.Add(Event{Type: "system", Subtype: "init", SessionID: "s2", Model: "haiku"}) {
		t.Error("second init reports a change")
	}
	if s.Add(Event{Type: "assistant"}) || s.Add(Event{Type: "system", Subtype: "other", SessionID: "x"}) {
		t.Error("other events report a change")
	}
	if s.SessionID != "s1" || s.Model != "opus" || s.HasResult {
		t.Fatalf("after init: %+v", s)
	}
	if !s.Add(Event{Type: "result", IsError: true, Result: "sub failed", CostUSD: 0.1, SessionID: "s2"}) {
		t.Error("result reports no change")
	}
	if !s.Add(Event{Type: "result", IsError: false, Result: "done", CostUSD: 0.5}) {
		t.Error("second result reports no change")
	}
	want := Summary{SessionID: "s1", Model: "opus", HasResult: true, IsError: false, Result: "done", CostUSD: 0.5}
	if s != want {
		t.Errorf("summary: %+v, want %+v", s, want)
	}
}

func TestSummaryResultSetsMissingSession(t *testing.T) {
	var s Summary
	s.Add(Event{Type: "result", SessionID: "s9"})
	if s.SessionID != "s9" || !s.HasResult {
		t.Errorf("summary: %+v", s)
	}
}

func TestSection(t *testing.T) {
	text := "# PRD\n\n## Goal\n\nx\n\n## Result\n\n  done\n  ### detail\n  more\n\n## Notes\nn\n"
	if got, ok := Section(text, "Result"); !ok || got != "done\n  ### detail\n  more" {
		t.Errorf("Result: %q, %v", got, ok)
	}
	if got, ok := Section(text, "Notes"); !ok || got != "n" {
		t.Errorf("Notes: %q, %v", got, ok)
	}
	if got, ok := Section(text, "Missing"); ok || got != "" {
		t.Errorf("Missing: %q, %v", got, ok)
	}
	if got, ok := Section("## Result\n\n", "Result"); !ok || got != "" {
		t.Errorf("empty section: %q, %v", got, ok)
	}
}

func TestFirstLine(t *testing.T) {
	for in, want := range map[string]string{
		"":                "",
		"\n \n\t\n":       "",
		"one":             "one",
		"\n  two  \nnext": "two",
	} {
		if got := FirstLine(in); got != want {
			t.Errorf("FirstLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithTimestamp(t *testing.T) {
	got := WithTimestamp([]byte(`{"type":"x"}` + "\n"))
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("%s: %v", got, err)
	}
	ts, _ := m["ts"].(string)
	if _, err := time.Parse(time.RFC3339, ts); err != nil || m["type"] != "x" {
		t.Errorf("object: %s", got)
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Errorf("newline lost: %q", got)
	}

	got = WithTimestamp([]byte(`{}`))
	if err := json.Unmarshal(got, &m); err != nil || !bytes.HasPrefix(got, []byte(`{"ts":"`)) {
		t.Errorf("empty object: %s, %v", got, err)
	}

	for _, line := range []string{"plain text\n", "[1,2]", "{", ""} {
		if got := WithTimestamp([]byte(line)); string(got) != line {
			t.Errorf("non-object %q became %q", line, got)
		}
	}
}

func TestAgents(t *testing.T) {
	agents, err := Agents()
	if err != nil {
		t.Fatal(err)
	}
	c, ok := agents["committer"]
	if !ok {
		t.Fatalf("committer missing: %v", agents)
	}
	if c.Model != "haiku" || !slices.Equal(c.Tools, []string{"Bash", "Read", "Grep", "Glob"}) ||
		c.Description == "" || strings.HasPrefix(c.Prompt, "---") || c.Prompt == "" {
		t.Errorf("committer: %+v", c)
	}
	r, ok := agents["structural-completeness-reviewer"]
	if !ok {
		t.Fatalf("structural-completeness-reviewer missing: %v", agents)
	}
	// Escaped newlines in the description become real ones, the description keeps its ": ".
	if r.Model != "sonnet" || r.Tools != nil || !strings.Contains(r.Description, "\n\nExamples:\n") ||
		strings.Contains(r.Description, `\n`) || r.Prompt == "" {
		t.Errorf("reviewer: model %q tools %v description %q", r.Model, r.Tools, r.Description)
	}
	if len(agents) != 2 {
		t.Errorf("agents: %d", len(agents))
	}
}

func TestParseAgentErrors(t *testing.T) {
	for _, text := range []string{
		"name: x\n",
		"---\nname: x\ndescription: y\n",
		"---\ndescription: y\n---\nbody\n",
		"---\nname: x\n---\nbody\n",
		"---\nname: x\ndescription: y\n---\n\n",
	} {
		if _, _, err := parseAgent(text); err == nil {
			t.Errorf("no error for %q", text)
		}
	}
}

func TestWriteAgents(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	p, err := WriteAgents()
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(state.Dir(), "agents.json") {
		t.Errorf("path: %s", p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var agents map[string]Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		t.Fatal(err)
	}
	if agents["committer"].Model != "haiku" || len(agents) != 2 {
		t.Errorf("agents.json: %s", data)
	}
}

// fakeScript prints stream-json on stdout and text on stderr. It records its environment, stdin,
// working folder and arguments in $FAKE_OUT.
const fakeScript = `#!/bin/sh
{
  echo "editor=$GIT_EDITOR"
  echo "stdin=$(readlink /proc/self/fd/0)"
  echo "pwd=$(pwd)"
  for a in "$@"; do echo "arg=$a"; done
} > "$FAKE_OUT"
echo '{"type":"system","subtype":"init","session_id":"s1","model":"opus"}'
echo 'warning one' >&2
echo 'not json'
echo '{"type":"system","subtype":"init","session_id":"s2","model":"haiku"}'
echo '{"type":"result","is_error":true,"result":"sub","total_cost_usd":0.1}'
printf '{"type":"result","is_error":false,"result":"done","total_cost_usd":0.25}'
exit 3
`

func TestRun(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	tmp := t.TempDir()
	script := filepath.Join(tmp, "claude")
	if err := os.WriteFile(script, []byte(fakeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out")
	t.Setenv("FAKE_OUT", out)
	old := Command
	Command = []string{script}
	t.Cleanup(func() { Command = old })

	dir := t.TempDir()
	var log bytes.Buffer
	var changes []Summary
	s, code, err := Run(context.Background(), Options{
		Dir: dir, Prompt: "# lazydev implement: x", AllowedTools: []string{"Read", "Edit"}, Log: &log,
		OnChange: func(s Summary) { changes = append(changes, s) },
	})
	if err != nil || code != 3 {
		t.Fatalf("exit %d, %v", code, err)
	}
	want := Summary{SessionID: "s1", Model: "opus", HasResult: true, Result: "done", CostUSD: 0.25}
	if s != want {
		t.Errorf("summary: %+v, want %+v", s, want)
	}
	// init once, then both results.
	if len(changes) != 3 || changes[0].SessionID != "s1" || changes[2] != want {
		t.Errorf("OnChange: %+v", changes)
	}

	lines := strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("log lines: %q", lines)
	}
	var stderrLines int
	for _, l := range lines {
		if l == "not json" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Errorf("log line %q: %v", l, err)
			continue
		}
		if _, ok := m["ts"].(string); !ok {
			t.Errorf("log line without ts: %s", l)
		}
		if m["type"] == StderrType {
			stderrLines++
			if m["text"] != "warning one" {
				t.Errorf("stderr line: %s", l)
			}
		}
	}
	if stderrLines != 1 {
		t.Errorf("stderr lines: %d", stderrLines)
	}
	if !strings.HasSuffix(log.String(), "\n") {
		t.Error("last line without newline in log")
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	rec := string(data)
	for _, w := range []string{
		"editor=true\n", "stdin=/dev/null\n", "pwd=" + dir + "\n",
		"arg=-p\narg=# lazydev implement: x\n", "arg=--output-format\narg=stream-json\narg=--verbose\n",
		"arg=--allowedTools\narg=Read,Edit\n", "arg=--agents\narg=" + filepath.Join(state.Dir(), "agents.json") + "\n",
	} {
		if !strings.Contains(rec, w) {
			t.Errorf("recorded call lacks %q:\n%s", w, rec)
		}
	}
}

func TestRunWithoutResult(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	script := filepath.Join(t.TempDir(), "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"type\":\"assistant\"}'\n"), 0o755)
	old := Command
	Command = []string{script}
	t.Cleanup(func() { Command = old })

	var log bytes.Buffer
	s, code, err := Run(context.Background(), Options{Dir: t.TempDir(), Log: &log})
	if err != nil || code != 0 || s.HasResult {
		t.Errorf("summary %+v, exit %d, %v", s, code, err)
	}
}

func TestRunMissingBinary(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	old := Command
	Command = []string{filepath.Join(t.TempDir(), "missing")}
	t.Cleanup(func() { Command = old })

	_, code, err := Run(context.Background(), Options{Dir: t.TempDir(), Log: &bytes.Buffer{}})
	if err == nil || code != -1 {
		t.Errorf("exit %d, %v", code, err)
	}
}
