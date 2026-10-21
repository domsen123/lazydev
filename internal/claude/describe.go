package claude

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RunnerType marks the runner's own lines in a step log, next to Claude's stream-json.
const RunnerType = "lazydev"

// maxDescribeLen cuts text and arguments in the log view.
const maxDescribeLen = 160

// toolArgs names per tool call the argument the log view shows.
var toolArgs = []string{"file_path", "command", "pattern", "path", "description", "prompt"}

// Entry kinds of the log view.
const (
	KindRunner = "runner"
	KindText   = "text"
	KindTool   = "tool"
	KindError  = "error"
	KindResult = "result"
)

// Entry is one line of the log view.
type Entry struct {
	At   time.Time
	Kind string
	Text string
	// Sub is true for lines of a subagent.
	Sub bool
}

type rawEvent struct {
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	Text     string `json:"text"`
	TS       string `json:"ts"`
	Time     string `json:"time"`
	ToolName string `json:"tool_name"`
	Message  struct {
		Content []struct {
			Type    string          `json:"type"`
			Text    string          `json:"text"`
			Name    string          `json:"name"`
			Input   map[string]any  `json:"input"`
			IsError bool            `json:"is_error"`
			Content json.RawMessage `json:"content"`
		} `json:"content"`
	} `json:"message"`
	ParentToolUseID string  `json:"parent_tool_use_id"`
	IsError         bool    `json:"is_error"`
	CostUSD         float64 `json:"total_cost_usd"`
	NumTurns        int     `json:"num_turns"`
	DurationMS      int     `json:"duration_ms"`
	Description     string  `json:"description"`
}

// Describe turns a log line into entries for the log view, one per content. Lines without content
// for humans give none.
func Describe(line []byte) []Entry {
	var e rawEvent
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	ts := e.TS
	if ts == "" {
		ts = e.Time
	}
	at, _ := time.Parse(time.RFC3339, ts)
	entry := func(kind, text string) Entry {
		return Entry{At: at, Kind: kind, Text: truncate(text), Sub: e.ParentToolUseID != ""}
	}
	var out []Entry
	switch e.Type {
	case RunnerType:
		out = append(out, entry(KindRunner, e.Text))
	case "assistant":
		for _, c := range e.Message.Content {
			switch c.Type {
			case "text":
				if l := FirstLine(c.Text); l != "" {
					out = append(out, entry(KindText, l))
				}
			case "tool_use":
				out = append(out, entry(KindTool, strings.TrimSpace(c.Name+" "+toolArg(c.Input))))
			}
		}
	case "user":
		for _, c := range e.Message.Content {
			if c.Type == "tool_result" && c.IsError {
				out = append(out, entry(KindError, "error: "+FirstLine(resultText(c.Content))))
			}
		}
	case "system":
		switch e.Subtype {
		case "permission_denied":
			out = append(out, entry(KindError, "denied: "+e.ToolName))
		case "task_started":
			out = append(out, entry(KindTool, "subagent: "+e.Description))
		}
	case "result":
		status := "ok"
		if e.IsError {
			status = "error"
		}
		out = append(out, entry(KindResult, fmt.Sprintf("result %s · %d turns · %.0f s · %.2f USD",
			status, e.NumTurns, float64(e.DurationMS)/1000, e.CostUSD)))
	case StderrType:
		out = append(out, entry(KindError, "stderr: "+e.Text))
	}
	return out
}

func toolArg(input map[string]any) string {
	for _, k := range toolArgs {
		if v, ok := input[k].(string); ok && v != "" {
			return FirstLine(v)
		}
	}
	return ""
}

// resultText reads the content of a tool_result, a string or a list of text parts.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	json.Unmarshal(raw, &parts)
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text + "\n")
	}
	return b.String()
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) <= maxDescribeLen {
		return s
	}
	return string(r[:maxDescribeLen-1]) + "…"
}
