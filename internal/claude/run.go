// Package claude starts Claude Code in print mode with stream-json and evaluates the stream.
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Command starts Claude Code. Tests replace it with a fake.
var Command = []string{"claude"}

// DefaultAllowedTools apply to every step, allowedTools of config.yaml add to them. Claude ignores
// permissions.allow in folders without a confirmed trust dialog, like new worktrees, so every
// permission comes through --allowedTools.
var DefaultAllowedTools = []string{
	"Read", "Edit", "Write", "Glob", "Grep", "Task",
	"Bash(git status:*)", "Bash(git diff:*)", "Bash(git log:*)", "Bash(git add:*)",
	"Bash(git commit:*)", "Bash(git switch:*)", "Bash(git rebase:*)", "Bash(git checkout:*)",
	"Bash(git rm:*)", "Bash(git mv:*)", "Bash(git show:*)", "Bash(git branch:*)",
}

// StderrType marks stderr lines in the log, so the log stays JSONL.
const StderrType = "stderr"

// Options describe a call of claude -p.
type Options struct {
	Dir, Prompt  string
	AllowedTools []string
	// Log gets every stdout line with a timestamp and every stderr line as an event of type
	// stderr.
	Log io.Writer
	// OnChange runs once session, model or result are known, e.g. to save the state.
	OnChange func(Summary)
}

// Event holds the fields of a stream-json event lazydev evaluates.
type Event struct {
	Type      string  `json:"type"`
	Subtype   string  `json:"subtype"`
	SessionID string  `json:"session_id"`
	Model     string  `json:"model"`
	IsError   bool    `json:"is_error"`
	Result    string  `json:"result"`
	CostUSD   float64 `json:"total_cost_usd"`
}

// Summary collects the outcome of a call. With subagents the stream has several result events,
// the last one counts.
type Summary struct {
	SessionID, Model string
	// HasResult is false if the stream ends without result event.
	HasResult bool
	IsError   bool
	Result    string
	CostUSD   float64
}

// Add takes an event and reports whether session, model or result changed.
func (s *Summary) Add(e Event) (changed bool) {
	switch {
	case e.Type == "system" && e.Subtype == "init":
		if e.SessionID != "" && s.SessionID == "" {
			s.SessionID, changed = e.SessionID, true
		}
		if e.Model != "" && s.Model == "" {
			s.Model, changed = e.Model, true
		}
		return changed
	case e.Type == "result":
		s.HasResult, s.IsError, s.Result, s.CostUSD = true, e.IsError, e.Result, e.CostUSD
		if s.SessionID == "" {
			s.SessionID = e.SessionID
		}
		return true
	}
	return false
}

// Run starts claude and waits for its end. exitCode is -1 if claude does not start or ends by a
// signal. If ctx ends, claude gets SIGTERM.
func Run(ctx context.Context, o Options) (s Summary, exitCode int, err error) {
	agents, err := WriteAgents()
	if err != nil {
		return s, -1, err
	}
	args := append(slices.Clone(Command[1:]), "-p", o.Prompt,
		"--output-format", "stream-json", "--verbose",
		"--permission-mode", "acceptEdits",
		"--allowedTools", strings.Join(o.AllowedTools, ","),
		"--agents", agents)
	cmd := exec.CommandContext(ctx, Command[0], args...)
	cmd.Dir = o.Dir
	// Without GIT_EDITOR, git rebase --continue opens an editor and waits.
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	// Without stdin, claude -p waits three seconds for input.
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return s, -1, err
	}
	defer devnull.Close()
	cmd.Stdin = devnull
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return s, -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return s, -1, err
	}
	if err := cmd.Start(); err != nil {
		return s, -1, err
	}
	log := &syncWriter{w: o.Log}
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			data, _ := json.Marshal(map[string]string{"type": StderrType, "text": sc.Text()})
			log.Write(WithTimestamp(append(data, '\n')))
		}
	}()
	r := bufio.NewReader(stdout)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				line = append(line, '\n')
			}
			log.Write(WithTimestamp(line))
			var e Event
			if json.Unmarshal(line, &e) == nil && e.Type != "" && s.Add(e) && o.OnChange != nil {
				o.OnChange(s)
			}
		}
		if readErr != nil {
			break
		}
	}
	<-done
	err = cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return s, 0, nil
	case errors.As(err, &exit):
		return s, exit.ExitCode(), nil
	}
	return s, -1, err
}

// syncWriter keeps the lines of stdout and stderr apart in the log.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// WithTimestamp adds the field "ts" to a JSON object, because stream-json has no time. Other
// lines stay as they are.
func WithTimestamp(line []byte) []byte {
	if len(line) < 2 || line[0] != '{' {
		return line
	}
	ts := `{"ts":"` + time.Now().Format(time.RFC3339) + `"`
	if line[1] != '}' {
		ts += ","
	}
	return append([]byte(ts), line[1:]...)
}

// Section is the text below "## <name>" up to the next heading of level 2, without blank lines at
// the edges. ok is false without the heading.
func Section(text, name string) (content string, ok bool) {
	var lines []string
	in := false
	for line := range strings.SplitSeq(text, "\n") {
		if h, isHeading := strings.CutPrefix(strings.TrimSpace(line), "## "); isHeading {
			if in {
				break
			}
			in = strings.TrimSpace(h) == name
			ok = ok || in
			continue
		}
		if in {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), ok
}

// FirstLine is the first non-empty line.
func FirstLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			return l
		}
	}
	return ""
}
