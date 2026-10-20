package claude

import (
	"encoding/json"
	"fmt"
	"strings"
)

// maxDescribeLen kürzt Text und Argumente in der Kurzfassung.
const maxDescribeLen = 120

// toolArgs nennt pro Tool das wichtigste Argument für die Kurzfassung.
var toolArgs = []string{"file_path", "command", "pattern", "path", "description", "prompt"}

type rawEvent struct {
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	Text     string `json:"text"`
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

// Describe liefert eine Kurzfassung einer stream-json-Zeile, eine Zeile pro Inhalt. Ereignisse
// ohne Inhalt für Menschen liefern nil.
func Describe(line []byte) []string {
	var e rawEvent
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	prefix := ""
	if e.ParentToolUseID != "" {
		prefix = "  "
	}
	var out []string
	switch e.Type {
	case "assistant":
		for _, c := range e.Message.Content {
			switch c.Type {
			case "text":
				if l := FirstLine(c.Text); l != "" {
					out = append(out, prefix+truncate(l))
				}
			case "tool_use":
				out = append(out, prefix+ToolLine(c.Name, c.Input))
			}
		}
	case "user":
		for _, c := range e.Message.Content {
			if c.Type == "tool_result" && c.IsError {
				out = append(out, prefix+"Fehler: "+truncate(FirstLine(resultText(c.Content))))
			}
		}
	case "system":
		switch e.Subtype {
		case "permission_denied":
			out = append(out, "abgelehnt: "+e.ToolName)
		case "task_started":
			out = append(out, "Subagent gestartet: "+truncate(e.Description))
		}
	case "result":
		status := "ok"
		if e.IsError {
			status = "Fehler"
		}
		out = append(out, fmt.Sprintf("Ergebnis %s · %d Runden · %.0f s · %.2f USD",
			status, e.NumTurns, float64(e.DurationMS)/1000, e.CostUSD))
	case StderrType:
		out = append(out, "stderr: "+truncate(e.Text))
	}
	return out
}

// ToolLine fasst einen Tool-Aufruf als "<Tool> <wichtigstes Argument>" zusammen.
func ToolLine(name string, input map[string]any) string {
	return strings.TrimSpace(name + " " + toolArg(input))
}

func toolArg(input map[string]any) string {
	for _, k := range toolArgs {
		if v, ok := input[k].(string); ok && v != "" {
			return truncate(FirstLine(v))
		}
	}
	return ""
}

// resultText liest content eines tool_result, das ein String oder eine Liste von Textteilen ist.
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
